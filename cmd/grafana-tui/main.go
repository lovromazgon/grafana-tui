package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lovromazgon/grafana-tui/grafana"
	"github.com/lovromazgon/grafana-tui/grafanatui"
	"github.com/spf13/cobra"
)

var version = "(devel)"

func main() {
	var flags flags

	rootCmd := &cobra.Command{ //nolint:exhaustruct // cobra commands use optional fields
		Use:   "grafana-tui",
		Short: "Browse Grafana dashboards in the terminal",
		Long: `Connect to a remote Grafana instance and browse dashboards in a
terminal UI. Authenticate with a service account token (recommended) or
basic auth credentials.

Authentication priority: command-line flags override environment variables.
For the URL set --url or GRAFANA_URL.
For token auth set --token or GRAFANA_SERVICE_ACCOUNT_TOKEN.
For basic auth set --username/--password or GRAFANA_USERNAME/GRAFANA_PASSWORD.
When both are available, token auth takes precedence.`,
		Args:    cobra.NoArgs,
		Version: version,
		RunE: func(_ *cobra.Command, _ []string) error {
			return run(flags)
		},
	}

	rootCmd.Flags().StringVar(
		&flags.url, "url", "",
		"Grafana instance URL (env: GRAFANA_URL)",
	)
	rootCmd.Flags().StringVar(
		&flags.token, "token", "",
		"Grafana service account token (env: GRAFANA_SERVICE_ACCOUNT_TOKEN)",
	)
	rootCmd.Flags().StringVar(
		&flags.username, "username", "",
		"Grafana basic auth username (env: GRAFANA_USERNAME)",
	)
	rootCmd.Flags().StringVar(
		&flags.password, "password", "",
		"Grafana basic auth password (env: GRAFANA_PASSWORD)",
	)
	rootCmd.Flags().DurationVar(
		&flags.refresh, "refresh", 30*time.Second, //nolint:mnd // default refresh interval
		"Auto-refresh interval",
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// flags holds the CLI flags for the root command.
type flags struct {
	url      string
	token    string
	username string
	password string
	refresh  time.Duration
}

// run creates the Grafana client and starts the TUI.
func run(f flags) error {
	cleanup := initStderr()
	defer cleanup()

	f.url = resolveEnv(f.url, "GRAFANA_URL")
	if f.url == "" {
		return fmt.Errorf("URL is required: set --url or GRAFANA_URL")
	}

	clientOpts := resolveAuth(f)

	client, err := grafana.NewClient(f.url, clientOpts...)
	if err != nil {
		return fmt.Errorf("creating grafana client: %w", err)
	}

	// Validate connection before entering TUI.
	if err := client.Ping(context.Background()); err != nil {
		return fmt.Errorf("connecting to grafana: %w", err)
	}

	app := grafanatui.NewApp(client, grafanatui.Options{
		TimeRange: grafana.TimeRange{From: "now-1h", To: "now"},
		Refresh:   f.refresh,
	})

	program := tea.NewProgram(app, tea.WithAltScreen())
	if _, err = program.Run(); err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}

	return nil
}

// resolveAuth builds client options from flags and env vars.
func resolveAuth(f flags) []grafana.ClientOption {
	resolvedToken := resolveEnv(
		f.token, "GRAFANA_SERVICE_ACCOUNT_TOKEN",
	)
	resolvedUsername := resolveEnv(
		f.username, "GRAFANA_USERNAME",
	)
	resolvedPassword := resolveEnv(
		f.password, "GRAFANA_PASSWORD",
	)

	switch {
	case resolvedToken != "":
		return []grafana.ClientOption{
			grafana.WithToken(resolvedToken),
		}
	case resolvedUsername != "":
		return []grafana.ClientOption{
			grafana.WithBasicAuth(resolvedUsername, resolvedPassword),
		}
	default:
		return nil
	}
}

// resolveEnv returns the flag value if non-empty, otherwise falls back
// to the named environment variable.
func resolveEnv(flagValue, envKey string) string {
	if flagValue != "" {
		return flagValue
	}

	return os.Getenv(envKey)
}

// initStderr redirects stderr to a log file so that log output does not
// interfere with the terminal UI. The returned function restores the
// original stderr and should be called via defer.
func initStderr() func() {
	cleanup := func() {}

	oldStderr := os.Stderr

	const dataPath = "/tmp/grafana-tui"

	err := os.MkdirAll(dataPath, 0o755)
	if err != nil {
		slog.Error("error creating data directory, logs will be written to stderr (can result in glitches)", "error", err)
		return cleanup
	}

	logFile, err := os.OpenFile(dataPath+"/grafana-tui.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o755)
	if err != nil {
		slog.Error("error opening log file, logs will be written to stderr (can result in glitches)", "error", err)
		return cleanup
	}

	os.Stderr = logFile
	slog.SetDefault(slog.New(slog.NewTextHandler(logFile, nil)))

	cleanup = func() {
		os.Stderr = oldStderr
		slog.SetDefault(slog.New(slog.NewTextHandler(oldStderr, nil)))

		if err := logFile.Close(); err != nil {
			slog.Error("error closing log file", "error", err)
		}
	}

	return cleanup
}
