package interactive

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/source"
)

func (m *model) waitForProgress() tea.Cmd {
	messages, ctx := m.progressCh, m.ctx
	return func() tea.Msg {
		select {
		case message := <-messages:
			return message
		case <-ctx.Done():
			return nil
		}
	}
}

func (m *model) download(ctx context.Context, chapters []*source.Chapter, progressCh chan tea.Msg) tea.Cmd {
	manga := m.manga
	provider, providerErr := m.app.Provider()
	service := m.app.Acquisition(provider)
	return func() tea.Msg {
		defer close(progressCh)
		if providerErr != nil {
			return downloadDone{err: providerErr, failed: len(chapters)}
		}
		record, err := service.Execute(ctx, manga, chapters, "", func(p downloader.DownloadProgress) {
			active := ""
			for _, c := range p.Chapters {
				if c.Active {
					active = c.Name
					break
				}
			}
			select {
			case progressCh <- downloadProgress{p.CompletedPages, p.TotalPages, p.CompletedChapters, p.TotalChapters, active}:
			default:
			}
		})
		done := downloadDone{err: err, paths: []string{}, results: []ChapterResult{}}
		for _, chapter := range record.Chapters {
			path := chapter.OutputPath
			if chapter.ArchivePath != "" {
				path = chapter.ArchivePath
			}
			status := "failed"
			switch chapter.Status {
			case "complete":
				done.completed++
				status = "success"
				done.paths = append(done.paths, path)
			case "skipped":
				done.skipped++
				status = "success"
				done.paths = append(done.paths, path)
			default:
				done.failed++
			}
			if m.integration {
				images := []string{}
				if status == "success" {
					images = chapter.ImagePaths
				}
				done.results = append(done.results, ChapterResult{ExpectedPages: chapter.ExpectedPages, Chapter: chapter.Chapter, Status: status, OutputDir: chapter.OutputPath, ImagePaths: images})
			}
		}
		return done
	}
}
