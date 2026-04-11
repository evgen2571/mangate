package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evgen2571/manga-downloader/internal/config"
	"github.com/evgen2571/manga-downloader/internal/downloader"
	"github.com/evgen2571/manga-downloader/internal/providers"
	"github.com/evgen2571/manga-downloader/internal/sources"
	"golang.org/x/sync/errgroup"
)

type screen int

const (
	screenSearch screen = iota
	screenMangasList
	screenChaptersList
	screenDownload
)

type appModel struct {
	current      screen
	search       searchModel
	mangasList   mangasListModel
	chaptersList chaptersListModel
	download     downloadModel
}

func New() tea.Model {
	return appModel{
		current:      screenSearch,
		search:       newSearchModel(),
		mangasList:   newMangasListModel(),
		chaptersList: newChaptersListModel(),
		download:     newDownloadModel(),
	}
}

func (m appModel) Init() tea.Cmd {
	return m.search.Init()
}

func loadChaptersCmd(manga *sources.Manga) tea.Cmd {
	return func() tea.Msg {
		chapters, err := providers.Provider.GetChapters(manga)
		return chaptersLoadedMsg{
			manga:    manga,
			chapters: chapters,
			err:      err,
		}
	}
}

func downloadChapterCmd(manga *sources.Manga, chapter *sources.Chapter) tea.Cmd {
	return func() tea.Msg {
		pages, err := providers.Provider.GetPages(chapter)
		if err != nil {
			return downloadFinishedMsg{err: err}
		}
		chapter.Pages = pages

		if err := downloader.DownloadChapter(chapter); err != nil {
			return downloadFinishedMsg{err: err}
		}

		return downloadFinishedMsg{}
	}
}

func downloadMangaCmd(manga *sources.Manga) tea.Cmd {
	return func() tea.Msg {
		chapters, err := providers.Provider.GetChapters(manga)
		if err != nil {
			return downloadFinishedMsg{err: err}
		}
		manga.Chapters = chapters

		// Set limit for concurrent page fetches
		var g errgroup.Group
		g.SetLimit(config.MaxConcurrentPageFetches)

		for _, chapter := range chapters {
			g.Go(func() error {
				pages, err := providers.Provider.GetPages(chapter)
				if err != nil {
					return fmt.Errorf("failed to get pages for %q: %w", chapter.Title, err)
				}
				chapter.Pages = pages
				return nil
			})
		}

		if err := downloader.DownloadManga(manga); err != nil {
			return downloadFinishedMsg{err: err}
		}

		return downloadFinishedMsg{}
	}
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case mangasLoadedMsg:
		if msg.err != nil {
			m.search.loading = false
			m.search.err = msg.err
			m.current = screenSearch
			return m, nil
		}

		m.search.loading = false
		m.search.err = nil
		m.mangasList = newMangasListModel()
		m.mangasList.query = msg.query
		m.mangasList.items = msg.items
		m.current = screenMangasList
		return m, nil

	case mangaSelectedMsg:
		m.chaptersList = newChaptersListModel()
		m.chaptersList.manga = msg.manga
		m.chaptersList.loading = true
		m.current = screenChaptersList
		return m, loadChaptersCmd(msg.manga)

	case chaptersLoadedMsg:
		m.chaptersList.loading = false
		m.chaptersList.err = msg.err
		m.chaptersList.manga = msg.manga
		m.chaptersList.items = msg.chapters
		m.current = screenChaptersList
		return m, nil

	case mangaDownloadRequestedMsg:
		m.download = newDownloadModel()
		m.download.kind = "manga"
		m.download.title = msg.manga.Title
		m.download.origin = screenMangasList
		m.download.loading = true
		m.current = screenDownload
		return m, downloadMangaCmd(msg.manga)

	case chapterDownloadRequestedMsg:
		m.download = newDownloadModel()
		m.download.kind = "chapter"
		m.download.title = msg.chapter.Title
		m.download.mangaTitle = msg.manga.Title
		m.download.origin = screenChaptersList
		m.download.loading = true
		m.current = screenDownload
		return m, downloadChapterCmd(msg.manga, msg.chapter)

	case downloadFinishedMsg:
		m.download.loading = false
		m.download.done = msg.err == nil
		m.download.err = msg.err
		return m, nil

	case backToSearchMsg:
		m.current = screenSearch
		return m, nil

	case backToMangasMsg:
		m.current = screenMangasList
		return m, nil

	case backFromDownloadMsg:
		m.current = m.download.origin
		return m, nil
	}

	switch m.current {
	case screenSearch:
		updated, cmd := m.search.Update(msg)
		m.search = updated
		return m, cmd

	case screenMangasList:
		updated, cmd := m.mangasList.Update(msg)
		m.mangasList = updated
		return m, cmd

	case screenChaptersList:
		updated, cmd := m.chaptersList.Update(msg)
		m.chaptersList = updated
		return m, cmd

	case screenDownload:
		updated, cmd := m.download.Update(msg)
		m.download = updated
		return m, cmd
	}

	return m, nil
}

func (m appModel) View() string {
	switch m.current {
	case screenSearch:
		return m.search.View()
	case screenMangasList:
		return m.mangasList.View()
	case screenChaptersList:
		return m.chaptersList.View()
	case screenDownload:
		return m.download.View()
	default:
		return ""
	}
}

func helpLine(parts ...string) string {
	return strings.Join(parts, " • ")
}
