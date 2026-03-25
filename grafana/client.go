package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

// Client is an HTTP client for the Grafana API.
type Client struct {
	baseURL    *url.URL
	token      string
	username   string
	password   string
	httpClient *http.Client
}

// ClientOption configures a [Client].
type ClientOption func(*Client)

// WithToken sets a bearer token for authentication.
func WithToken(token string) ClientOption {
	return func(c *Client) { c.token = token }
}

// WithBasicAuth sets basic auth credentials for authentication.
func WithBasicAuth(username, password string) ClientOption {
	return func(c *Client) {
		c.username = username
		c.password = password
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = httpClient }
}

// NewClient creates a new Grafana API client.
func NewClient(baseURL string, opts ...ClientOption) (*Client, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing grafana URL: %w", err)
	}

	client := &Client{
		baseURL:    parsedURL,
		token:      "",
		username:   "",
		password:   "",
		httpClient: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// BaseURL returns the Grafana instance base URL as a string.
func (c *Client) BaseURL() string {
	return c.baseURL.String()
}

// Ping verifies that the Grafana instance is reachable and the
// credentials are valid. It returns an error if the connection fails.
func (c *Client) Ping(ctx context.Context) error {
	var results []json.RawMessage

	err := c.doJSON(
		ctx,
		"/api/search?limit=1",
		&results,
	)
	if err != nil {
		return fmt.Errorf("pinging grafana: %w", err)
	}

	return nil
}

// doJSON executes an HTTP GET request and decodes the JSON response
// into dst.
func (c *Client) doJSON(
	ctx context.Context,
	path string,
	dst any,
) error {
	fullURL := resolveURL(c.baseURL, path)

	request, err := http.NewRequestWithContext(
		ctx, http.MethodGet, fullURL, nil,
	)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	c.setAuthHeader(request)

	slog.Debug("grafana request", "url", fullURL)

	response, err := c.httpClient.Do(request)
	if err != nil {
		slog.Debug("grafana request error", "url", fullURL, "error", err)

		return fmt.Errorf("executing request: %w", err)
	}

	// Read response body for logging, then close.
	responseBody, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()

	slog.Debug("grafana response",
		"url", fullURL,
		"status", response.Status, "body", string(responseBody),
	)

	if response.StatusCode != http.StatusOK {
		return &apiError{
			StatusCode: response.StatusCode,
			Status:     response.Status,
			Body:       string(responseBody),
		}
	}

	if err := json.Unmarshal(responseBody, dst); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	return nil
}

// resolveURL joins the base URL with a path that may contain
// query parameters.
func resolveURL(base *url.URL, path string) string {
	ref, err := url.Parse(path)
	if err != nil {
		return base.String() + path
	}

	ref.Path = strings.TrimRight(base.Path, "/") + ref.Path

	return base.ResolveReference(ref).String()
}

// setAuthHeader sets the appropriate auth header on the request.
func (c *Client) setAuthHeader(request *http.Request) {
	switch {
	case c.token != "":
		request.Header.Set("Authorization", "Bearer "+c.token)
	case c.username != "":
		request.SetBasicAuth(c.username, c.password)
	}
}

// apiError represents an unexpected HTTP status from the Grafana API.
type apiError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *apiError) Error() string {
	return fmt.Sprintf(
		"grafana API error: %s: %s", e.Status, e.Body,
	)
}
