package interactive

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/evgen2571/mangate/internal/util"
)

type resultItem struct{ manga *source.Manga }

func (i resultItem) Title() string {
	if i.manga == nil {
		return "Unknown title"
	}
	return util.SanitizeTerminalText(i.manga.Title)
}
func (i resultItem) Description() string {
	if i.manga == nil {
		return ""
	}
	m := i.manga.Metadata
	parts := []string{m.AlternativeTitle, m.ContentType, m.Language}
	if m.Year > 0 {
		parts = append(parts, fmt.Sprintf("%d", m.Year))
	}
	return util.SanitizeTerminalText(strings.Join(nonEmpty(parts), " · "))
}
func (i resultItem) FilterValue() string { return i.Title() + " " + i.Description() }

func (m *model) newResultsList(results []*source.Manga) {
	items := make([]list.Item, 0, len(results))
	for _, manga := range results {
		items = append(items, resultItem{manga})
	}
	d := list.NewDefaultDelegate()
	d.SetHeight(2)
	d.Styles.SelectedTitle = d.Styles.SelectedTitle.Copy().Foreground(lipgloss.Color("12")).Bold(true)
	d.Styles.SelectedDesc = d.Styles.SelectedDesc.Copy().Foreground(lipgloss.Color("8"))
	m.resultsList = list.New(items, d, max(1, m.contentWidth()), max(1, m.mainHeight()))
	m.resultsList.Title = ""
	m.resultsList.SetShowTitle(false)
	m.resultsList.SetShowStatusBar(false)
	m.resultsList.SetShowPagination(true)
	m.resultsList.SetShowHelp(false)
	m.resultsList.DisableQuitKeybindings()
}

func nonEmpty(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			out = append(out, part)
		}
	}
	return out
}
