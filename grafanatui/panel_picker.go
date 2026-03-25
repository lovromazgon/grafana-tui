package grafanatui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lovromazgon/grafana-tui/grafana"
)

// panelPickerItem adapts a panel for the picker list.
type panelPickerItem struct {
	index int
	panel grafana.Panel
}

func (p panelPickerItem) Title() string {
	return fmt.Sprintf("%d. %s", p.index+1, p.panel.Title)
}

func (p panelPickerItem) Description() string {
	return p.panel.Type
}

func (p panelPickerItem) FilterValue() string {
	return p.panel.Title
}

// panelPickerView shows a numbered list of panels for quick jumping.
type panelPickerView struct {
	list list.Model
}

// newPanelPickerView creates a panel picker from the given panels.
func newPanelPickerView(panels []grafana.Panel, currentIndex int) *panelPickerView {
	items := make([]list.Item, len(panels))

	for i, panel := range panels {
		items[i] = panelPickerItem{index: i, panel: panel}
	}

	delegate := list.NewDefaultDelegate()
	listModel := list.New(items, delegate, 0, 0)
	listModel.Title = "Jump to Panel"
	listModel.SetFilteringEnabled(true)
	listModel.Select(currentIndex)

	return &panelPickerView{list: listModel}
}

// Init is a no-op for the panel picker.
func (p *panelPickerView) Init() tea.Cmd { return nil }

// Update handles messages for the panel picker.
func (p *panelPickerView) Update(msg tea.Msg) (view, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return p.handleKeyMsg(msg)
	default:
		var cmd tea.Cmd
		p.list, cmd = p.list.Update(msg)

		return p, cmd
	}
}

// handleKeyMsg processes key events for the panel picker.
func (p *panelPickerView) handleKeyMsg(msg tea.KeyMsg) (view, tea.Cmd) {
	switch msg.String() {
	case keyEnter:
		if p.list.SettingFilter() {
			var cmd tea.Cmd
			p.list, cmd = p.list.Update(msg)

			return p, cmd
		}

		return p.selectPanel()
	case keyEsc:
		if p.list.SettingFilter() {
			var cmd tea.Cmd
			p.list, cmd = p.list.Update(msg)

			return p, cmd
		}

		return p, func() tea.Msg { return popViewMsg{} }
	default:
		var cmd tea.Cmd
		p.list, cmd = p.list.Update(msg)

		return p, cmd
	}
}

// selectPanel returns commands to pop this view and jump to the
// selected panel.
func (p *panelPickerView) selectPanel() (view, tea.Cmd) {
	selected, ok := p.list.SelectedItem().(panelPickerItem)
	if !ok {
		return p, nil
	}

	index := selected.index

	// Send popViewMsg first, then jumpToPanelMsg after the pop is
	// processed. Using tea.Sequence ensures ordering so the jump
	// message reaches the panelView (not this picker).
	return p, tea.Sequence(
		func() tea.Msg { return popViewMsg{} },
		func() tea.Msg { return jumpToPanelMsg{index: index} },
	)
}

// View renders the panel picker.
func (p *panelPickerView) View(width, height int) string {
	p.list.SetSize(width, height)

	return p.list.View()
}

// Title returns the view title for breadcrumbs.
func (p *panelPickerView) Title() string {
	return "Jump to Panel"
}

// IsFiltering reports whether the list is in an active filter state.
func (p *panelPickerView) IsFiltering() bool {
	return p.list.SettingFilter()
}

// KeyBindings returns the key bindings for the help bar.
func (p *panelPickerView) KeyBindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(
			key.WithKeys(keyEnter),
			key.WithHelp(keyEnter, "select"),
		),
		key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		key.NewBinding(
			key.WithKeys(keyEsc),
			key.WithHelp(keyEsc, "back"),
		),
	}
}
