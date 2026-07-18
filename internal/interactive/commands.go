package interactive

import (
	"context"
	"errors"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/evgen2571/mangate/internal/usecase"
)

func (m *model) waitForProgress() tea.Cmd { return func() tea.Msg { return <-m.progressCh } }

func (m *model) download(ctx context.Context, chapters []*source.Chapter, progressCh chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		go func() {
			var completed, skipped, failed int
			paths := []string{}
			err := m.app.UseCases().DownloadChapters(ctx, m.manga, chapters, func(p usecase.DownloadProgress) {
				active := ""
				for _, c := range p.Chapters {
					if c.Active {
						active = c.Name
						break
					}
				}
				progressCh <- downloadProgress{p.CompletedPages, p.TotalPages, p.CompletedChapters, p.TotalChapters, active}
			})
			if err == nil {
				chapterPaths := chapterDownloadPaths(m.app.Cfg.Download.Dir, m.manga, chapters)
				for index, chapter := range chapters {
					path := chapterPaths[index]
					if m.format.IsArchive() {
						progressCh <- downloadProgress{completed: completed, total: len(chapters), completedChapters: completed, totalChapters: len(chapters), active: "Creating and validating archive..."}
						result, archiveErr := archive.CreateFromDirectoryContext(ctx, archive.Options{Format: m.format, SourceDir: path, OutputPath: path + m.format.Extension(), ExistingFileMode: archive.ExistingFileMode(m.app.Cfg.Download.ExistingFileMode), RemoveSource: true, Metadata: archive.Metadata{Provider: m.app.Cfg.Provider, TitleID: m.manga.ID, Title: m.manga.Title, ChapterID: chapter.ID, ChapterNumber: chapter.Index, ChapterTitle: chapter.Title, ExpectedPages: chapter.PageCount}})
						if archiveErr != nil {
							failed++
							err = errors.Join(err, archiveErr)
							continue
						}
						path += m.format.Extension()
						if result.Status == archive.StatusSkipped {
							skipped++
							paths = append(paths, path)
							continue
						}
					}
					completed++
					paths = append(paths, path)
				}
			}
			progressCh <- downloadDone{err, completed, skipped, failed, paths}
			close(progressCh)
		}()
		return nil
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
