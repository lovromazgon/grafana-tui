package grafana_test

import (
	"encoding/json"
	"testing"

	"github.com/lovromazgon/grafana-tui/grafana"
)

func TestResolveVariables_SimpleVar(t *testing.T) {
	t.Parallel()

	variables := []grafana.TemplateVariable{
		{ //nolint:exhaustruct // test only sets relevant fields
			Name: "instance",
			Type: "query",
			Current: grafana.TemplateVariableCurrent{
				Text:  json.RawMessage(`"localhost"`),
				Value: json.RawMessage(`"localhost:9090"`),
			},
		},
	}

	got := grafana.ResolveVariables(`up{instance="$instance"}`, variables, nil)
	want := `up{instance="localhost:9090"}`

	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveVariables_BracedVar(t *testing.T) {
	t.Parallel()

	variables := []grafana.TemplateVariable{
		{ //nolint:exhaustruct // test only sets relevant fields
			Name: "job",
			Type: "query",
			Current: grafana.TemplateVariableCurrent{
				Text:  json.RawMessage(`"prometheus"`),
				Value: json.RawMessage(`"prometheus"`),
			},
		},
	}

	got := grafana.ResolveVariables(`up{job="${job}"}`, variables, nil)
	want := `up{job="prometheus"}`

	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveVariables_MultiValue(t *testing.T) {
	t.Parallel()

	variables := []grafana.TemplateVariable{
		{ //nolint:exhaustruct // test only sets relevant fields
			Name: "instance",
			Type: "query",
			Current: grafana.TemplateVariableCurrent{
				Text:  json.RawMessage(`"All"`),
				Value: json.RawMessage(`["host1:9090","host2:9090","host3:9090"]`),
			},
		},
	}

	got := grafana.ResolveVariables(`up{instance=~"$instance"}`, variables, nil)
	want := `up{instance=~"host1:9090,host2:9090,host3:9090"}`

	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveVariables_NoVariables(t *testing.T) {
	t.Parallel()

	expr := `rate(http_requests_total[5m])`
	got := grafana.ResolveVariables(expr, nil, nil)

	if got != expr {
		t.Errorf("got %q, want %q", got, expr)
	}
}

func TestResolveVariables_UnknownVariable(t *testing.T) {
	t.Parallel()

	variables := []grafana.TemplateVariable{
		{ //nolint:exhaustruct // test only sets relevant fields
			Name: "known",
			Type: "query",
			Current: grafana.TemplateVariableCurrent{
				Text:  json.RawMessage(`"value"`),
				Value: json.RawMessage(`"value"`),
			},
		},
	}

	expr := `up{instance="$unknown", job="$known"}`
	got := grafana.ResolveVariables(expr, variables, nil)
	want := `up{instance="$unknown", job="value"}`

	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveVariables_BracedWithFormat(t *testing.T) {
	t.Parallel()

	variables := []grafana.TemplateVariable{
		{ //nolint:exhaustruct // test only sets relevant fields
			Name: "instance",
			Type: "query",
			Current: grafana.TemplateVariableCurrent{
				Text:  json.RawMessage(`"host1"`),
				Value: json.RawMessage(`"host1:9090"`),
			},
		},
	}

	got := grafana.ResolveVariables(`up{instance="${instance:regex}"}`, variables, nil)
	want := `up{instance="host1:9090"}`

	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
