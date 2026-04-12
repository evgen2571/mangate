package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evgen2571/manga-downloader/internal/config"
	"github.com/evgen2571/manga-downloader/internal/downloader"
	"github.com/evgen2571/manga-downloader/internal/providers"
	"github.com/evgen2571/manga-downloader/internal/source"
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
	width        int
	height       int
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

func loadChaptersCmd(manga *source.Manga) tea.Cmd {
	return func() tea.Msg {
		chapters, err := providers.Provider.GetChapters(manga)
		return chaptersLoadedMsg{
			manga:    manga,
			chapters: chapters,
			err:      err,
		}
	}
}

func downloadChapterCmd(chapter *source.Chapter) tea.Cmd {
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

func downloadChaptersCmd(chapters []*source.Chapter) tea.Cmd {
	return func() tea.Msg {
		for _, chapter := range chapters {
			pages, err := providers.Provider.GetPages(chapter)
			if err != nil {
				return downloadFinishedMsg{err: err}
			}
			chapter.Pages = pages

			if err := downloader.DownloadChapter(chapter); err != nil {
				return downloadFinishedMsg{err: err}
			}
		}
		return downloadFinishedMsg{}
	}
}

func downloadMangaCmd(manga *source.Manga) tea.Cmd {
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
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		m.search.width = msg.Width
		m.search.height = msg.Height
		m.mangasList.width = msg.Width
		m.mangasList.height = msg.Height
		m.chaptersList.width = msg.Width
		m.chaptersList.height = msg.Height

	case mangasLoadedMsg:
		m.search.loading = false

		if msg.err != nil {
			m.search.err = msg.err
			m.current = screenSearch
			return m, nil
		}

		m.search.err = nil

		m.mangasList = newMangasListModel()
		m.mangasList.query = msg.query
		m.mangasList.items = msg.items
		m.mangasList.width = m.width
		m.mangasList.height = m.height

		m.current = screenMangasList
		return m, nil

	case mangaSelectedMsg:
		m.chaptersList = newChaptersListModel()
		m.chaptersList.manga = msg.manga
		m.chaptersList.loading = true
		m.chaptersList.width = m.width
		m.chaptersList.height = m.height
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
		return m, downloadChapterCmd(msg.chapter)

	case chaptersDownloadRequestedMsg:
		m.download = newDownloadModel()
		m.download.kind = "chapter"
		if len(msg.chapters) == 1 {
			m.download.title = msg.chapters[0].Title
		} else {
			m.download.title = fmt.Sprintf("%d chapters", len(msg.chapters))
		}
		m.download.mangaTitle = msg.manga.Title
		m.download.origin = screenChaptersList
		m.download.loading = true
		m.current = screenDownload
		return m, downloadChaptersCmd(msg.chapters)

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
