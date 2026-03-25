package grafanatui

import (
	"fmt"
	"maps"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lovromazgon/grafana-tui/grafana"
)

const allOptionValue = "$__all"

// variablePickerView lets the user view and change dashboard
// template variable values. It has two modes: variable list and
// option selection.
type variablePickerView struct {
	variables []grafana.TemplateVariable
	options   map[string][]string
	values    map[string]string

	// Navigation.
	varCursor int
	optCursor int
	editing   bool

	// Scroll state.
	varOffset int
	optOffset int
	height    int
}

// newVariablePickerView creates a variable picker from the dashboard
// variables, their resolved options, and current values.
func newVariablePickerView(
	variables []grafana.TemplateVariable,
	options map[string][]string,
	values map[string]string,
) *variablePickerView {
	// Copy values so we don't mutate the caller's map.
	valuesCopy := make(map[string]string, len(values))
	maps.Copy(valuesCopy, values)

	return &variablePickerView{
		variables: variables,
		options:   options,
		values:    valuesCopy,
		varCursor: 0,
		optCursor: 0,
		editing:   false,
		varOffset: 0,
		optOffset: 0,
		height:    20, //nolint:mnd // updated by View
	}
}

func (v *variablePickerView) Init() tea.Cmd { return nil }

func (v *variablePickerView) Update(msg tea.Msg) (view, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if v.editing {
			return v.handleOptionKeyMsg(msg)
		}

		return v.handleVariableKeyMsg(msg)
	default:
		return v, nil
	}
}

// handleVariableKeyMsg handles keys in the variable list mode.
func (v *variablePickerView) handleVariableKeyMsg(
	msg tea.KeyMsg,
) (view, tea.Cmd) {
	switch msg.String() {
	case "j", keyDown:
		if v.varCursor < len(v.variables)-1 {
			v.varCursor++

			if v.varCursor-v.varOffset >= v.height {
				v.varOffset++
			}
		}
	case "k", keyUp:
		if v.varCursor > 0 {
			v.varCursor--

			if v.varCursor < v.varOffset {
				v.varOffset = v.varCursor
			}
		}
	case keyEnter:
		return v.enterEditMode()
	case keyEsc:
		return v, func() tea.Msg {
			return popAndSetVariablesMsg{values: v.values}
		}
	}

	return v, nil
}

// enterEditMode switches to option selection for the current
// variable.
func (v *variablePickerView) enterEditMode() (view, tea.Cmd) {
	if v.varCursor >= len(v.variables) {
		return v, nil
	}

	varName := v.variables[v.varCursor].Name
	opts := v.options[varName]

	if len(opts) == 0 {
		return v, nil
	}

	// Pre-select the current value.
	v.optCursor = 0
	v.optOffset = 0

	currentVal := v.values[varName]

	for i, opt := range opts {
		if opt == currentVal {
			v.optCursor = i

			break
		}
	}

	// Include "All" option at position 0 if the variable supports
	// it.
	if v.variables[v.varCursor].IncludeAll {
		if currentVal == ".*" || currentVal == v.variables[v.varCursor].AllValue {
			v.optCursor = 0
		} else {
			v.optCursor++ // shift by 1 for the "All" entry
		}
	}

	v.editing = true

	return v, nil
}

// handleOptionKeyMsg handles keys in the option selection mode.
func (v *variablePickerView) handleOptionKeyMsg(
	msg tea.KeyMsg,
) (view, tea.Cmd) {
	opts := v.currentOptions()

	switch msg.String() {
	case "j", keyDown:
		if v.optCursor < len(opts)-1 {
			v.optCursor++

			if v.optCursor-v.optOffset >= v.height {
				v.optOffset++
			}
		}
	case "k", keyUp:
		if v.optCursor > 0 {
			v.optCursor--

			if v.optCursor < v.optOffset {
				v.optOffset = v.optCursor
			}
		}
	case keyEnter:
		return v.selectOption(opts)
	case keyEsc:
		v.editing = false
		v.optCursor = 0
		v.optOffset = 0
	}

	return v, nil
}

// currentOptions returns the options for the currently selected
// variable, prepending "All" if the variable supports it.
func (v *variablePickerView) currentOptions() []string {
	if v.varCursor >= len(v.variables) {
		return nil
	}

	variable := v.variables[v.varCursor]
	opts := v.options[variable.Name]

	if variable.IncludeAll {
		return append([]string{allOptionValue}, opts...)
	}

	return opts
}

// selectOption applies the selected option and returns to variable
// list mode.
func (v *variablePickerView) selectOption(
	opts []string,
) (view, tea.Cmd) {
	if v.optCursor >= len(opts) {
		return v, nil
	}

	variable := v.variables[v.varCursor]
	selected := opts[v.optCursor]

	if selected == allOptionValue {
		if variable.AllValue != "" {
			v.values[variable.Name] = variable.AllValue
		} else {
			v.values[variable.Name] = ".*"
		}
	} else {
		v.values[variable.Name] = selected
	}

	v.editing = false
	v.optCursor = 0
	v.optOffset = 0

	return v, nil
}

func (v *variablePickerView) View(width, height int) string {
	v.height = max(height-2, 1) //nolint:mnd // leave room for title

	if v.editing {
		return v.renderOptions(width)
	}

	return v.renderVariables(width)
}

// renderVariables renders the variable list.
func (v *variablePickerView) renderVariables(_ int) string {
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

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{
			Light: "#0066CC",
			Dark:  "#66BBFF",
		})

	lines := []string{
		titleStyle.Render("  Variables (enter: edit, esc: apply)"),
	}

	end := min(v.varOffset+v.height, len(v.variables))

	for i := v.varOffset; i < end; i++ {
		variable := v.variables[i]
		val := v.values[variable.Name]

		if val == "" {
			val = "(empty)"
		}

		var line string

		if i == v.varCursor {
			line = fmt.Sprintf(
				"  %s = %s",
				cursorStyle.Render(variable.Name),
				valueStyle.Render(val),
			)
		} else {
			line = fmt.Sprintf(
				"  %s = %s",
				dimStyle.Render(variable.Name),
				valueStyle.Render(val),
			)
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// renderOptions renders the option list for the selected variable.
func (v *variablePickerView) renderOptions(_ int) string {
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

	variable := v.variables[v.varCursor]
	opts := v.currentOptions()

	lines := []string{
		titleStyle.Render(
			fmt.Sprintf("  %s (enter: select, esc: back)", variable.Name),
		),
	}

	end := min(v.optOffset+v.height, len(opts))

	for i := v.optOffset; i < end; i++ {
		opt := opts[i]
		display := opt

		if opt == allOptionValue {
			display = "All"
		}

		currentVal := v.values[variable.Name]
		isSelected := opt == currentVal ||
			(opt == allOptionValue && (currentVal == ".*" || currentVal == variable.AllValue))

		prefix := "  "
		if isSelected {
			prefix = "* "
		}

		line := fmt.Sprintf("  %s%s", prefix, display)

		if i == v.optCursor {
			lines = append(lines, cursorStyle.Render(line))
		} else {
			lines = append(lines, dimStyle.Render(line))
		}
	}

	return strings.Join(lines, "\n")
}

func (v *variablePickerView) Title() string {
	if v.editing && v.varCursor < len(v.variables) {
		return "Variables > " + v.variables[v.varCursor].Name
	}

	return "Variables"
}

// IsFiltering always returns false; variablePickerView has no list filter state.
func (v *variablePickerView) IsFiltering() bool { return false }

func (v *variablePickerView) KeyBindings() []key.Binding {
	if v.editing {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("j", "k"),
				key.WithHelp("j/k", "navigate"),
			),
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

	return []key.Binding{
		key.NewBinding(
			key.WithKeys("j", "k"),
			key.WithHelp("j/k", "navigate"),
		),
		key.NewBinding(
			key.WithKeys(keyEnter),
			key.WithHelp(keyEnter, "edit"),
		),
		key.NewBinding(
			key.WithKeys(keyEsc),
			key.WithHelp(keyEsc, "apply"),
		),
	}
}
