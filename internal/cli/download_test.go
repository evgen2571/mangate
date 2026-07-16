package cli

import (
	"strings"
	"testing"

	"github.com/evgen2571/mangate/internal/source"
)

func TestSelectChaptersUsesNumericRangeAndStableIDs(t *testing.T) {
	chapters := []*source.Chapter{
		{ID: "one", Index: "1"},
		{ID: "two", Index: "2"},
		{ID: "ten", Index: "10"},
	}
	selected, err := selectChapters(chapters, chapterSelection{Range: "2-10"})
	if err != nil {
		t.Fatalf("selectChapters() error = %v", err)
	}
	if len(selected) != 2 || selected[0].ID != "two" || selected[1].ID != "ten" {
		t.Fatalf("selected = %#v, want chapters two and ten", selected)
	}

	selected, err = selectChapters(chapters, chapterSelection{IDs: []string{"ten"}})
	if err != nil || len(selected) != 1 || selected[0].ID != "ten" {
		t.Fatalf("stable ID selection = %#v, %v", selected, err)
	}
}

func TestSelectChaptersRejectsAmbiguousNumber(t *testing.T) {
	_, err := selectChapters([]*source.Chapter{{ID: "a", Index: "1"}, {ID: "b", Index: "1"}}, chapterSelection{Numbers: []string{"1"}})
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("selectChapters() error = %v, want ambiguity error", err)
	}
}
