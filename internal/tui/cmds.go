package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evgen2571/mangate/internal/tuiapp"
)

func (m model) loadChaptersCmd(result tuiapp.SearchResult) tea.Cmd {
	return func() tea.Msg {
		if result.ID == "" {
			return nil
		}

		details, chapters, err := m.svc.LoadChapters(nil, result)
		if err != nil {
			return chaptersFailedMsg{
				MangaID: result.ID,
				Err:     err,
			}
		}

		return chaptersLoadedMsg{
			Manga:    details,
			Chapters: chapters,
		}
	}
}

func (m model) downloadChaptersCmd(manga tuiapp.MangaDetails, chapters []tuiapp.ChapterItem, progressCh chan tea.Msg) tea.Cmd {
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

			err := m.svc.Download(nil, tuiapp.DownloadRequest{
				Manga:    manga,
				Chapters: chapters,
			}, func(progress tuiapp.DownloadProgress) {
				progressCh <- downloadProgressMsgFromTUIApp(progress)
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

func downloadProgressMsgFromTUIApp(progress tuiapp.DownloadProgress) downloadProgressMsg {
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

func toChapterProgressViews(chapters []tuiapp.ChapterProgress) []chapterProgressView {
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
