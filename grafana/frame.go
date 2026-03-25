package grafana

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	fieldTypeNumber = "number"
	fieldTypeTime   = "time"
)

// Errors returned by DataFrame parsing methods.
var (
	ErrNoTimeField        = errors.New("no time field found in data frame")
	ErrNoStringField      = errors.New("no string field found for categories")
	ErrUnexpectedTimeType = errors.New("unexpected time value type")
	ErrUnexpectedNumType  = errors.New("unexpected number value type")
	ErrFieldOutOfRange    = errors.New("field index out of range")
)

// TimeSeries extracts time series data from a DataFrame.
// Returns timestamps (epoch ms) and a map of series label to values.
func (f *DataFrame) TimeSeries() (
	[]int64, map[string][]float64, error,
) {
	timeIdx, err := f.findTimeFieldIndex()
	if err != nil {
		return nil, nil, err
	}

	timestamps, err := parseTimeValues(f.Data.Values[timeIdx])
	if err != nil {
		return nil, nil, err
	}

	series := make(map[string][]float64)

	for fieldIdx, field := range f.Schema.Fields {
		if fieldIdx == timeIdx || field.Type != fieldTypeNumber {
			continue
		}

		values, parseErr := parseNumberValues(
			f.Data.Values[fieldIdx],
		)
		if parseErr != nil {
			return nil, nil, parseErr
		}

		key := formatFieldKey(field)
		series[key] = values
	}

	return timestamps, series, nil
}

// NumericValues extracts scalar number values from a DataFrame,
// keyed by field name and labels.
func (f *DataFrame) NumericValues() (map[string]float64, error) {
	result := make(map[string]float64)

	for fieldIdx, field := range f.Schema.Fields {
		if field.Type != fieldTypeNumber {
			continue
		}

		values, err := parseNumberValues(f.Data.Values[fieldIdx])
		if err != nil {
			return nil, err
		}

		if len(values) == 0 {
			continue
		}

		key := formatFieldKey(field)
		result[key] = values[len(values)-1]
	}

	return result, nil
}

// CategorizedValues extracts data where one string field provides
// category labels and the remaining numeric fields provide values per
// category. Returns (categories, fieldName→values) or an error if the
// frame has no string field.
func (f *DataFrame) CategorizedValues() (
	[]string, map[string][]float64, error,
) {
	// Find the first string field for category labels.
	catIdx := -1

	for i, field := range f.Schema.Fields {
		if field.Type == "string" {
			catIdx = i
			break
		}
	}

	if catIdx < 0 {
		return nil, nil, ErrNoStringField
	}

	categories, err := parseStringValues(f.Data.Values[catIdx])
	if err != nil {
		return nil, nil, err
	}

	series := make(map[string][]float64)

	for i, field := range f.Schema.Fields {
		if i == catIdx || field.Type != fieldTypeNumber {
			continue
		}

		values, parseErr := parseNumberValues(f.Data.Values[i])
		if parseErr != nil {
			return nil, nil, parseErr
		}

		series[field.Name] = values
	}

	return categories, series, nil
}

// NumericColumn returns all float64 values for the field at the given
// index.
func (f *DataFrame) NumericColumn(fieldIdx int) ([]float64, error) {
	if fieldIdx < 0 || fieldIdx >= len(f.Data.Values) {
		return nil, ErrFieldOutOfRange
	}

	return parseNumberValues(f.Data.Values[fieldIdx])
}

// TableData extracts all fields as string columns, returning headers
// and rows.
func (f *DataFrame) TableData() ([]string, [][]string, error) {
	headers := make([]string, len(f.Schema.Fields))
	columns := make([][]string, len(f.Schema.Fields))

	for fieldIdx, field := range f.Schema.Fields {
		headers[fieldIdx] = formatFieldKey(field)

		col, err := parseStringValues(f.Data.Values[fieldIdx])
		if err != nil {
			return nil, nil, err
		}

		columns[fieldIdx] = col
	}

	rows := transposeColumns(columns)

	return headers, rows, nil
}

// findTimeFieldIndex returns the index of the first time-type field.
func (f *DataFrame) findTimeFieldIndex() (int, error) {
	for fieldIdx, field := range f.Schema.Fields {
		if field.Type == fieldTypeTime {
			return fieldIdx, nil
		}
	}

	return 0, ErrNoTimeField
}

// parseTimeValues parses a JSON array of int64 epoch milliseconds.
func parseTimeValues(raw json.RawMessage) ([]int64, error) {
	var values []any
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("parsing time values: %w", err)
	}

	result := make([]int64, len(values))

	for i, v := range values {
		num, ok := v.(float64)
		if !ok {
			return nil, fmt.Errorf(
				"index %d: %w", i, ErrUnexpectedTimeType,
			)
		}

		result[i] = int64(num)
	}

	return result, nil
}

// parseNumberValues parses a JSON array of nullable float64 values.
// Null values are represented as NaN.
func parseNumberValues(raw json.RawMessage) ([]float64, error) {
	var values []any
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("parsing number values: %w", err)
	}

	result := make([]float64, len(values))

	for i, v := range values {
		if v == nil {
			result[i] = math.NaN()
			continue
		}

		num, ok := v.(float64)
		if !ok {
			return nil, fmt.Errorf(
				"index %d: %w", i, ErrUnexpectedNumType,
			)
		}

		result[i] = num
	}

	return result, nil
}

// parseStringValues parses a JSON array of values as strings.
func parseStringValues(raw json.RawMessage) ([]string, error) {
	var values []any
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("parsing string values: %w", err)
	}

	result := make([]string, len(values))

	for i, v := range values {
		if v == nil {
			result[i] = ""
			continue
		}

		result[i] = fmt.Sprintf("%v", v)
	}

	return result, nil
}

// formatFieldKey builds a key from a field name and labels.
// Format: "name{label1=val1,label2=val2}" or just "name".
func formatFieldKey(field DataFrameField) string {
	if len(field.Labels) == 0 {
		return field.Name
	}

	keys := make([]string, 0, len(field.Labels))
	for k := range field.Labels {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+field.Labels[k])
	}

	return field.Name + "{" + strings.Join(pairs, ",") + "}"
}

// transposeColumns converts column-oriented data to row-oriented.
func transposeColumns(columns [][]string) [][]string {
	if len(columns) == 0 {
		return nil
	}

	rowCount := len(columns[0])
	rows := make([][]string, rowCount)

	for rowIdx := range rowCount {
		row := make([]string, len(columns))

		for colIdx := range columns {
			if rowIdx < len(columns[colIdx]) {
				row[colIdx] = columns[colIdx][rowIdx]
			}
		}

		rows[rowIdx] = row
	}

	return rows
}
