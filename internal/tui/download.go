package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type downloadKeyMap struct {
	Back key.Binding
	Quit key.Binding
}

func newDownloadKeyMap() downloadKeyMap {
	return downloadKeyMap{
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

type downloadModel struct {
	kind       string
	title      string
	mangaTitle string
	origin     screen
	loading    bool
	done       bool
	err        error
	keys       downloadKeyMap
}

func newDownloadModel() downloadModel {
	return downloadModel{
		keys: newDownloadKeyMap(),
	}
}

func (m downloadModel) Init() tea.Cmd {
	return nil
}

func (m downloadModel) Update(msg tea.Msg) (downloadModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Back):
			if m.loading {
				return m, nil
			}
			return m, func() tea.Msg { return backFromDownloadMsg{} }
		}
	}

	return m, nil
}

func (m downloadModel) View() string {
	s := "Manga Downloader\n"
	s += "================\n\n"
	s += "Download\n"
	s += "--------\n\n"

	switch m.kind {
	case "manga":
		s += fmt.Sprintf("Manga: %s\n\n", m.title)
	case "chapter":
		s += fmt.Sprintf("Manga: %s\n", m.mangaTitle)
		s += fmt.Sprintf("Chapter: %s\n\n", m.title)
	}

	if m.loading {
		s += "Downloading...\n\n"
	} else if m.err != nil {
		s += "Download failed: " + m.err.Error() + "\n\n"
	} else if m.done {
		s += "Download completed.\n\n"
	}

	s += helpLine(
		m.keys.Back.Help().Key+": "+m.keys.Back.Help().Desc,
		m.keys.Quit.Help().Key+": "+m.keys.Quit.Help().Desc,
	)

	return s
}
