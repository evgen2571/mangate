package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evgen2571/mangate/internal/app"
)

type state int

const (
	stateSearch state = iota
	stateLoading
	stateResults
)

type model struct {
	app *app.App

	state  state
	width  int
	height int

	keys keyMap
	help help.Model

	search  searchModel
	loading loadingModel
	results resultsModel
}

func New(a *app.App) tea.Model {
	h := help.New()
	h.ShowAll = false

	return &model{
		app:    a,
		state:  stateSearch,
		keys:   newKeyMap(),
		help:   h,
		search: newSearchModel(),
	}
}

func (m model) Init() tea.Cmd {
	return m.search.Init()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeActiveModel()
		return m, nil

	case searchSubmittedMsg:
		m.loading = newLoadingModel(msg.Query)
		m.state = stateLoading
		m.resizeActiveModel()
		return m, m.searchMangaCmd(msg.Query)

	case searchSucceededMsg:
		m.results = newResultsModel(msg.Query, msg.Results)
		m.state = stateResults
		m.resizeActiveModel()
		return m, nil

	case searchFailedMsg:
		m.state = stateSearch
		m.resizeActiveModel()
		return m, nil

	case goBackMsg:
		m.state = stateSearch
		m.resizeActiveModel()
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
			m.resizeActiveModel()
			return m, nil
		}
	}

	switch m.state {
	case stateSearch:
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		return m, cmd

	case stateLoading:
		var cmd tea.Cmd
		m.loading, cmd = m.loading.Update(msg)
		return m, cmd

	case stateResults:
		var cmd tea.Cmd
		m.results, cmd = m.results.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	var body string

	switch m.state {
	case stateSearch:
		body = m.search.View()
	case stateLoading:
		body = m.loading.View()
	case stateResults:
		body = m.results.View()
	default:
		body = ""
	}

	helpView := m.help.View(m.currentHelp())

	return lipgloss.JoinVertical(
		lipgloss.Left,
		body,
		"",
		helpView,
	)
}

func (m model) currentHelp() help.KeyMap {
	switch m.state {
	case stateSearch:
		return m.search.HelpKeys(m.keys)
	case stateLoading:
		return m.loading.HelpKeys(m.keys)
	case stateResults:
		return m.results.HelpKeys(m.keys)
	default:
		return m.search.HelpKeys(m.keys)
	}
}

func (m *model) resizeActiveModel() {
	if m.width == 0 || m.height == 0 {
		return
	}

	helpView := m.help.View(m.currentHelp())
	helpHeight := lipgloss.Height(helpView)

	// one blank line between body and help
	bodyHeight := max(1, m.height-helpHeight-1)

	switch m.state {
	case stateSearch:
		m.search.SetSize(m.width, bodyHeight)
	case stateLoading:
		m.loading.SetSize(m.width, bodyHeight)
	case stateResults:
		m.results.SetSize(m.width, bodyHeight)
	}
}
