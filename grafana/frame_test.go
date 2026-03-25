package grafana_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/lovromazgon/grafana-tui/grafana"
)

func TestTimeSeries_Multi(t *testing.T) {
	t.Parallel()

	frame := newTimeSeriesMultiFrame()

	timestamps, series, err := frame.TimeSeries()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(timestamps) != 3 {
		t.Fatalf("timestamps len = %d, want 3", len(timestamps))
	}

	if timestamps[0] != 1700000000000 {
		t.Errorf("timestamps[0] = %d, want 1700000000000",
			timestamps[0])
	}

	key := "Value{instance=host1:9090,job=node}"
	values, ok := series[key]

	if !ok {
		t.Fatalf("series key %q not found, got keys: %v",
			key, seriesKeys(series))
	}

	if len(values) != 3 {
		t.Fatalf("values len = %d, want 3", len(values))
	}

	if values[0] != 1.5 {
		t.Errorf("values[0] = %f, want 1.5", values[0])
	}
}

func TestTimeSeries_Wide(t *testing.T) {
	t.Parallel()

	frame := newTimeSeriesWideFrame()

	timestamps, series, err := frame.TimeSeries()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(timestamps) != 2 {
		t.Fatalf("timestamps len = %d, want 2", len(timestamps))
	}

	if len(series) != 2 {
		t.Fatalf("series len = %d, want 2", len(series))
	}

	cpuUser := series["cpu_user"]
	if cpuUser[0] != 10.5 {
		t.Errorf("cpu_user[0] = %f, want 10.5", cpuUser[0])
	}

	cpuSystem := series["cpu_system"]
	if cpuSystem[1] != 4.1 {
		t.Errorf("cpu_system[1] = %f, want 4.1", cpuSystem[1])
	}
}

//nolint:exhaustruct // test data intentionally omits fields
func TestTimeSeries_NullValues(t *testing.T) {
	t.Parallel()

	frame := grafana.DataFrame{
		Schema: grafana.DataFrameSchema{
			Fields: []grafana.DataFrameField{
				{Name: "Time", Type: "time"},
				{Name: "Value", Type: "number"},
			},
		},
		Data: grafana.DataFrameData{
			Values: []json.RawMessage{
				json.RawMessage(`[1700000000000, 1700000060000]`),
				json.RawMessage(`[1.5, null]`),
			},
		},
	}

	_, series, err := frame.TimeSeries()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	values := series["Value"]
	if !math.IsNaN(values[1]) {
		t.Errorf("values[1] = %f, want NaN", values[1])
	}
}

//nolint:exhaustruct // test data intentionally omits fields
func TestNumericValues(t *testing.T) {
	t.Parallel()

	frame := grafana.DataFrame{
		Schema: grafana.DataFrameSchema{
			Fields: []grafana.DataFrameField{
				{
					Name: "cpu", Type: "number",
					Labels: map[string]string{"host": "a"},
				},
				{
					Name: "mem", Type: "number",
					Labels: map[string]string{"host": "a"},
				},
			},
		},
		Data: grafana.DataFrameData{
			Values: []json.RawMessage{
				json.RawMessage(`[42.5]`),
				json.RawMessage(`[78.3]`),
			},
		},
	}

	result, err := frame.NumericValues()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["cpu{host=a}"] != 42.5 {
		t.Errorf("cpu = %f, want 42.5", result["cpu{host=a}"])
	}

	if result["mem{host=a}"] != 78.3 {
		t.Errorf("mem = %f, want 78.3", result["mem{host=a}"])
	}
}

//nolint:exhaustruct // test data intentionally omits fields
func TestTableData(t *testing.T) {
	t.Parallel()

	frame := grafana.DataFrame{
		Schema: grafana.DataFrameSchema{
			Fields: []grafana.DataFrameField{
				{Name: "name", Type: "string"},
				{Name: "value", Type: "number"},
				{Name: "active", Type: "boolean"},
			},
		},
		Data: grafana.DataFrameData{
			Values: []json.RawMessage{
				json.RawMessage(`["host1", "host2"]`),
				json.RawMessage(`[99.5, 45.2]`),
				json.RawMessage(`[true, false]`),
			},
		},
	}

	headers, rows, err := frame.TableData()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(headers) != 3 {
		t.Fatalf("headers len = %d, want 3", len(headers))
	}

	if headers[0] != "name" {
		t.Errorf("headers[0] = %q, want %q", headers[0], "name")
	}

	if len(rows) != 2 {
		t.Fatalf("rows len = %d, want 2", len(rows))
	}

	if rows[0][0] != "host1" {
		t.Errorf("rows[0][0] = %q, want %q", rows[0][0], "host1")
	}

	if rows[1][1] != "45.2" {
		t.Errorf("rows[1][1] = %q, want %q", rows[1][1], "45.2")
	}
}

//nolint:exhaustruct // test helpers build partial structs
func newTimeSeriesMultiFrame() grafana.DataFrame {
	return grafana.DataFrame{
		Schema: grafana.DataFrameSchema{
			RefID: "A",
			Meta: grafana.DataFrameMeta{
				Type: "timeseries-multi",
			},
			Fields: []grafana.DataFrameField{
				{Name: "Time", Type: "time"},
				{
					Name: "Value",
					Type: "number",
					Labels: map[string]string{
						"instance": "host1:9090",
						"job":      "node",
					},
				},
			},
		},
		Data: grafana.DataFrameData{
			Values: []json.RawMessage{
				json.RawMessage(
					`[1700000000000, 1700000060000, 1700000120000]`,
				),
				json.RawMessage(`[1.5, 2.3, 3.1]`),
			},
		},
	}
}

//nolint:exhaustruct // test helpers build partial structs
func newTimeSeriesWideFrame() grafana.DataFrame {
	return grafana.DataFrame{
		Schema: grafana.DataFrameSchema{
			RefID: "A",
			Meta: grafana.DataFrameMeta{
				Type: "timeseries-wide",
			},
			Fields: []grafana.DataFrameField{
				{Name: "Time", Type: "time"},
				{Name: "cpu_user", Type: "number"},
				{Name: "cpu_system", Type: "number"},
			},
		},
		Data: grafana.DataFrameData{
			Values: []json.RawMessage{
				json.RawMessage(
					`[1700000000000, 1700000060000]`,
				),
				json.RawMessage(`[10.5, 11.2]`),
				json.RawMessage(`[3.2, 4.1]`),
			},
		},
	}
}

func seriesKeys(series map[string][]float64) []string {
	keys := make([]string, 0, len(series))
	for k := range series {
		keys = append(keys, k)
	}

	return keys
}
