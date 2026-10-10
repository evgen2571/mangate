package interactive

import (
	"context"
	"errors"
	"path/filepath"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/evgen2571/mangate/internal/usecase"
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
	service, manga, cfg, format := m.app.UseCases(), m.manga, m.app.Cfg, m.format
	return func() tea.Msg {
		defer close(progressCh)
		notify := func(message tea.Msg) {
			select {
			case progressCh <- message:
			default:
			}
		}
		var completed, skipped, failed int
		var progressMu sync.Mutex
		finished := make(map[string]usecase.ChapterDownloadProgress)
		paths := []string{}
		err := service.DownloadChapters(ctx, manga, chapters, func(p usecase.DownloadProgress) {
			progressMu.Lock()
			for _, chapter := range p.Chapters {
				if chapter.Completed {
					finished[chapter.ChapterID] = chapter
				}
			}

			progressMu.Unlock()
			active := ""
			for _, c := range p.Chapters {
				if c.Active {
					active = c.Name
					break
				}
			}

			notify(downloadProgress{p.CompletedPages, p.TotalPages, p.CompletedChapters, p.TotalChapters, active})
		})
		chapterPaths := chapterDownloadPaths(cfg.Download.Dir, manga, chapters)
		for index, chapter := range chapters {
			path := chapterPaths[index]
			progress, done := finished[chapter.ID]
			if !done {
				failed++
				continue
			}

			if progress.Reused {
				skipped++
				paths = append(paths, path+format.Extension())
				continue
			}

			if format.IsArchive() {
				notify(downloadProgress{
					completed:         completed,
					total:             len(chapters),
					completedChapters: completed,
					totalChapters:     len(chapters),
					active:            "Creating and validating archive...",
				})
				result, archiveErr := archive.CreateFromDirectoryContext(ctx, archive.Options{
					Format:           format,
					SourceDir:        path,
					OutputPath:       path + format.Extension(),
					ExistingFileMode: archive.ExistingFileMode(cfg.Download.ExistingFileMode),
					RemoveSource:     true,
					Metadata: archive.Metadata{
						Provider:      cfg.Provider,
						TitleID:       manga.ID,
						Title:         manga.Title,
						ChapterID:     chapter.ID,
						ChapterNumber: chapter.Index,
						ChapterTitle:  chapter.Title,
						ExpectedPages: progress.TotalPages,
					},
				})
				if archiveErr != nil {
					failed++
					err = errors.Join(err, archiveErr)
					continue
				}

				path += format.Extension()
				if result.Status == archive.StatusSkipped {
					skipped++
					paths = append(paths, path)
					continue
				}
			}

			completed++
			paths = append(paths, path)
		}

		return downloadDone{err, completed, skipped, failed, paths}
	}
}

func chapterDownloadPaths(root string, manga *source.Manga, chapters []*source.Chapter) []string {
	names := downloader.ChapterDirectoryNames(chapters)
	paths := make([]string, len(names))
	for index, name := range names {
		paths[index] = filepath.Join(root, downloader.TitleDirectoryName(manga), name)
	}

	return paths
}
