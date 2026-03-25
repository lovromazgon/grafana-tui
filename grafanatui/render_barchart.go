package grafanatui

import (
	"fmt"
	"math"
	"strings"

	"github.com/NimbleMarkets/ntcharts/barchart"
	"github.com/charmbracelet/lipgloss"
	"github.com/lovromazgon/grafana-tui/grafana"
)

// barChartRenderer renders data as a vertical bar chart.
type barChartRenderer struct{}

func (r *barChartRenderer) Render(
	result *grafana.QueryResult,
	_ grafana.Panel,
	width, height int,
	hiddenSeries map[string]bool,
) string {
	if rendered, ok := r.renderCategorized(
		result, width, height, hiddenSeries,
	); ok {
		return rendered
	}

	// Fall back to flat numeric values for simple data.
	return r.renderFlat(result, width, height, hiddenSeries)
}

// renderCategorized handles frames with a string category field and
// numeric value fields (e.g., grouped bar charts).
func (r *barChartRenderer) renderCategorized(
	result *grafana.QueryResult,
	width, height int,
	hiddenSeries map[string]bool,
) (string, bool) {
	for _, resultData := range result.Results {
		for i := range resultData.Frames {
			frame := &resultData.Frames[i]

			categories, series, err := frame.CategorizedValues()
			if err != nil || len(categories) == 0 || len(series) == 0 {
				continue
			}

			fieldNames := sortedKeys(series)
			legend := renderBarLegend(fieldNames)
			legendHeight := lipgloss.Height(legend)

			maxVal := collectCategorizedMax(
				categories, series, hiddenSeries,
			)
			chartHeight := max(height-legendHeight-1, 4) //nolint:mnd // min chart height
			yAxis := renderYAxis(maxVal, chartHeight)
			yAxisWidth := lipgloss.Width(yAxis)
			chartWidth := max(width-yAxisWidth, 10) //nolint:mnd // min chart width
			chart := barchart.New(chartWidth, chartHeight)

			for catIdx, category := range categories {
				if hiddenSeries[category] {
					continue
				}

				var barValues []barchart.BarValue

				for colorIdx, fieldName := range fieldNames {
					values := series[fieldName]
					if catIdx >= len(values) {
						continue
					}

					color := chartColor(colorIdx)
					style := lipgloss.NewStyle().Foreground(color)

					barValues = append(barValues, barchart.BarValue{
						Name:  fieldName,
						Value: values[catIdx],
						Style: style,
					})
				}

				chart.Push(barchart.BarData{
					Label:  category,
					Values: barValues,
				})
			}

			chart.Draw()

			chartWithAxis := lipgloss.JoinHorizontal(
				lipgloss.Bottom, yAxis, chart.View(),
			)

			return chartWithAxis + "\n" + legend, true
		}
	}

	return "", false
}

// renderFlat handles simple single-value-per-series data.
func (r *barChartRenderer) renderFlat(
	result *grafana.QueryResult,
	width, height int,
	hiddenSeries map[string]bool,
) string {
	values := extractAllNumericValues(result)
	if len(values) == 0 {
		return lipgloss.Place(
			width, height,
			lipgloss.Center, lipgloss.Center,
			"no data",
		)
	}

	maxVal := collectMaxValue(values)
	yAxis := renderYAxis(maxVal, height)
	yAxisWidth := lipgloss.Width(yAxis)
	chartWidth := max(width-yAxisWidth, 10) //nolint:mnd // min chart width

	chart := barchart.New(chartWidth, height)
	colorIndex := 0

	for name, value := range values {
		if hiddenSeries[name] {
			colorIndex++
			continue
		}

		color := chartColor(colorIndex)
		style := lipgloss.NewStyle().Foreground(color)

		chart.Push(barchart.BarData{
			Label: name,
			Values: []barchart.BarValue{{
				Name:  name,
				Value: value,
				Style: style,
			}},
		})

		colorIndex++
	}

	chart.Draw()

	return lipgloss.JoinHorizontal(
		lipgloss.Bottom, yAxis, chart.View(),
	)
}

// renderBarLegend draws a horizontal legend showing field names with
// colored markers.
func renderBarLegend(fieldNames []string) string {
	parts := make([]string, 0, len(fieldNames))

	for i, name := range fieldNames {
		color := chartColor(i)
		marker := lipgloss.NewStyle().Foreground(color).Render("■")
		parts = append(parts, fmt.Sprintf("%s %s", marker, name))
	}

	return "  " + strings.Join(parts, "   ")
}

// renderYAxis renders a y-axis with tick labels for the given chart
// height and max value. The returned string is meant to be joined to
// the left of the bar chart. chartHeight should match the bar chart's
// data area height (excluding x-axis label row).
func renderYAxis(maxVal float64, chartHeight int) string {
	if chartHeight <= 0 || maxVal <= 0 {
		return ""
	}

	// Determine tick label width from the max value.
	topLabel := FormatValue(maxVal, "")
	labelWidth := max(len(topLabel), 4) //nolint:mnd // minimum label width

	axisStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{
			Light: "#999999",
			Dark:  "#777777",
		})

	// Build lines from top to bottom. The chart area is chartHeight
	// rows tall (the last row is the x-axis labels, which we skip).
	// We place labels at evenly spaced positions.
	dataRows := chartHeight - 1 // exclude x-axis label row
	if dataRows < 1 {
		return ""
	}

	const tickCount = 5
	lines := make([]string, chartHeight)

	for i := range chartHeight {
		lines[i] = strings.Repeat(" ", labelWidth+1)
	}

	for tick := range tickCount {
		// Row position: top (row 0) = max, bottom (dataRows-1) = 0.
		row := dataRows - 1 - tick*(dataRows-1)/(tickCount-1)
		if row < 0 || row >= chartHeight {
			continue
		}

		val := maxVal * float64(tick) / float64(tickCount-1)
		label := FormatValue(val, "")

		padded := fmt.Sprintf("%*s ", labelWidth, label)
		lines[row] = axisStyle.Render(padded)
	}

	return strings.Join(lines, "\n")
}

// collectMaxValue finds the maximum value across all bar data to
// scale the y-axis.
func collectMaxValue(values map[string]float64) float64 {
	maxVal := 0.0

	for _, v := range values {
		maxVal = math.Max(maxVal, v)
	}

	return maxVal
}

// collectCategorizedMax finds the maximum bar group sum across
// categorized data.
func collectCategorizedMax(
	categories []string,
	series map[string][]float64,
	hiddenSeries map[string]bool,
) float64 {
	maxVal := 0.0

	for catIdx, cat := range categories {
		if hiddenSeries[cat] {
			continue
		}

		var sum float64

		for _, fieldValues := range series {
			if catIdx < len(fieldValues) {
				sum += fieldValues[catIdx]
			}
		}

		maxVal = math.Max(maxVal, sum)
	}

	return maxVal
}

// sortedKeys returns the keys of a map in sorted order.
func sortedKeys(m map[string][]float64) []string {
	keys := make([]string, 0, len(m))

	for k := range m {
		keys = append(keys, k)
	}

	sortStrings(keys)

	return keys
}

// sortStrings sorts a string slice in place.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
