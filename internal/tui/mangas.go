package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/evgen2571/manga-downloader/internal/sources"
)

type mangasListKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Select   key.Binding
	Download key.Binding
	Back     key.Binding
	Quit     key.Binding
}

func newMangasListKeyMap() mangasListKeyMap {
	return mangasListKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "chapters"),
		),
		Download: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "download manga"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

type mangasListModel struct {
	items  []*sources.Manga
	cursor int
	query  string
	keys   mangasListKeyMap
	err    error
}

func newMangasListModel() mangasListModel {
	return mangasListModel{
		keys: newMangasListKeyMap(),
	}
}

func (m mangasListModel) Init() tea.Cmd {
	return nil
}

func (m mangasListModel) Update(msg tea.Msg) (mangasListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Back):
			return m, func() tea.Msg { return backToSearchMsg{} }

		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
			return m, nil

		case key.Matches(msg, m.keys.Select):
			if len(m.items) == 0 {
				return m, nil
			}

			selected := m.items[m.cursor]
			return m, func() tea.Msg {
				return mangaSelectedMsg{manga: selected}
			}

		case key.Matches(msg, m.keys.Download):
			if len(m.items) == 0 {
				return m, nil
			}

			selected := m.items[m.cursor]
			return m, func() tea.Msg {
				return mangaDownloadRequestedMsg{manga: selected}
			}
		}
	}

	return m, nil
}

func (m mangasListModel) View() string {
	s := "Manga Downloader\n"
	s += "================\n\n"
	s += fmt.Sprintf("Results for: %q\n\n", m.query)

	if len(m.items) == 0 {
		s += "No manga found.\n\n"
	} else {
		for i, manga := range m.items {
			cursor := " "
			if i == m.cursor {
				cursor = ">"
			}
			s += fmt.Sprintf("%s %s\n", cursor, manga.Title)
		}
		s += "\n"
	}

	if m.err != nil {
		s += "Info: " + m.err.Error() + "\n\n"
	}

	s += helpLine(
		m.keys.Up.Help().Key+": "+m.keys.Up.Help().Desc,
		m.keys.Down.Help().Key+": "+m.keys.Down.Help().Desc,
		m.keys.Select.Help().Key+": "+m.keys.Select.Help().Desc,
		m.keys.Download.Help().Key+": "+m.keys.Download.Help().Desc,
		m.keys.Back.Help().Key+": "+m.keys.Back.Help().Desc,
		m.keys.Quit.Help().Key+": "+m.keys.Quit.Help().Desc,
	)

	return s
}
