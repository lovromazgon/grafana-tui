package grafana

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"
)

var (
	errNoDatasource      = errors.New("no datasource for variable")
	errLabelValuesFailed = errors.New("label values query failed")
)

// labelValuesPattern matches label_values(metric{filters}, label)
// and label_values(label) query formats.
var labelValuesPattern = regexp.MustCompile(
	`label_values\(\s*([^,)]*?)\s*(?:,\s*(\w+)\s*)?\)`,
)

// promLabelValuesResult is the Prometheus /api/v1/label/.../values
// response.
type promLabelValuesResult struct {
	Status string   `json:"status"`
	Data   []string `json:"data"`
}

// ResolveVariableOptions queries Prometheus to get available options
// for each template variable. Variables are resolved in order since
// later variables may reference earlier ones in their queries.
// Returns a map of variable name to available options, and a map of
// variable name to selected default value.
func (c *Client) ResolveVariableOptions(
	ctx context.Context,
	variables []TemplateVariable,
) (map[string][]string, map[string]string, error) {
	options := make(map[string][]string, len(variables))
	defaults := make(map[string]string, len(variables))

	// Find a fallback datasource UID from variables that have one.
	// Grafana uses the default datasource when none is specified.
	fallbackDSUID := findFallbackDatasource(variables)

	for _, v := range variables {
		var opts []string

		switch v.Type {
		case "custom", "constant":
			opts = parseCustomVariableOptions(v)
		case "query":
			fetched, err := c.fetchQueryVariableOptions(
				ctx, v, defaults, fallbackDSUID,
			)
			if err != nil {
				slog.Debug("grafana failed to resolve variable", "variable", v.Name, "error", err)
			} else {
				opts = fetched
			}
		}

		options[v.Name] = opts

		// Pick default: current value if non-empty and valid,
		// then "all" when available, then first option.
		current := resolveCurrentValue(v)

		switch {
		case current != "" && isValidOption(current, opts, v.IncludeAll):
			defaults[v.Name] = current
		case v.IncludeAll:
			if v.AllValue != "" {
				defaults[v.Name] = v.AllValue
			} else {
				defaults[v.Name] = ".*"
			}
		case len(opts) > 0:
			defaults[v.Name] = opts[0]
		}
	}

	return options, defaults, nil
}

// isValidOption checks whether the current value is a valid selection.
// "All" regex patterns (like ".*") are valid when includeAll is true.
// Otherwise the value must appear in the fetched options list.
func isValidOption(
	current string, opts []string, includeAll bool,
) bool {
	// If options weren't fetched (custom vars, errors), trust current.
	if len(opts) == 0 {
		return true
	}

	// "All" patterns are valid when the variable supports it.
	if includeAll && (current == ".*" || current == grafanaAllToken) {
		return true
	}

	return slices.Contains(opts, current)
}

// parseCustomVariableOptions extracts options from a custom or
// constant variable.
func parseCustomVariableOptions(v TemplateVariable) []string {
	// Use pre-populated options if available.
	if len(v.Options) > 0 {
		opts := make([]string, 0, len(v.Options))
		for _, o := range v.Options {
			opts = append(opts, o.Value)
		}

		return opts
	}

	// Fall back to parsing the query as comma-separated values.
	query := queryDefinition(v)
	if query == "" {
		return nil
	}

	parts := strings.Split(query, ",")
	opts := make([]string, 0, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			opts = append(opts, p)
		}
	}

	return opts
}

// findFallbackDatasource returns the UID of the first Prometheus
// datasource found among the given variables. Returns empty string
// if none is found.
func findFallbackDatasource(variables []TemplateVariable) string {
	for _, v := range variables {
		if v.Datasource != nil && v.Datasource.UID != "" {
			return v.Datasource.UID
		}
	}

	return ""
}

// fetchQueryVariableOptions resolves a query-type variable by calling
// the Prometheus label values API.
func (c *Client) fetchQueryVariableOptions(
	ctx context.Context,
	v TemplateVariable,
	resolved map[string]string,
	fallbackDSUID string,
) ([]string, error) {
	query := queryDefinition(v)
	if query == "" {
		return nil, nil
	}

	// Substitute already-resolved variables into the query.
	for name, value := range resolved {
		query = strings.ReplaceAll(query, "$"+name, value)
		query = strings.ReplaceAll(query, "${"+name+"}", value)
	}

	matches := labelValuesPattern.FindStringSubmatch(query)
	if matches == nil {
		slog.Debug("grafana unsupported variable query format", "query", query)

		return nil, nil
	}

	metric := strings.TrimSpace(matches[1])
	label := strings.TrimSpace(matches[2])

	// Form: label_values(label) — metric is actually the label.
	if label == "" {
		label = metric
		metric = ""
	}

	dsUID := ""
	if v.Datasource != nil {
		dsUID = v.Datasource.UID
	}

	if dsUID == "" {
		dsUID = fallbackDSUID
	}

	if dsUID == "" {
		return nil, fmt.Errorf(
			"%w %q", errNoDatasource, v.Name,
		)
	}

	return c.queryLabelValues(ctx, dsUID, label, metric)
}

// queryLabelValues calls the Prometheus label values API via the
// Grafana datasource proxy.
func (c *Client) queryLabelValues(
	ctx context.Context,
	dsUID, label, metric string,
) ([]string, error) {
	path := fmt.Sprintf(
		"/api/datasources/proxy/uid/%s/api/v1/label/%s/values",
		url.PathEscape(dsUID), url.PathEscape(label),
	)

	if metric != "" {
		path += "?" + url.Values{"match[]": {metric}}.Encode()
	}

	var result promLabelValuesResult
	if err := c.doJSON(ctx, path, &result); err != nil {
		return nil, err
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("%w: %s", errLabelValuesFailed, label)
	}

	sort.Strings(result.Data)

	return result.Data, nil
}

// queryDefinition extracts the query string from a template variable.
// The query can be a plain string or an object with a "query" field.
func queryDefinition(v TemplateVariable) string {
	if v.Definition != "" {
		return v.Definition
	}

	if len(v.Query) == 0 {
		return ""
	}

	// Try plain string first.
	var s string
	if err := json.Unmarshal(v.Query, &s); err == nil {
		return s
	}

	// Try object with "query" field.
	var obj struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(v.Query, &obj); err == nil {
		return obj.Query
	}

	return ""
}
