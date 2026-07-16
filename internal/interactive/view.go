package interactive

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/util"
)

func (m *model) View() string {
	if m.width > 0 && (m.width < minWidth || m.height < minHeight) {
		return m.frame("", "")
	}
	header := m.header()
	if m.loading {
		return m.frame(header, m.loadingView())
	}
	switch m.screen {
	case searchScreen:
		return m.frame(header, m.searchView())
	case resultsScreen:
		return m.frame(header, m.resultsView())
	case chaptersScreen:
		return m.frame(header, m.chaptersView())
	case formatScreen:
		return m.frame(header, m.formatView())
	case outputScreen:
		return m.frame(header, m.outputView())
	case reviewScreen:
		return m.frame(header, m.reviewView())
	case configScreen:
		return m.frame(header, m.configView())
	case workingScreen:
		return m.frame(header, m.workingView())
	case doneScreen:
		return m.frame(header, m.doneView())
	default:
		return m.frame(header, "")
	}
}

func (m *model) header() string {
	s := newStyles()
	title := map[screen]string{searchScreen: "Search", resultsScreen: "Results", chaptersScreen: "Chapters", formatScreen: "Format", outputScreen: "Output", reviewScreen: "Review", configScreen: "Settings", workingScreen: "Downloading", doneScreen: "Download result"}[m.screen]
	context := ""
	if m.manga != nil {
		context = util.SanitizeTerminalText(m.manga.Title)
	}
	line := s.brand.Render("MANGATE") + s.muted.Render("  /  ") + s.heading.Render(title)
	if context != "" {
		line += "\n" + s.muted.Render(context)
	}
	if m.screen >= resultsScreen && m.screen <= reviewScreen && m.width >= 70 {
		line += "\n" + workflow(m.screen, s)
	}
	return line
}

func workflow(current screen, s styles) string {
	steps := []struct {
		screen screen
		label  string
	}{{searchScreen, "Search"}, {resultsScreen, "Title"}, {chaptersScreen, "Chapters"}, {formatScreen, "Format"}, {reviewScreen, "Review"}}
	parts := make([]string, 0, len(steps))
	for _, step := range steps {
		if step.screen == current {
			parts = append(parts, s.accent.Render(step.label))
		} else {
			parts = append(parts, s.muted.Render(step.label))
		}
	}
	return strings.Join(parts, s.muted.Render(" › "))
}

func (m *model) searchView() string {
	s := newStyles()
	m.input.Placeholder = "Search by title"
	m.input.PromptStyle = s.accent
	return lipgloss.JoinVertical(lipgloss.Left, s.heading.Render("Search manga and manhwa"), s.muted.Render("Find a title, then choose the chapters you want."), "", s.input.Render(m.input.View()), "", s.muted.Render("Provider: "+m.app.Cfg.Provider+" · Language: "+m.app.Cfg.Language))
}

func (m *model) loadingView() string {
	s := newStyles()
	return lipgloss.JoinVertical(lipgloss.Left, s.heading.Render(m.status), "", m.spinner.View()+" "+s.muted.Render("Please wait. Press q to cancel an active download."))
}

func (m *model) resultsView() string {
	s := newStyles()
	if len(m.results) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, s.heading.Render("No titles found"), "", s.muted.Render("No matches for "+m.query+". Press esc to change the search."))
	}
	heading := fmt.Sprintf("Results for %q · %d titles", m.query, len(m.results))
	return lipgloss.JoinVertical(lipgloss.Left, s.heading.Render(heading), "", m.resultsList.View())
}

func (m *model) chaptersView() string {
	s := newStyles()
	visible := m.visibleChapters()
	summary := fmt.Sprintf("%d chapters · %d selected", len(m.chapters), len(m.selected))
	if m.chapterFilter != "" {
		summary += " · Filter: " + m.chapterFilter
	}
	parts := []string{s.heading.Render(util.SanitizeTerminalText(m.manga.Title)), s.muted.Render(summary)}
	if m.filtering {
		parts = append(parts, "", s.input.Render(m.input.View()))
	}
	if len(visible) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, append(parts, "", s.warning.Render("No chapters match this filter"), s.muted.Render("Edit the filter or press esc, then / to try again."))...)
	}
	available := max(3, m.mainHeight()-len(parts)-2)
	start := max(0, min(m.chapterCursor-available/2, len(visible)-available))
	end := min(len(visible), start+available)
	for position := start; position < end; position++ {
		index := visible[position]
		marker := "  "
		if position == m.chapterCursor {
			marker = s.cursor.Render("› ")
		}
		check := "[ ]"
		if m.selected[index] {
			check = "[x]"
		}
		label := truncate(m.chapterLabel(index), m.contentWidth()-12)
		parts = append(parts, marker+check+" "+label)
	}
	parts = append(parts, "", s.muted.Render("space toggle · a all visible · d clear · l latest · r range"))
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m *model) formatView() string {
	s := newStyles()
	descriptions := map[archive.Format]string{archive.FormatDirectory: "Keep pages as individual image files", archive.FormatCBZ: "Comic-book archive, one file per chapter", archive.FormatZIP: "Standard ZIP archive, one file per chapter"}
	parts := []string{s.heading.Render("Choose download format")}
	for _, f := range []archive.Format{archive.FormatDirectory, archive.FormatCBZ, archive.FormatZIP} {
		marker := "○"
		style := s.muted
		if f == m.format {
			marker, style = "●", s.selected
		}
		parts = append(parts, style.Render(marker+" "+string(f)), s.muted.Render("    "+descriptions[f]))
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m *model) outputView() string {
	s := newStyles()
	path := strings.TrimSpace(m.input.Value())
	preview := filepath.Join(path, downloader.TitleDirectoryName(m.manga))
	parts := []string{s.heading.Render("Output directory"), s.muted.Render("Files will be created under:"), s.muted.Render(truncate(preview, m.contentWidth())), "", s.input.Render(m.input.View())}
	if m.status != "" {
		parts = append(parts, s.danger.Render("Error: "+m.status))
	}
	parts = append(parts, "", s.muted.Render("Default: "+m.app.Cfg.Download.Dir))
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m *model) reviewView() string {
	s := newStyles()
	rows := [][2]string{{"Title", util.SanitizeTerminalText(m.manga.Title)}, {"Provider", m.app.Cfg.Provider}, {"Chapters", fmt.Sprintf("%d selected", len(m.selectedChapters()))}, {"Format", string(m.format)}, {"Output", m.app.Cfg.Download.Dir}, {"Existing files", m.app.Cfg.Download.ExistingFileMode}}
	parts := []string{s.heading.Render("Ready to download")}
	for _, row := range rows {
		parts = append(parts, s.muted.Render(fmt.Sprintf("%-16s", row[0]))+truncate(row[1], m.contentWidth()-18))
	}
	parts = append(parts, "", s.accent.Render("enter Download"))
	return s.panel.Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}

func (m *model) workingView() string {
	s := newStyles()
	pct := 0.0
	if m.progress.total > 0 {
		pct = float64(m.progress.completed) / float64(m.progress.total)
	}
	stage := "Downloading"
	if m.status == "cancelling" {
		stage = "Cancelling..."
	}
	return lipgloss.JoinVertical(lipgloss.Left, s.heading.Render(stage), "", m.spinner.View()+" "+truncate(m.progress.active, m.contentWidth()-4), "", m.progressBar.ViewAs(pct), fmt.Sprintf("Pages     %d / %d", m.progress.completed, m.progress.total), fmt.Sprintf("Chapters  %d / %d", m.progress.completedChapters, m.progress.totalChapters), "Format    "+string(m.format), "", s.warning.Render("q requests cancellation. Completed files remain."))
}

func (m *model) doneView() string {
	s := newStyles()
	heading := s.success.Render("Download complete")
	if errors.Is(m.doneErr, context.Canceled) {
		heading = s.warning.Render("Download cancelled")
	} else if m.doneErr != nil && m.doneCompleted == 0 {
		heading = s.danger.Render("Download failed")
	} else if m.doneFailed > 0 || m.doneErr != nil {
		heading = s.warning.Render("Download finished with failures")
	}
	return lipgloss.JoinVertical(lipgloss.Left, heading, "", m.doneViewport.View(), "", s.muted.Render("enter return to chapters · q quit"))
}

func (m *model) configView() string {
	s := newStyles()
	parts := []string{s.heading.Render("Settings")}
	for i, label := range configLabels {
		marker := "  "
		style := s.muted
		if i == m.configCursor {
			marker, style = "› ", s.selected
		}
		parts = append(parts, style.Render(marker+fmt.Sprintf("%-20s", label)+truncate(m.configValueAt(i), m.contentWidth()-24)))
	}
	if m.configEditing {
		parts = append(parts, "", s.input.Render(m.input.View()))
	}
	parts = append(parts, "", s.muted.Render("a apply for this session · s save permanently"))
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func truncate(value string, width int) string {
	if width < 4 {
		return "..."
	}
	runes := []rune(util.SanitizeTerminalText(value))
	if len(runes) <= width {
		return string(runes)
	}
	return string(runes[:width-3]) + "..."
}
