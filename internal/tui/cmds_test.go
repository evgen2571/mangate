package tui

import (
	"testing"

	"github.com/evgen2571/mangate/internal/source"
)

func TestBuildDownloadMangaLeavesPagesForLazyLoading(t *testing.T) {
	manga := &source.Manga{ID: "manga-id", Title: "My Manga"}
	chapter := &source.Chapter{
		ID:        "chapter-1",
		Index:     "1",
		Title:     "Intro",
		PageCount: 23,
		Pages:     []*source.Page{{URL: "https://example.com/page.png"}},
	}

	downloadManga, err := buildDownloadManga(manga, []*source.Chapter{chapter})
	if err != nil {
		t.Fatalf("buildDownloadManga() error = %v", err)
	}
	if len(downloadManga.Chapters) != 1 {
		t.Fatalf("len(downloadManga.Chapters) = %d, want 1", len(downloadManga.Chapters))
	}

	downloadChapter := downloadManga.Chapters[0]
	if len(downloadChapter.Pages) != 0 {
		t.Fatalf("len(downloadChapter.Pages) = %d, want 0 for lazy loading", len(downloadChapter.Pages))
	}
	if downloadChapter.PageCount != 23 {
		t.Fatalf("download chapter PageCount = %d, want 23", downloadChapter.PageCount)
	}
	if downloadChapter == chapter {
		t.Fatalf("download chapter reuses original pointer")
	}
	if downloadChapter.From != downloadManga {
		t.Fatalf("download chapter parent = %p, want download manga %p", downloadChapter.From, downloadManga)
	}
	if len(chapter.Pages) != 1 {
		t.Fatalf("original chapter pages were mutated")
	}
}
