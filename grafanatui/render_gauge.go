package grafanatui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lovromazgon/grafana-tui/grafana"
)

const (
	gaugeMin      = 0.0
	gaugeMax      = 100.0
	gaugeBarWidth = 30

	gaugeThresholdLow = 33.0
	gaugeThresholdMid = 66.0
)

// gaugeRenderer renders numeric values as horizontal gauge bars.
type gaugeRenderer struct{}

func (r *gaugeRenderer) Render(
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

	bars := make([]string, 0, len(values))
	for name, value := range values {
		bars = append(bars, renderGaugeBar(name, value, unit, width))
	}

	content := strings.Join(bars, "\n\n")

	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

// renderGaugeBar renders a single horizontal gauge bar with label.
func renderGaugeBar(
	name string,
	value float64,
	unit string,
	maxWidth int,
) string {
	if math.IsNaN(value) {
		return name + "\n[no data]"
	}

	barWidth := min(gaugeBarWidth, maxWidth-4) //nolint:mnd // padding for brackets and value
	barWidth = max(barWidth, 5)                //nolint:mnd // minimum usable bar width

	proportion := clampProportion(value)
	filled := int(proportion * float64(barWidth))
	empty := barWidth - filled

	color := gaugeColor(proportion)
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
func clampProportion(value float64) float64 {
	proportion := (value - gaugeMin) / (gaugeMax - gaugeMin)

	if proportion < 0 {
		return 0
	}

	if proportion > 1 {
		return 1
	}

	return proportion
}

// gaugeColor returns a color based on the gauge proportion.
func gaugeColor(proportion float64) lipgloss.Color {
	percentage := proportion * 100 //nolint:mnd // convert to percentage

	switch {
	case percentage < gaugeThresholdLow:
		return lipgloss.Color("#00FF00") // green
	case percentage < gaugeThresholdMid:
		return lipgloss.Color("#FFFF00") // yellow
	default:
		return lipgloss.Color("#FF5555") // red
	}
}
