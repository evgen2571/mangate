package downloader

import (
	"testing"

	"github.com/evgen2571/mangate/internal/source"
)

func TestNewProgressReporterUsesKnownPageCounts(t *testing.T) {
	manga := &source.Manga{
		Chapters: []*source.Chapter{
			{Index: "1", PageCount: 12},
			{Index: "2", Pages: []*source.Page{{URL: "one"}, {URL: "two"}}},
		},
	}

	reporter := newProgressReporter(manga, nil)

	if reporter.progress.TotalPages != 14 {
		t.Fatalf("TotalPages = %d, want 14", reporter.progress.TotalPages)
	}
	if reporter.progress.Chapters[0].TotalPages != 12 {
		t.Fatalf("first chapter TotalPages = %d, want 12", reporter.progress.Chapters[0].TotalPages)
	}
}

func TestProgressDistinguishesReleasesWithSameDisplayName(t *testing.T) {
	first := &source.Chapter{ID: "first", Index: "1", PageCount: 1}
	second := &source.Chapter{ID: "second", Index: "1", PageCount: 1}
	reporter := newProgressReporter(&source.Manga{Chapters: []*source.Chapter{first, second}}, func(DownloadProgress) {})
	reporter.chapterCompleted(second)
	if reporter.progress.Chapters[0].Completed || !reporter.progress.Chapters[1].Completed {
		t.Fatalf("wrong chapter completed: %#v", reporter.progress.Chapters)
	}
}
