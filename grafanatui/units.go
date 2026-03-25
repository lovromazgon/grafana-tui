package grafanatui

import (
	"encoding/json"
	"fmt"
	"math"
)

// SI magnitude thresholds and suffixes.
const (
	siKilo = 1_000.0
	siMega = 1_000_000.0
	siGiga = 1_000_000_000.0
	siTera = 1_000_000_000_000.0
)

// Binary byte thresholds.
const (
	binaryKibi = 1024.0
	binaryMebi = 1024.0 * 1024.0
	binaryGibi = 1024.0 * 1024.0 * 1024.0
	binaryTebi = 1024.0 * 1024.0 * 1024.0 * 1024.0
)

// Decimal byte thresholds.
const (
	decKilo = 1_000.0
	decMega = 1_000_000.0
	decGiga = 1_000_000_000.0
	decTera = 1_000_000_000_000.0
)

// Time and percentage constants.
const (
	secondsPerMinute      = 60
	secondsPerHour        = 3600
	millisecondsPerSecond = 1000.0
	percentMultiplier     = 100.0
)

// unitFormatters maps Grafana unit strings to their formatting
// functions.
var unitFormatters = map[string]func(float64) string{ //nolint:gochecknoglobals // registry is read-only after init
	"short":       formatSI,
	"":            formatSI,
	"bytes":       formatBinaryBytes,
	"decbytes":    formatDecimalBytes,
	"bits":        formatBits,
	"s":           formatSeconds,
	"ms":          formatMilliseconds,
	"ns":          formatNanoseconds,
	"percent":     formatPercent,
	"percentunit": formatPercentUnit,
	"reqps":       formatReqPerSec,
	"ops":         formatOpsPerSec,
	"none":        formatRawNumber,
	"Bps":         formatBytesPerSec,
	"binBps":      formatBinaryBytesPerSec,
	"decBps":      formatDecimalBytesPerSec,
	"bps":         formatBitsPerSec,
	"pps":         formatPacketsPerSec,
	"iops":        formatIOPS,
	"hertz":       formatHertz,
	"watt":        formatWatt,
	"kwatt":       formatKilowatt,
	"voltamp":     formatVoltAmp,
}

const (
	labelPosInf = "+Inf"
	labelNegInf = "-Inf"
)

// FormatValue formats a float64 value according to the given Grafana
// unit string.
func FormatValue(value float64, unit string) string {
	if math.IsNaN(value) {
		return "NaN"
	}

	if math.IsInf(value, 1) {
		return labelPosInf
	}

	if math.IsInf(value, -1) {
		return labelNegInf
	}

	if formatter, ok := unitFormatters[unit]; ok {
		return formatter(value)
	}

	return formatRawNumber(value)
}

// formatSI formats a number with SI suffixes (K, M, G, T).
func formatSI(value float64) string {
	absValue := math.Abs(value)

	switch {
	case absValue >= siTera:
		return trimTrailingZeros(value/siTera, "T")
	case absValue >= siGiga:
		return trimTrailingZeros(value/siGiga, "G")
	case absValue >= siMega:
		return trimTrailingZeros(value/siMega, "M")
	case absValue >= siKilo:
		return trimTrailingZeros(value/siKilo, "K")
	default:
		return formatRawNumber(value)
	}
}

// formatBinaryBytes formats bytes using binary (IEC) prefixes.
func formatBinaryBytes(value float64) string {
	absValue := math.Abs(value)

	switch {
	case absValue >= binaryTebi:
		return trimTrailingZeros(value/binaryTebi, " TiB")
	case absValue >= binaryGibi:
		return trimTrailingZeros(value/binaryGibi, " GiB")
	case absValue >= binaryMebi:
		return trimTrailingZeros(value/binaryMebi, " MiB")
	case absValue >= binaryKibi:
		return trimTrailingZeros(value/binaryKibi, " KiB")
	default:
		return trimTrailingZeros(value, " B")
	}
}

// formatDecimalBytes formats bytes using decimal (SI) prefixes.
func formatDecimalBytes(value float64) string {
	absValue := math.Abs(value)

	switch {
	case absValue >= decTera:
		return trimTrailingZeros(value/decTera, " TB")
	case absValue >= decGiga:
		return trimTrailingZeros(value/decGiga, " GB")
	case absValue >= decMega:
		return trimTrailingZeros(value/decMega, " MB")
	case absValue >= decKilo:
		return trimTrailingZeros(value/decKilo, " KB")
	default:
		return trimTrailingZeros(value, " B")
	}
}

// formatSeconds formats a duration in seconds as a human-readable
// string using hours, minutes, seconds, and milliseconds.
func formatSeconds(value float64) string {
	if value < 0 {
		return "-" + formatSeconds(-value)
	}

	if value == 0 {
		return "0s"
	}

	if value < 1 {
		ms := value * millisecondsPerSecond
		return fmt.Sprintf("%.0fms", ms)
	}

	totalSeconds := int(value)
	hours := totalSeconds / secondsPerHour
	minutes := (totalSeconds % secondsPerHour) / secondsPerMinute
	seconds := totalSeconds % secondsPerMinute

	result := ""
	if hours > 0 {
		result += fmt.Sprintf("%dh ", hours)
	}

	if minutes > 0 {
		result += fmt.Sprintf("%dm ", minutes)
	}

	if seconds > 0 {
		result += fmt.Sprintf("%ds", seconds)
	}

	if result == "" {
		return "0s"
	}

	// Trim trailing space from "Xh " or "Xm " when there is no
	// smaller component.
	if result[len(result)-1] == ' ' {
		result = result[:len(result)-1]
	}

	return result
}

// formatMilliseconds formats a duration in milliseconds as a
// human-readable string.
func formatMilliseconds(value float64) string {
	if value < 0 {
		return "-" + formatMilliseconds(-value)
	}

	if value >= millisecondsPerSecond {
		return trimTrailingZeros(value/millisecondsPerSecond, "s")
	}

	return fmt.Sprintf("%.0fms", value)
}

// formatPercent formats a value as a percentage.
func formatPercent(value float64) string {
	return trimTrailingZeros(value, "%")
}

// formatPercentUnit formats a unit-based percentage (0-1 range).
func formatPercentUnit(value float64) string {
	return trimTrailingZeros(value*percentMultiplier, "%")
}

// formatReqPerSec formats a value as requests per second.
func formatReqPerSec(value float64) string {
	return formatSI(value) + " req/s"
}

// formatOpsPerSec formats a value as operations per second.
func formatOpsPerSec(value float64) string {
	return formatSI(value) + " ops/s"
}

// formatBytesPerSec formats a rate in bytes per second using decimal prefixes.
func formatBytesPerSec(value float64) string {
	return formatDecimalBytes(value) + "/s"
}

// formatBinaryBytesPerSec formats a rate in bytes per second using binary prefixes.
func formatBinaryBytesPerSec(value float64) string {
	return formatBinaryBytes(value) + "/s"
}

// formatDecimalBytesPerSec formats a rate in bytes per second using decimal prefixes.
func formatDecimalBytesPerSec(value float64) string {
	return formatDecimalBytes(value) + "/s"
}

// formatBitsPerSec formats a rate in bits per second.
func formatBitsPerSec(value float64) string {
	return formatBits(value) + "/s"
}

// formatBits formats a value in bits using SI prefixes.
func formatBits(value float64) string {
	absValue := math.Abs(value)

	switch {
	case absValue >= siTera:
		return trimTrailingZeros(value/siTera, " Tb")
	case absValue >= siGiga:
		return trimTrailingZeros(value/siGiga, " Gb")
	case absValue >= siMega:
		return trimTrailingZeros(value/siMega, " Mb")
	case absValue >= siKilo:
		return trimTrailingZeros(value/siKilo, " Kb")
	default:
		return trimTrailingZeros(value, " b")
	}
}

// formatNanoseconds formats a duration in nanoseconds.
func formatNanoseconds(value float64) string {
	if value < 0 {
		return "-" + formatNanoseconds(-value)
	}

	nsPerUs := 1000.0
	nsPerMs := 1_000_000.0
	nsPerSec := 1_000_000_000.0

	switch {
	case value >= nsPerSec:
		return formatSeconds(value / nsPerSec)
	case value >= nsPerMs:
		return fmt.Sprintf("%.0fms", value/nsPerMs)
	case value >= nsPerUs:
		return fmt.Sprintf("%.0fµs", value/nsPerUs)
	default:
		return fmt.Sprintf("%.0fns", value)
	}
}

// formatPacketsPerSec formats a value as packets per second.
func formatPacketsPerSec(value float64) string {
	return formatSI(value) + " pps"
}

// formatIOPS formats a value as I/O operations per second.
func formatIOPS(value float64) string {
	return formatSI(value) + " iops"
}

// formatHertz formats a value in hertz.
func formatHertz(value float64) string {
	absValue := math.Abs(value)

	switch {
	case absValue >= siGiga:
		return trimTrailingZeros(value/siGiga, " GHz")
	case absValue >= siMega:
		return trimTrailingZeros(value/siMega, " MHz")
	case absValue >= siKilo:
		return trimTrailingZeros(value/siKilo, " kHz")
	default:
		return trimTrailingZeros(value, " Hz")
	}
}

// formatWatt formats a value in watts.
func formatWatt(value float64) string {
	absValue := math.Abs(value)

	switch {
	case absValue >= siKilo:
		return trimTrailingZeros(value/siKilo, " kW")
	default:
		return trimTrailingZeros(value, " W")
	}
}

// formatKilowatt formats a value in kilowatts.
func formatKilowatt(value float64) string {
	return trimTrailingZeros(value, " kW")
}

// formatVoltAmp formats a value in volt-amperes.
func formatVoltAmp(value float64) string {
	return trimTrailingZeros(value, " VA")
}

// formatRawNumber formats a number with appropriate decimal places.
func formatRawNumber(value float64) string {
	if value == math.Trunc(value) && math.Abs(value) < 1e15 {
		return fmt.Sprintf("%.0f", value)
	}

	return fmt.Sprintf("%.2f", value)
}

// trimTrailingZeros formats a number to one decimal place and
// appends a suffix.
func trimTrailingZeros(value float64, suffix string) string {
	return fmt.Sprintf("%.1f%s", value, suffix)
}

// fieldConfigDefaults is the parsed structure for extracting the
// unit, thresholds, and min/max from a panel's FieldConfig JSON.
type fieldConfigDefaults struct {
	Defaults struct {
		Unit       string           `json:"unit"`
		Min        *float64         `json:"min,omitempty"`
		Max        *float64         `json:"max,omitempty"`
		Thresholds *thresholdConfig `json:"thresholds,omitempty"`
	} `json:"defaults"`
}

// thresholdConfig represents Grafana's threshold configuration.
type thresholdConfig struct {
	Mode  string          `json:"mode"` // "absolute" or "percentage"
	Steps []thresholdStep `json:"steps"`
}

// thresholdStep represents a single threshold step.
type thresholdStep struct {
	Color string   `json:"color"`
	Value *float64 `json:"value"` // nil for the base step
}

// GaugeConfig holds the parsed gauge-relevant settings from a
// panel's fieldConfig.
type GaugeConfig struct {
	Min           float64
	Max           float64
	MinSet        bool // true if min was explicitly configured
	MaxSet        bool // true if max was explicitly configured
	Thresholds    []thresholdStep
	ThresholdMode string // "absolute" or "percentage"
}

// ExtractGaugeConfig reads min, max, and thresholds from a panel's
// FieldConfig JSON.
func ExtractGaugeConfig(fieldConfig json.RawMessage) GaugeConfig {
	cfg := GaugeConfig{
		Min:           0,
		Max:           100, //nolint:mnd // Grafana default
		MinSet:        false,
		MaxSet:        false,
		Thresholds:    nil,
		ThresholdMode: "absolute",
	}

	if len(fieldConfig) == 0 {
		return cfg
	}

	var parsed fieldConfigDefaults
	if err := json.Unmarshal(fieldConfig, &parsed); err != nil {
		return cfg
	}

	if parsed.Defaults.Min != nil {
		cfg.Min = *parsed.Defaults.Min
		cfg.MinSet = true
	}

	if parsed.Defaults.Max != nil {
		cfg.Max = *parsed.Defaults.Max
		cfg.MaxSet = true
	}

	if parsed.Defaults.Thresholds != nil {
		cfg.Thresholds = parsed.Defaults.Thresholds.Steps

		if parsed.Defaults.Thresholds.Mode != "" {
			cfg.ThresholdMode = parsed.Defaults.Thresholds.Mode
		}
	}

	return cfg
}

// ThresholdColor returns the hex color for a value based on the
// threshold steps. Steps are evaluated in reverse order (highest
// first). In percentage mode, thresholds are relative to the min/max
// range. Grafana named colors are resolved to hex.
func (g *GaugeConfig) ThresholdColor(value float64) string {
	if len(g.Thresholds) == 0 {
		return "#00FF00" // default green
	}

	// In percentage mode, convert value to a percentage of the range.
	compareValue := value
	if g.ThresholdMode == "percentage" {
		rangeVal := g.Max - g.Min
		if rangeVal > 0 {
			compareValue = (value - g.Min) / rangeVal * 100 //nolint:mnd // percentage
		}
	}

	// Walk steps in reverse to find the highest matching threshold.
	for i := len(g.Thresholds) - 1; i >= 0; i-- {
		step := g.Thresholds[i]
		if step.Value == nil {
			return resolveGrafanaColor(step.Color)
		}

		if compareValue >= *step.Value {
			return resolveGrafanaColor(step.Color)
		}
	}

	// Fallback to base step color if present.
	if g.Thresholds[0].Value == nil {
		return resolveGrafanaColor(g.Thresholds[0].Color)
	}

	return "#00FF00"
}

// grafanaColors maps Grafana's named color tokens to hex values.
var grafanaColors = map[string]string{ //nolint:gochecknoglobals // read-only lookup
	"green":             "#73BF69",
	"dark-green":        "#1A7311",
	"semi-dark-green":   "#37872D",
	"light-green":       "#96D98D",
	"super-light-green": "#C8F2C2",

	"red":             "#F2495C",
	"dark-red":        "#AD0317",
	"semi-dark-red":   "#E02F44",
	"light-red":       "#FF7383",
	"super-light-red": "#FFA6B0",

	"blue":             "#5794F2",
	"dark-blue":        "#1F60C4",
	"semi-dark-blue":   "#3274D9",
	"light-blue":       "#8AB8FF",
	"super-light-blue": "#C0D8FF",

	"yellow":             "#FADE2A",
	"dark-yellow":        "#CC9D00",
	"semi-dark-yellow":   "#E0B400",
	"light-yellow":       "#FFEE52",
	"super-light-yellow": "#FFF899",

	"orange":             "#FF9830",
	"dark-orange":        "#C4641A",
	"semi-dark-orange":   "#E0752D",
	"light-orange":       "#FFB357",
	"super-light-orange": "#FFCB7D",

	"purple":             "#B877D9",
	"dark-purple":        "#7C2EA3",
	"semi-dark-purple":   "#8F3BB8",
	"light-purple":       "#CA95E5",
	"super-light-purple": "#DEB6F2",

	"white": "#FFFFFF",
	"gray":  "#808080",
}

// resolveGrafanaColor converts a Grafana color (named or hex) to a
// hex string usable by lipgloss.
func resolveGrafanaColor(color string) string {
	if len(color) > 0 && color[0] == '#' {
		return color
	}

	if hex, ok := grafanaColors[color]; ok {
		return hex
	}

	return color
}

// gaugeDisplayModes lists the table cell display modes that should
// render as gauges rather than plain text.
var gaugeDisplayModes = map[string]bool{ //nolint:gochecknoglobals // read-only set
	"basic":          true,
	"gradient-gauge": true,
	"lcd-gauge":      true,
}

// isGaugeDisplayMode checks if the panel's field config has a
// gauge-like display mode set.
func isGaugeDisplayMode(fieldConfig json.RawMessage) bool {
	if len(fieldConfig) == 0 {
		return false
	}

	var config struct {
		Defaults struct {
			Custom struct {
				DisplayMode string `json:"displayMode"`
			} `json:"custom"`
		} `json:"defaults"`
	}

	if err := json.Unmarshal(fieldConfig, &config); err != nil {
		return false
	}

	return gaugeDisplayModes[config.Defaults.Custom.DisplayMode]
}

// ExtractUnit reads the unit string from a panel's FieldConfig JSON.
// It parses the path: {"defaults": {"unit": "bytes"}}.
func ExtractUnit(fieldConfig json.RawMessage) string {
	if len(fieldConfig) == 0 {
		return ""
	}

	var config fieldConfigDefaults
	if err := json.Unmarshal(fieldConfig, &config); err != nil {
		return ""
	}

	return config.Defaults.Unit
}
