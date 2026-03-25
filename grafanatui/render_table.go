package grafanatui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lovromazgon/grafana-tui/grafana"
)

const (
	tableColumnPadding = 2
	tableMinColWidth   = 3
)

// tableRenderer renders data in aligned columns with a header.
type tableRenderer struct{}

func (r *tableRenderer) Render(
	result *grafana.QueryResult,
	_ grafana.Panel,
	width, height int,
	_ map[string]bool,
) string {
	headers, rows := collectTableData(result)
	if len(headers) == 0 {
		return lipgloss.Place(
			width, height,
			lipgloss.Center, lipgloss.Center,
			"no table data",
		)
	}

	colWidths := calculateColumnWidths(headers, rows, width)
	lines := renderTableLines(headers, rows, colWidths, height)

	return strings.Join(lines, "\n")
}

// collectTableData gathers headers and rows from all frames.
func collectTableData(
	result *grafana.QueryResult,
) ([]string, [][]string) {
	for _, resultData := range result.Results {
		for _, frame := range resultData.Frames {
			headers, rows, err := frame.TableData()
			if err != nil {
				continue
			}

			if len(headers) > 0 {
				return headers, rows
			}
		}
	}

	return nil, nil
}

// calculateColumnWidths determines each column's width, capped to
// fit the terminal.
func calculateColumnWidths(
	headers []string,
	rows [][]string,
	maxWidth int,
) []int {
	widths := make([]int, len(headers))

	for i, header := range headers {
		widths[i] = len(header)
	}

	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Cap total width to terminal width.
	totalPadding := (len(widths) - 1) * tableColumnPadding
	availableWidth := maxWidth - totalPadding

	availableWidth = max(availableWidth, len(widths)*tableMinColWidth)

	totalWidth := 0
	for _, w := range widths {
		totalWidth += w
	}

	if totalWidth > availableWidth {
		scaleColumnWidths(widths, availableWidth, totalWidth)
	}

	return widths
}

// scaleColumnWidths proportionally scales column widths to fit.
func scaleColumnWidths(
	widths []int,
	availableWidth, totalWidth int,
) {
	for i := range widths {
		widths[i] = max(
			widths[i]*availableWidth/totalWidth,
			tableMinColWidth,
		)
	}
}

// renderTableLines builds the formatted table output.
func renderTableLines(
	headers []string,
	rows [][]string,
	colWidths []int,
	maxHeight int,
) []string {
	headerStyle := lipgloss.NewStyle().Bold(true)

	lines := make([]string, 0, len(rows)+2) //nolint:mnd // header + separator

	// Header row.
	headerLine := formatRow(headers, colWidths)
	lines = append(lines, headerStyle.Render(headerLine))

	// Separator.
	lines = append(lines, buildSeparator(colWidths))

	// Data rows.
	maxDataRows := max(maxHeight-2, 1) //nolint:mnd // header + separator

	remaining := len(rows) - maxDataRows
	displayRows := rows

	if remaining > 0 {
		displayRows = rows[:maxDataRows]
	}

	for _, row := range displayRows {
		lines = append(lines, formatRow(row, colWidths))
	}

	if remaining > 0 {
		lines = append(
			lines,
			fmt.Sprintf("... and %d more rows", remaining),
		)
	}

	return lines
}

// formatRow formats a single row with proper column alignment.
func formatRow(cells []string, colWidths []int) string {
	parts := make([]string, len(colWidths))

	for i, width := range colWidths {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}

		if len(cell) > width {
			cell = cell[:width]
		}

		parts[i] = fmt.Sprintf("%-*s", width, cell)
	}

	return strings.Join(parts, strings.Repeat(" ", tableColumnPadding))
}

// buildSeparator creates a horizontal line matching column widths.
func buildSeparator(colWidths []int) string {
	parts := make([]string, len(colWidths))

	for i, width := range colWidths {
		parts[i] = strings.Repeat("─", width)
	}

	return strings.Join(parts, strings.Repeat(" ", tableColumnPadding))
}
