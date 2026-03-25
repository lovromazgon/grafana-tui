package grafanatui

import (
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

	chart := barchart.New(width, height)

	colorIndex := 0

	for name, value := range values {
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

	return chart.View()
}
