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
// unit from a panel's FieldConfig JSON.
type fieldConfigDefaults struct {
	Defaults struct {
		Unit string `json:"unit"`
	} `json:"defaults"`
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
