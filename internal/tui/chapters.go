package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/evgen2571/manga-downloader/internal/source"
)

type chaptersListKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Select key.Binding
	Back   key.Binding
	Quit   key.Binding
}

func newChaptersListKeyMap() chaptersListKeyMap {
	return chaptersListKeyMap{
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
			key.WithHelp("enter", "download chapter"),
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

type chaptersListModel struct {
	manga   *source.Manga
	items   []*source.Chapter
	cursor  int
	loading bool
	err     error
	keys    chaptersListKeyMap
}

func newChaptersListModel() chaptersListModel {
	return chaptersListModel{
		keys: newChaptersListKeyMap(),
	}
}

func (m chaptersListModel) Init() tea.Cmd {
	return nil
}

func (m chaptersListModel) Update(msg tea.Msg) (chaptersListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Back):
			if m.loading {
				return m, nil
			}
			return m, func() tea.Msg { return backToMangasMsg{} }

		case key.Matches(msg, m.keys.Up):
			if !m.loading && m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case key.Matches(msg, m.keys.Down):
			if !m.loading && m.cursor < len(m.items)-1 {
				m.cursor++
			}
			return m, nil

		case key.Matches(msg, m.keys.Select):
			if m.loading || len(m.items) == 0 {
				return m, nil
			}

			selected := m.items[m.cursor]
			return m, func() tea.Msg {
				return chapterDownloadRequestedMsg{
					manga:   m.manga,
					chapter: selected,
				}
			}
		}
	}

	return m, nil
}

func (m chaptersListModel) View() string {
	s := "Manga Downloader\n"
	s += "================\n\n"

	if m.manga != nil {
		s += fmt.Sprintf("Manga: %s\n\n", m.manga.Title)
	}

	if m.loading {
		s += "Loading chapters...\n\n"
	} else if m.err != nil {
		s += "Error: " + m.err.Error() + "\n\n"
	}

	if !m.loading {
		if len(m.items) == 0 {
			s += "No chapters found.\n\n"
		} else {
			for i, chapter := range m.items {
				cursor := " "
				if i == m.cursor {
					cursor = ">"
				}
				s += fmt.Sprintf("%s %s\n", cursor, chapter.Title)
			}
			s += "\n"
		}
	}

	s += helpLine(
		m.keys.Up.Help().Key+": "+m.keys.Up.Help().Desc,
		m.keys.Down.Help().Key+": "+m.keys.Down.Help().Desc,
		m.keys.Select.Help().Key+": "+m.keys.Select.Help().Desc,
		m.keys.Back.Help().Key+": "+m.keys.Back.Help().Desc,
		m.keys.Quit.Help().Key+": "+m.keys.Quit.Help().Desc,
	)

	return s
}
