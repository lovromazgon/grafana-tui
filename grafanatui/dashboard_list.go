package grafanatui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lovromazgon/grafana-tui/grafana"
)

// dashboardsLoadedMsg is sent when dashboards have been fetched.
type dashboardsLoadedMsg struct {
	dashboards []grafana.DashboardSearchResult
}

// dashboardItem adapts a DashboardSearchResult for bubbles/list.
type dashboardItem struct {
	result grafana.DashboardSearchResult
}

func (d dashboardItem) Title() string {
	if d.result.FolderTitle != "" {
		return fmt.Sprintf("[%s] %s", d.result.FolderTitle, d.result.Title)
	}

	return d.result.Title
}

func (d dashboardItem) Description() string {
	return strings.Join(d.result.Tags, ", ")
}

func (d dashboardItem) FilterValue() string {
	return d.result.Title
}

// dashboardListView shows a filterable list of Grafana dashboards.
type dashboardListView struct {
	client *grafana.Client
	list   list.Model
}

// newDashboardListView creates a dashboard list view.
func newDashboardListView(client *grafana.Client) *dashboardListView {
	delegate := list.NewDefaultDelegate()
	listModel := list.New(nil, delegate, 0, 0)
	listModel.Title = "Dashboards"
	listModel.SetShowStatusBar(true)
	listModel.SetFilteringEnabled(true)

	return &dashboardListView{
		client: client,
		list:   listModel,
	}
}

// Init starts loading dashboards from the Grafana API.
func (d *dashboardListView) Init() tea.Cmd {
	return d.fetchDashboards()
}

// Update handles messages for the dashboard list view.
func (d *dashboardListView) Update(msg tea.Msg) (view, tea.Cmd) {
	switch msg := msg.(type) {
	case dashboardsLoadedMsg:
		return d.handleDashboardsLoaded(msg)
	case tea.KeyMsg:
		return d.handleKeyMsg(msg)
	default:
		var cmd tea.Cmd
		d.list, cmd = d.list.Update(msg)

		return d, cmd
	}
}

// handleDashboardsLoaded populates the list with fetched dashboards.
func (d *dashboardListView) handleDashboardsLoaded(msg dashboardsLoadedMsg) (view, tea.Cmd) {
	items := make([]list.Item, len(msg.dashboards))

	for i, dashboard := range msg.dashboards {
		items[i] = dashboardItem{result: dashboard}
	}

	cmd := d.list.SetItems(items)

	return d, cmd
}

// handleKeyMsg processes key events for the dashboard list.
func (d *dashboardListView) handleKeyMsg(msg tea.KeyMsg) (view, tea.Cmd) {
	if msg.String() == keyEnter && !d.list.SettingFilter() {
		return d.selectDashboard()
	}

	var cmd tea.Cmd
	d.list, cmd = d.list.Update(msg)

	return d, cmd
}

// selectDashboard pushes a panel view for the selected dashboard.
func (d *dashboardListView) selectDashboard() (view, tea.Cmd) {
	selected, ok := d.list.SelectedItem().(dashboardItem)
	if !ok {
		return d, nil
	}

	pv := newPanelView(d.client, selected.result.UID)

	return d, func() tea.Msg {
		return pushViewMsg{v: pv}
	}
}

// fetchDashboards returns a command that fetches dashboards from
// the Grafana API.
func (d *dashboardListView) fetchDashboards() tea.Cmd {
	client := d.client

	return func() tea.Msg {
		dashboards, err := client.SearchDashboards(
			contextBackground(), "",
		)
		if err != nil {
			return errMsg{err: err}
		}

		return dashboardsLoadedMsg{dashboards: dashboards}
	}
}

// View renders the dashboard list.
func (d *dashboardListView) View(width, height int) string {
	d.list.SetSize(width, height)

	return d.list.View()
}

// Title returns the view title for breadcrumbs.
func (d *dashboardListView) Title() string {
	return "Dashboards"
}

// IsFiltering reports whether the list is in an active filter state.
func (d *dashboardListView) IsFiltering() bool {
	return d.list.SettingFilter()
}

// KeyBindings returns the key bindings for the help bar.
func (d *dashboardListView) KeyBindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(
			key.WithKeys(keyEnter),
			key.WithHelp(keyEnter, "select"),
		),
		key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
	}
}
