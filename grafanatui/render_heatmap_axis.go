package grafanatui

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// lePattern matches histogram bucket labels in the format produced by
// formatFieldKey: A{le=0.5} or A{le=+Inf} (no quotes around value).
var lePattern = regexp.MustCompile(`\ble=([^,}]+)`)

// sortSeriesNames sorts series names numerically by their le= bucket
// value when present, falling back to alphabetical sort.
func sortSeriesNames(names []string) {
	// Check if these look like histogram buckets.
	leValues := make(map[string]float64, len(names))
	isHistogram := true

	for _, name := range names {
		m := lePattern.FindStringSubmatch(name)
		if m == nil {
			isHistogram = false

			break
		}

		leStr := m[1]
		if leStr == labelPosInf {
			leValues[name] = math.Inf(1)
		} else {
			v, err := strconv.ParseFloat(leStr, 64)
			if err != nil {
				isHistogram = false

				break
			}

			leValues[name] = v
		}
	}

	if isHistogram {
		sort.Slice(names, func(i, j int) bool {
			return leValues[names[i]] < leValues[names[j]]
		})
	} else {
		sort.Strings(names)
	}
}

// decumulateHistogram converts cumulative Prometheus histogram data
// to per-bucket counts. Names must be sorted in ascending le= order.
// For each time point, the value becomes bucket[i] - bucket[i-1].
// Non-histogram series are left unchanged.
func decumulateHistogram(
	data map[string]timeSeriesEntry, names []string,
) {
	// Check that all series look like histogram buckets.
	for _, name := range names {
		if lePattern.FindStringSubmatch(name) == nil {
			return
		}
	}

	if len(names) < 2 {
		return
	}

	// Walk buckets from top to bottom so we can subtract without
	// clobbering values we still need.
	for i := len(names) - 1; i >= 1; i-- {
		upper := data[names[i]]
		lower := data[names[i-1]]

		numPoints := min(len(upper.values), len(lower.values))
		for j := range numPoints {
			if math.IsNaN(upper.values[j]) || math.IsNaN(lower.values[j]) {
				continue
			}

			diff := upper.values[j] - lower.values[j]
			if diff < 0 {
				diff = 0
			}

			upper.values[j] = diff
		}
	}
}

// heatmapDisplayLabels extracts short display labels for the Y axis.
// For histogram buckets, it shows the le= value formatted with the
// panel unit (e.g., "500ms", labelPosInf). For other series, it uses the
// full name.
func heatmapDisplayLabels(names []string, unit string) []string {
	labels := make([]string, len(names))

	for i, name := range names {
		m := lePattern.FindStringSubmatch(name)
		if m == nil {
			labels[i] = name

			continue
		}

		leStr := m[1]
		if leStr == labelPosInf {
			labels[i] = labelPosInf

			continue
		}

		v, err := strconv.ParseFloat(leStr, 64)
		if err != nil {
			labels[i] = leStr

			continue
		}

		if unit != "" {
			labels[i] = FormatValue(v, unit)
		} else {
			labels[i] = strconv.FormatFloat(v, 'g', -1, 64)
		}
	}

	return labels
}

// extractHeatmapUnit extracts the Y-axis unit from heatmap panel
// options (options.yAxis.unit).
func extractHeatmapUnit(options json.RawMessage) string {
	if len(options) == 0 {
		return ""
	}

	var opts struct {
		YAxis struct {
			Unit string `json:"unit"`
		} `json:"yAxis"`
	}

	if err := json.Unmarshal(options, &opts); err != nil {
		return ""
	}

	return opts.YAxis.Unit
}

// heatmapYLabelWidth computes the width needed for Y-axis labels.
func heatmapYLabelWidth(labels []string) int {
	maxWidth := 0
	for _, label := range labels {
		if len(label) > maxWidth {
			maxWidth = len(label)
		}
	}

	const maxLabelWidth = 16
	if maxWidth > maxLabelWidth {
		maxWidth = maxLabelWidth
	}

	return maxWidth
}

// heatmapYLabelLines builds the Y-axis labels as a slice of strings,
// one per terminal line, vertically centered within each series band.
func heatmapYLabelLines(
	labels []string, gridHeight, labelWidth int,
) []string {
	numSeries := len(labels)
	lines := make([]string, gridHeight)
	termLines := len(lines)

	for i, label := range labels {
		bandStart := termLines - (i+1)*termLines/numSeries
		bandEnd := termLines - i*termLines/numSeries
		midLine := (bandStart + bandEnd) / 2

		if midLine < 0 || midLine >= len(lines) {
			continue
		}

		if len(label) > labelWidth {
			label = label[:labelWidth]
		}

		lines[midLine] = fmt.Sprintf("%*s", labelWidth, label)
	}

	blank := strings.Repeat(" ", labelWidth)
	for i, line := range lines {
		if line == "" {
			lines[i] = blank
		}
	}

	return lines
}

// heatmapXAxis builds the X-axis timestamp labels.
func heatmapXAxis(
	data map[string]timeSeriesEntry,
	names []string,
	gridWidth, yLabelWidth int,
) string {
	// Find timestamps from first series with data.
	var timestamps []int64

	for _, name := range names {
		if ts := data[name].timestamps; len(ts) > 0 {
			timestamps = ts

			break
		}
	}

	if len(timestamps) == 0 {
		return ""
	}

	// Determine time format based on range.
	rangeMs := timestamps[len(timestamps)-1] - timestamps[0]
	dayMs := int64(24 * 60 * 60 * 1000) //nolint:mnd // 24h in ms
	format := "15:04"

	if rangeMs > dayMs {
		format = "Jan 02 15:04"
	}

	// Place a few evenly spaced labels.
	const maxLabels = 6
	numLabels := min(maxLabels, gridWidth/12) //nolint:mnd // ~12 chars per label

	numLabels = max(numLabels, 2) //nolint:mnd // at least 2 labels

	line := make([]byte, gridWidth)
	for i := range line {
		line[i] = ' '
	}

	for i := range numLabels {
		col := i * (gridWidth - 1) / (numLabels - 1)
		tsIdx := i * (len(timestamps) - 1) / (numLabels - 1)
		tsUnix := timestamps[tsIdx] / 1000       //nolint:mnd // ms to s
		t := time.Unix(tsUnix, 0).In(time.Local) //nolint:gosmopolitan // TUI shows user-local time
		label := t.Format(format)

		// Center the label on the column.
		start := max(col-len(label)/2, 0)

		if start+len(label) > gridWidth {
			start = max(gridWidth-len(label), 0)
			label = label[:min(len(label), gridWidth)]
		}

		copy(line[start:], label)
	}

	padding := strings.Repeat(" ", yLabelWidth+1)

	return padding + string(line)
}

// heatmapLegend draws a horizontal color scale bar with min/max labels.
func heatmapLegend(
	minVal, maxVal float64,
	gridWidth, yLabelWidth int,
) string {
	padding := strings.Repeat(" ", yLabelWidth+1)

	// Color bar.
	const maxBarWidth = 40
	barWidth := min(gridWidth, maxBarWidth)

	var bar strings.Builder

	for i := range barWidth {
		t := float64(i) / float64(max(barWidth-1, 1))
		idx := int(t * float64(len(heatmapColorScale)-1))
		c := heatmapColorScale[idx]
		bar.WriteString(c.fgANSI())
		bar.WriteString("█")
	}

	bar.WriteString("\x1b[0m")

	// Labels below the bar using human-friendly formatting.
	minLabel := formatHeatmapValue(minVal)
	maxLabel := formatHeatmapValue(maxVal)
	labelLine := minLabel +
		strings.Repeat(" ", max(0, barWidth-len(minLabel)-len(maxLabel))) +
		maxLabel

	return padding + bar.String() + "\n" + padding + labelLine
}

// formatHeatmapValue formats a value for the heatmap legend using
// SI suffixes for large numbers.
func formatHeatmapValue(v float64) string {
	absV := math.Abs(v)

	switch {
	case absV == 0:
		return "0"
	case absV >= 1e12: //nolint:mnd // tera
		return fmt.Sprintf("%.4gT", v/1e12) //nolint:mnd // tera
	case absV >= 1e9: //nolint:mnd // giga
		return fmt.Sprintf("%.4gG", v/1e9) //nolint:mnd // giga
	case absV >= 1e6: //nolint:mnd // mega
		return fmt.Sprintf("%.4gM", v/1e6) //nolint:mnd // mega
	case absV >= 1e3: //nolint:mnd // kilo
		return fmt.Sprintf("%.4gK", v/1e3) //nolint:mnd // kilo
	case absV < 0.01: //nolint:mnd // small values
		return fmt.Sprintf("%.2e", v)
	default:
		return fmt.Sprintf("%.4g", v)
	}
}
