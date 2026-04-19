package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evgen2571/mangate/internal/source"
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

func (m model) loadChaptersCmd(manga *source.Manga) tea.Cmd {
	return func() tea.Msg {
		if manga == nil {
			return nil
		}

		provider, err := m.app.Registry.New(m.app.Cfg.Provider, m.app.Cfg, m.app.Client)
		if err != nil {
			return chaptersFailedMsg{Manga: manga, Err: err}
		}

		ctx, cancel := context.WithTimeout(context.Background(), m.app.Cfg.HTTP.Timeout)
		defer cancel()

		chapters, err := provider.Chapters(ctx, manga)
		if err != nil {
			return chaptersFailedMsg{Manga: manga, Err: err}
		}

		return chaptersLoadedMsg{
			Manga:    manga,
			Chapters: chapters,
		}
	}
}

func (m model) loadCoverCmd(manga *source.Manga, width, height int) tea.Cmd {
	return func() tea.Msg {
		if manga == nil {
			return nil
		}

		provider, err := m.app.Registry.New(m.app.Cfg.Provider, m.app.Cfg, m.app.Client)
		if err != nil {
			return coverFailedMsg{MangaID: manga.ID, Err: err}
		}

		ctx, cancel := context.WithTimeout(context.Background(), m.app.Cfg.HTTP.Timeout)
		defer cancel()

		path, err := m.app.Cache.Get(ctx, provider, manga)
		if err != nil {
			return coverFailedMsg{MangaID: manga.ID, Err: err}
		}

		render, err := renderCoverText(path, width, height)
		if err != nil {
			return coverFailedMsg{MangaID: manga.ID, Err: err}
		}

		return coverLoadedMsg{
			MangaID: manga.ID,
			Path:    path,
			Render:  render,
		}
	}
}
