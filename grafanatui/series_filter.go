package grafanatui

import (
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lovromazgon/grafana-tui/grafana"
)

// seriesFilterView presents a checklist of series names for toggling
// visibility.
type seriesFilterView struct {
	names  []string
	hidden map[string]bool
	cursor int
	offset int
	height int
}

// newSeriesFilterView creates a filter view from the current query
// result and the existing hidden set.
func newSeriesFilterView(
	result *grafana.QueryResult,
	hidden map[string]bool,
) *seriesFilterView {
	names := collectSeriesNames(result)

	// Copy hidden map so toggling doesn't mutate the caller's map.
	hiddenCopy := make(map[string]bool, len(hidden))
	maps.Copy(hiddenCopy, hidden)

	return &seriesFilterView{
		names:  names,
		hidden: hiddenCopy,
		cursor: 0,
		offset: 0,
		height: 20, //nolint:mnd // will be updated by View
	}
}

// collectSeriesNames gathers all unique series names from a query
// result. It checks both time series frames and categorized frames
// (where a string field provides category labels).
func collectSeriesNames(result *grafana.QueryResult) []string {
	seen := make(map[string]bool)

	for _, rd := range result.Results {
		for i := range rd.Frames {
			frame := &rd.Frames[i]
			collectTimeSeriesNames(frame, seen)
			collectCategoryNames(frame, seen)
		}
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

func collectTimeSeriesNames(
	frame *grafana.DataFrame, seen map[string]bool,
) {
	_, series, err := frame.TimeSeries()
	if err != nil {
		return
	}

	for name := range series {
		seen[name] = true
	}
}

func collectCategoryNames(
	frame *grafana.DataFrame, seen map[string]bool,
) {
	categories, _, err := frame.CategorizedValues()
	if err != nil {
		return
	}

	for _, name := range categories {
		seen[name] = true
	}
}

func (s *seriesFilterView) Init() tea.Cmd { return nil }

func (s *seriesFilterView) Update(msg tea.Msg) (view, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return s.handleKeyMsg(msg)
	default:
		return s, nil
	}
}

func (s *seriesFilterView) handleKeyMsg(msg tea.KeyMsg) (view, tea.Cmd) {
	switch msg.String() {
	case "j", keyDown:
		s.moveCursor(1)
	case "k", keyUp:
		s.moveCursor(-1)
	case " ", keyEnter:
		s.toggleCurrent()
	case "a":
		s.toggleAll()
	case keyEsc:
		return s, s.applyFilter()
	default:
		return s, nil
	}

	return s, nil
}

func (s *seriesFilterView) moveCursor(delta int) {
	s.cursor += delta
	s.cursor = max(0, min(s.cursor, len(s.names)-1))

	if s.cursor-s.offset >= s.height {
		s.offset = s.cursor - s.height + 1
	}

	if s.cursor < s.offset {
		s.offset = s.cursor
	}
}

func (s *seriesFilterView) toggleCurrent() {
	if s.cursor < len(s.names) {
		name := s.names[s.cursor]
		s.hidden[name] = !s.hidden[name]
	}
}

func (s *seriesFilterView) toggleAll() {
	anyHidden := false

	for _, name := range s.names {
		if s.hidden[name] {
			anyHidden = true

			break
		}
	}

	for _, name := range s.names {
		s.hidden[name] = !anyHidden
	}
}

func (s *seriesFilterView) applyFilter() tea.Cmd {
	cleaned := make(map[string]bool)

	for k, v := range s.hidden {
		if v {
			cleaned[k] = true
		}
	}

	return func() tea.Msg {
		return popAndFilterMsg{hidden: cleaned}
	}
}

func (s *seriesFilterView) View(_, height int) string {
	s.height = max(height-2, 1) //nolint:mnd // leave room for title

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.AdaptiveColor{
			Light: "#333333",
			Dark:  "#EEEEEE",
		})

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{
			Light: "#999999",
			Dark:  "#777777",
		})

	cursorStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.AdaptiveColor{
			Light: "#000000",
			Dark:  "#FFFFFF",
		})

	lines := []string{titleStyle.Render("  Filter Series (space: toggle, a: all, esc: apply)")}

	end := min(s.offset+s.height, len(s.names))

	for i := s.offset; i < end; i++ {
		name := s.names[i]
		check := "[x]"

		if s.hidden[name] {
			check = "[ ]"
		}

		line := fmt.Sprintf("  %s %s", check, name)

		if i == s.cursor {
			lines = append(lines, cursorStyle.Render(line))
		} else {
			lines = append(lines, dimStyle.Render(line))
		}
	}

	return strings.Join(lines, "\n")
}

func (s *seriesFilterView) Title() string {
	return "Filter Series"
}

// IsFiltering always returns false; seriesFilterView has no list filter state.
func (s *seriesFilterView) IsFiltering() bool { return false }

func (s *seriesFilterView) KeyBindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(
			key.WithKeys("j", "k"),
			key.WithHelp("j/k", "navigate"),
		),
		key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle"),
		),
		key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "toggle all"),
		),
		key.NewBinding(
			key.WithKeys(keyEsc),
			key.WithHelp(keyEsc, "apply"),
		),
	}
}
