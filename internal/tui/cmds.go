package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/source"
)

func (m model) searchMangaCmd(query string) tea.Cmd {
	return func() tea.Msg {
		results, err := m.app.UseCases().SearchManga(nil, query)
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

		chapters, err := m.app.UseCases().Chapters(nil, manga)
		if err != nil {
			return chaptersFailedMsg{Manga: manga, Err: err}
		}

		return chaptersLoadedMsg{
			Manga:    manga,
			Chapters: chapters,
		}
	}
}

func (m model) downloadChaptersCmd(manga *source.Manga, chapters []*source.Chapter, progressCh chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		go func() {
			defer close(progressCh)

			progressCh <- downloadProgressMsg{
				Title:     "Downloading pages",
				Detail:    downloadDetailText(chapters),
				Status:    "Starting downloads...",
				Completed: 0,
				Total:     0,
			}

			err := m.app.UseCases().DownloadChapters(nil, manga, chapters, func(progress downloader.DownloadProgress) {
				progressCh <- downloadProgressMsg{
					Title:     "Downloading pages",
					Detail:    progressSummaryDetail(progress.Chapters),
					Status:    fmt.Sprintf("Downloaded %d/%d chapters", progress.CompletedChapters, progress.TotalChapters),
					Completed: progress.CompletedPages,
					Total:     progress.TotalPages,
					Chapters:  toChapterProgressViews(progress.Chapters),
				}
			})
			if err != nil {
				progressCh <- downloadFailedMsg{Manga: manga, Chapters: chapters, Err: err}
				return
			}

			progressCh <- downloadSucceededMsg{Manga: manga, Chapters: chapters}
		}()

		return nil
	}
}

func (m model) loadCoverCmd(manga *source.Manga, width, height int) tea.Cmd {
	return func() tea.Msg {
		if manga == nil {
			return nil
		}

		path, err := m.app.UseCases().CoverPath(nil, manga)
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

func chapterDisplayName(chapter *source.Chapter) string {
	if chapter == nil {
		return "unknown chapter"
	}

	index := strings.TrimSpace(chapter.Index)
	title := strings.TrimSpace(chapter.Title)

	switch {
	case index != "" && title != "":
		return fmt.Sprintf("chapter %s (%s)", index, title)
	case index != "":
		return fmt.Sprintf("chapter %s", index)
	case title != "":
		return title
	default:
		return "unknown chapter"
	}
}

func toChapterProgressViews(chapters []downloader.ChapterDownloadProgress) []chapterProgressView {
	views := make([]chapterProgressView, 0, len(chapters))
	for _, chapter := range chapters {
		views = append(views, chapterProgressView{
			Name:           chapter.Name,
			CompletedPages: chapter.CompletedPages,
			TotalPages:     chapter.TotalPages,
			Active:         chapter.Active,
			Completed:      chapter.Completed,
		})
	}
	return views
}

func progressSummaryDetail(chapters []downloader.ChapterDownloadProgress) string {
	active := make([]string, 0)
	queued := 0

	for _, chapter := range chapters {
		switch {
		case chapter.Active:
			active = append(active, chapter.Name)
		case !chapter.Completed:
			queued++
		}
	}

	switch {
	case len(active) == 1 && queued > 0:
		return fmt.Sprintf("Now: %s • queued: %d", active[0], queued)
	case len(active) > 1 && queued > 0:
		return fmt.Sprintf("Active: %d chapters • queued: %d", len(active), queued)
	case len(active) == 1:
		return fmt.Sprintf("Now: %s", active[0])
	case len(active) > 1:
		return fmt.Sprintf("Active: %d chapters", len(active))
	case queued > 0:
		return fmt.Sprintf("Queued: %d chapters", queued)
	default:
		return "Finishing download..."
	}
}
