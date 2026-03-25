package grafanatui

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lovromazgon/grafana-tui/grafana"
)

const gaugeBarWidth = 30

// gaugeRenderer renders numeric values as horizontal gauge bars.
type gaugeRenderer struct{}

func (r *gaugeRenderer) Render(
	result *grafana.QueryResult,
	panel grafana.Panel,
	width, height int,
	hiddenSeries map[string]bool,
) string {
	entries := extractGaugeEntries(result)
	entries = filterGaugeEntries(entries, hiddenSeries)

	if len(entries) == 0 {
		return lipgloss.Place(
			width, height,
			lipgloss.Center, lipgloss.Center,
			"no data",
		)
	}

	unit := ExtractUnit(panel.FieldConfig)
	gaugeCfg := ExtractGaugeConfig(panel.FieldConfig)
	resolveGaugeRange(&gaugeCfg, entries)

	bars := make([]string, 0, len(entries))
	for _, e := range entries {
		bars = append(bars,
			renderGaugeBar(e.name, e.value, unit, width, &gaugeCfg),
		)
	}

	content := strings.Join(bars, "\n")

	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

// gaugeEntry is a named value for a gauge bar.
type gaugeEntry struct {
	name  string
	value float64
}

// extractGaugeEntries extracts all numeric values from the result,
// returning one entry per value (not just the last per field). For
// categorized data, uses category names as labels. For multi-row
// single-field data, labels each row with the field name and index.
func extractGaugeEntries(result *grafana.QueryResult) []gaugeEntry {
	var entries []gaugeEntry

	// Iterate in sorted refID order for stable display.
	refIDs := make([]string, 0, len(result.Results))
	for refID := range result.Results {
		refIDs = append(refIDs, refID)
	}

	sort.Strings(refIDs)

	for _, refID := range refIDs {
		rd := result.Results[refID]
		for i := range rd.Frames {
			frame := &rd.Frames[i]

			// Try categorized data first.
			if e := categorizedGaugeEntries(frame); len(e) > 0 {
				entries = append(entries, e...)
				continue
			}

			// Time series: take last value per numeric field.
			if frameHasTimeField(frame) {
				entries = append(entries,
					timeSeriesGaugeEntries(frame)...)
				continue
			}

			// Non-time-series: one entry per row.
			entries = append(entries, numericGaugeEntries(frame)...)
		}
	}

	if len(entries) > 0 {
		return entries
	}

	// Last resort: single scalar per field.
	for name, value := range extractAllNumericValues(result) {
		entries = append(entries, gaugeEntry{name: name, value: value})
	}

	return entries
}

func categorizedGaugeEntries(frame *grafana.DataFrame) []gaugeEntry {
	categories, series, err := frame.CategorizedValues()
	if err != nil || len(categories) == 0 || len(series) == 0 {
		return nil
	}

	var entries []gaugeEntry

	for _, fieldValues := range series {
		for j, cat := range categories {
			if j < len(fieldValues) {
				entries = append(entries, gaugeEntry{
					name: cat, value: fieldValues[j],
				})
			}
		}

		break // use first numeric field
	}

	return entries
}

func frameHasTimeField(frame *grafana.DataFrame) bool {
	for _, field := range frame.Schema.Fields {
		if field.Type == "time" {
			return true
		}
	}

	return false
}

// timeSeriesGaugeEntries takes the last value of each numeric field.
func timeSeriesGaugeEntries(frame *grafana.DataFrame) []gaugeEntry {
	var entries []gaugeEntry

	for fieldIdx, field := range frame.Schema.Fields {
		if field.Type != "number" {
			continue
		}

		values, err := frame.NumericColumn(fieldIdx)
		if err != nil || len(values) == 0 {
			continue
		}

		last := values[len(values)-1]
		entries = append(entries, gaugeEntry{
			name: field.Name, value: last,
		})
	}

	return entries
}

func numericGaugeEntries(frame *grafana.DataFrame) []gaugeEntry {
	var entries []gaugeEntry

	for fieldIdx, field := range frame.Schema.Fields {
		if field.Type != "number" {
			continue
		}

		values, err := frame.NumericColumn(fieldIdx)
		if err != nil {
			continue
		}

		for i, v := range values {
			name := fmt.Sprintf("%s [%d]", field.Name, i)
			entries = append(entries, gaugeEntry{name: name, value: v})
		}
	}

	return entries
}

// resolveGaugeRange fills in min/max from the data when not
// explicitly configured in the panel's fieldConfig.
func resolveGaugeRange(cfg *GaugeConfig, entries []gaugeEntry) {
	if (cfg.MinSet && cfg.MaxSet) || len(entries) == 0 {
		return
	}

	dataMin, dataMax := gaugeDataBounds(entries)
	if math.IsInf(dataMin, 0) {
		return
	}

	shouldUpdate := len(entries) > 1
	resolveGaugeMin(cfg, dataMin, shouldUpdate)
	resolveGaugeMax(cfg, dataMax, shouldUpdate)
}

func resolveGaugeMin(cfg *GaugeConfig, dataMin float64, force bool) {
	if !cfg.MinSet && (force || dataMin < cfg.Min) {
		cfg.Min = dataMin
	}
}

func resolveGaugeMax(cfg *GaugeConfig, dataMax float64, force bool) {
	if !cfg.MaxSet && (force || dataMax > cfg.Max) {
		cfg.Max = dataMax
	}
}

// gaugeDataBounds returns the min and max non-NaN values from entries.
func gaugeDataBounds(entries []gaugeEntry) (float64, float64) {
	dataMin := math.Inf(1)
	dataMax := math.Inf(-1)

	for _, e := range entries {
		if !math.IsNaN(e.value) {
			dataMin = math.Min(dataMin, e.value)
			dataMax = math.Max(dataMax, e.value)
		}
	}

	return dataMin, dataMax
}

// filterGaugeEntries removes entries whose name is in the hidden set.
func filterGaugeEntries(
	entries []gaugeEntry, hidden map[string]bool,
) []gaugeEntry {
	if len(hidden) == 0 {
		return entries
	}

	filtered := make([]gaugeEntry, 0, len(entries))

	for _, e := range entries {
		if !hidden[e.name] {
			filtered = append(filtered, e)
		}
	}

	return filtered
}

// renderGaugeBar renders a single horizontal gauge bar with label.
func renderGaugeBar(
	name string,
	value float64,
	unit string,
	maxWidth int,
	cfg *GaugeConfig,
) string {
	if math.IsNaN(value) {
		return name + "\n[no data]"
	}

	barWidth := min(gaugeBarWidth, maxWidth-4) //nolint:mnd // padding
	barWidth = max(barWidth, 5)                //nolint:mnd // minimum

	proportion := clampProportion(value, cfg.Min, cfg.Max)
	filled := int(proportion * float64(barWidth))
	empty := barWidth - filled

	color := lipgloss.Color(cfg.ThresholdColor(value))
	filledStyle := lipgloss.NewStyle().Foreground(color)

	bar := filledStyle.Render(strings.Repeat("█", filled)) +
		strings.Repeat("░", empty)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888"))

	formatted := FormatValue(value, unit)

	return labelStyle.Render(name) + "\n" +
		fmt.Sprintf("[%s] %s", bar, formatted)
}

// clampProportion returns a value between 0 and 1 representing the
// proportion of the gauge filled.
func clampProportion(value, minVal, maxVal float64) float64 {
	rangeVal := maxVal - minVal
	if rangeVal <= 0 {
		return 0
	}

	proportion := (value - minVal) / rangeVal

	if proportion < 0 {
		return 0
	}

	if proportion > 1 {
		return 1
	}

	return proportion
}
