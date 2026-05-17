package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evgen2571/mangate/internal/tuiapp"
)

func TestResultsModelFullMangaDownloadKeyRequestsSelectedManga(t *testing.T) {
	result := tuiapp.SearchResult{
		ID:           "manga-a",
		Title:        "Manga A",
		URL:          "https://example.com/manga-a",
		SummaryMD:    "A test summary.",
		ChapterCount: 42,
	}
	m := newResultsModel("query", []tuiapp.SearchResult{result})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	if cmd == nil {
		t.Fatalf("Update(f) returned nil command")
	}

	msg, ok := cmd().(fullMangaDownloadRequestedMsg)
	if !ok {
		t.Fatalf("Update(f) command returned %T, want fullMangaDownloadRequestedMsg", msg)
	}
	if msg.Result != result {
		t.Fatalf("Result = %#v, want selected result %#v", msg.Result, result)
	}
}

func TestResultsModelMetadataUsesSearchResultFields(t *testing.T) {
	m := newResultsModel("query", []tuiapp.SearchResult{{
		ID:           "manga-a",
		Title:        "Manga A",
		URL:          "https://example.com/manga-a",
		SummaryMD:    "A test summary.",
		ChapterCount: 42,
	}})

	content := m.metadataContent()
	for _, want := range []string{
		"Title: Manga A",
		"ID: manga-a",
		"URL: https://example.com/manga-a",
		"Chapters: 42",
		"A test summary.",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("metadataContent() missing %q:\n%s", want, content)
		}
	}
}
