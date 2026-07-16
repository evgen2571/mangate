package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evgen2571/mangate/internal/source"
)

func TestResultsModelFullMangaDownloadKeyRequestsSelectedManga(t *testing.T) {
	manga := &source.Manga{ID: "manga-a", Title: "Manga A"}
	m := newResultsModel("query", "mangadex", []*source.Manga{manga})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	if cmd == nil {
		t.Fatalf("Update(f) returned nil command")
	}

	msg, ok := cmd().(fullMangaDownloadRequestedMsg)
	if !ok {
		t.Fatalf("Update(f) command returned %T, want fullMangaDownloadRequestedMsg", msg)
	}
	if msg.Manga != manga {
		t.Fatalf("Manga = %#v, want selected manga", msg.Manga)
	}
}

func TestResultsModelShowsDistinguishingMetadata(t *testing.T) {
	m := newResultsModel("query", "mangadex", []*source.Manga{{
		ID: "stable-id", Title: "Primary", URL: "https://example.test/title/stable-id",
		Metadata: source.MangaMetadata{AlternativeTitle: "Alternative", ContentType: "safe", Status: "ongoing", Language: "ja", Year: 2024},
	}})

	content := m.metadataContent()
	for _, want := range []string{"Provider: mangadex", "Alternative title: Alternative", "Content type: safe", "Status: ongoing", "Language: ja", "Year: 2024", "Reference: stable-id"} {
		if !strings.Contains(content, want) {
			t.Fatalf("metadata missing %q\nmetadata:\n%s", want, content)
		}
	}
}

func TestResultsModelFiltersAlternativeTitles(t *testing.T) {
	m := newResultsModel("query", "mangadex", []*source.Manga{
		{ID: "one", Title: "Primary", Metadata: source.MangaMetadata{AlternativeTitle: "Hidden Gem"}},
		{ID: "two", Title: "Other"},
	})
	m.list.SetFilterText("hidden")
	if got := len(m.list.VisibleItems()); got != 1 {
		t.Fatalf("visible items = %d, want 1", got)
	}
}

func TestEmptyResultsExplainRecoveryActions(t *testing.T) {
	m := newResultsModel("missing", "mangadex", nil)
	m.SetSize(100, 30)
	view := m.View()
	for _, want := range []string{"No results found", "missing", "mangadex", "Esc: edit query", "q: quit"} {
		if !strings.Contains(view, want) {
			t.Fatalf("empty results view missing %q:\n%s", want, view)
		}
	}
}
