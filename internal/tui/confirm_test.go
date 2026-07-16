package tui

import (
	"strings"
	"testing"

	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/source"
)

func TestConfirmationShowsLanguageAndReleaseDetails(t *testing.T) {
	m := newConfirmModel(config.DefaultConfig(), &source.Manga{Title: "Example"}, []*source.Chapter{
		{Index: "1", Language: "en", ReleaseGroup: "Group B"},
		{Index: "1", Language: "ja", ReleaseGroup: "Group A"},
		{Index: "2", Language: "en", ReleaseGroup: "Group A"},
	}, archive.FormatCBZ)
	m.SetSize(120, 40)
	view := m.View()
	for _, want := range []string{"Languages: en, ja", "Release groups: Group A, Group B"} {
		if !strings.Contains(view, want) {
			t.Fatalf("confirmation missing %q:\n%s", want, view)
		}
	}
}
