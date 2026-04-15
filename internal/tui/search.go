package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evgen2571/mangate/internal/constant"
)

type searchModel struct {
	width  int
	height int

	input textinput.Model
	keys  searchKeyMap
}

func newSearchModel() searchModel {
	in := textinput.New()
	in.Placeholder = "Search manga..."
	in.Focus()
	in.CharLimit = constant.CharLimit
	in.Width = constant.InputWidth
	in.PromptStyle = lipgloss.NewStyle().Foreground(constant.LogoColor)
	in.TextStyle = lipgloss.NewStyle().Foreground(constant.TextColor)
	in.PlaceholderStyle = lipgloss.NewStyle().Foreground(constant.MutedColor)

	return searchModel{
		input: in,
		keys:  newSearchKeyMap(),
	}
}

func (m searchModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *searchModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m searchModel) Update(msg tea.Msg) (searchModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Submit):
			// TODO: Process search
			return m, nil

		case key.Matches(msg, m.keys.Clear):
			m.input.SetValue("")
			return m, nil
		}

	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m searchModel) View() string {
	logo := lipgloss.NewStyle().
		Bold(true).
		Foreground(constant.LogoColor).
		Render(constant.AsciiLogo)

	inputBox := lipgloss.NewStyle().
		Width(constant.InputWidth).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(constant.InputBorderColor).
		Render(m.input.View())

	inner := lipgloss.JoinVertical(
		lipgloss.Center,
		logo,
		"",
		inputBox,
	)

	panel := lipgloss.NewStyle().
		Padding(1, 3).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(constant.OuterBorderColor).
		Render(inner)

	if m.width == 0 || m.height == 0 {
		return panel
	}

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		panel,
	)
}

func (m searchModel) HelpKeys(global keyMap) searchHelpKeyMap {
	return searchHelpKeyMap{
		global: global,
		local:  m.keys,
	}
}
