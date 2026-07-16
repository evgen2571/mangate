package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/lipgloss"
	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/constant"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/evgen2571/mangate/internal/util"
)

func (m confirmModel) HelpKeys(global keyMap) help.KeyMap { return confirmHelpKeyMap{global: global} }

type confirmModel struct {
	manga             *source.Manga
	chapters          []*source.Chapter
	provider          string
	format            archive.Format
	output            string
	existing          string
	retainSource      bool
	expectedPages     int
	unknownPageCounts int
	plannedPaths      []string
	existingPaths     []string
	width             int
	height            int
}

func newConfirmModel(cfg config.Config, manga *source.Manga, chapters []*source.Chapter, format archive.Format) confirmModel {
	model := confirmModel{
		manga:        manga,
		chapters:     chapters,
		provider:     cfg.Provider,
		format:       format,
		output:       cfg.Download.Dir,
		existing:     cfg.Download.ExistingFileMode,
		retainSource: cfg.Download.RetainSource,
	}
	if manga == nil {
		return model
	}
	titleDir := filepath.Join(cfg.Download.Dir, downloader.TitleDirectoryName(manga))
	chapterNames := downloader.ChapterDirectoryNames(chapters)
	for index, chapter := range chapters {
		if chapter == nil {
			continue
		}
		if chapter.PageCount > 0 {
			model.expectedPages += chapter.PageCount
		} else {
			model.unknownPageCounts++
		}
		path := filepath.Join(titleDir, chapterNames[index])
		if format != archive.FormatDirectory {
			path += format.Extension()
		}
		model.plannedPaths = append(model.plannedPaths, path)
		if _, err := os.Stat(path); err == nil {
			model.existingPaths = append(model.existingPaths, path)
		}
	}
	return model
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
	}
	if m.expectedPages > 0 {
		line := fmt.Sprintf("Known pages: %d", m.expectedPages)
		if m.unknownPageCounts > 0 {
			line += fmt.Sprintf(" (%d chapter(s) unknown)", m.unknownPageCounts)
		}
		lines = append(lines, line)
	} else {
		lines = append(lines, "Known pages: unknown")
	}
	if m.format == archive.FormatDirectory {
		lines = append(lines, "Source pages: kept as the requested directory output")
	} else if m.retainSource {
		lines = append(lines, "Source pages: retained after archive validation")
	} else {
		lines = append(lines, "Source pages: removed after archive validation")
	}
	if len(m.plannedPaths) > 0 {
		lines = append(lines, "Planned outputs:")
		for _, path := range m.plannedPaths {
			lines = append(lines, "  "+util.SanitizeTerminalText(path))
		}
	}
	if len(m.existingPaths) > 0 {
		lines = append(lines, fmt.Sprintf("Existing outputs: %d (%s policy)", len(m.existingPaths), util.SanitizeTerminalText(m.existing)))
	}
	lines = append(lines, "", "enter: start download  esc: change format  ctrl+g: edit output settings")
	return lipgloss.NewStyle().
		Width(max(1, m.width-2)).
		Height(max(1, m.height-2)).
		Padding(1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(constant.OuterBorderColor).
		Render(strings.Join(lines, "\n"))
}
