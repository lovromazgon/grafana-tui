package grafanatui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lovromazgon/grafana-tui/grafana"
)

// Messages for async dashboard and panel data loading.
type (
	dashboardLoadedMsg struct{ dashboard *grafana.Dashboard }
	panelDataMsg       struct{ result *grafana.QueryResult }
	panelErrorMsg      struct{ err error }
)

// panelView shows one panel at a time with navigation.
type panelView struct {
	client          *grafana.Client
	uid             string
	dashboard       *grafana.Dashboard
	panels          []grafana.Panel
	index           int
	data            *grafana.QueryResult
	loading         bool
	width           int
	height          int
	timeRange       grafana.TimeRange
	err             error
	hiddenSeries    map[string]bool
	variableValues  map[string]string
	variableOptions map[string][]string
	initialPanelID  int // if >0, jump to this panel after loading
}

// newPanelView creates a panel view that will load the given
// dashboard by UID.
func newPanelView(client *grafana.Client, uid string) *panelView {
	return &panelView{
		client:          client,
		uid:             uid,
		dashboard:       nil,
		panels:          nil,
		index:           0,
		data:            nil,
		loading:         true,
		width:           0,
		height:          0,
		timeRange:       grafana.TimeRange{From: "now-1h", To: "now"},
		err:             nil,
		hiddenSeries:    nil,
		variableValues:  nil,
		variableOptions: nil,
		initialPanelID:  0,
	}
}

// Init starts loading the dashboard.
func (p *panelView) Init() tea.Cmd {
	return p.fetchDashboard()
}

// Update handles messages for the panel view.
func (p *panelView) Update(msg tea.Msg) (view, tea.Cmd) {
	switch msg := msg.(type) {
	case dashboardLoadedMsg:
		return p.handleDashboardLoaded(msg)
	case variablesResolvedMsg:
		return p.handleVariablesResolved(msg)
	case panelDataMsg, panelErrorMsg:
		return p.handlePanelResult(msg)
	case setVariablesMsg:
		return p.handleSetVariables(msg)
	case setTimeRangeMsg:
		return p.handleSetTimeRange(msg)
	case popAndFilterMsg:
		return p.handlePopAndFilter(msg)
	case jumpToPanelMsg:
		return p.jumpToPanel(msg.index)
	case tea.KeyMsg:
		return p.handleKeyMsgGuarded(msg)
	default:
		return p, nil
	}
}

func (p *panelView) handlePanelResult(msg tea.Msg) (view, tea.Cmd) {
	switch msg := msg.(type) {
	case panelDataMsg:
		p.data = msg.result
	case panelErrorMsg:
		p.err = msg.err
	}

	p.loading = false

	return p, nil
}

func (p *panelView) handleSetVariables(msg setVariablesMsg) (view, tea.Cmd) {
	p.variableValues = msg.values
	p.loading = true
	p.data = nil
	p.err = nil

	return p, p.fetchPanelData()
}

func (p *panelView) handleSetTimeRange(msg setTimeRangeMsg) (view, tea.Cmd) {
	p.timeRange = msg.timeRange
	p.loading = true
	p.data = nil
	p.err = nil

	return p, p.fetchPanelData()
}

// handleDashboardLoaded processes a loaded dashboard.
func (p *panelView) handleDashboardLoaded(msg dashboardLoadedMsg) (view, tea.Cmd) {
	p.dashboard = msg.dashboard
	p.panels = grafana.FlattenPanels(msg.dashboard.Panels)

	if len(p.panels) == 0 {
		p.loading = false

		return p, nil
	}

	// Jump to the requested panel if an initial panel ID was set.
	if p.initialPanelID > 0 {
		for i, panel := range p.panels {
			if panel.ID == p.initialPanelID {
				p.index = i
				break
			}
		}

		p.initialPanelID = 0
	}

	// Resolve template variable options before fetching panel data.
	return p, p.resolveVariables()
}

// handleVariablesResolved stores resolved variable options and
// defaults, then fetches panel data.
func (p *panelView) handleVariablesResolved(
	msg variablesResolvedMsg,
) (view, tea.Cmd) {
	p.variableOptions = msg.options
	p.variableValues = msg.defaults

	return p, p.fetchPanelData()
}

// resolveVariables returns a command to resolve template variable
// options via the Prometheus API.
func (p *panelView) resolveVariables() tea.Cmd {
	client := p.client
	variables := p.dashboard.Templating.List

	return func() tea.Msg {
		options, defaults, err := client.ResolveVariableOptions(
			contextBackground(), variables,
		)
		if err != nil {
			return panelErrorMsg{err: err}
		}

		return variablesResolvedMsg{
			options:  options,
			defaults: defaults,
		}
	}
}

func (p *panelView) handlePopAndFilter(msg popAndFilterMsg) (view, tea.Cmd) {
	p.hiddenSeries = msg.hidden

	return p, nil
}

// handleKeyMsgGuarded ignores key events while variables are being
// resolved or the dashboard is still loading. Escape is always
// allowed so the user can go back.
func (p *panelView) handleKeyMsgGuarded(msg tea.KeyMsg) (view, tea.Cmd) {
	if msg.String() == keyEsc {
		return p, func() tea.Msg { return popViewMsg{} }
	}

	if p.dashboard == nil || p.variableValues == nil {
		return p, nil
	}

	return p.handleKeyMsg(msg)
}

// handleKeyMsg processes key events for the panel view.
func (p *panelView) handleKeyMsg(msg tea.KeyMsg) (view, tea.Cmd) {
	switch msg.String() {
	case "k", keyUp:
		return p.navigatePanel(-1)
	case "j", keyDown:
		return p.navigatePanel(1)
	case "g":
		return p.openPanelPicker()
	case "t":
		return p.openTimePicker()
	case "v":
		return p.openVariablePicker()
	case "f":
		return p.openSeriesFilter()
	case "r":
		return p, p.fetchPanelData()
	case keyEsc:
		return p, func() tea.Msg { return popViewMsg{} }
	default:
		return p, nil
	}
}

// navigatePanel moves to the next or previous panel.
func (p *panelView) navigatePanel(delta int) (view, tea.Cmd) {
	if len(p.panels) == 0 {
		return p, nil
	}

	p.index = (p.index + delta + len(p.panels)) % len(p.panels)
	p.loading = true
	p.data = nil
	p.err = nil
	p.hiddenSeries = nil

	return p, p.fetchPanelData()
}

// jumpToPanel navigates to a specific panel index.
func (p *panelView) jumpToPanel(index int) (view, tea.Cmd) {
	if index < 0 || index >= len(p.panels) {
		return p, nil
	}

	p.index = index
	p.loading = true
	p.data = nil
	p.err = nil

	return p, p.fetchPanelData()
}

// openPanelPicker pushes the panel picker view.
func (p *panelView) openPanelPicker() (view, tea.Cmd) {
	if len(p.panels) == 0 {
		return p, nil
	}

	picker := newPanelPickerView(p.panels, p.index)

	return p, func() tea.Msg {
		return pushViewMsg{v: picker}
	}
}

// openTimePicker pushes the time range picker view.
func (p *panelView) openTimePicker() (view, tea.Cmd) {
	picker := newTimeRangePickerView(p.timeRange)

	return p, func() tea.Msg {
		return pushViewMsg{v: picker}
	}
}

// openVariablePicker pushes the variable picker view.
func (p *panelView) openVariablePicker() (view, tea.Cmd) {
	if p.dashboard == nil {
		return p, nil
	}

	picker := newVariablePickerView(
		p.dashboard.Templating.List,
		p.variableOptions,
		p.variableValues,
	)

	return p, func() tea.Msg {
		return pushViewMsg{v: picker}
	}
}

// openSeriesFilter pushes the series filter view.
func (p *panelView) openSeriesFilter() (view, tea.Cmd) {
	if p.data == nil {
		return p, nil
	}

	filter := newSeriesFilterView(p.data, p.hiddenSeries)

	return p, func() tea.Msg {
		return pushViewMsg{v: filter}
	}
}

// fetchDashboard returns a command to load the dashboard.
func (p *panelView) fetchDashboard() tea.Cmd {
	client := p.client
	uid := p.uid

	return func() tea.Msg {
		dashboard, err := client.GetDashboard(
			contextBackground(), uid,
		)
		if err != nil {
			return panelErrorMsg{err: err}
		}

		return dashboardLoadedMsg{dashboard: dashboard}
	}
}

// fetchPanelData returns a command to load the current panel's data.
func (p *panelView) fetchPanelData() tea.Cmd {
	if len(p.panels) == 0 {
		return nil
	}

	client := p.client
	panel := p.panels[p.index]
	allPanels := p.panels
	timeRange := p.timeRange
	variables := p.dashboard.Templating.List
	variableOverrides := p.variableValues
	width := p.width

	if width <= 0 {
		width = 80 //nolint:mnd // sensible default
	}

	p.loading = true

	return func() tea.Msg {
		result, err := client.QueryPanel(
			contextBackground(), panel, timeRange, width,
			variables, variableOverrides, allPanels,
		)
		if err != nil {
			return panelErrorMsg{err: err}
		}

		return panelDataMsg{result: result}
	}
}

// View renders the panel view.
func (p *panelView) View(width, height int) string {
	p.width = width
	p.height = height

	if p.dashboard == nil {
		if p.err != nil {
			return centerText("Error: "+p.err.Error(), width, height)
		}

		return centerText("Loading dashboard...", width, height)
	}

	if len(p.panels) == 0 {
		return centerText("No panels found", width, height)
	}

	titleBar := p.renderTitleBar(width)
	contentHeight := max(height-2, 0) //nolint:mnd // title + blank line
	content := p.renderContent(width, contentHeight)

	return titleBar + "\n" + content
}

// renderTitleBar renders the panel title and index.
func (p *panelView) renderTitleBar(width int) string {
	panel := p.panels[p.index]

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.AdaptiveColor{
			Light: "#333333",
			Dark:  "#EEEEEE",
		})

	panelURL := fmt.Sprintf(
		"%s/d/%s?viewPanel=%d",
		p.client.BaseURL(), p.uid, panel.ID,
	)

	title := fmt.Sprintf(
		" %s  (%d/%d)  %s",
		panel.Title, p.index+1, len(p.panels), panelURL,
	)

	return titleStyle.Render(
		lipgloss.NewStyle().Width(width).Render(title),
	)
}

// renderContent renders either loading state, error, or panel data.
func (p *panelView) renderContent(width, height int) (rendered string) {
	defer func() {
		if r := recover(); r != nil {
			rendered = centerText(
				fmt.Sprintf("render error: %v", r), width, height,
			)
		}
	}()

	if p.loading {
		return centerText("Loading panel data...", width, height)
	}

	if p.err != nil {
		return centerText("Error: "+p.err.Error(), width, height)
	}

	if p.data == nil {
		return centerText("No data", width, height)
	}

	panel := p.panels[p.index]
	renderer := RendererForPanel(panel.Type)

	return renderer.Render(p.data, panel, width, height, p.hiddenSeries)
}

// Title returns the view title for breadcrumbs.
func (p *panelView) Title() string {
	if p.dashboard == nil {
		return "Loading..."
	}

	if len(p.panels) == 0 {
		return p.dashboard.Title
	}

	return fmt.Sprintf(
		"%s > Panel %d/%d",
		p.dashboard.Title, p.index+1, len(p.panels),
	)
}

// IsFiltering always returns false; panelView has no filtering state.
func (p *panelView) IsFiltering() bool { return false }

// KeyBindings returns the key bindings for the help bar.
func (p *panelView) KeyBindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(
			key.WithKeys("k", keyUp),
			key.WithHelp("j/k", "next/prev"),
		),
		key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "jump"),
		),
		key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "time"),
		),
		key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "vars"),
		),
		key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "filter"),
		),
		key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		key.NewBinding(
			key.WithKeys(keyEsc),
			key.WithHelp(keyEsc, "back"),
		),
	}
}

// centerText centers text within the given dimensions.
func centerText(text string, width, height int) string {
	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		text,
	)
}

// padContent pads the content to exactly width x height, inserting an
// ANSI reset (\x1b[0m) before padding on each line. This prevents
// background colors (e.g. from heatmap cells) from bleeding into the
// padding spaces or subsequent UI elements.
func padContent(content string, width, height int) string {
	lines := strings.Split(content, "\n")

	// Pad or truncate to the target height.
	for len(lines) < height {
		lines = append(lines, "")
	}

	lines = lines[:height]

	for i, line := range lines {
		// Use ANSI-aware width to compute how much padding is needed.
		lineWidth := lipgloss.Width(line)
		if lineWidth < width {
			// Reset ANSI state before adding padding spaces.
			lines[i] = line + "\x1b[0m" + strings.Repeat(" ", width-lineWidth)
		}
	}

	return strings.Join(lines, "\n")
}
