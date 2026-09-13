package util

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSanitizeStringPortableNames(t *testing.T) {
	for input, want := range map[string]string{
		".": "unknown", "name. ": "name", "con": "_con", "NUL.txt": "_NUL.txt",
		"COM1": "_COM1", "lpt9.png": "_lpt9.png", "COM¹": "_COM¹", "LPT²": "_LPT²",
		"COM10": "COM10", "chapter\x00name\x1b": "chapter_name",
	} {
		if got := SanitizeString(input); got != want {
			t.Errorf("SanitizeString(%q) = %q, want %q", input, got, want)
		}
	}
	for _, input := range []string{strings.Repeat("a", 400), strings.Repeat("界", 200)} {
		got := SanitizeString(input)
		if len(got) > 200 || !utf8.ValidString(got) {
			t.Errorf("long name has %d bytes, valid UTF-8 = %v", len(got), utf8.ValidString(got))
		}
		if got == SanitizeString(input+"b") {
			t.Error("different long names collide")
		}
		if SanitizeString(got) != got {
			t.Error("sanitizing twice changes the name")
		}
	}
}

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
