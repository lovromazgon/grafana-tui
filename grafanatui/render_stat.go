package grafanatui

import (
	"maps"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lovromazgon/grafana-tui/grafana"
)

// statRenderer renders numeric values as a large centered display.
type statRenderer struct{}

func (r *statRenderer) Render(
	result *grafana.QueryResult,
	panel grafana.Panel,
	width, height int,
	_ map[string]bool,
) string {
	values := extractAllNumericValues(result)
	if len(values) == 0 {
		return lipgloss.Place(
			width, height,
			lipgloss.Center, lipgloss.Center,
			"no data",
		)
	}

	unit := ExtractUnit(panel.FieldConfig)

	if len(values) == 1 {
		return renderSingleStat(values, unit, width, height)
	}

	return renderMultipleStats(values, unit, width, height)
}

// renderSingleStat renders one large centered value.
func renderSingleStat(
	values map[string]float64,
	unit string,
	width, height int,
) string {
	valueStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00FFFF"))

	for _, value := range values {
		formatted := FormatValue(value, unit)

		return lipgloss.Place(
			width, height,
			lipgloss.Center, lipgloss.Center,
			valueStyle.Render(formatted),
		)
	}

	return ""
}

// renderMultipleStats renders values side by side with labels.
func renderMultipleStats(
	values map[string]float64,
	unit string,
	width, height int,
) string {
	nameStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Align(lipgloss.Center)

	valueStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00FFFF")).
		Align(lipgloss.Center)

	blocks := make([]string, 0, len(values))

	for name, value := range values {
		formatted := FormatValue(value, unit)
		block := nameStyle.Render(name) + "\n" +
			valueStyle.Render(formatted)
		blocks = append(blocks, block)
	}

	joined := strings.Join(blocks, "   ")

	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		joined,
	)
}

// extractAllNumericValues collects all numeric values from a query
// result, falling back to last time series values if needed.
func extractAllNumericValues(
	result *grafana.QueryResult,
) map[string]float64 {
	values := make(map[string]float64)

	for _, resultData := range result.Results {
		for _, frame := range resultData.Frames {
			extractFrameValues(&frame, values)
		}
	}

	return values
}

// extractFrameValues extracts numeric values from a single frame,
// trying NumericValues first, then falling back to last time series
// value.
func extractFrameValues(
	frame *grafana.DataFrame,
	values map[string]float64,
) {
	numericValues, err := frame.NumericValues()
	if err == nil && len(numericValues) > 0 {
		maps.Copy(values, numericValues)

		return
	}

	// Fall back to last time series value.
	_, series, tsErr := frame.TimeSeries()
	if tsErr != nil {
		return
	}

	for name, seriesValues := range series {
		if len(seriesValues) > 0 {
			values[name] = seriesValues[len(seriesValues)-1]
		}
	}
}
