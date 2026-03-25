# grafana-tui

[![License](https://img.shields.io/github/license/lovromazgon/grafana-tui)](https://github.com/lovromazgon/grafana-tui/blob/main/LICENSE)
[![Test](https://github.com/lovromazgon/grafana-tui/actions/workflows/test.yml/badge.svg)](https://github.com/lovromazgon/grafana-tui/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/lovromazgon/grafana-tui)](https://goreportcard.com/report/github.com/lovromazgon/grafana-tui)

Browse Grafana dashboards in the terminal.
Connect to a remote Grafana instance and explore dashboards, panels, and
time series data without leaving the command line.

## Installation

Install using homebrew:

```sh
brew install lovromazgon/tap/grafana-tui
```

Or download the binary manually from the [latest release](https://github.com/lovromazgon/grafana-tui/releases/latest).

> [!NOTE]
> When downloading grafana-tui manually on MacOS you will get a warning about a safety issue.
> That's because grafana-tui is currently not a signed binary, you have to do some
> [extra steps](https://support.apple.com/en-us/102445#openanyway) to make it run.

Once you have downloaded `grafana-tui`, you can try it out using this runnable example:

```sh
grafana-tui --url https://play.grafana.org
```

## Usage

```sh
grafana-tui --url URL [flags]
```

### Deep linking

The `--url` flag accepts a full Grafana dashboard or panel URL.
This opens the dashboard (and optionally a specific panel) directly,
skipping the dashboard list.

```sh
# Open a specific dashboard
grafana-tui --url https://grafana.example.com/d/abc123

# Open a specific panel within a dashboard
grafana-tui --url https://grafana.example.com/d/abc123?viewPanel=22
```

Press `esc` to navigate back to the dashboard list.

### Authentication

Authenticate with a service account token (recommended) or basic auth
credentials.
Command-line flags override environment variables.

```sh
# Token auth (recommended)
grafana-tui --url https://grafana.example.com --token YOUR_TOKEN

# Or via environment variables
export GRAFANA_URL=https://grafana.example.com
export GRAFANA_SERVICE_ACCOUNT_TOKEN=YOUR_TOKEN
grafana-tui

# Basic auth
grafana-tui --url https://grafana.example.com --username admin --password secret
```

### Flags

```
--url         Grafana instance URL (env: GRAFANA_URL)
--token       Grafana service account token (env: GRAFANA_SERVICE_ACCOUNT_TOKEN)
--username    Grafana basic auth username (env: GRAFANA_USERNAME)
--password    Grafana basic auth password (env: GRAFANA_PASSWORD)
--refresh     Auto-refresh interval (default: 30s)
```

### Keybindings

| Key   | Action                  |
|-------|-------------------------|
| `g`   | Fetch latest data       |
| `t`   | Change time range       |
| `v`   | Change template variables |
| `f`   | Filter series           |
| `j/k` | Scroll down/up          |
| `q`   | Back / quit             |
