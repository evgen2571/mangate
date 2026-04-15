package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) searchMangaCmd(query string) tea.Cmd {
	return func() tea.Msg {
		provider, err := m.app.Registry.New(m.app.Cfg.Provider, m.app.Cfg, m.app.Client)
		if err != nil {
			return searchFailedMsg{Err: err}
		}

		ctx, cancel := context.WithTimeout(context.Background(), m.app.Cfg.HTTP.Timeout)
		defer cancel()

		results, err := provider.Search(ctx, query)
		if err != nil {
			return searchFailedMsg{Err: err}
		}

		return searchSucceededMsg{
			Query:   query,
			Results: results,
		}
	}
}
