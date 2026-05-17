package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type loadingModel struct {
	width  int
	height int

	title   string
	detail  string
	spinner spinner.Model
}

func newLoadingModel(title, detail string) loadingModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(logoColor)

	return loadingModel{
		title:   title,
		detail:  detail,
		spinner: s,
	}
}

func (m *loadingModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m loadingModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m loadingModel) Update(msg tea.Msg) (loadingModel, tea.Cmd) {
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m loadingModel) View() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(logoColor).
		Render(m.title)

	body := lipgloss.NewStyle().
		Foreground(textColor).
		Render(fmt.Sprintf("%s %q", m.spinner.View(), m.detail))

	panel := lipgloss.NewStyle().
		Padding(1, 3).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(outerBorderColor).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Center,
				title,
				"",
				body,
			),
		)

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

type loadingHelpKeyMap struct {
	global keyMap
}

func (m loadingModel) HelpKeys(global keyMap) help.KeyMap {
	return loadingHelpKeyMap{global: global}
}
