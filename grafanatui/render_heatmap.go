package grafanatui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lovromazgon/grafana-tui/grafana"
)

// heatmapRGB holds an RGB color triplet for raw ANSI output.
type heatmapRGB struct{ r, g, b uint8 }

// heatmapColorScale defines the gradient for mapping values to colors.
// Matches Grafana's default heatmap palette: dark blue (low) through
// purple/orange to bright yellow (high).
var heatmapColorScale = []heatmapRGB{ //nolint:gochecknoglobals // read-only color lookup table
	{0x0a, 0x0a, 0x2e}, // very dark blue
	{0x1a, 0x0c, 0x5a}, // dark indigo
	{0x3b, 0x04, 0x9a}, // indigo
	{0x5c, 0x01, 0xa6}, // purple
	{0x7e, 0x03, 0xa8}, // violet
	{0xb5, 0x2f, 0x8c}, // magenta
	{0xcc, 0x47, 0x78}, // pink-red
	{0xde, 0x5f, 0x65}, // salmon
	{0xed, 0x79, 0x53}, // orange
	{0xf8, 0x95, 0x40}, // dark orange
	{0xfd, 0xb4, 0x2f}, // amber
	{0xfb, 0xd5, 0x24}, // yellow
	{0xf0, 0xf9, 0x21}, // bright yellow
}

// fgANSI returns a raw ANSI foreground color escape for 24-bit RGB.
func (c heatmapRGB) fgANSI() string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.r, c.g, c.b)
}

const (
	heatmapLegendHeight = 2 // color scale bar + labels
	heatmapXAxisHeight  = 1 // timestamp labels below chart
)

// heatmapRenderer renders time series data as a heatmap grid with
// Y-axis labels, X-axis timestamps, and a color scale legend.
type heatmapRenderer struct{}

func (r *heatmapRenderer) Render(
	result *grafana.QueryResult,
	panel grafana.Panel,
	width, height int,
	_ map[string]bool,
) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	seriesData := collectTimeSeries(result)
	if len(seriesData) == 0 {
		return lipgloss.Place(
			width, height,
			lipgloss.Center, lipgloss.Center,
			"no heatmap data",
		)
	}

	seriesNames := make([]string, 0, len(seriesData))
	for name := range seriesData {
		seriesNames = append(seriesNames, name)
	}

	sortSeriesNames(seriesNames)

	// De-cumulate histogram data. Prometheus histograms are
	// cumulative (le="+Inf" contains all observations). Grafana's
	// heatmap visualization subtracts each bucket from the one
	// below to show per-bucket counts.
	decumulateHistogram(seriesData, seriesNames)

	// Extract short display labels for the Y axis, formatted with
	// the panel's unit when available. Heatmap panels store the unit
	// in options.yAxis.unit rather than fieldConfig.defaults.unit.
	unit := extractHeatmapUnit(panel.Options)
	if unit == "" {
		unit = ExtractUnit(panel.FieldConfig)
	}

	displayLabels := heatmapDisplayLabels(seriesNames, unit)

	// Compute Y-axis label width from display labels.
	yLabelWidth := heatmapYLabelWidth(displayLabels)

	// Reserve space for axes and legend.
	gridWidth := width - yLabelWidth - 1 // -1 for separator
	gridHeight := height - heatmapXAxisHeight - heatmapLegendHeight

	if gridWidth < 1 || gridHeight < 1 {
		return lipgloss.Place(
			width, height,
			lipgloss.Center, lipgloss.Center,
			"terminal too small",
		)
	}

	// Find global value range for color mapping.
	minVal, maxVal := heatmapValueRange(seriesData, seriesNames)

	// Build the grid: each series gets proportional rows.
	grid := heatmapBuildGrid(
		seriesData, seriesNames, gridWidth, gridHeight, minVal, maxVal,
	)

	// Build Y-axis labels aligned to grid rows.
	yLabelLines := heatmapYLabelLines(displayLabels, gridHeight, yLabelWidth)

	// Combine Y labels + grid line by line. We avoid
	// lipgloss.JoinHorizontal because it can propagate unclosed
	// ANSI background sequences from the grid into padding.
	chartBody := heatmapJoinChart(yLabelLines, grid, gridHeight)

	// Build X-axis with timestamp labels.
	xAxis := heatmapXAxis(seriesData, seriesNames, gridWidth, yLabelWidth)

	// Build color scale legend.
	legend := heatmapLegend(minVal, maxVal, gridWidth, yLabelWidth)

	return chartBody + "\n" + xAxis + "\n" + legend
}

// heatmapValueRange finds the min/max across all series, ignoring NaN.
func heatmapValueRange(
	data map[string]timeSeriesEntry, names []string,
) (float64, float64) {
	minVal := math.Inf(1)
	maxVal := math.Inf(-1)

	for _, name := range names {
		for _, v := range data[name].values {
			if math.IsNaN(v) {
				continue
			}

			if v < minVal {
				minVal = v
			}

			if v > maxVal {
				maxVal = v
			}
		}
	}

	if math.IsInf(minVal, 1) {
		minVal = 0
	}

	if math.IsInf(maxVal, -1) {
		maxVal = 1
	}

	if minVal == maxVal {
		maxVal = minVal + 1
	}

	return minVal, maxVal
}

// heatmapValueToColor maps a value to a color using the heatmap scale.
// Returns nil for NaN or zero values (no data).
func heatmapValueToColor(v, minVal, maxVal float64) *heatmapRGB {
	if math.IsNaN(v) || v == 0 {
		return nil
	}

	t := (v - minVal) / (maxVal - minVal)
	t = math.Max(0, math.Min(1, t))

	idx := int(t * float64(len(heatmapColorScale)-1))
	if idx >= len(heatmapColorScale) {
		idx = len(heatmapColorScale) - 1
	}

	c := heatmapColorScale[idx]

	return &c
}

// heatmapBuildGrid renders the heatmap cells as a block of colored
// half-block characters. Each series gets proportional vertical space.
// Uses upper-half block (▀) with fg=top color and bg=bottom color
// to get two rows per terminal line.
func heatmapBuildGrid(
	data map[string]timeSeriesEntry,
	names []string,
	gridWidth, gridHeight int,
	minVal, maxVal float64,
) string {
	colorGrid := heatmapBuildColorGrid(
		data, names, gridWidth, gridHeight, minVal, maxVal,
	)
	if colorGrid == nil {
		return ""
	}

	return heatmapRenderColorGrid(colorGrid, gridWidth, gridHeight)
}

// heatmapBuildColorGrid maps series data to a 2D color grid where
// row 0 is the top of the chart (highest bucket).
func heatmapBuildColorGrid(
	data map[string]timeSeriesEntry,
	names []string,
	gridWidth, gridHeight int,
	minVal, maxVal float64,
) [][]*heatmapRGB {
	numSeries := len(names)
	numTimePoints := 0

	for _, name := range names {
		numTimePoints = max(numTimePoints, len(data[name].values))
	}

	if numTimePoints == 0 {
		return nil
	}

	colorGrid := make([][]*heatmapRGB, gridHeight)

	for row := range gridHeight {
		colorGrid[row] = make([]*heatmapRGB, gridWidth)
		seriesIdx := min(
			max(numSeries-1-(row*numSeries/gridHeight), 0),
			numSeries-1,
		)
		entry := data[names[seriesIdx]]

		for col := range gridWidth {
			timeIdx := min(col*numTimePoints/gridWidth, len(entry.values)-1)
			colorGrid[row][col] = heatmapValueToColor(
				entry.values[timeIdx], minVal, maxVal,
			)
		}
	}

	return colorGrid
}

// heatmapRenderColorGrid renders the color grid using full-block
// characters with foreground color only.
func heatmapRenderColorGrid(
	colorGrid [][]*heatmapRGB, gridWidth, gridHeight int,
) string {
	const ansiReset = "\x1b[0m"

	var sb strings.Builder

	for row := range gridHeight {
		if row > 0 {
			sb.WriteByte('\n')
		}

		for col := range gridWidth {
			c := colorGrid[row][col]
			if c == nil {
				sb.WriteByte(' ')
			} else {
				sb.WriteString(c.fgANSI())
				sb.WriteString("█")
				sb.WriteString(ansiReset)
			}
		}
	}

	return sb.String()
}

// heatmapJoinChart combines Y-axis label lines with grid lines,
// inserting an ANSI reset between each grid line and the next label
// to prevent background color bleed.
func heatmapJoinChart(
	yLabels []string, grid string, gridHeight int,
) string {
	gridLines := strings.Split(grid, "\n")

	var sb strings.Builder

	for i := range gridHeight {
		if i > 0 {
			sb.WriteByte('\n')
		}

		// Y-axis label (plain text, no ANSI).
		if i < len(yLabels) {
			sb.WriteString(yLabels[i])
		}

		sb.WriteByte(' ')

		// Grid line (may contain ANSI color codes).
		if i < len(gridLines) {
			sb.WriteString(gridLines[i])
		}

		// Reset ANSI state at end of every line so background
		// colors don't leak into padding or subsequent repaints.
		sb.WriteString("\x1b[0m")
	}

	return sb.String()
}
