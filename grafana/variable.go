package grafana

import (
	"encoding/json"
	"maps"
	"regexp"
	"strings"
)

// grafanaAllToken is Grafana's special value indicating "All" selected.
const grafanaAllToken = "$__all"

// variablePattern matches $var, ${var}, and ${var:format} patterns.
var variablePattern = regexp.MustCompile(`\$\{(\w+)(?::[^}]*)?\}|\$(\w+)`)

// ResolveVariables replaces $var, ${var}, and ${var:format} references
// in expr using the current values from the provided variables.
// Overrides take precedence over the variables' current values.
// After substitution, any `=".*"` (literal equality match for ".*")
// is converted to `=~".*"` so Prometheus treats it as a regex match.
func ResolveVariables(
	expr string,
	variables []TemplateVariable,
	overrides map[string]string,
) string {
	variableMap := buildVariableMap(variables)

	maps.Copy(variableMap, overrides)

	resolved := variablePattern.ReplaceAllStringFunc(expr, func(match string) string {
		name := extractVariableName(match)
		if value, ok := variableMap[name]; ok {
			return value
		}

		return match
	})

	// Fix literal `=".*"` to regex `=~".*"`. This happens when a
	// query uses `label="$var"` and the variable resolves to ".*".
	resolved = strings.ReplaceAll(resolved, `=".*"`, `=~".*"`)

	return resolved
}

// buildVariableMap creates a lookup from variable name to its resolved value.
// It uses the current (default) value saved in the dashboard JSON for each
// variable. When the current value is grafanaAllToken, resolveCurrentValue converts
// it to ".*" for Prometheus compatibility.
func buildVariableMap(variables []TemplateVariable) map[string]string {
	result := make(map[string]string, len(variables))

	for _, variable := range variables {
		result[variable.Name] = resolveCurrentValue(variable)
	}

	return result
}

// extractVariableName returns the variable name from a $var or ${var:...} match.
func extractVariableName(match string) string {
	submatches := variablePattern.FindStringSubmatch(match)
	if submatches == nil {
		return ""
	}

	// ${var} or ${var:format} form.
	if submatches[1] != "" {
		return submatches[1]
	}

	// $var form.
	return submatches[2]
}

// resolveCurrentValue extracts the current value from a template variable.
// The value can be either a JSON string or a JSON array of strings.
// Grafana's special grafanaAllToken token is resolved using AllValue if set,
// otherwise falling back to ".*" for Prometheus compatibility.
func resolveCurrentValue(variable TemplateVariable) string {
	current := variable.Current
	if len(current.Value) == 0 {
		return rawMessageToString(current.Text)
	}

	// Try string first.
	var stringValue string
	if err := json.Unmarshal(current.Value, &stringValue); err == nil {
		if stringValue == grafanaAllToken {
			if variable.AllValue != "" {
				return variable.AllValue
			}

			return ".*"
		}

		return stringValue
	}

	// Try []string for multi-value variables.
	var arrayValue []string
	if err := json.Unmarshal(current.Value, &arrayValue); err == nil {
		// Check for $__all in the array (Grafana stores it as
		// [grafanaAllToken] for multi-value variables).
		if len(arrayValue) == 1 && arrayValue[0] == grafanaAllToken {
			if variable.AllValue != "" {
				return variable.AllValue
			}

			return ".*"
		}

		return strings.Join(arrayValue, ",")
	}

	return rawMessageToString(current.Text)
}

// rawMessageToString extracts a string from a json.RawMessage that
// may be a JSON string or a JSON array of strings.
func rawMessageToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var stringValue string
	if err := json.Unmarshal(raw, &stringValue); err == nil {
		return stringValue
	}

	var arrayValue []string
	if err := json.Unmarshal(raw, &arrayValue); err == nil {
		return strings.Join(arrayValue, ",")
	}

	return string(raw)
}
