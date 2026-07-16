package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/constant"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/evgen2571/mangate/internal/util"
)

type confirmModel struct {
	manga    *source.Manga
	chapters []*source.Chapter
	provider string
	format   archive.Format
	output   string
	existing string
	width    int
	height   int
}

func (m *confirmModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m confirmModel) View() string {
	title := "Unknown title"
	if m.manga != nil && strings.TrimSpace(m.manga.Title) != "" {
		title = util.SanitizeTerminalText(m.manga.Title)
	}
	labels := make([]string, 0, len(m.chapters))
	for _, chapter := range m.chapters {
		if chapter != nil {
			labels = append(labels, chapter.DisplayName())
		}
	}
	lines := []string{
		"Review download",
		fmt.Sprintf("Provider: %s", util.SanitizeTerminalText(m.provider)),
		fmt.Sprintf("Title: %s", title),
		fmt.Sprintf("Chapters: %d", len(labels)),
		fmt.Sprintf("Selection: %s", strings.Join(labels, ", ")),
		fmt.Sprintf("Format: %s", m.format),
		fmt.Sprintf("Output: %s", util.SanitizeTerminalText(m.output)),
		fmt.Sprintf("Existing files: %s", util.SanitizeTerminalText(m.existing)),
		"",
		"enter: start download  esc: change format",
	}
	return lipgloss.NewStyle().
		Width(max(1, m.width-2)).
		Height(max(1, m.height-2)).
		Padding(1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(constant.OuterBorderColor).
		Render(strings.Join(lines, "\n"))
}
