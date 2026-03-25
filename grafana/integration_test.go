//go:build integration

package grafana_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/lovromazgon/grafana-tui/grafana"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	grafanaUser     = "admin"
	grafanaPassword = "admin"
	grafanaImage    = "grafana/grafana:latest"
)

// setupGrafana starts a Grafana container, creates a TestData datasource
// and a test dashboard, then returns a configured client. The caller
// must invoke the returned cleanup function when done.
func setupGrafana(t *testing.T) *grafana.Client {
	t.Helper()

	ctx, cancel := context.WithTimeout(
		context.Background(), 120*time.Second,
	)
	defer cancel()

	container, err := testcontainers.GenericContainer(ctx,
		testcontainers.GenericContainerRequest{ //nolint:exhaustruct // external type with many optional fields
			ContainerRequest: testcontainers.ContainerRequest{ //nolint:exhaustruct // external type with many optional fields
				Image:        grafanaImage,
				ExposedPorts: []string{"3000/tcp"},
				Env: map[string]string{
					"GF_SECURITY_ADMIN_USER":     grafanaUser,
					"GF_SECURITY_ADMIN_PASSWORD": grafanaPassword,
				},
				WaitingFor: wait.ForHTTP("/api/health").
					WithPort("3000/tcp").
					WithStartupTimeout(60 * time.Second),
			},
			Started: true,
		},
	)
	if err != nil {
		t.Skipf("skipping integration test: could not start grafana container: %v", err)
	}

	t.Cleanup(func() {
		if termErr := container.Terminate(context.Background()); termErr != nil {
			t.Logf("warning: failed to terminate container: %v", termErr)
		}
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "3000/tcp")
	require.NoError(t, err)

	baseURL := "http://" + net.JoinHostPort(host, port.Port())

	client, err := grafana.NewClient(
		baseURL,
		grafana.WithBasicAuth(grafanaUser, grafanaPassword),
	)
	require.NoError(t, err)

	datasourceUID := createTestDataDatasource(t, baseURL)
	createTestDashboard(t, baseURL, datasourceUID)

	return client
}

// createTestDataDatasource creates a TestData datasource via the
// Grafana HTTP API and returns its UID.
func createTestDataDatasource(t *testing.T, baseURL string) string {
	t.Helper()

	payload := map[string]string{
		"name":   "TestData",
		"type":   "grafana-testdata-datasource",
		"access": "proxy",
	}

	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		baseURL+"/api/datasources",
		bytes.NewReader(body),
	)
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(grafanaUser, grafanaPassword)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	require.Equal(t, http.StatusOK, resp.StatusCode,
		"failed to create datasource")

	var result struct {
		Datasource struct {
			UID string `json:"uid"`
		} `json:"datasource"`
	}

	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.NotEmpty(t, result.Datasource.UID)

	return result.Datasource.UID
}

// createTestDashboard creates a dashboard with three panels (timeseries,
// stat, table) that use the TestData datasource.
func createTestDashboard(t *testing.T, baseURL, datasourceUID string) {
	t.Helper()

	dsRef := map[string]string{
		"type": "grafana-testdata-datasource",
		"uid":  datasourceUID,
	}

	dashboard := map[string]any{
		"dashboard": map[string]any{
			"title": "Integration Test Dashboard",
			"panels": []map[string]any{
				{
					"id":         1,
					"type":       "timeseries",
					"title":      "Time Series Panel",
					"datasource": dsRef,
					"targets": []map[string]any{{
						"refId":      "A",
						"datasource": dsRef,
						"scenarioId": "random_walk",
						"alias":      "test-series",
					}},
					"gridPos": map[string]int{
						"h": 8, "w": 12, "x": 0, "y": 0,
					},
				},
				{
					"id":         2,
					"type":       "stat",
					"title":      "Stat Panel",
					"datasource": dsRef,
					"targets": []map[string]any{{
						"refId":      "A",
						"datasource": dsRef,
						"scenarioId": "random_walk",
						"alias":      "stat-value",
					}},
					"gridPos": map[string]int{
						"h": 4, "w": 6, "x": 0, "y": 8,
					},
				},
				{
					"id":         3,
					"type":       "table",
					"title":      "Table Panel",
					"datasource": dsRef,
					"targets": []map[string]any{{
						"refId":      "A",
						"datasource": dsRef,
						"scenarioId": "random_walk",
						"alias":      "table-data",
					}},
					"gridPos": map[string]int{
						"h": 8, "w": 12, "x": 12, "y": 0,
					},
				},
			},
		},
		"overwrite": true,
	}

	body, err := json.Marshal(dashboard)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		baseURL+"/api/dashboards/db",
		bytes.NewReader(body),
	)
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(grafanaUser, grafanaPassword)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	require.Equal(t, http.StatusOK, resp.StatusCode,
		"failed to create dashboard")
}

func TestIntegration_SearchDashboards(t *testing.T) {
	t.Parallel()

	client := setupGrafana(t)

	results, err := client.SearchDashboards(
		context.Background(), "",
	)
	require.NoError(t, err)
	require.NotEmpty(t, results)

	var found bool
	for _, r := range results {
		if r.Title == "Integration Test Dashboard" {
			found = true
			break
		}
	}

	assert.True(t, found,
		"expected to find Integration Test Dashboard in search results")
}

func TestIntegration_GetDashboard(t *testing.T) {
	t.Parallel()

	client := setupGrafana(t)

	results, err := client.SearchDashboards(
		context.Background(), "Integration Test",
	)
	require.NoError(t, err)
	require.NotEmpty(t, results)

	dashboard, err := client.GetDashboard(
		context.Background(), results[0].UID,
	)
	require.NoError(t, err)

	assert.Equal(t, "Integration Test Dashboard", dashboard.Title)
	assert.Len(t, dashboard.Panels, 3)
}

func TestIntegration_QueryPanel(t *testing.T) {
	t.Parallel()

	client := setupGrafana(t)

	results, err := client.SearchDashboards(
		context.Background(), "Integration Test",
	)
	require.NoError(t, err)
	require.NotEmpty(t, results)

	dashboard, err := client.GetDashboard(
		context.Background(), results[0].UID,
	)
	require.NoError(t, err)

	panels := grafana.FlattenPanels(dashboard.Panels)
	require.NotEmpty(t, panels)

	timeRange := grafana.TimeRange{From: "now-1h", To: "now"}
	result, err := client.QueryPanel(
		context.Background(), panels[0], timeRange, 100, nil,
	)
	require.NoError(t, err)
	require.NotEmpty(t, result.Results)

	for _, data := range result.Results {
		require.NotEmpty(t, data.Frames)

		for _, frame := range data.Frames {
			timestamps, series, tsErr := frame.TimeSeries()
			require.NoError(t, tsErr)
			assert.NotEmpty(t, timestamps)
			assert.NotEmpty(t, series)
		}
	}
}

func TestIntegration_FullFlow(t *testing.T) {
	t.Parallel()

	client := setupGrafana(t)

	// Search for dashboards.
	results, err := client.SearchDashboards(
		context.Background(), "",
	)
	require.NoError(t, err)
	require.NotEmpty(t, results)

	// Get the test dashboard.
	var dashboardUID string
	for _, r := range results {
		if r.Title == "Integration Test Dashboard" {
			dashboardUID = r.UID
			break
		}
	}

	require.NotEmpty(t, dashboardUID,
		"expected to find Integration Test Dashboard")

	dashboard, err := client.GetDashboard(
		context.Background(), dashboardUID,
	)
	require.NoError(t, err)

	panels := grafana.FlattenPanels(dashboard.Panels)
	require.Len(t, panels, 3)

	// Query each panel.
	timeRange := grafana.TimeRange{From: "now-1h", To: "now"}

	for _, panel := range panels {
		result, queryErr := client.QueryPanel(
			context.Background(), panel, timeRange, 80, nil,
		)
		require.NoError(t, queryErr)
		assert.NotEmpty(t, result.Results,
			"panel %q should return data", panel.Title)
	}
}
