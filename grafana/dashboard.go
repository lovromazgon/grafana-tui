package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// DashboardSearchResult represents a dashboard in search results.
type DashboardSearchResult struct {
	UID         string   `json:"uid"`
	Title       string   `json:"title"`
	URI         string   `json:"uri"`
	Type        string   `json:"type"`
	Tags        []string `json:"tags"`
	FolderUID   string   `json:"folderUid"`
	FolderTitle string   `json:"folderTitle"`
}

// Dashboard represents a Grafana dashboard.
type Dashboard struct {
	Title      string        `json:"title"`
	Panels     []Panel       `json:"panels"`
	Templating Templating    `json:"templating"`
	Time       DashboardTime `json:"time"`
}

// Panel represents a single panel within a dashboard.
type Panel struct {
	ID              int              `json:"id"`
	Type            string           `json:"type"`
	Title           string           `json:"title"`
	Targets         []Target         `json:"targets"`
	Datasource      *DatasourceRef   `json:"datasource,omitempty"`
	GridPos         GridPos          `json:"gridPos"`
	FieldConfig     json.RawMessage  `json:"fieldConfig,omitempty"`
	Options         json.RawMessage  `json:"options,omitempty"`
	Panels          []Panel          `json:"panels,omitempty"`
	Transformations []Transformation `json:"transformations,omitempty"`
}

// Transformation represents a single panel data transformation.
type Transformation struct {
	ID      string          `json:"id"`
	Options json.RawMessage `json:"options,omitempty"`
}

// Target represents a query target within a panel. It preserves the
// raw JSON so it can be forwarded verbatim to /api/ds/query.
type Target struct {
	RefID      string         `json:"refId"`
	Datasource *DatasourceRef `json:"datasource,omitempty"`
	raw        json.RawMessage
}

// UnmarshalJSON parses known fields and stores the raw bytes.
func (t *Target) UnmarshalJSON(data []byte) error {
	// Store the raw bytes for later forwarding.
	t.raw = make(json.RawMessage, len(data))
	copy(t.raw, data)

	// Parse known fields into a temporary struct to avoid recursion.
	var fields struct {
		RefID      string         `json:"refId"`
		Datasource *DatasourceRef `json:"datasource,omitempty"`
	}
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("unmarshaling target fields: %w", err)
	}

	t.RefID = fields.RefID
	t.Datasource = fields.Datasource

	return nil
}

// MarshalJSON returns the original raw bytes.
func (t Target) MarshalJSON() ([]byte, error) {
	if t.raw != nil {
		return t.raw, nil
	}

	// Fallback: marshal the known fields.
	type plain struct {
		RefID      string         `json:"refId"`
		Datasource *DatasourceRef `json:"datasource,omitempty"`
	}

	data, err := json.Marshal(plain{
		RefID: t.RefID, Datasource: t.Datasource,
	})
	if err != nil {
		return nil, fmt.Errorf("marshaling target: %w", err)
	}

	return data, nil
}

// Raw returns the raw JSON bytes for this target.
func (t *Target) Raw() json.RawMessage {
	return t.raw
}

// DatasourceRef identifies a datasource by UID and type. Older
// dashboards may encode this as a plain string instead of an object.
type DatasourceRef struct {
	UID  string `json:"uid"`
	Type string `json:"type"`
}

// UnmarshalJSON handles both string and object forms of a datasource
// reference. A plain string like "Static" is stored as the UID.
func (d *DatasourceRef) UnmarshalJSON(data []byte) error {
	// Try as a plain string first.
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		d.UID = str
		d.Type = ""

		return nil
	}

	// Fall back to object form.
	type plain DatasourceRef

	var obj plain
	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("unmarshaling datasource ref: %w", err)
	}

	*d = DatasourceRef(obj)

	return nil
}

// DashboardTime holds the dashboard's default time range.
type DashboardTime struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// GridPos describes a panel's position and size on the grid.
type GridPos struct {
	H int `json:"h"`
	W int `json:"w"`
	X int `json:"x"`
	Y int `json:"y"`
}

// Templating holds a dashboard's template variable definitions.
type Templating struct {
	List []TemplateVariable `json:"list"`
}

// TemplateVariable represents a single template variable.
type TemplateVariable struct {
	Name       string                   `json:"name"`
	Type       string                   `json:"type"`
	Current    TemplateVariableCurrent  `json:"current"`
	IncludeAll bool                     `json:"includeAll,omitempty"`
	AllValue   string                   `json:"allValue,omitempty"`
	Multi      bool                     `json:"multi,omitempty"`
	Definition string                   `json:"definition,omitempty"`
	Query      json.RawMessage          `json:"query,omitempty"`
	Datasource *DatasourceRef           `json:"datasource,omitempty"`
	Options    []TemplateVariableOption `json:"options,omitempty"`
}

// TemplateVariableOption represents a selectable option for a
// template variable.
type TemplateVariableOption struct {
	Text     string `json:"text"`
	Value    string `json:"value"`
	Selected bool   `json:"selected"`
}

// TemplateVariableCurrent holds the current value of a template
// variable. Both Text and Value can be either a string or []string.
type TemplateVariableCurrent struct {
	Text  json.RawMessage `json:"text"`
	Value json.RawMessage `json:"value"`
}

// dashboardResponse wraps the Grafana API response for
// GET /api/dashboards/uid/:uid.
type dashboardResponse struct {
	Dashboard Dashboard `json:"dashboard"`
}

// SearchDashboards returns all dashboards matching the query.
func (c *Client) SearchDashboards(
	ctx context.Context,
	query string,
) ([]DashboardSearchResult, error) {
	params := url.Values{
		"type": {"dash-db"},
	}
	if query != "" {
		params.Set("query", query)
	}

	path := "/api/search?" + params.Encode()

	var results []DashboardSearchResult
	if err := c.doJSON(ctx, path, &results); err != nil {
		return nil, fmt.Errorf("searching dashboards: %w", err)
	}

	return results, nil
}

// GetDashboard returns the full dashboard by UID.
func (c *Client) GetDashboard(
	ctx context.Context,
	uid string,
) (*Dashboard, error) {
	path := "/api/dashboards/uid/" + url.PathEscape(uid)

	var wrapper dashboardResponse
	if err := c.doJSON(ctx, path, &wrapper); err != nil {
		return nil, fmt.Errorf("getting dashboard: %w", err)
	}

	return &wrapper.Dashboard, nil
}

// skipPanelTypes lists panel types that are not useful in terminal
// mode and should be excluded from navigation.
var skipPanelTypes = map[string]bool{ //nolint:gochecknoglobals // read-only set
	"row":       true,
	"text":      true,
	"dashlist":  true,
	"alertlist": true,
	"news":               true,
	"innius-video-panel": true,
}

// FlattenPanels returns all renderable panels, expanding row panels
// that contain nested sub-panels. Non-data panels (text, dashlist,
// etc.) are skipped.
func FlattenPanels(panels []Panel) []Panel {
	var result []Panel

	for _, panel := range panels {
		if panel.Type == "row" {
			result = append(result, FlattenPanels(panel.Panels)...)
			continue
		}

		if skipPanelTypes[panel.Type] {
			continue
		}

		result = append(result, panel)
	}

	return result
}
