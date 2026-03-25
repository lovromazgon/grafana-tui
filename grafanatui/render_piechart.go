package grafanatui

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lovromazgon/grafana-tui/grafana"
)

// piechartRenderer renders data as a braille-drawn pie chart with a
// color-coded legend.
type piechartRenderer struct{}

func newPiechartRenderer() PanelRenderer { return &piechartRenderer{} }

// pieSlice holds one named value for the chart.
type pieSlice struct {
	name  string
	value float64
	index int
}

func (r *piechartRenderer) Render(
	result *grafana.QueryResult,
	panel grafana.Panel,
	width, height int,
	hiddenSeries map[string]bool,
) string {
	values := extractCategorizedOrNumeric(result, hiddenSeries)
	if len(values) == 0 {
		return lipgloss.Place(
			width, height,
			lipgloss.Center, lipgloss.Center,
			"no data",
		)
	}

	slices := buildSlices(values)
	total := sliceTotal(slices)

	if total == 0 {
		return lipgloss.Place(
			width, height,
			lipgloss.Center, lipgloss.Center,
			"no data (all zero)",
		)
	}

	unit := ExtractUnit(panel.FieldConfig)
	legend := renderPieLegend(slices, total, unit)
	legendWidth := lipgloss.Width(legend)
	legendHeight := lipgloss.Height(legend)

	// Determine layout: pie on left, legend on right (or stacked if
	// too narrow).
	const legendGap = 3
	pieAreaWidth := width - legendWidth - legendGap

	sideBySide := pieAreaWidth >= 8 && height >= 4 //nolint:mnd // min usable pie
	if !sideBySide {
		// Stack vertically: pie on top, legend below.
		pieHeight := max(height-legendHeight-1, 4) //nolint:mnd // min pie height
		pie := renderBraillePie(slices, total, width, pieHeight)

		content := pie + "\n" + legend

		return lipgloss.Place(
			width, height,
			lipgloss.Center, lipgloss.Center,
			content,
		)
	}

	pie := renderBraillePie(slices, total, pieAreaWidth, height)

	legendBlock := lipgloss.Place(
		legendWidth, height,
		lipgloss.Left, lipgloss.Center,
		legend,
	)

	return lipgloss.JoinHorizontal(
		lipgloss.Center,
		pie,
		strings.Repeat(" ", legendGap),
		legendBlock,
	)
}

// extractCategorizedOrNumeric tries to extract categorized values
// (string labels + one numeric column) first, falling back to flat
// numeric values. Hidden series are excluded.
func extractCategorizedOrNumeric(
	result *grafana.QueryResult,
	hiddenSeries map[string]bool,
) map[string]float64 {
	if values := extractFirstCategorized(result, hiddenSeries); len(values) > 0 {
		return values
	}

	// Fall back to flat numeric values.
	values := extractAllNumericValues(result)

	for name := range hiddenSeries {
		delete(values, name)
	}

	return values
}

// extractFirstCategorized finds the first frame with categorized data
// and returns category→value using the first numeric field.
func extractFirstCategorized(
	result *grafana.QueryResult,
	hiddenSeries map[string]bool,
) map[string]float64 {
	for _, rd := range result.Results {
		for i := range rd.Frames {
			frame := &rd.Frames[i]

			categories, series, err := frame.CategorizedValues()
			if err != nil || len(categories) == 0 || len(series) == 0 {
				continue
			}

			return categorizedToMap(categories, series, hiddenSeries)
		}
	}

	return nil
}

// categorizedToMap converts the first numeric field's values into a
// category→value map, skipping hidden series.
func categorizedToMap(
	categories []string,
	series map[string][]float64,
	hiddenSeries map[string]bool,
) map[string]float64 {
	values := make(map[string]float64)

	// Use the first numeric field.
	for _, fieldValues := range series {
		for j, cat := range categories {
			if hiddenSeries[cat] || j >= len(fieldValues) {
				continue
			}

			values[cat] = fieldValues[j]
		}

		break // use first numeric field only
	}

	return values
}

// buildSlices converts a name→value map into a sorted slice list.
func buildSlices(values map[string]float64) []pieSlice {
	slices := make([]pieSlice, 0, len(values))
	index := 0

	for name, value := range values {
		if math.IsNaN(value) || value < 0 {
			continue
		}

		slices = append(slices, pieSlice{
			name:  name,
			value: value,
			index: index,
		})
		index++
	}

	sort.Slice(slices, func(i, j int) bool {
		return slices[i].value > slices[j].value
	})

	// Re-assign indices after sort so colors are stable by rank.
	for i := range slices {
		slices[i].index = i
	}

	return slices
}

func sliceTotal(slices []pieSlice) float64 {
	var total float64
	for _, s := range slices {
		total += s.value
	}

	return total
}

// braille pixel offsets within a cell. Each cell is a 2-wide × 4-tall
// grid of dots, encoded as bits in a Unicode braille character
// (U+2800 base).
//
//	col 0    col 1
//	0x01     0x08    row 0
//	0x02     0x10    row 1
//	0x04     0x20    row 2
//	0x40     0x80    row 3
var brailleBits = [4][2]rune{ //nolint:gochecknoglobals // lookup table
	{0x01, 0x08},
	{0x02, 0x10},
	{0x04, 0x20},
	{0x40, 0x80},
}

const (
	brailleCols = 2 // pixel columns per terminal cell
	brailleRows = 4 // pixel rows per terminal cell
	brailleBase = 0x2800
)

// sliceNone means a pixel is outside the pie.
const sliceNone = -1

// renderBraillePie draws a filled pie chart using braille characters.
func renderBraillePie(
	slices []pieSlice, total float64, width, height int,
) string {
	// Pixel dimensions of the braille canvas.
	pxW := width * brailleCols
	pxH := height * brailleRows

	// Center and radius in pixel coordinates. Braille gives 2 wide
	// × 4 tall pixels per cell, and terminal cells are ~2:1, so
	// braille pixels are roughly square — no aspect correction needed.
	centerX := float64(pxW) / 2
	centerY := float64(pxH) / 2

	radius := math.Min(centerX, centerY) - 1

	if radius < 2 { //nolint:mnd // minimum drawable
		return "too small"
	}

	// Build angle boundaries for each slice, starting at -π/2 (top).
	angles := make([]float64, len(slices)+1)
	angles[0] = -math.Pi / 2 //nolint:mnd // start at 12 o'clock

	for i, s := range slices {
		angles[i+1] = angles[i] + (s.value/total)*2*math.Pi
	}

	// Map every braille pixel to a slice index.
	pixels := make([]int, pxW*pxH)
	for i := range pixels {
		pixels[i] = sliceNone
	}

	for py := range pxH {
		for px := range pxW {
			dx := float64(px) - centerX
			dy := float64(py) - centerY

			dist := math.Sqrt(dx*dx + dy*dy)
			if dist > radius {
				continue
			}

			angle := math.Atan2(dy, dx)
			// Normalize angle into [startAngle, startAngle+2π) so
			// it always falls within the slice boundaries.
			if angle < angles[0] {
				angle += 2 * math.Pi
			}

			pixels[py*pxW+px] = angleToSlice(angle, angles)
		}
	}

	// Render braille cells. For each cell, determine which slice
	// owns the majority of the set pixels and color the cell.
	var lines []string

	for cellY := range height {
		var lineBuilder strings.Builder

		for cellX := range width {
			lineBuilder.WriteString(
				renderBrailleCell(cellX, cellY, pxW, pixels, slices),
			)
		}

		lines = append(lines, lineBuilder.String())
	}

	return strings.Join(lines, "\n")
}

// renderBrailleCell renders a single terminal cell from the pixel
// grid, coloring it by whichever slice owns the most pixels.
func renderBrailleCell(
	cellX, cellY, pxW int,
	pixels []int,
	slices []pieSlice,
) string {
	var brailleChar rune

	counts := make([]int, len(slices))
	anySet := false

	for row := range brailleRows {
		for col := range brailleCols {
			px := cellX*brailleCols + col
			py := cellY*brailleRows + row
			sliceIdx := pixels[py*pxW+px]

			if sliceIdx == sliceNone {
				continue
			}

			brailleChar |= brailleBits[row][col]
			counts[sliceIdx]++
			anySet = true
		}
	}

	if !anySet {
		return " "
	}

	bestSlice := 0

	for i := 1; i < len(counts); i++ {
		if counts[i] > counts[bestSlice] {
			bestSlice = i
		}
	}

	color := chartColor(slices[bestSlice].index)
	style := lipgloss.NewStyle().Foreground(color)

	return style.Render(string(brailleBase + brailleChar))
}

// angleToSlice returns which slice index an angle belongs to.
func angleToSlice(angle float64, boundaries []float64) int {
	for i := range len(boundaries) - 1 {
		if angle >= boundaries[i] && angle < boundaries[i+1] {
			return i
		}
	}

	// Floating-point edge case: assign to last slice.
	return len(boundaries) - 2 //nolint:mnd // last valid index
}

// renderPieLegend draws a vertical legend with colored markers,
// names, values, and percentages.
func renderPieLegend(slices []pieSlice, total float64, unit string) string {
	markerStyle := func(index int) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(chartColor(index))
	}

	nameStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#CCCCCC"))
	pctStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888"))

	lines := make([]string, 0, len(slices))

	for _, s := range slices {
		pct := s.value / total * 100 //nolint:mnd // percentage
		formatted := FormatValue(s.value, unit)

		line := fmt.Sprintf("%s %s  %s  %s",
			markerStyle(s.index).Render("■"),
			nameStyle.Render(s.name),
			formatted,
			pctStyle.Render(fmt.Sprintf("(%.1f%%)", pct)),
		)

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}
