package grafanatui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lovromazgon/grafana-tui/grafana"
)

// timePreset defines a named time range preset.
type timePreset struct {
	label string
	from  string
	to    string
}

func (t timePreset) Title() string       { return t.label }
func (t timePreset) Description() string { return t.from + " to " + t.to }
func (t timePreset) FilterValue() string { return t.label }

// getTimePresets returns the available quick-select time ranges.
func getTimePresets() []timePreset {
	return []timePreset{
		{label: "Last 5 minutes", from: "now-5m", to: "now"},
		{label: "Last 15 minutes", from: "now-15m", to: "now"},
		{label: "Last 1 hour", from: "now-1h", to: "now"},
		{label: "Last 6 hours", from: "now-6h", to: "now"},
		{label: "Last 24 hours", from: "now-24h", to: "now"},
		{label: "Last 7 days", from: "now-7d", to: "now"},
	}
}

// timeRangePickerView shows a list of time range presets.
type timeRangePickerView struct {
	list list.Model
}

// newTimeRangePickerView creates a time range picker with the
// current time range pre-selected.
func newTimeRangePickerView(current grafana.TimeRange) *timeRangePickerView {
	presets := getTimePresets()
	items := make([]list.Item, len(presets))

	selectedIdx := 0

	for i, preset := range presets {
		items[i] = preset

		if preset.from == current.From && preset.to == current.To {
			selectedIdx = i
		}
	}

	delegate := list.NewDefaultDelegate()
	listModel := list.New(items, delegate, 0, 0)
	listModel.Title = "Time Range"
	listModel.SetFilteringEnabled(false)
	listModel.Select(selectedIdx)

	return &timeRangePickerView{list: listModel}
}

// Init is a no-op for the time range picker.
func (t *timeRangePickerView) Init() tea.Cmd { return nil }

// Update handles messages for the time range picker.
func (t *timeRangePickerView) Update(msg tea.Msg) (view, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return t.handleKeyMsg(msg)
	default:
		var cmd tea.Cmd
		t.list, cmd = t.list.Update(msg)

		return t, cmd
	}
}

// handleKeyMsg processes key events for the time range picker.
func (t *timeRangePickerView) handleKeyMsg(msg tea.KeyMsg) (view, tea.Cmd) {
	switch msg.String() {
	case keyEnter:
		return t.selectTimeRange()
	case keyEsc:
		return t, func() tea.Msg { return popViewMsg{} }
	default:
		var cmd tea.Cmd
		t.list, cmd = t.list.Update(msg)

		return t, cmd
	}
}

// selectTimeRange pops this view and sends the selected time range.
// We use popAndSetTimeRangeMsg so the app can pop first, then
// forward the time range to the now-active view.
func (t *timeRangePickerView) selectTimeRange() (view, tea.Cmd) {
	selected, ok := t.list.SelectedItem().(timePreset)
	if !ok {
		return t, nil
	}

	timeRange := grafana.TimeRange{
		From: selected.from,
		To:   selected.to,
	}

	return t, func() tea.Msg {
		return popAndSetTimeRangeMsg{timeRange: timeRange}
	}
}

// View renders the time range picker.
func (t *timeRangePickerView) View(width, height int) string {
	t.list.SetSize(width, height)

	return t.list.View()
}

// Title returns the view title for breadcrumbs.
func (t *timeRangePickerView) Title() string {
	return "Time Range"
}

// IsFiltering always returns false; timeRangePickerView has no filter state.
func (t *timeRangePickerView) IsFiltering() bool { return false }

// KeyBindings returns the key bindings for the help bar.
func (t *timeRangePickerView) KeyBindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(
			key.WithKeys(keyEnter),
			key.WithHelp(keyEnter, "select"),
		),
		key.NewBinding(
			key.WithKeys(keyEsc),
			key.WithHelp(keyEsc, "back"),
		),
	}
}
