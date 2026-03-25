package grafanatui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/lovromazgon/grafana-tui/grafana"
)

// PanelRenderer renders query results for a specific panel type.
type PanelRenderer interface {
	Render(
		result *grafana.QueryResult,
		panel grafana.Panel,
		width, height int,
		hiddenSeries map[string]bool,
	) string
}

// rendererFactories maps panel type strings to their renderer
// constructors.
var rendererFactories = map[string]func() PanelRenderer{ //nolint:gochecknoglobals // registry is read-only after init
	"timeseries":     newTimeSeriesRenderer,
	"graph":          newTimeSeriesRenderer,
	"stat":           newStatRenderer,
	"gauge":          newGaugeRenderer,
	"bargauge":       newGaugeRenderer,
	"table":          newTableRenderer,
	"barchart":       newBarChartRenderer,
	"heatmap":        newHeatmapRenderer,
	"histogram":      newTimeSeriesRenderer,
	"text":           newTextRenderer,
	"logs":           newTextRenderer,
	"piechart":       newPiechartRenderer,
	"alertlist":      newTextRenderer,
	"dashlist":       newTextRenderer,
	"state-timeline": newTimeSeriesRenderer,
	"status-history": newTimeSeriesRenderer,
	"candlestick":    newTimeSeriesRenderer,
	"trend":          newTimeSeriesRenderer,
}

func newTimeSeriesRenderer() PanelRenderer { return &timeSeriesRenderer{} }
func newStatRenderer() PanelRenderer       { return &statRenderer{} }
func newGaugeRenderer() PanelRenderer      { return &gaugeRenderer{} }
func newTableRenderer() PanelRenderer      { return &tableRenderer{} }
func newBarChartRenderer() PanelRenderer   { return &barChartRenderer{} }
func newHeatmapRenderer() PanelRenderer    { return &heatmapRenderer{} }
func newTextRenderer() PanelRenderer       { return &textRenderer{} }

// RendererForPanel returns the appropriate renderer for a panel type.
// Unsupported types get a fallback renderer that shows a message.
func RendererForPanel(panelType string) PanelRenderer {
	if factory, ok := rendererFactories[panelType]; ok {
		return factory()
	}

	return &unsupportedRenderer{panelType: panelType}
}

// textRenderer renders text content.
type textRenderer struct{}

func (r *textRenderer) Render(
	_ *grafana.QueryResult,
	panel grafana.Panel,
	width, height int,
	_ map[string]bool,
) string {
	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		"Text panel: "+panel.Title,
	)
}

// unsupportedRenderer shows a message for unsupported panel types.
type unsupportedRenderer struct {
	panelType string
}

func (r *unsupportedRenderer) Render(
	_ *grafana.QueryResult,
	_ grafana.Panel,
	width, height int,
	_ map[string]bool,
) string {
	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		fmt.Sprintf(
			"Panel type %q is not supported in terminal mode",
			r.panelType,
		),
	)
}
