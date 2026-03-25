package grafana_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lovromazgon/grafana-tui/grafana"
)

//nolint:exhaustruct // test data intentionally omits fields
func TestQueryPanel_ProxyEndpoint(t *testing.T) {
	t.Parallel()

	var gotQueryParams map[string]string

	server := newProxyServer(t, &gotQueryParams)
	defer server.Close()

	client, err := grafana.NewClient(
		server.URL, grafana.WithHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	panel := grafana.Panel{
		ID:   1,
		Type: "timeseries",
		Datasource: &grafana.DatasourceRef{
			UID:  "prom1",
			Type: "prometheus",
		},
		Targets: []grafana.Target{
			mustUnmarshalTarget(t,
				`{"refId":"A","expr":"up","datasource":{"uid":"prom1","type":"prometheus"}}`),
		},
	}

	timeRange := grafana.TimeRange{
		From: "1700000000000",
		To:   "1700003600000",
	}

	result, err := client.QueryPanel(
		t.Context(), panel, timeRange, 100, nil, nil, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotQueryParams["query"] != "up" {
		t.Errorf("query = %q, want %q", gotQueryParams["query"], "up")
	}

	if _, ok := result.Results["A"]; !ok {
		t.Error("expected result for refId A")
	}
}

//nolint:exhaustruct // test data intentionally omits fields
func TestQueryPanel_VariableResolution(t *testing.T) {
	t.Parallel()

	var gotQueryParams map[string]string

	server := newProxyServer(t, &gotQueryParams)
	defer server.Close()

	client, err := grafana.NewClient(
		server.URL, grafana.WithHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	panel := grafana.Panel{
		ID:   1,
		Type: "timeseries",
		Datasource: &grafana.DatasourceRef{
			UID:  "prom1",
			Type: "prometheus",
		},
		Targets: []grafana.Target{
			mustUnmarshalTarget(t,
				`{"refId":"A","expr":"up{instance=\"$instance\"}","datasource":{"uid":"prom1","type":"prometheus"}}`),
		},
	}

	variables := []grafana.TemplateVariable{
		{
			Name: "instance",
			Type: "query",
			Current: grafana.TemplateVariableCurrent{
				Text:  json.RawMessage(`"localhost"`),
				Value: json.RawMessage(`"localhost:9090"`),
			},
		},
	}

	timeRange := grafana.TimeRange{From: "now-1h", To: "now"}

	_, err = client.QueryPanel(
		t.Context(), panel, timeRange, 100, variables, nil, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := `up{instance="localhost:9090"}`
	if gotQueryParams["query"] != want {
		t.Errorf("query = %q, want %q", gotQueryParams["query"], want)
	}
}

//nolint:exhaustruct // test data intentionally omits fields
func TestQueryPanel_EmptyTargets(t *testing.T) {
	t.Parallel()

	client, err := grafana.NewClient("http://unused")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	panel := grafana.Panel{
		ID:      1,
		Type:    "timeseries",
		Targets: nil,
	}

	result, err := client.QueryPanel(
		t.Context(), panel,
		grafana.TimeRange{From: "now-1h", To: "now"}, 100, nil, nil, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Results) != 0 {
		t.Errorf("expected empty results, got %d", len(result.Results))
	}
}

// newProxyServer creates a test server that handles both datasource
// lookup and the Prometheus proxy query_range endpoint.
func newProxyServer(
	t *testing.T,
	gotParams *map[string]string,
) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet &&
				r.URL.Path == "/api/datasources/proxy/uid/prom1/api/v1/query_range":
				*gotParams = map[string]string{
					"query": r.URL.Query().Get("query"),
					"start": r.URL.Query().Get("start"),
					"end":   r.URL.Query().Get("end"),
					"step":  r.URL.Query().Get("step"),
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"status": "success",
					"data": {
						"resultType": "matrix",
						"result": [{
							"metric": {"__name__": "up", "instance": "localhost:9090"},
							"values": [[1700000000, "1"], [1700000060, "1"]]
						}]
					}
				}`))

			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		},
	))
}

func mustUnmarshalTarget(
	t *testing.T,
	data string,
) grafana.Target {
	t.Helper()

	var target grafana.Target
	if err := json.Unmarshal([]byte(data), &target); err != nil {
		t.Fatalf("unmarshaling target: %v", err)
	}

	return target
}
