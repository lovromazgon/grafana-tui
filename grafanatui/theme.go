package grafanatui

import "github.com/charmbracelet/lipgloss"

// chartColors defines a shared color palette for chart rendering.
var chartColors = []lipgloss.Color{ //nolint:gochecknoglobals // read-only palette
	lipgloss.Color("#00FFFF"), // cyan
	lipgloss.Color("#00FF00"), // green
	lipgloss.Color("#FFFF00"), // yellow
	lipgloss.Color("#FF00FF"), // magenta
	lipgloss.Color("#5555FF"), // blue
	lipgloss.Color("#FF5555"), // red
	lipgloss.Color("#FF8800"), // orange
	lipgloss.Color("#88FF88"), // light green
}

// chartColor returns a color from the palette, cycling on overflow.
func chartColor(index int) lipgloss.Color {
	return chartColors[index%len(chartColors)]
}
