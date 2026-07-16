package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evgen2571/mangate/internal/source"
)

func TestChaptersModelSelectAllSelectsEveryNonNilChapterWhenSelectionIsPartial(t *testing.T) {
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

func TestChaptersModelSelectAllClearsSelectionWhenEveryChapterIsSelected(t *testing.T) {
	m := newChaptersModel(&source.Manga{Title: "Test"}, []*source.Chapter{
		{ID: "a", Index: "1"},
		nil,
		{ID: "c", Index: "3"},
	})
	m.selectAll()

	m.selectAll()

	if got := m.selectedCount(); got != 0 {
		t.Fatalf("selectedCount() = %d, want 0", got)
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
	if got := m.footerText(); strings.Contains(got, "selected:") {
		t.Fatalf("footerText() = %q, want selection count hidden after deselect all", got)
	}
}

func TestChaptersModelDeselectAllKeyClearsSelectionAndSetsStatus(t *testing.T) {
	m := newChaptersModel(&source.Manga{Title: "Test"}, []*source.Chapter{
		{ID: "a", Index: "1"},
		{ID: "b", Index: "2"},
	})
	m.selectAll()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})

	if cmd != nil {
		t.Fatalf("Update(d) returned unexpected command")
	}
	if got := updated.selectedCount(); got != 0 {
		t.Fatalf("selectedCount() = %d, want 0", got)
	}
	if got, want := updated.status, "cleared selection"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
}

func TestChaptersModelSelectAllKeyClearsSelectionWhenEveryChapterIsSelected(t *testing.T) {
	m := newChaptersModel(&source.Manga{Title: "Test"}, []*source.Chapter{
		{ID: "a", Index: "1"},
		{ID: "b", Index: "2"},
	})
	m.selectAll()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})

	if cmd != nil {
		t.Fatalf("Update(a) returned unexpected command")
	}
	if got := updated.selectedCount(); got != 0 {
		t.Fatalf("selectedCount() = %d, want 0", got)
	}
	if got, want := updated.status, "cleared selection"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
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

func TestChapterItemDescriptionShowsLanguagePagesAndStableID(t *testing.T) {
	item := chapterItem{value: &source.Chapter{ID: "chapter-id", Language: "en", PageCount: 12}}
	description := item.Description()
	for _, want := range []string{"Language: en", "Pages: 12", "ID: chapter-id"} {
		if !strings.Contains(description, want) {
			t.Fatalf("Description() = %q, want %q", description, want)
		}
	}
}
