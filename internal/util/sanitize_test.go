package util

import "testing"

func TestSanitizeStringRemovesDotDotPathTraversal(t *testing.T) {
	got := SanitizeString("../evil")
	if got == "..evil" || got == "../evil" || got == ".." {
		t.Fatalf("SanitizeString() = %q, want traversal segments removed", got)
	}
}

func TestSanitizeStringRemovesTildeWithoutExtraSeparator(t *testing.T) {
	got := SanitizeString("Chapter ~ 1")
	if got != "Chapter-1" {
		t.Fatalf("SanitizeString() = %q, want %q", got, "Chapter-1")
	}
}

func TestSanitizeStringCollapsesWhitespaceAndTrimsSeparators(t *testing.T) {
	got := SanitizeString("  Chapter\t\n  Name  ")
	if got != "Chapter-Name" {
		t.Fatalf("SanitizeString() = %q, want %q", got, "Chapter-Name")
	}
}

func TestSanitizeStringCollapsesRepeatedHyphensAndUnderscores(t *testing.T) {
	got := SanitizeString("__Chapter--Name__")
	if got != "Chapter-Name" {
		t.Fatalf("SanitizeString() = %q, want %q", got, "Chapter-Name")
	}
}

func TestSanitizeTerminalTextReplacesControlSequences(t *testing.T) {
	got := SanitizeTerminalText("Title\x1b[2J\nnext\tcell")
	want := "Title�[2J�next�cell"
	if got != want {
		t.Fatalf("SanitizeTerminalText() = %q, want %q", got, want)
	}
}
