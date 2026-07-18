package interactive

import (
	"testing"

	"github.com/evgen2571/mangate/internal/source"
)

func TestArchiveUsesNamesFromSelectedChapterSlice(t *testing.T) {
	allChapters := []*source.Chapter{
		{ID: "chapter-a", Index: "1"},
		{ID: "chapter-b", Index: "1"},
	}
	selected := allChapters[1:]

	paths := chapterDownloadPaths("/downloads", &source.Manga{ID: "title-id", Title: "Example"}, selected)
	if got, want := paths[0], "/downloads/Example-title-id/Chapter-1"; got != want {
		t.Fatalf("selected chapter path = %q, want %q", got, want)
	}
}
