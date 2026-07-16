package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/evgen2571/mangate/internal/usecase"
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

func (m model) downloadChaptersCmd(ctx context.Context, manga *source.Manga, chapters []*source.Chapter, progressCh chan tea.Msg) tea.Cmd {
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

			downloadErr := m.app.UseCases().DownloadChapters(ctx, manga, chapters, func(progress usecase.DownloadProgress) {
				progressCh <- downloadProgressMsgFromUsecase(progress)
			})
			cancelled := errors.Is(downloadErr, context.Canceled)
			var outcomes []chapterOutcome
			var archiveErr error
			if cancelled {
				outcomes = m.directoryChapterOutcomes(manga, chapters)
			} else {
				outcomes, archiveErr = m.archiveChapters(ctx, manga, chapters)
			}
			if operationErr := errors.Join(downloadErr, archiveErr); operationErr != nil {
				progressCh <- downloadFailedMsg{Manga: manga, Chapters: chapters, Outcomes: outcomes, Cancelled: cancelled, Err: operationErr}
				return
			}

			progressCh <- downloadSucceededMsg{Manga: manga, Chapters: chapters, Outcomes: outcomes}
		}()

		return nil
	}
}

type chapterOutcome struct {
	Name   string
	Status string
	Path   string
	Error  string
}

func (m model) archiveChapters(ctx context.Context, manga *source.Manga, chapters []*source.Chapter) ([]chapterOutcome, error) {
	format, err := archive.ParseFormat(m.app.Cfg.Download.Format)
	if err != nil {
		return nil, err
	}
	outcomes := m.directoryChapterOutcomes(manga, chapters)
	if format == archive.FormatDirectory {
		return outcomes, nil
	}

	var failures []error
	for index, chapter := range chapters {
		if chapter == nil || outcomes[index].Status != "complete" {
			continue
		}
		sourceDir := outcomes[index].Path
		archivePath := sourceDir + format.Extension()
		result, err := archive.CreateFromDirectoryContext(ctx, archive.Options{
			Format:           format,
			SourceDir:        sourceDir,
			OutputPath:       archivePath,
			ExistingFileMode: archive.ExistingFileMode(m.app.Cfg.Download.ExistingFileMode),
			RemoveSource:     !m.app.Cfg.Download.RetainSource,
			Metadata: archive.Metadata{
				Provider:      m.app.Cfg.Provider,
				TitleID:       manga.ID,
				Title:         manga.Title,
				ChapterID:     chapter.ID,
				Volume:        chapter.Volume,
				ChapterNumber: chapter.Index,
				ChapterTitle:  chapter.Title,
				Language:      chapter.Language,
				ReleaseGroup:  chapter.ReleaseGroup,
				PublishedAt:   chapter.PublishedAt,
				ExpectedPages: chapter.PageCount,
			},
		})
		if err != nil {
			outcomes[index].Status = "archive_failed"
			outcomes[index].Error = err.Error()
			outcomes[index].Path = archivePath
			failures = append(failures, fmt.Errorf("create %s for %s: %w", format, chapter.LogName(), err))
			continue
		}
		outcomes[index].Path = archivePath
		if result.Status == archive.StatusSkipped {
			outcomes[index].Status = "skipped"
		}
	}
	return outcomes, errors.Join(failures...)
}

func (m model) directoryChapterOutcomes(manga *source.Manga, chapters []*source.Chapter) []chapterOutcome {
	names := downloader.ChapterDirectoryNames(chapters)
	titleDir := downloader.TitleDirectoryName(manga)
	outcomes := make([]chapterOutcome, len(chapters))
	for index, chapter := range chapters {
		path := filepath.Join(m.app.Cfg.Download.Dir, titleDir, names[index])
		name := "Unknown chapter"
		if chapter != nil {
			name = chapter.DisplayName()
		}
		outcomes[index] = chapterOutcome{Name: name, Status: "incomplete", Path: path}
		if chapter == nil {
			outcomes[index].Error = "selected chapter is nil"
			continue
		}
		if chapterDirectoryComplete(path) {
			outcomes[index].Status = "complete"
		}
	}
	return outcomes
}

func chapterDirectoryComplete(path string) bool {
	data, err := os.ReadFile(filepath.Join(path, ".mangate.json"))
	if err != nil {
		return false
	}
	var state struct {
		Complete bool `json:"complete"`
	}
	return json.Unmarshal(data, &state) == nil && state.Complete
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
