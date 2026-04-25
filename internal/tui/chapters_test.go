package tui

import (
	"strings"
	"testing"

	"github.com/evgen2571/mangate/internal/source"
)

func TestChaptersModelSelectAllSelectsEveryNonNilChapter(t *testing.T) {
	chapters := []*source.Chapter{
		{ID: "a", Index: "1"},
		nil,
		{ID: "c", Index: "3"},
		{Index: "4"},
	}
	m := newChaptersModel(&source.Manga{Title: "Test"}, chapters)

	m.selectAll()

	if got, want := m.selectedCount(), 3; got != want {
		t.Fatalf("selectedCount() = %d, want %d", got, want)
	}
	gotChapters := m.chaptersForDownload()
	if len(gotChapters) != 3 {
		t.Fatalf("len(chaptersForDownload()) = %d, want 3", len(gotChapters))
	}
	if gotChapters[0] != chapters[0] || gotChapters[1] != chapters[2] || gotChapters[2] != chapters[3] {
		t.Fatalf("chaptersForDownload() = %#v, want non-nil chapters in list order", gotChapters)
	}
}

func TestChaptersModelDeselectAllClearsSelection(t *testing.T) {
	m := newChaptersModel(&source.Manga{Title: "Test"}, []*source.Chapter{
		{ID: "a", Index: "1"},
		{ID: "b", Index: "2"},
	})
	m.selectAll()

	m.deselectAll()

	if got := m.selectedCount(); got != 0 {
		t.Fatalf("selectedCount() = %d, want 0", got)
	}
}

func TestChaptersModelFooterShowsTotalChapterCount(t *testing.T) {
	m := newChaptersModel(&source.Manga{Title: "Test"}, []*source.Chapter{
		{ID: "a", Index: "1"},
		{ID: "b", Index: "2"},
	})

	got := m.footerText()
	if want := "chapters: 2"; !strings.Contains(got, want) {
		t.Fatalf("footerText() = %q, want to contain %q", got, want)
	}
}
