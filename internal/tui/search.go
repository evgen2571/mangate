package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/evgen2571/manga-downloader/internal/providers"
)

type searchKeyMap struct {
	Submit key.Binding
	Clear  key.Binding
	Quit   key.Binding
}

func newSearchKeyMap() searchKeyMap {
	return searchKeyMap{
		Submit: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "search"),
		),
		Clear: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "clear"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

type searchModel struct {
	input   textinput.Model
	keys    searchKeyMap
	width   int
	height  int
	loading bool
	err     error
	styles  uiStyles
	logo    string
}

func newSearchModel() searchModel {
	input := textinput.New()
	input.Placeholder = "Search manga..."
	input.Focus()
	input.CharLimit = 120
	input.Width = 46

	return searchModel{
		input:  input,
		keys:   newSearchKeyMap(),
		styles: newUIStyles(),
		logo:   searchLogo,
	}
}

func (m searchModel) Init() tea.Cmd {
	return textinput.Blink
}

func searchMangaCmd(query string) tea.Cmd {
	return func() tea.Msg {
		mangas, err := providers.Provider.GetManga(query)
		return mangasLoadedMsg{
			items: mangas,
			err:   err,
			query: query,
		}
	}
}

func (m searchModel) Update(msg tea.Msg) (searchModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Clear):
			if m.loading {
				return m, nil
			}
			m.input.SetValue("")
			m.err = nil
			return m, nil

		case key.Matches(msg, m.keys.Submit):
			if m.loading {
				return m, nil
			}

			query := strings.TrimSpace(m.input.Value())
			if query == "" {
				m.err = nil
				return m, nil
			}

			m.loading = true
			m.err = nil
			return m, searchMangaCmd(query)
		}
	}

	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m searchModel) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	maxCardW := min(58, m.width-4)
	if maxCardW < 24 {
		maxCardW = 24
	}

	cardFrameW, _ := m.styles.Card.GetFrameSize()
	inputFrameW, _ := m.styles.InputBox.GetFrameSize()

	inputW := max(10, maxCardW-cardFrameW-inputFrameW-2)
	m.input.Width = inputW

	logo := m.styles.Logo.Render(m.logo)
	title := m.styles.Title.Render("Find your next manga")
	subtitle := m.styles.Subtitle.Render("Search by title and press Enter")

	input := m.styles.InputBox.Width(inputW).Render(m.input.View())

	var state string
	switch {
	case m.loading:
		state = m.styles.Status.Render("Searching...")
	case m.err != nil:
		state = m.styles.Error.Render("Error: " + m.err.Error())
	default:
		state = m.styles.Status.Render(" ")
	}

	cardContent := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		subtitle,
		m.styles.InputWrap.Render(input),
		state,
	)

	card := m.styles.Card.Width(max(1, maxCardW-cardFrameW)).Render(cardContent)
	footer := m.styles.Hint.Render("Enter search • Esc clear • q quit")

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		logo,
		card,
		footer,
	)

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		m.styles.App.Render(content),
	)
}
