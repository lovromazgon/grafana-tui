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
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

// promConfig is a minimal Prometheus configuration that scrapes itself
// every second so integration tests don't have to wait for the default
// 15s interval.
const promConfig = `global:
  scrape_interval: 1s
scrape_configs:
  - job_name: prometheus
    static_configs:
      - targets: ["localhost:9090"]
`

const (
	grafanaUser     = "admin"
	grafanaPassword = "admin"
	grafanaImage    = "grafana/grafana:latest"
	prometheusImage = "prom/prometheus:latest"
)

// setupGrafana starts Prometheus and Grafana containers on a shared
// Docker network, adds Prometheus as a datasource in Grafana, creates
// a test dashboard with PromQL panels, and returns a configured client.
func setupGrafana(t *testing.T) *grafana.Client {
	t.Helper()

	ctx := t.Context()

	// Create a shared network so Grafana can reach Prometheus by name.
	dockerNet, err := network.New(ctx)
	if err != nil {
		t.Skipf("skipping integration test: could not create docker network: %v", err)
	}

	t.Cleanup(func() {
		if removeErr := dockerNet.Remove(context.Background()); removeErr != nil {
			t.Logf("warning: failed to remove network: %v", removeErr)
		}
	})

	// Start Prometheus with a fast scrape interval (1s).
	promContainer, err := testcontainers.GenericContainer(ctx,
		testcontainers.GenericContainerRequest{ //nolint:exhaustruct
			ContainerRequest: testcontainers.ContainerRequest{ //nolint:exhaustruct
				Image:        prometheusImage,
				ExposedPorts: []string{"9090/tcp"},
				Networks:     []string{dockerNet.Name},
				NetworkAliases: map[string][]string{
					dockerNet.Name: {"prometheus"},
				},
				Files: []testcontainers.ContainerFile{{
					Reader:            bytes.NewReader([]byte(promConfig)),
					ContainerFilePath: "/etc/prometheus/prometheus.yml",
					FileMode:          0o644,
				}},
				WaitingFor: wait.ForHTTP("/-/ready").
					WithPort("9090/tcp").
					WithStartupTimeout(60 * time.Second),
			},
			Started: true,
		},
	)
	if err != nil {
		t.Skipf("skipping integration test: could not start prometheus container: %v", err)
	}

	t.Cleanup(func() {
		if termErr := promContainer.Terminate(context.Background(), testcontainers.StopTimeout(0)); termErr != nil {
			t.Logf("warning: failed to terminate prometheus container: %v", termErr)
		}
	})

	// Start Grafana on the same network.
	grafanaContainer, err := testcontainers.GenericContainer(ctx,
		testcontainers.GenericContainerRequest{ //nolint:exhaustruct
			ContainerRequest: testcontainers.ContainerRequest{ //nolint:exhaustruct
				Image:        grafanaImage,
				ExposedPorts: []string{"3000/tcp"},
				Networks:     []string{dockerNet.Name},
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
		if termErr := grafanaContainer.Terminate(context.Background(), testcontainers.StopTimeout(0)); termErr != nil {
			t.Logf("warning: failed to terminate grafana container: %v", termErr)
		}
	})

	host, err := grafanaContainer.Host(ctx)
	require.NoError(t, err)

	port, err := grafanaContainer.MappedPort(ctx, "3000/tcp")
	require.NoError(t, err)

	baseURL := "http://" + net.JoinHostPort(host, port.Port())

	client, err := grafana.NewClient(
		baseURL,
		grafana.WithBasicAuth(grafanaUser, grafanaPassword),
	)
	require.NoError(t, err)

	// Wait for Prometheus to have scraped data before proceeding.
	waitForPrometheusData(t, promContainer, ctx)

	datasourceUID := createPrometheusDatasource(t, baseURL)
	createTestDashboard(t, baseURL, datasourceUID)

	return client
}

// waitForPrometheusData polls Prometheus until it has at least one
// sample for the up metric.
func waitForPrometheusData(
	t *testing.T,
	container testcontainers.Container,
	ctx context.Context,
) {
	t.Helper()

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "9090/tcp")
	require.NoError(t, err)

	promURL := "http://" + net.JoinHostPort(host, port.Port())

	deadline := time.Now().Add(30 * time.Second) //nolint:mnd
	for time.Now().Before(deadline) {
		req, reqErr := http.NewRequestWithContext(
			ctx, http.MethodGet,
			promURL+"/api/v1/query?query=up",
			nil,
		)
		require.NoError(t, reqErr)

		resp, doErr := http.DefaultClient.Do(req)
		if doErr != nil {
			time.Sleep(500 * time.Millisecond) //nolint:mnd
			continue
		}

		var result struct {
			Data struct {
				Result []any `json:"result"`
			} `json:"data"`
		}

		_ = json.NewDecoder(resp.Body).Decode(&result)
		_ = resp.Body.Close()

		if len(result.Data.Result) > 0 {
			// With a 1s scrape interval a couple more seconds is
			// enough for query_range to return multiple data points.
			time.Sleep(3 * time.Second) //nolint:mnd
			return
		}

		time.Sleep(500 * time.Millisecond) //nolint:mnd
	}

	t.Fatal("timed out waiting for prometheus to have data")
}

// createPrometheusDatasource creates a Prometheus datasource in
// Grafana pointing at the Prometheus container and returns its UID.
func createPrometheusDatasource(t *testing.T, baseURL string) string {
	t.Helper()

	payload := map[string]any{
		"name":   "Prometheus",
		"type":   "prometheus",
		"access": "proxy",
		"url":    "http://prometheus:9090",
	}

	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(
		t.Context(),
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

// createTestDashboard creates a dashboard with three panels
// (timeseries, stat, table) that use the Prometheus datasource.
func createTestDashboard(t *testing.T, baseURL, datasourceUID string) {
	t.Helper()

	dsRef := map[string]string{
		"type": "prometheus",
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
						"expr":       "up",
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
						"expr":       "up",
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
						"expr":       "prometheus_build_info",
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
		t.Context(),
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

func TestIntegration(t *testing.T) {
	client := setupGrafana(t)

	t.Run("SearchDashboards", func(t *testing.T) {
		results, err := client.SearchDashboards(t.Context(), "")
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
	})

	t.Run("GetDashboard", func(t *testing.T) {
		results, err := client.SearchDashboards(
			t.Context(), "Integration Test",
		)
		require.NoError(t, err)
		require.NotEmpty(t, results)

		dashboard, err := client.GetDashboard(
			t.Context(), results[0].UID,
		)
		require.NoError(t, err)

		assert.Equal(t, "Integration Test Dashboard", dashboard.Title)
		assert.Len(t, dashboard.Panels, 3)
	})

	t.Run("QueryPanel", func(t *testing.T) {
		results, err := client.SearchDashboards(
			t.Context(), "Integration Test",
		)
		require.NoError(t, err)
		require.NotEmpty(t, results)

		dashboard, err := client.GetDashboard(
			t.Context(), results[0].UID,
		)
		require.NoError(t, err)

		panels := grafana.FlattenPanels(dashboard.Panels)
		require.NotEmpty(t, panels)

		timeRange := grafana.TimeRange{From: "now-1h", To: "now"}
		result, err := client.QueryPanel(
			t.Context(), panels[0], timeRange, 100, nil, nil, panels,
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
	})

	t.Run("FullFlow", func(t *testing.T) {
		// Search for dashboards.
		results, err := client.SearchDashboards(t.Context(), "")
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
			t.Context(), dashboardUID,
		)
		require.NoError(t, err)

		panels := grafana.FlattenPanels(dashboard.Panels)
		require.Len(t, panels, 3)

		// Query each panel.
		timeRange := grafana.TimeRange{From: "now-1h", To: "now"}

		for _, panel := range panels {
			result, queryErr := client.QueryPanel(
				t.Context(), panel, timeRange, 80, nil, nil, panels,
			)
			require.NoError(t, queryErr,
				"panel %q query failed", panel.Title)
			assert.NotEmpty(t, result.Results,
				"panel %q should return data", panel.Title)

			for refID, data := range result.Results {
				assert.NotEmpty(t, data.Frames,
					"panel %q refID %q should have frames",
					panel.Title, refID)
			}
		}
	})
}
