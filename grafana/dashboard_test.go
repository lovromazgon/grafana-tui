package grafana_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lovromazgon/grafana-tui/grafana"
)

func TestSearchDashboards(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("type") != "dash-db" {
				t.Errorf("expected type=dash-db, got %q",
					r.URL.Query().Get("type"))
			}

			if r.URL.Query().Get("query") != "cpu" {
				t.Errorf("expected query=cpu, got %q",
					r.URL.Query().Get("query"))
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(searchResponse))
		},
	))
	defer server.Close()

	client, err := grafana.NewClient(
		server.URL, grafana.WithHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	results, err := client.SearchDashboards(
		t.Context(), "cpu",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	assertSearchResult(t, results[0])
}

func TestGetDashboard(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(dashboardResponse))
		},
	))
	defer server.Close()

	client, err := grafana.NewClient(
		server.URL, grafana.WithHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dashboard, err := client.GetDashboard(
		t.Context(), "test-uid",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertDashboard(t, dashboard)
}

//nolint:exhaustruct // test data intentionally omits fields
func TestFlattenPanels(t *testing.T) {
	t.Parallel()

	panels := []grafana.Panel{
		{ID: 1, Type: "timeseries", Title: "Panel 1"},
		{
			ID:    2,
			Type:  "row",
			Title: "Row 1",
			Panels: []grafana.Panel{
				{ID: 3, Type: "stat", Title: "Panel 3"},
				{ID: 4, Type: "gauge", Title: "Panel 4"},
			},
		},
		{ID: 5, Type: "table", Title: "Panel 5"},
		{ID: 6, Type: "row", Title: "Empty Row"},
	}

	result := grafana.FlattenPanels(panels)

	expectedTitles := []string{
		"Panel 1", "Panel 3", "Panel 4", "Panel 5",
	}
	if len(result) != len(expectedTitles) {
		t.Fatalf("got %d panels, want %d",
			len(result), len(expectedTitles))
	}

	for i, panel := range result {
		if panel.Title != expectedTitles[i] {
			t.Errorf("panel[%d].Title = %q, want %q",
				i, panel.Title, expectedTitles[i])
		}

		if panel.Type == "row" {
			t.Errorf("panel[%d] is a row, expected non-row", i)
		}
	}
}

func TestSearchDashboards_APIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"invalid API key"}`))
		},
	))
	defer server.Close()

	client, err := grafana.NewClient(
		server.URL, grafana.WithHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = client.SearchDashboards(t.Context(), "")
	if err == nil {
		t.Fatal("expected error for unauthorized request")
	}
}

func assertSearchResult(
	t *testing.T,
	result grafana.DashboardSearchResult,
) {
	t.Helper()

	if result.UID != "abc123" {
		t.Errorf("uid = %q, want %q", result.UID, "abc123")
	}

	if result.Title != "CPU Dashboard" {
		t.Errorf("title = %q, want %q", result.Title, "CPU Dashboard")
	}

	if len(result.Tags) != 2 {
		t.Errorf("tags len = %d, want 2", len(result.Tags))
	}

	if result.FolderTitle != "Production" {
		t.Errorf("folderTitle = %q, want %q",
			result.FolderTitle, "Production")
	}
}

func assertDashboard(t *testing.T, dashboard *grafana.Dashboard) {
	t.Helper()

	if dashboard.Title != "My Dashboard" {
		t.Errorf("title = %q, want %q",
			dashboard.Title, "My Dashboard")
	}

	if len(dashboard.Panels) != 1 {
		t.Fatalf("panels len = %d, want 1", len(dashboard.Panels))
	}

	panel := dashboard.Panels[0]
	if panel.Type != "timeseries" {
		t.Errorf("panel type = %q, want %q", panel.Type, "timeseries")
	}

	if len(panel.Targets) != 1 {
		t.Fatalf("targets len = %d, want 1", len(panel.Targets))
	}

	if panel.Targets[0].RefID != "A" {
		t.Errorf("target refId = %q, want %q",
			panel.Targets[0].RefID, "A")
	}

	if len(dashboard.Templating.List) != 1 {
		t.Fatalf("templating list len = %d, want 1",
			len(dashboard.Templating.List))
	}

	if dashboard.Templating.List[0].Name != "instance" {
		t.Errorf("variable name = %q, want %q",
			dashboard.Templating.List[0].Name, "instance")
	}
}

const searchResponse = `[
	{
		"uid": "abc123",
		"title": "CPU Dashboard",
		"uri": "db/cpu-dashboard",
		"type": "dash-db",
		"tags": ["production", "cpu"],
		"folderUid": "folder1",
		"folderTitle": "Production"
	}
]`

const dashboardResponse = `{
	"dashboard": {
		"title": "My Dashboard",
		"panels": [
			{
				"id": 1,
				"type": "timeseries",
				"title": "Request Rate",
				"targets": [
					{
						"refId": "A",
						"expr": "rate(http_requests_total[5m])"
					}
				],
				"gridPos": {"h": 8, "w": 12, "x": 0, "y": 0}
			}
		],
		"templating": {
			"list": [
				{
					"name": "instance",
					"type": "query",
					"current": {
						"text": "localhost",
						"value": "\"localhost\""
					}
				}
			]
		},
		"time": {"from": "now-1h", "to": "now"}
	},
	"meta": {}
}`
