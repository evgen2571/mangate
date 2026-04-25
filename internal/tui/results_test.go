package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evgen2571/mangate/internal/source"
)

func TestResultsModelFullMangaDownloadKeyRequestsSelectedManga(t *testing.T) {
	manga := &source.Manga{ID: "manga-a", Title: "Manga A"}
	m := newResultsModel("query", []*source.Manga{manga})

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
