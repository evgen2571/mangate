package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

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
}

func newSearchModel() searchModel {
	input := textinput.New()
	input.Placeholder = "Enter manga title..."
	input.Focus()
	input.CharLimit = 120
	input.Width = 40

	return searchModel{
		input: input,
		keys:  newSearchKeyMap(),
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
	s := "Manga Downloader\n"
	s += "================\n\n"
	s += "Search manga by title:\n"
	s += m.input.View() + "\n\n"

	if m.loading {
		s += "Searching...\n\n"
	} else if m.err != nil {
		s += "Error: " + m.err.Error() + "\n\n"
	}

	s += helpLine(
		m.keys.Submit.Help().Key+": "+m.keys.Submit.Help().Desc,
		m.keys.Clear.Help().Key+": "+m.keys.Clear.Help().Desc,
		m.keys.Quit.Help().Key+": "+m.keys.Quit.Help().Desc,
	)

	return s
}
