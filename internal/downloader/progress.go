package downloader

import (
	"fmt"
	"strings"
	"sync"

	"github.com/evgen2571/mangate/internal/source"
)

type ChapterDownloadProgress struct {
	Name           string
	CompletedPages int
	TotalPages     int
	Active         bool
	Completed      bool
}

type DownloadProgress struct {
	CompletedPages    int
	TotalPages        int
	CompletedChapters int
	TotalChapters     int
	Chapters          []ChapterDownloadProgress
}

type progressReporter struct {
	mu       sync.Mutex
	progress DownloadProgress
	notify   func(DownloadProgress)
	order    []string
}

func newProgressReporter(manga *source.Manga, notify func(DownloadProgress)) *progressReporter {
	totalPages := 0
	chapters := make([]ChapterDownloadProgress, 0)
	order := make([]string, 0)

	if manga != nil {
		for _, chapter := range manga.Chapters {
			if chapter == nil {
				continue
			}

			name := downloadProgressChapterName(chapter)
			pageCount := knownPageCount(chapter)
			totalPages += pageCount
			chapters = append(chapters, ChapterDownloadProgress{
				Name:       name,
				TotalPages: pageCount,
			})
			order = append(order, name)
		}
	}

	return &progressReporter{
		progress: DownloadProgress{
			TotalPages:    totalPages,
			TotalChapters: len(chapters),
			Chapters:      chapters,
		},
		notify: notify,
		order:  order,
	}
}

func (r *progressReporter) chapterStarted(chapter *source.Chapter) {
	if r == nil || r.notify == nil {
		return
	}

	r.mu.Lock()
	if current := r.chapterProgress(chapter); current != nil {
		current.Active = true
	}
	progress := r.snapshotLocked()
	r.mu.Unlock()

	r.notify(progress)
}

func (r *progressReporter) pagesDiscovered(chapter *source.Chapter) {
	if r == nil || r.notify == nil || chapter == nil {
		return
	}

	r.mu.Lock()
	if current := r.chapterProgress(chapter); current != nil {
		pageCount := knownPageCount(chapter)
		delta := pageCount - current.TotalPages
		current.TotalPages = pageCount
		r.progress.TotalPages += delta
	}
	progress := r.snapshotLocked()
	r.mu.Unlock()

	r.notify(progress)
}

func (r *progressReporter) pageCompleted(chapter *source.Chapter) {
	if r == nil || r.notify == nil {
		return
	}

	r.mu.Lock()
	r.progress.CompletedPages++
	if current := r.chapterProgress(chapter); current != nil {
		current.Active = true
		current.CompletedPages++
	}
	progress := r.snapshotLocked()
	r.mu.Unlock()

	r.notify(progress)
}

func (r *progressReporter) chapterCompleted(chapter *source.Chapter) {
	if r == nil || r.notify == nil {
		return
	}

	r.mu.Lock()
	r.progress.CompletedChapters++
	if current := r.chapterProgress(chapter); current != nil {
		current.Active = false
		current.Completed = true
	}
	progress := r.snapshotLocked()
	r.mu.Unlock()

	r.notify(progress)
}

func (r *progressReporter) chapterProgress(chapter *source.Chapter) *ChapterDownloadProgress {
	name := downloadProgressChapterName(chapter)
	for idx := range r.progress.Chapters {
		if r.progress.Chapters[idx].Name == name {
			return &r.progress.Chapters[idx]
		}
	}
	return nil
}

func (r *progressReporter) snapshotLocked() DownloadProgress {
	chapters := make([]ChapterDownloadProgress, len(r.progress.Chapters))
	copy(chapters, r.progress.Chapters)

	return DownloadProgress{
		CompletedPages:    r.progress.CompletedPages,
		TotalPages:        r.progress.TotalPages,
		CompletedChapters: r.progress.CompletedChapters,
		TotalChapters:     r.progress.TotalChapters,
		Chapters:          chapters,
	}
}

func knownPageCount(chapter *source.Chapter) int {
	if chapter == nil {
		return 0
	}
	if chapter.PageCount > 0 {
		return chapter.PageCount
	}
	return len(chapter.Pages)
}

func downloadProgressChapterName(chapter *source.Chapter) string {
	if chapter == nil {
		return "Unknown chapter"
	}

	index := strings.TrimSpace(chapter.Index)
	title := strings.TrimSpace(chapter.Title)

	switch {
	case index != "" && title != "":
		return fmt.Sprintf("Chapter %s - %s", index, title)
	case title != "":
		return title
	case index != "":
		return fmt.Sprintf("Chapter %s", index)
	default:
		return "Unknown chapter"
	}
}
