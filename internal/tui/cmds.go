package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/evgen2571/mangate/internal/tuiapp"
	"github.com/evgen2571/mangate/internal/usecase"
)

func (m model) searchMangaCmd(query string) tea.Cmd {
	return func() tea.Msg {
		results, err := m.svc.Search(nil, query)
		if err != nil {
			return searchFailedMsg{Err: err}
		}

		history, _ := m.svc.SearchHistory(nil)
		return searchSucceededMsg{
			Query:   query,
			Results: results,
			History: history,
		}
	}
}

func (m model) loadChaptersCmd(result tuiapp.SearchResult) tea.Cmd {
	return func() tea.Msg {
		if result.ID == "" {
			return nil
		}

		details, chapters, err := m.svc.LoadChapters(nil, result)
		if err != nil {
			return chaptersFailedMsg{
				Manga: &source.Manga{ID: result.ID, Title: result.Title, URL: result.URL},
				Err:   err,
			}
		}

		return chaptersLoadedMsg{
			Manga:    sourceMangaFromDetails(details),
			Chapters: sourceChaptersFromItems(chapters),
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

			err := m.app.UseCases().DownloadChapters(nil, manga, chapters, func(progress usecase.DownloadProgress) {
				progressCh <- downloadProgressMsgFromUsecase(progress)
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

func (m model) loadCoverCmd(result tuiapp.SearchResult, width, height int) tea.Cmd {
	return func() tea.Msg {
		if result.ID == "" {
			return nil
		}

		cover, err := m.svc.LoadCover(nil, result, tuiapp.CoverSize{Width: width, Height: height})
		if err != nil {
			return coverFailedMsg{MangaID: result.ID, Err: err}
		}

		render, err := renderCoverText(cover.Path, width, height)
		if err != nil {
			return coverFailedMsg{MangaID: result.ID, Err: err}
		}

		return coverLoadedMsg{
			MangaID: cover.MangaID,
			Path:    cover.Path,
			Render:  render,
		}
	}
}

func sourceMangaFromDetails(details tuiapp.MangaDetails) *source.Manga {
	return &source.Manga{
		ID:    details.ID,
		Title: details.Title,
		URL:   details.URL,
		Metadata: source.MangaMetadata{
			ChapterCount: details.ChapterCount,
		},
	}
}

func sourceChaptersFromItems(items []tuiapp.ChapterItem) []*source.Chapter {
	chapters := make([]*source.Chapter, 0, len(items))
	for _, item := range items {
		chapters = append(chapters, &source.Chapter{
			ID:    item.ID,
			Index: item.Index,
			Title: item.Title,
			URL:   item.URL,
		})
	}
	return chapters
}

func downloadProgressMsgFromUsecase(progress usecase.DownloadProgress) downloadProgressMsg {
	chapters := toChapterProgressViews(progress.Chapters)
	return downloadProgressMsg{
		Title:     "Downloading pages",
		Detail:    progressSummaryDetail(chapters),
		Status:    fmt.Sprintf("Downloaded %d/%d chapters", progress.CompletedChapters, progress.TotalChapters),
		Completed: progress.CompletedPages,
		Total:     progress.TotalPages,
		Chapters:  chapters,
	}
}

func toChapterProgressViews(chapters []usecase.ChapterDownloadProgress) []chapterProgressView {
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

func progressSummaryDetail(chapters []chapterProgressView) string {
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
