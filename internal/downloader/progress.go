package downloader

import (
	"sync"

	"github.com/evgen2571/mangate/internal/source"
)

type ChapterDownloadProgress struct {
	ChapterID      string
	Name           string
	CompletedPages int
	TotalPages     int
	Active         bool
	Completed      bool
	Reused         bool
}

func (r *progressReporter) chapterReused(chapter *source.Chapter, pages int) {
	if r == nil || r.notify == nil {
		return
	}
	r.mu.Lock()
	if current := r.chapterProgress(chapter); current != nil {
		r.progress.TotalPages += pages - current.TotalPages
		current.TotalPages, current.CompletedPages = pages, pages
		current.Completed, current.Reused, current.Active = true, true, false
		r.progress.CompletedPages += pages
		r.progress.CompletedChapters++
	}
	progress := r.snapshotLocked()
	r.mu.Unlock()
	r.notify(progress)
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
	indices  map[*source.Chapter]int
}

func newProgressReporter(manga *source.Manga, notify func(DownloadProgress)) *progressReporter {
	totalPages := 0
	chapters := make([]ChapterDownloadProgress, 0)
	indices := make(map[*source.Chapter]int)

	if manga != nil {
		for _, chapter := range manga.Chapters {
			if chapter == nil {
				continue
			}

			name := chapter.DisplayName()
			pageCount := knownPageCount(chapter)
			totalPages += pageCount
			indices[chapter] = len(chapters)
			chapters = append(chapters, ChapterDownloadProgress{
				ChapterID:  chapter.ID,
				Name:       name,
				TotalPages: pageCount,
			})
		}
	}

	return &progressReporter{
		progress: DownloadProgress{
			TotalPages:    totalPages,
			TotalChapters: len(chapters),
			Chapters:      chapters,
		},
		notify:  notify,
		indices: indices,
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
	index, ok := r.indices[chapter]
	if ok {
		return &r.progress.Chapters[index]
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
