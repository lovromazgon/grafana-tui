package grafana_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lovromazgon/grafana-tui/grafana"
)

func TestNewClient_InvalidURL(t *testing.T) {
	t.Parallel()

	_, err := grafana.NewClient("://invalid")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestClient_BearerTokenAuth(t *testing.T) {
	t.Parallel()

	var gotAuth string

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		},
	))
	defer server.Close()

	client, err := grafana.NewClient(server.URL,
		grafana.WithToken("my-secret-token"),
		grafana.WithHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, _ = client.SearchDashboards(t.Context(), "")

	expected := "Bearer my-secret-token"
	if gotAuth != expected {
		t.Errorf("auth header = %q, want %q", gotAuth, expected)
	}
}

func TestClient_BasicAuth(t *testing.T) {
	t.Parallel()

	var gotUsername, gotPassword string
	var gotOK bool

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotUsername, gotPassword, gotOK = r.BasicAuth()
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		},
	))
	defer server.Close()

	client, err := grafana.NewClient(server.URL,
		grafana.WithBasicAuth("admin", "password123"),
		grafana.WithHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, _ = client.SearchDashboards(t.Context(), "")

	if !gotOK {
		t.Fatal("expected basic auth to be set")
	}

	if gotUsername != "admin" {
		t.Errorf("username = %q, want %q", gotUsername, "admin")
	}

	if gotPassword != "password123" {
		t.Errorf("password = %q, want %q", gotPassword, "password123")
	}
}

func TestClient_URLConstruction(t *testing.T) {
	t.Parallel()

	var gotPath string

	dashboardJSON := `{"dashboard":{"title":"test","panels":[],` +
		`"templating":{"list":[]},"time":{"from":"now-1h","to":"now"}}}`

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(dashboardJSON))
		},
	))
	defer server.Close()

	client, err := grafana.NewClient(server.URL+"/grafana",
		grafana.WithHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, _ = client.GetDashboard(t.Context(), "abc123")

	expected := "/grafana/api/dashboards/uid/abc123"
	if gotPath != expected {
		t.Errorf("path = %q, want %q", gotPath, expected)
	}
}
