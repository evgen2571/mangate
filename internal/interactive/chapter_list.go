package interactive

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/evgen2571/mangate/internal/util"
)

const (
	chapterCursorColumnWidth    = 2
	chapterSelectionColumnWidth = 3 // dot plus the gap before the label
	chapterColumnGapWidth       = 2
	maxLanguageColumnWidth      = 8
)

type chapterColumns struct {
	labelWidth, languageWidth, pageWidth int
}

func (m *model) calculateChapterColumns(visible []int) chapterColumns {
	columns := chapterColumns{languageWidth: 1, pageWidth: lipgloss.Width("0 pages")}
	for _, index := range visible {
		chapter := m.chapters[index]
		columns.languageWidth = max(columns.languageWidth, lipgloss.Width(util.SanitizeTerminalText(chapter.Language)))
		columns.pageWidth = max(columns.pageWidth, lipgloss.Width(chapterPageCount(chapter)))
	}
	columns.languageWidth = min(columns.languageWidth, maxLanguageColumnWidth)
	fixedWidth := chapterCursorColumnWidth + chapterSelectionColumnWidth + 2*chapterColumnGapWidth + columns.languageWidth + columns.pageWidth
	columns.labelWidth = max(8, m.contentWidth()-fixedWidth)
	return columns
}

func (m *model) chapterRow(index int, current bool, columns chapterColumns, s styles) string {
	chapter := m.chapters[index]
	cursor := strings.Repeat(" ", chapterCursorColumnWidth)
	if current {
		cursor = s.cursor.Render("› ")
	}
	dot := s.muted.Render("○")
	if m.selected[index] {
		dot = s.selected.Render("●")
	} else if current {
		dot = s.accent.Render("●")
	}
	label := padTo(truncate(chapter.DisplayName(), columns.labelWidth), columns.labelWidth)
	language := padTo(clipToWidth(util.SanitizeTerminalText(chapter.Language), columns.languageWidth), columns.languageWidth)
	pages := padTo(chapterPageCount(chapter), columns.pageWidth)
	selectionGap := chapterSelectionColumnWidth - lipgloss.Width("○")
	return cursor + dot + strings.Repeat(" ", selectionGap) + label + strings.Repeat(" ", chapterColumnGapWidth) + language + strings.Repeat(" ", chapterColumnGapWidth) + pages
}

func chapterPageCount(chapter *source.Chapter) string {
	return fmt.Sprintf("%d pages", chapter.PageCount)
}

func padTo(value string, width int) string {
	return value + strings.Repeat(" ", max(0, width-lipgloss.Width(value)))
}

func clipToWidth(value string, width int) string {
	if lipgloss.Width(value) <= width {
		return value
	}
	var out strings.Builder
	for _, r := range value {
		if lipgloss.Width(out.String()+string(r)) > width {
			break
		}
		out.WriteRune(r)
	}
	return out.String()
}
