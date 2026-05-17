package tui

import (
	"fmt"
	"strings"

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

	history         []string
	suggestionIndex int
	status          string
}

func newSearchModel(history []string) searchModel {
	in := textinput.New()
	in.Placeholder = "Search manga..."
	in.Focus()
	in.CharLimit = constant.CharLimit
	in.Width = constant.InputWidth
	in.PromptStyle = lipgloss.NewStyle().Foreground(constant.LogoColor)
	in.TextStyle = lipgloss.NewStyle().Foreground(constant.TextColor)
	in.PlaceholderStyle = lipgloss.NewStyle().Foreground(constant.MutedColor)

	return searchModel{
		input:           in,
		keys:            newSearchKeyMap(),
		history:         cleanSearchHistory(history),
		suggestionIndex: -1,
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
			query := strings.TrimSpace(m.input.Value())
			if query == "" {
				return m, nil
			}

			m.status = ""
			return m, func() tea.Msg {
				return searchSubmittedMsg{Query: query}
			}

		case key.Matches(msg, m.keys.Clear):
			m.input.SetValue("")
			m.suggestionIndex = -1
			return m, nil

		case key.Matches(msg, m.keys.Complete):
			if suggestion, ok := m.currentSuggestion(); ok {
				m.input.SetValue(suggestion)
				m.input.CursorEnd()
			}
			return m, nil

		case key.Matches(msg, m.keys.Previous):
			m.moveSuggestion(-1)
			return m, nil

		case key.Matches(msg, m.keys.Next):
			m.moveSuggestion(1)
			return m, nil
		}

	}

	before := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != before {
		m.suggestionIndex = -1
	}
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

	lines := []string{
		logo,
		"",
		inputBox,
	}

	if suggestion := m.virtualSuggestionText(); suggestion != "" {
		suggestionText := lipgloss.NewStyle().
			Foreground(constant.MutedColor).
			Render(fmt.Sprintf("Search '%s'?", suggestion))

		acceptHint := lipgloss.NewStyle().
			Foreground(constant.MutedColor).
			Render("Press tab to accept")

		lines = append(lines,
			suggestionText,
			acceptHint,
		)
	}

	if strings.TrimSpace(m.status) != "" {
		statusText := lipgloss.NewStyle().
			Foreground(constant.MutedColor).
			Render(m.status)
		lines = append(lines, "", statusText)
	}

	inner := lipgloss.JoinVertical(
		lipgloss.Center,
		lines...,
	)

	panel := lipgloss.NewStyle().
		Padding(5, 7).
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

func (m *searchModel) SetHistory(history []string) {
	m.history = cleanSearchHistory(history)
	m.suggestionIndex = -1
}

func (m *searchModel) setStatus(status string) {
	m.status = strings.TrimSpace(status)
}

func (m searchModel) matchingSuggestions() []string {
	prefix := strings.ToLower(strings.TrimSpace(m.input.Value()))
	if prefix == "" {
		return m.history
	}

	matches := make([]string, 0, len(m.history))
	for _, query := range m.history {
		if strings.HasPrefix(strings.ToLower(query), prefix) {
			matches = append(matches, query)
		}
	}
	return matches
}

func (m searchModel) currentSuggestion() (string, bool) {
	matches := m.matchingSuggestions()
	if len(matches) == 0 {
		return "", false
	}
	idx := m.suggestionIndex
	if idx < 0 || idx >= len(matches) {
		idx = 0
	}
	return matches[idx], true
}

func (m *searchModel) moveSuggestion(delta int) {
	matches := m.matchingSuggestions()
	if len(matches) == 0 {
		m.suggestionIndex = -1
		return
	}
	if m.suggestionIndex < 0 || m.suggestionIndex >= len(matches) {
		if delta > 0 {
			m.suggestionIndex = 1 % len(matches)
		} else {
			m.suggestionIndex = len(matches) - 1
		}
	} else {
		m.suggestionIndex = (m.suggestionIndex + delta + len(matches)) % len(matches)
	}
}

func (m searchModel) virtualSuggestionText() string {
	typed := strings.TrimSpace(m.input.Value())
	if typed == "" {
		return ""
	}

	suggestion, ok := m.currentSuggestion()
	if !ok {
		return ""
	}

	if !strings.HasPrefix(strings.ToLower(suggestion), strings.ToLower(typed)) {
		return ""
	}

	if strings.EqualFold(suggestion, typed) {
		return ""
	}

	return suggestion
}

func cleanSearchHistory(history []string) []string {
	result := make([]string, 0, len(history))
	seen := make(map[string]struct{}, len(history))
	for _, query := range history {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}
		key := strings.ToLower(query)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, query)
	}
	return result
}
