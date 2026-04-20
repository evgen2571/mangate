package tui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

func TestTruncateTextTruncatesWideRunesToDisplayWidth(t *testing.T) {
	input := "進撃の巨人 特別版"

	got := truncateText(input, 10)

	if got == input {
		t.Fatalf("expected wide text to be truncated, got %q", got)
	}
	if width := lipgloss.Width(got); width > 10 {
		t.Fatalf("expected truncated text width <= 10, got %d for %q", width, got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("expected ellipsis suffix, got %q", got)
	}
}

func TestChapterProgressListKeepsWideTitleOnSingleLine(t *testing.T) {
	m := newDownloadingModel("Downloading pages", "Now: 進撃の巨人 特別版", nil)
	m.chapters = []chapterProgressView{{
		Name:           "進撃の巨人 特別版",
		CompletedPages: 3,
		TotalPages:     12,
		Active:         true,
	}}

	rendered := stripANSI(m.chapterProgressList(20, 3))
	lines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")

	nonEmpty := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmpty = append(nonEmpty, line)
		}
	}

	if len(nonEmpty) != 1 {
		t.Fatalf("expected one rendered chapter line, got %d lines: %#v", len(nonEmpty), nonEmpty)
	}
	if width := lipgloss.Width(nonEmpty[0]); width > 20 {
		t.Fatalf("expected rendered line width <= 20, got %d for %q", width, nonEmpty[0])
	}
}

func TestChapterProgressListClampsVeryNarrowWidths(t *testing.T) {
	m := newDownloadingModel("Downloading pages", "Now: 進撃の巨人 特別版", nil)
	m.chapters = []chapterProgressView{{
		Name:           "進撃の巨人 特別版",
		CompletedPages: 3,
		TotalPages:     12,
		Active:         true,
	}}

	rendered := stripANSI(m.chapterProgressList(8, 3))
	lines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if width := lipgloss.Width(line); width > 8 {
			t.Fatalf("expected rendered line width <= 8, got %d for %q", width, line)
		}
	}
}
