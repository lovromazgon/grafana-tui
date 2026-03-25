package grafanatui_test

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lovromazgon/grafana-tui/grafana"
	"github.com/lovromazgon/grafana-tui/grafanatui"
)

// mustMarshalJSON marshals a value or panics. Only for test helpers.
func mustMarshalJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic("test: json.Marshal: " + err.Error())
	}

	return data
}

// fixtureMultiSeriesResult builds a QueryResult with multiple time
// series in separate frames.
func fixtureMultiSeriesResult(
	seriesNames []string,
	points int,
) *grafana.QueryResult {
	now := time.Now().UnixMilli()
	frames := make([]grafana.DataFrame, 0, len(seriesNames))

	for _, name := range seriesNames {
		timestamps := make([]float64, points)
		values := make([]float64, points)

		for i := range points {
			timestamps[i] = float64(now + int64(i)*1000) //nolint:mnd // 1s intervals
			values[i] = float64(i) * 2.0                 //nolint:mnd // arbitrary test values
		}

		timestampJSON := mustMarshalJSON(timestamps)
		valuesJSON := mustMarshalJSON(values)

		frames = append(frames, grafana.DataFrame{
			Schema: grafana.DataFrameSchema{
				RefID: "A",
				Meta:  grafana.DataFrameMeta{}, //nolint:exhaustruct // test helper
				Fields: []grafana.DataFrameField{
					{Name: "time", Type: "time", Labels: nil, Config: nil},
					{Name: name, Type: "number", Labels: nil, Config: nil},
				},
			},
			Data: grafana.DataFrameData{
				Values: []json.RawMessage{
					timestampJSON,
					valuesJSON,
				},
			},
		})
	}

	return &grafana.QueryResult{
		Results: map[string]grafana.QueryResultData{
			"A": {Frames: frames},
		},
	}
}

// fixtureNumericResult builds a QueryResult with numeric values.
func fixtureNumericResult(
	values map[string]float64,
) *grafana.QueryResult {
	fields := make([]grafana.DataFrameField, 0, len(values))
	rawValues := make([]json.RawMessage, 0, len(values))

	for name, value := range values {
		fields = append(fields, grafana.DataFrameField{
			Name:   name,
			Type:   "number",
			Labels: nil,
			Config: nil,
		})

		valueJSON := mustMarshalJSON([]float64{value})
		rawValues = append(rawValues, valueJSON)
	}

	return &grafana.QueryResult{
		Results: map[string]grafana.QueryResultData{
			"A": {
				Frames: []grafana.DataFrame{{
					Schema: grafana.DataFrameSchema{
						RefID:  "A",
						Meta:   grafana.DataFrameMeta{}, //nolint:exhaustruct // test helper
						Fields: fields,
					},
					Data: grafana.DataFrameData{
						Values: rawValues,
					},
				}},
			},
		},
	}
}

// fixtureTableResult builds a QueryResult with table data.
func fixtureTableResult(
	headers []string,
	rows [][]string,
) *grafana.QueryResult {
	fields := make([]grafana.DataFrameField, len(headers))
	columns := make([][]any, len(headers))

	for i, header := range headers {
		fields[i] = grafana.DataFrameField{
			Name:   header,
			Type:   "string",
			Labels: nil,
			Config: nil,
		}

		columns[i] = make([]any, len(rows))
	}

	for rowIdx, row := range rows {
		for colIdx := range headers {
			if colIdx < len(row) {
				columns[colIdx][rowIdx] = row[colIdx]
			}
		}
	}

	rawValues := make([]json.RawMessage, len(headers))

	for i, col := range columns {
		colJSON := mustMarshalJSON(col)
		rawValues[i] = colJSON
	}

	return &grafana.QueryResult{
		Results: map[string]grafana.QueryResultData{
			"A": {
				Frames: []grafana.DataFrame{{
					Schema: grafana.DataFrameSchema{
						RefID:  "A",
						Meta:   grafana.DataFrameMeta{}, //nolint:exhaustruct // test helper
						Fields: fields,
					},
					Data: grafana.DataFrameData{
						Values: rawValues,
					},
				}},
			},
		},
	}
}

func emptyPanel() grafana.Panel {
	return grafana.Panel{} //nolint:exhaustruct // test helper
}

func TestTimeSeriesRenderer(t *testing.T) {
	t.Parallel()

	renderer := grafanatui.RendererForPanel("timeseries")

	result := fixtureMultiSeriesResult(
		[]string{"cpu", "memory"}, 5, //nolint:mnd // test fixture
	)

	output := renderer.Render(result, emptyPanel(), 80, 20, nil) //nolint:mnd // test dimensions

	if output == "" {
		t.Fatal("expected non-empty output")
	}

	if !strings.Contains(output, "cpu") {
		t.Error("output should contain legend name 'cpu'")
	}

	if !strings.Contains(output, "memory") {
		t.Error("output should contain legend name 'memory'")
	}
}

func TestStatRenderer_SingleValue(t *testing.T) {
	t.Parallel()

	renderer := grafanatui.RendererForPanel("stat")
	result := fixtureNumericResult(map[string]float64{
		"value": 42,
	})

	output := renderer.Render(result, emptyPanel(), 80, 10, nil) //nolint:mnd // test dimensions

	if output == "" {
		t.Fatal("expected non-empty output")
	}

	if !strings.Contains(output, "42") {
		t.Error("output should contain the value '42'")
	}
}

func TestStatRenderer_MultipleValues(t *testing.T) {
	t.Parallel()

	renderer := grafanatui.RendererForPanel("stat")
	result := fixtureNumericResult(map[string]float64{
		"cpu":    75.5,
		"memory": 60.0,
	})

	output := renderer.Render(result, emptyPanel(), 80, 10, nil) //nolint:mnd // test dimensions

	if output == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestStatRenderer_EmptyResult(t *testing.T) {
	t.Parallel()

	renderer := grafanatui.RendererForPanel("stat")
	result := &grafana.QueryResult{
		Results: map[string]grafana.QueryResultData{},
	}

	output := renderer.Render(result, emptyPanel(), 80, 10, nil) //nolint:mnd // test dimensions

	if !strings.Contains(output, "no data") {
		t.Errorf("expected 'no data', got %q", output)
	}
}

func TestStatRenderer_WithUnit(t *testing.T) {
	t.Parallel()

	renderer := grafanatui.RendererForPanel("stat")
	result := fixtureNumericResult(map[string]float64{
		"value": 1536,
	})
	panel := grafana.Panel{ //nolint:exhaustruct // test
		FieldConfig: json.RawMessage(
			`{"defaults":{"unit":"bytes"}}`,
		),
	}

	output := renderer.Render(result, panel, 80, 10, nil) //nolint:mnd // test dimensions

	if !strings.Contains(output, "KiB") {
		t.Errorf("expected output to contain 'KiB', got %q", output)
	}
}

func TestGaugeRenderer_Percentages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value float64
	}{
		{name: "0%", value: 0},
		{name: "50%", value: 50},
		{name: "100%", value: 100},
	}

	renderer := grafanatui.RendererForPanel("gauge")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := fixtureNumericResult(map[string]float64{
				"gauge": tt.value,
			})

			output := renderer.Render(
				result, emptyPanel(), 80, 10, nil, //nolint:mnd // test dimensions
			)

			if tt.value > 0 && !strings.Contains(output, "\u2588") {
				t.Error("expected filled blocks in gauge bar")
			}

			if tt.value < 100 && !strings.Contains(output, "\u2591") {
				t.Error("expected empty blocks in gauge bar")
			}
		})
	}
}

func TestTableRenderer(t *testing.T) {
	t.Parallel()

	renderer := grafanatui.RendererForPanel("table")

	headers := []string{"name", "status", "count"}
	rows := make([][]string, 5) //nolint:mnd // test fixture

	for i := range rows {
		rows[i] = []string{
			"item-" + strconv.Itoa(i),
			"ok",
			strconv.Itoa(i * 10), //nolint:mnd // test data
		}
	}

	result := fixtureTableResult(headers, rows)
	output := renderer.Render(result, emptyPanel(), 80, 4, nil) //nolint:mnd // test dimensions

	for _, header := range headers {
		if !strings.Contains(output, header) {
			t.Errorf(
				"output should contain header %q", header,
			)
		}
	}
}

func TestTableRenderer_Truncation(t *testing.T) {
	t.Parallel()

	renderer := grafanatui.RendererForPanel("table")

	headers := []string{"name", "value"}
	rows := make([][]string, 20) //nolint:mnd // many rows to trigger truncation

	for i := range rows {
		rows[i] = []string{
			"row-" + strconv.Itoa(i),
			strconv.Itoa(i),
		}
	}

	result := fixtureTableResult(headers, rows)
	// Height of 5 = header + separator + 3 data rows, leaving
	// 17 rows truncated.
	output := renderer.Render(result, emptyPanel(), 80, 5, nil) //nolint:mnd // test dimensions

	if !strings.Contains(output, "more rows") {
		t.Error("expected truncation message in output")
	}
}

func TestBarChartRenderer(t *testing.T) {
	t.Parallel()

	renderer := grafanatui.RendererForPanel("barchart")
	result := fixtureNumericResult(map[string]float64{
		"alpha":   10,
		"beta":    20,
		"charlie": 30,
	})

	output := renderer.Render(result, emptyPanel(), 80, 20, nil) //nolint:mnd // test dimensions

	if output == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestUnsupportedRenderer(t *testing.T) {
	t.Parallel()

	renderer := grafanatui.RendererForPanel("flamegraph")
	result := &grafana.QueryResult{
		Results: map[string]grafana.QueryResultData{},
	}

	output := renderer.Render(result, emptyPanel(), 80, 10, nil) //nolint:mnd // test dimensions

	if !strings.Contains(output, "flamegraph") {
		t.Errorf(
			"expected panel type 'flamegraph' in output, got %q",
			output,
		)
	}
}
