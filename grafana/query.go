package grafana

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var errPromQueryFailed = errors.New("prometheus query failed")

// TimeRange represents a time range for a query.
type TimeRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// QueryResult holds the result of a panel query.
type QueryResult struct {
	Results map[string]QueryResultData `json:"results"`
}

// QueryResultData holds the data for a single query result.
type QueryResultData struct {
	Frames []DataFrame `json:"frames"`
}

// DataFrame represents a single data frame in a query result.
type DataFrame struct {
	Schema DataFrameSchema `json:"schema"`
	Data   DataFrameData   `json:"data"`
}

// DataFrameSchema describes the structure of a data frame.
type DataFrameSchema struct {
	RefID  string           `json:"refId"`
	Meta   DataFrameMeta    `json:"meta"`
	Fields []DataFrameField `json:"fields"`
}

// DataFrameMeta holds metadata about a data frame.
type DataFrameMeta struct {
	Type        string `json:"type"`
	TypeVersion []int  `json:"typeVersion"`
}

// DataFrameField describes a single field in a data frame.
type DataFrameField struct {
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Labels map[string]string `json:"labels,omitempty"`
	Config json.RawMessage   `json:"config,omitempty"`
}

// DataFrameData holds the values for each field as parallel arrays.
type DataFrameData struct {
	Values []json.RawMessage `json:"values"`
}

// QueryPanel executes all targets for a panel and returns the
// results. For Prometheus targets (those with an expr field), it uses
// the datasource proxy endpoint. For all other datasources, it uses
// the generic /api/ds/query endpoint. The allPanels slice is used to
// resolve "-- Dashboard --" datasource references that point to
// another panel's query.
func (c *Client) QueryPanel( //nolint:funlen,cyclop // orchestration function
	ctx context.Context,
	panel Panel,
	timeRange TimeRange,
	maxDataPoints int,
	variables []TemplateVariable,
	variableOverrides map[string]string,
	allPanels []Panel,
) (*QueryResult, error) {
	panel = resolveDashboardDatasource(panel, allPanels)

	if len(panel.Targets) == 0 {
		return &QueryResult{Results: map[string]QueryResultData{}}, nil
	}

	intervalMs := computeIntervalMs(timeRange, maxDataPoints)
	stepSec := max(intervalMs/1000, 1)                          //nolint:mnd // ms to seconds
	start := time.Unix(resolveTimeToMs(timeRange.From)/1000, 0) //nolint:mnd // ms to seconds
	end := time.Unix(resolveTimeToMs(timeRange.To)/1000, 0)     //nolint:mnd // ms to seconds

	results := make(map[string]QueryResultData, len(panel.Targets))

	// Collect non-Prometheus targets for batch query via /api/ds/query.
	var genericTargets []json.RawMessage

	for _, target := range panel.Targets {
		fields := extractTargetFields(target)

		dsRef := target.Datasource
		if dsRef == nil {
			dsRef = panel.Datasource
		}

		if dsRef == nil {
			slog.Debug("skipping target with no datasource",
				"panel", panel.Title, "target", target.RefID)

			continue
		}

		// Resolve datasource names or ${DS_*} variable UIDs.
		if dsRef.Type == "" || isDatasourceVariable(dsRef.UID) {
			dsRef = &DatasourceRef{
				UID:  c.ResolveDatasourceUID(ctx, dsRef.UID, dsRef.Type),
				Type: dsRef.Type,
			}
		}

		// Inline frame data: use it directly without querying.
		if frame, ok := extractInlineFrame(target, dsRef); ok {
			results[target.RefID] = QueryResultData{
				Frames: []DataFrame{frame},
			}

			continue
		}

		// Prometheus targets: use the proxy endpoint.
		if fields.Expr != "" {
			expr := ResolveVariables(fields.Expr, variables, variableOverrides)
			expr = resolveBuiltinVariables(expr, intervalMs)

			slog.Debug("grafana resolved expr", "panel", panel.Title, "target", target.RefID, "expr", expr)

			promResult, err := c.queryPrometheus(ctx, dsRef.UID, dsRef.Type, expr, start, end, stepSec)
			if err != nil {
				return nil, fmt.Errorf("querying %q: %w", target.RefID, err)
			}

			frames := prometheusToDataFrames(promResult, target.RefID, fields.LegendFormat)
			results[target.RefID] = QueryResultData{Frames: frames}

			continue
		}

		// Non-Prometheus targets: build a query for /api/ds/query.
		queryJSON := buildDSQueryTarget(target, dsRef, intervalMs, maxDataPoints)
		genericTargets = append(genericTargets, queryJSON)
	}

	// Execute generic targets in a single batch request.
	if len(genericTargets) > 0 {
		fromMs := resolveTimeToMs(timeRange.From)
		toMs := resolveTimeToMs(timeRange.To)

		dsResult, err := c.queryDatasource(ctx, genericTargets, fromMs, toMs)
		if err != nil {
			return nil, fmt.Errorf("querying datasource: %w", err)
		}

		maps.Copy(results, dsResult.Results)
	}

	queryResult := &QueryResult{Results: results}
	ApplyTransformations(queryResult, panel.Transformations)

	return queryResult, nil
}

// inlineFrameFields represents the "frame" field in a target that
// contains inline data from the Static datasource.
type inlineFrameFields struct {
	Fields []inlineField `json:"fields"`
}

type inlineField struct {
	Name   string          `json:"name"`
	Type   string          `json:"type"`
	Values json.RawMessage `json:"values"`
}

// extractInlineFrame checks if a target contains inline frame data
// (used by the Static datasource plugin). Returns the constructed
// DataFrame and true if found.
func extractInlineFrame(
	target Target, dsRef *DatasourceRef,
) (DataFrame, bool) {
	raw := target.Raw()
	if raw == nil {
		return DataFrame{}, false //nolint:exhaustruct // zero value is intentional
	}

	var wrapper struct {
		Frame *inlineFrameFields `json:"frame"`
	}

	if err := json.Unmarshal(raw, &wrapper); err != nil || wrapper.Frame == nil {
		return DataFrame{}, false //nolint:exhaustruct // zero value is intentional
	}

	if len(wrapper.Frame.Fields) == 0 {
		return DataFrame{}, false //nolint:exhaustruct // zero value is intentional
	}

	slog.Debug("using inline frame data",
		"target", target.RefID,
		"datasource", dsRef.UID,
		"fields", len(wrapper.Frame.Fields))

	schemaFields := make([]DataFrameField, len(wrapper.Frame.Fields))
	dataValues := make([]json.RawMessage, len(wrapper.Frame.Fields))

	for i, f := range wrapper.Frame.Fields {
		schemaFields[i] = DataFrameField{ //nolint:exhaustruct // inline data has no labels/config
			Name: f.Name,
			Type: f.Type,
		}
		dataValues[i] = f.Values
	}

	frame := DataFrame{
		Schema: DataFrameSchema{ //nolint:exhaustruct // inline data has no meta
			RefID:  target.RefID,
			Fields: schemaFields,
		},
		Data: DataFrameData{Values: dataValues},
	}

	return frame, true
}

// isDatasourceVariable checks if a UID is a Grafana provisioning
// variable like ${DS_PROMETHEUS}.
func isDatasourceVariable(uid string) bool {
	return strings.HasPrefix(uid, "${")
}

// resolveDashboardDatasource replaces targets that reference the
// special "-- Dashboard --" datasource with the targets from the
// referenced panel.
func resolveDashboardDatasource(panel Panel, allPanels []Panel) Panel {
	if len(panel.Targets) == 0 || len(allPanels) == 0 {
		return panel
	}

	dsRef := panel.Targets[0].Datasource
	if dsRef == nil {
		dsRef = panel.Datasource
	}

	if dsRef == nil || dsRef.UID != "-- Dashboard --" {
		return panel
	}

	// Extract the referenced panelId from the first target.
	refPanelID := extractPanelID(panel.Targets[0])
	if refPanelID == 0 {
		slog.Debug("-- Dashboard -- target missing panelId",
			"panel", panel.Title)
		return panel
	}

	// Find the referenced panel and use its targets/datasource.
	for _, p := range allPanels {
		if p.ID == refPanelID {
			slog.Debug("resolved -- Dashboard -- reference",
				"panel", panel.Title, "refPanelId", refPanelID,
				"refPanel", p.Title)

			panel.Targets = p.Targets
			panel.Datasource = p.Datasource

			return panel
		}
	}

	slog.Debug("-- Dashboard -- referenced panel not found",
		"panel", panel.Title, "refPanelId", refPanelID)

	return panel
}

// extractPanelID pulls the panelId field from a target's raw JSON.
func extractPanelID(target Target) int {
	raw := target.Raw()
	if raw == nil {
		return 0
	}

	var fields struct {
		PanelID int `json:"panelId"`
	}

	if err := json.Unmarshal(raw, &fields); err != nil {
		return 0
	}

	return fields.PanelID
}

// buildDSQueryTarget builds a query JSON object for /api/ds/query
// from a target, merging in the datasource reference and interval.
func buildDSQueryTarget(
	target Target,
	dsRef *DatasourceRef,
	intervalMs, maxDataPoints int,
) json.RawMessage {
	// Start with the raw target JSON and merge in required fields.
	var targetMap map[string]any

	raw := target.Raw()
	if raw == nil {
		targetMap = make(map[string]any)
	} else if err := json.Unmarshal(raw, &targetMap); err != nil {
		targetMap = make(map[string]any)
	}

	targetMap["datasource"] = dsRef
	targetMap["intervalMs"] = intervalMs
	targetMap["maxDataPoints"] = maxDataPoints

	data, err := json.Marshal(targetMap)
	if err != nil {
		return json.RawMessage(`{}`)
	}

	return data
}

// dsQueryRequest is the request body for POST /api/ds/query.
type dsQueryRequest struct {
	Queries []json.RawMessage `json:"queries"`
	From    string            `json:"from"`
	To      string            `json:"to"`
}

// queryDatasource queries via the generic /api/ds/query endpoint,
// which supports all datasource types.
func (c *Client) queryDatasource(
	ctx context.Context,
	queries []json.RawMessage,
	fromMs, toMs int64,
) (*QueryResult, error) {
	reqBody := dsQueryRequest{
		Queries: queries,
		From:    strconv.FormatInt(fromMs, 10),
		To:      strconv.FormatInt(toMs, 10),
	}

	var result QueryResult
	if err := c.doPostJSON(ctx, "/api/ds/query", reqBody, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// targetFields holds the fields we extract from a target's raw JSON.
type targetFields struct {
	Expr         string `json:"expr"`
	LegendFormat string `json:"legendFormat"`
}

// extractTargetFields pulls known fields from a target's raw JSON.
func extractTargetFields(target Target) targetFields {
	raw := target.Raw()
	if raw == nil {
		return targetFields{} //nolint:exhaustruct // zero value is intentional
	}

	var fields targetFields
	if err := json.Unmarshal(raw, &fields); err != nil {
		return targetFields{} //nolint:exhaustruct // zero value is intentional
	}

	return fields
}

// promQueryResult is the Prometheus /api/v1/query_range response.
type promQueryResult struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string           `json:"resultType"`
		Result     []promSeriesData `json:"result"`
	} `json:"data"`
	Error     string `json:"error,omitempty"`
	ErrorType string `json:"errorType,omitempty"`
}

// promSeriesData is a single series in a Prometheus query result.
type promSeriesData struct {
	Metric map[string]string `json:"metric"`
	Values [][]any           `json:"values"`
}

// queryPrometheus queries via the datasource proxy endpoint. For
// Loki datasources, the API path includes a /loki prefix.
func (c *Client) queryPrometheus(
	ctx context.Context,
	dsUID, dsType string,
	expr string,
	start, end time.Time,
	stepSec int,
) (*promQueryResult, error) {
	params := url.Values{
		"query": {expr},
		"start": {strconv.FormatInt(start.Unix(), 10)},
		"end":   {strconv.FormatInt(end.Unix(), 10)},
		"step":  {strconv.Itoa(stepSec)},
	}

	apiPrefix := "/api/v1/query_range"
	if dsType == "loki" {
		apiPrefix = "/loki/api/v1/query_range"
	}

	path := fmt.Sprintf(
		"/api/datasources/proxy/uid/%s%s?%s",
		url.PathEscape(dsUID), apiPrefix, params.Encode(),
	)

	var result promQueryResult
	if err := c.doJSON(ctx, path, &result); err != nil {
		return nil, err
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("%w: %s: %s", errPromQueryFailed, result.ErrorType, result.Error)
	}

	return &result, nil
}

// prometheusToDataFrames converts a Prometheus query result to our
// DataFrame format. Each series becomes a separate frame.
func prometheusToDataFrames(
	result *promQueryResult, refID, legendFormat string,
) []DataFrame {
	if len(result.Data.Result) == 0 {
		return nil
	}

	frames := make([]DataFrame, 0, len(result.Data.Result))

	for _, series := range result.Data.Result {
		frame := promSeriesToDataFrame(series, refID, legendFormat)
		frames = append(frames, frame)
	}

	return frames
}

// promSeriesToDataFrame converts one Prometheus series to a DataFrame.
func promSeriesToDataFrame(
	series promSeriesData, refID, legendFormat string,
) DataFrame {
	timestamps := make([]int64, 0, len(series.Values))
	values := make([]*float64, 0, len(series.Values))

	for _, point := range series.Values {
		if len(point) < 2 { //nolint:mnd // [timestamp, value]
			continue
		}

		ts, ok := point[0].(float64)
		if !ok {
			continue
		}

		timestamps = append(timestamps, int64(ts*1000)) //nolint:mnd // seconds to ms

		valStr, ok := point[1].(string)
		if !ok {
			values = append(values, nil)
			continue
		}

		v, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			values = append(values, nil)
			continue
		}

		values = append(values, &v)
	}

	valueName := resolveSeriesName(series.Metric, legendFormat, refID)

	timeValues := mustMarshal(timestamps)
	valueValues := mustMarshal(values)

	return DataFrame{
		Schema: DataFrameSchema{
			RefID: refID,
			Meta:  DataFrameMeta{Type: "timeseries-multi", TypeVersion: nil},
			Fields: []DataFrameField{
				{Name: "Time", Type: "time", Labels: nil, Config: nil},
				{Name: valueName, Type: "number", Labels: series.Metric, Config: nil},
			},
		},
		Data: DataFrameData{
			Values: []json.RawMessage{timeValues, valueValues},
		},
	}
}

// resolveSeriesName determines the display name for a series.
// It uses legendFormat if set, falling back to metric name or refID.
func resolveSeriesName(
	metric map[string]string, legendFormat, refID string,
) string {
	// Use legendFormat if it's set and not the auto placeholder.
	if legendFormat != "" && legendFormat != "__auto" {
		name := legendFormatPattern.ReplaceAllStringFunc(
			legendFormat, func(match string) string {
				label := strings.TrimSpace(match[2 : len(match)-2]) // strip {{ }} and whitespace
				if v, ok := metric[label]; ok {
					return v
				}

				return match
			},
		)
		if name != "" {
			return name
		}
	}

	// Fall back to __name__ label or refID.
	if name, ok := metric["__name__"]; ok {
		return name
	}

	return refID
}

var legendFormatPattern = regexp.MustCompile(`\{\{\s*(\w+)\s*\}\}`)

// resolveBuiltinVariables replaces Grafana built-in variables like
// $__rate_interval, $__interval, $__interval_ms, and $__all.
func resolveBuiltinVariables(expr string, intervalMs int) string {
	// max(4*scrapeInterval, step); scrapeInterval defaults to 15s.
	rateIntervalMs := max(4*intervalMs, 60000) //nolint:mnd // 1 min floor
	rateInterval := formatDuration(rateIntervalMs)
	interval := formatDuration(intervalMs)

	expr = strings.ReplaceAll(expr, "$__rate_interval", rateInterval)
	expr = strings.ReplaceAll(expr, "$__interval_ms", strconv.Itoa(intervalMs))
	expr = strings.ReplaceAll(expr, "$__interval", interval)
	expr = strings.ReplaceAll(expr, "$__auto", interval)
	expr = strings.ReplaceAll(expr, "$__all", ".*")
	expr = autoIntervalPattern.ReplaceAllString(expr, interval)

	return expr
}

var autoIntervalPattern = regexp.MustCompile(`\$__auto_interval_\w+`)

// formatDuration converts milliseconds to a human-friendly duration.
func formatDuration(ms int) string {
	switch {
	case ms >= 3600000 && ms%3600000 == 0: //nolint:mnd // hours
		return strconv.Itoa(ms/3600000) + "h"
	case ms >= 60000 && ms%60000 == 0: //nolint:mnd // minutes
		return strconv.Itoa(ms/60000) + "m"
	case ms >= 1000 && ms%1000 == 0: //nolint:mnd // seconds
		return strconv.Itoa(ms/1000) + "s"
	default:
		return strconv.Itoa(ms) + "ms"
	}
}

// relativeTimePattern matches patterns like "now", "now-1h", "now-30m".
var relativeTimePattern = regexp.MustCompile(
	`^now(?:-(\d+)([smhdwMy]))?$`,
)

// computeIntervalMs calculates the interval in milliseconds.
func computeIntervalMs(timeRange TimeRange, maxDataPoints int) int {
	if maxDataPoints <= 0 {
		maxDataPoints = 1
	}

	fromMs := resolveTimeToMs(timeRange.From)
	toMs := resolveTimeToMs(timeRange.To)
	rangeMs := toMs - fromMs

	if rangeMs <= 0 {
		return 1
	}

	return max(int(rangeMs)/maxDataPoints, 1)
}

// resolveTimeToMs converts a time string to epoch milliseconds.
func resolveTimeToMs(timeStr string) int64 {
	if epochMs, err := strconv.ParseInt(timeStr, 10, 64); err == nil {
		return epochMs
	}

	now := time.Now()

	matches := relativeTimePattern.FindStringSubmatch(timeStr)
	if matches == nil {
		return now.UnixMilli()
	}

	if matches[1] == "" {
		return now.UnixMilli()
	}

	return subtractDuration(now, matches[1], matches[2]).UnixMilli()
}

// subtractDuration subtracts an amount with a given unit from a time.
func subtractDuration(
	base time.Time,
	amountStr, unit string,
) time.Time {
	amount, _ := strconv.Atoi(amountStr)

	unitDurations := map[string]time.Duration{
		"s": time.Second,
		"m": time.Minute,
		"h": time.Hour,
		"d": 24 * time.Hour,     //nolint:mnd // day
		"w": 7 * 24 * time.Hour, //nolint:mnd // week
	}

	if duration, ok := unitDurations[unit]; ok {
		return base.Add(-time.Duration(amount) * duration)
	}

	switch unit {
	case "M":
		return base.AddDate(0, -amount, 0)
	case "y":
		return base.AddDate(-amount, 0, 0)
	}

	return base
}

// mustMarshal marshals a value to JSON. Falls back to null on error.
func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`null`)
	}

	return data
}
