package grafanatui

import (
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/NimbleMarkets/ntcharts/canvas/runes"
	"github.com/NimbleMarkets/ntcharts/linechart"
	"github.com/NimbleMarkets/ntcharts/linechart/timeserieslinechart"
	"github.com/charmbracelet/lipgloss"
	"github.com/lovromazgon/grafana-tui/grafana"
)

const legendLines = 2

// timeSeriesRenderer renders time series data as a braille line chart.
type timeSeriesRenderer struct{}

func (r *timeSeriesRenderer) Render(
	result *grafana.QueryResult,
	panel grafana.Panel,
	width, height int,
	hiddenSeries map[string]bool,
) (rendered string) {
	if width <= 0 || height <= 0 {
		return ""
	}

	// Recover from panics in the charting library.
	defer func() {
		if r := recover(); r != nil {
			rendered = lipgloss.Place(
				width, height,
				lipgloss.Center, lipgloss.Center,
				fmt.Sprintf("chart rendering error: %v", r),
			)
		}
	}()

	seriesData := collectTimeSeries(result)
	filterHiddenSeries(seriesData, hiddenSeries)

	slog.Debug("render panel series count", "panel", panel.Title, "series", len(seriesData))
	for name, entry := range seriesData {
		nonNaN := 0
		for _, v := range entry.values {
			if !math.IsNaN(v) {
				nonNaN++
			}
		}
		slog.Debug("render series", "name", name, "points", len(entry.timestamps), "non_nan", nonNaN)
	}

	if len(seriesData) == 0 {
		slog.Debug("render panel no series data", "panel", panel.Title)

		return lipgloss.Place(
			width, height,
			lipgloss.Center, lipgloss.Center,
			"no time series data",
		)
	}

	chartHeight := max(height-legendLines, 3) //nolint:mnd // minimum chart height

	unit := ExtractUnit(panel.FieldConfig)
	chart := timeserieslinechart.New(width, chartHeight,
		timeserieslinechart.WithXLabelFormatter(smartTimeLabelFormatter(seriesData)),
		timeserieslinechart.WithYLabelFormatter(unitLabelFormatter(unit)),
	)

	seriesNames := sortedSeriesNames(seriesData)

	pointCount := pushTimeSeriesData(&chart, seriesData, seriesNames)
	slog.Debug("render panel pushed points", "panel", panel.Title, "points", pointCount)

	if pointCount == 0 {
		return lipgloss.Place(
			width, height,
			lipgloss.Center, lipgloss.Center,
			"no time series data",
		)
	}

	styleTimeSeriesDataSets(&chart, seriesNames)

	chart.DrawBrailleAll()

	legend := buildLegend(seriesNames, width)

	return chart.View() + "\n" + legend
}

// smartTimeLabelFormatter returns a label formatter that shows hours
// for ranges <= 24h and date+time for longer ranges.
func smartTimeLabelFormatter(
	seriesData map[string]timeSeriesEntry,
) linechart.LabelFormatter {
	rangeMs := timeRangeMs(seriesData)
	dayMs := int64(24 * 60 * 60 * 1000) //nolint:mnd // 24 hours in ms

	if rangeMs <= dayMs {
		return func(_ int, v float64) string {
			t := time.Unix(int64(v), 0).In(time.Local) //nolint:gosmopolitan // TUI shows user-local time

			return t.Format("15:04")
		}
	}

	return func(_ int, v float64) string {
		t := time.Unix(int64(v), 0).In(time.Local) //nolint:gosmopolitan // TUI shows user-local time

		return t.Format("Jan 02 15:04")
	}
}

// timeRangeMs computes the total time span across all series.
func timeRangeMs(seriesData map[string]timeSeriesEntry) int64 {
	var minTs, maxTs int64

	first := true

	for _, entry := range seriesData {
		if len(entry.timestamps) == 0 {
			continue
		}

		lo := entry.timestamps[0]
		hi := entry.timestamps[len(entry.timestamps)-1]

		if first || lo < minTs {
			minTs = lo
		}

		if first || hi > maxTs {
			maxTs = hi
		}

		first = false
	}

	return maxTs - minTs
}

// unitLabelFormatter returns a Y axis label formatter that uses the
// panel's configured Grafana unit.
func unitLabelFormatter(unit string) linechart.LabelFormatter {
	return func(_ int, v float64) string {
		if unit != "" {
			return FormatValue(v, unit)
		}

		return fmt.Sprintf("%.2g", v)
	}
}

// timeSeriesEntry holds timestamps and values for a single series.
type timeSeriesEntry struct {
	timestamps []int64
	values     []float64
}

// collectTimeSeries gathers all time series from a query result.
// Series where every value is NaN are excluded.
func collectTimeSeries(
	result *grafana.QueryResult,
) map[string]timeSeriesEntry {
	seriesData := make(map[string]timeSeriesEntry)

	for _, resultData := range result.Results {
		for _, frame := range resultData.Frames {
			timestamps, series, err := frame.TimeSeries()
			if err != nil {
				continue
			}

			for name, values := range series {
				if allNaN(values) {
					continue
				}

				seriesData[name] = timeSeriesEntry{
					timestamps: timestamps,
					values:     values,
				}
			}
		}
	}

	return seriesData
}

// allNaN returns true if every value in the slice is NaN.
func allNaN(values []float64) bool {
	for _, v := range values {
		if !math.IsNaN(v) {
			return false
		}
	}

	return true
}

// filterHiddenSeries removes hidden series from the data map.
func filterHiddenSeries(
	seriesData map[string]timeSeriesEntry,
	hidden map[string]bool,
) {
	for name := range seriesData {
		if hidden[name] {
			delete(seriesData, name)
		}
	}
}

// sortedSeriesNames returns series names in sorted order for
// deterministic rendering.
func sortedSeriesNames(
	seriesData map[string]timeSeriesEntry,
) []string {
	names := make([]string, 0, len(seriesData))
	for name := range seriesData {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

// pushTimeSeriesData pushes all data points into the chart model
// and returns the total number of points pushed.
func pushTimeSeriesData(
	chart *timeserieslinechart.Model,
	seriesData map[string]timeSeriesEntry,
	seriesNames []string,
) int {
	count := 0

	for _, name := range seriesNames {
		entry := seriesData[name]

		for i, ts := range entry.timestamps {
			value := entry.values[i]
			if math.IsNaN(value) {
				continue
			}

			chart.PushDataSet(name, timeserieslinechart.TimePoint{
				Time:  time.UnixMilli(ts),
				Value: value,
			})

			count++
		}
	}

	return count
}

// styleTimeSeriesDataSets applies distinct colors and line styles to
// each dataset.
func styleTimeSeriesDataSets(
	chart *timeserieslinechart.Model,
	seriesNames []string,
) {
	for i, name := range seriesNames {
		color := chartColor(i)
		style := lipgloss.NewStyle().Foreground(color)

		chart.SetDataSetStyle(name, style)
		chart.SetDataSetLineStyle(name, runes.ThinLineStyle)
	}
}

// buildLegend renders a colored legend line for the given series.
func buildLegend(seriesNames []string, maxWidth int) string {
	parts := make([]string, 0, len(seriesNames))

	for i, name := range seriesNames {
		color := chartColor(i)
		style := lipgloss.NewStyle().Foreground(color)
		parts = append(parts, style.Render("■ "+name))
	}

	legend := strings.Join(parts, "  ")
	if lipgloss.Width(legend) > maxWidth && maxWidth > 0 {
		legend = lipgloss.NewStyle().MaxWidth(maxWidth).Render(legend)
	}

	return legend
}
