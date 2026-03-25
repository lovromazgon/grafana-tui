package grafanatui_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/lovromazgon/grafana-tui/grafanatui"
)

func TestFormatValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    float64
		unit     string
		expected string
	}{
		// short / empty (SI suffixes)
		{name: "short 1500", value: 1500, unit: "short", expected: "1.5K"},
		{name: "short 1500000", value: 1500000, unit: "short", expected: "1.5M"},
		{name: "short 1500000000", value: 1500000000, unit: "short", expected: "1.5G"},
		{name: "short 1500000000000", value: 1500000000000, unit: "short", expected: "1.5T"},
		{name: "empty unit 1500", value: 1500, unit: "", expected: "1.5K"},
		{name: "short small value", value: 42, unit: "short", expected: "42"},

		// bytes (binary)
		{name: "bytes 1536", value: 1536, unit: "bytes", expected: "1.5 KiB"},
		{name: "bytes 1073741824", value: 1073741824, unit: "bytes", expected: "1.0 GiB"},
		{name: "bytes small", value: 100, unit: "bytes", expected: "100.0 B"},

		// decbytes (decimal)
		{name: "decbytes 1500", value: 1500, unit: "decbytes", expected: "1.5 KB"},
		{name: "decbytes 1500000", value: 1500000, unit: "decbytes", expected: "1.5 MB"},
		{name: "decbytes small", value: 500, unit: "decbytes", expected: "500.0 B"},

		// seconds
		{name: "seconds 90", value: 90, unit: "s", expected: "1m 30s"},
		{name: "seconds 3661", value: 3661, unit: "s", expected: "1h 1m 1s"},
		{name: "seconds 0.5", value: 0.5, unit: "s", expected: "500ms"},
		{name: "seconds 0", value: 0, unit: "s", expected: "0s"},
		{name: "seconds 3600", value: 3600, unit: "s", expected: "1h"},

		// milliseconds
		{name: "ms 1500", value: 1500, unit: "ms", expected: "1.5s"},
		{name: "ms 45", value: 45, unit: "ms", expected: "45ms"},

		// percent
		{name: "percent 67.5", value: 67.5, unit: "percent", expected: "67.5%"},

		// percentunit
		{name: "percentunit 0.675", value: 0.675, unit: "percentunit", expected: "67.5%"},

		// reqps
		{name: "reqps 1500", value: 1500, unit: "reqps", expected: "1.5K req/s"},

		// ops
		{name: "ops 1500", value: 1500, unit: "ops", expected: "1.5K ops/s"},

		// none
		{name: "none integer", value: 42, unit: "none", expected: "42"},
		{name: "none float", value: 42.56, unit: "none", expected: "42.56"},

		// unknown unit
		{name: "unknown unit", value: 123, unit: "foobar", expected: "123"},

		// edge cases
		{name: "zero", value: 0, unit: "short", expected: "0"},
		{name: "negative", value: -1500, unit: "short", expected: "-1.5K"},
		{name: "very large", value: 1e18, unit: "short", expected: "1000000.0T"},
		{name: "NaN", value: math.NaN(), unit: "short", expected: "NaN"},
		{name: "+Inf", value: math.Inf(1), unit: "short", expected: "+Inf"},
		{name: "-Inf", value: math.Inf(-1), unit: "short", expected: "-Inf"},
		{name: "negative seconds", value: -90, unit: "s", expected: "-1m 30s"},
		{name: "negative ms", value: -500, unit: "ms", expected: "-500ms"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := grafanatui.FormatValue(tt.value, tt.unit)
			if got != tt.expected {
				t.Errorf(
					"FormatValue(%v, %q) = %q, want %q",
					tt.value, tt.unit, got, tt.expected,
				)
			}
		})
	}
}

func TestExtractUnit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    json.RawMessage
		expected string
	}{
		{
			name:     "bytes unit",
			input:    json.RawMessage(`{"defaults":{"unit":"bytes"}}`),
			expected: "bytes",
		},
		{
			name:     "empty field config",
			input:    nil,
			expected: "",
		},
		{
			name:     "no unit",
			input:    json.RawMessage(`{"defaults":{}}`),
			expected: "",
		},
		{
			name:     "invalid json",
			input:    json.RawMessage(`{invalid`),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := grafanatui.ExtractUnit(tt.input)
			if got != tt.expected {
				t.Errorf(
					"ExtractUnit(%s) = %q, want %q",
					string(tt.input), got, tt.expected,
				)
			}
		})
	}
}
