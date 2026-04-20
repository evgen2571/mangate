package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evgen2571/mangate/internal/constant"
	"github.com/evgen2571/mangate/internal/source"
)

type chapterItem struct {
	idx      int
	value    *source.Chapter
	selected bool
}

func (i chapterItem) FilterValue() string {
	if i.value == nil {
		return ""
	}

	parts := []string{
		strings.TrimSpace(i.value.Index),
		strings.TrimSpace(i.value.Title),
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func (i chapterItem) Title() string {
	var text string

	if i.value == nil {
		text = fmt.Sprintf("Unknown chapter #%d", i.idx+1)
	} else {
		index := strings.TrimSpace(i.value.Index)
		title := strings.TrimSpace(i.value.Title)

		switch {
		case index != "" && title != "":
			text = fmt.Sprintf("Chapter %s - %s", index, title)
		case title != "":
			text = title
		case index != "":
			text = fmt.Sprintf("Chapter %s", index)
		default:
			text = fmt.Sprintf("Unknown chapter #%d", i.idx+1)
		}
	}

	if i.selected {
		marker := lipgloss.NewStyle().Foreground(constant.LogoColor).Bold(true).Render("● ")
		styledText := lipgloss.NewStyle().Foreground(constant.LogoColor).Bold(true).Render(text)
		return marker + styledText
	}

	marker := lipgloss.NewStyle().Foreground(constant.MutedColor).Render("○ ")
	return marker + text
}

func (i chapterItem) Description() string {
	if i.value == nil {
		return ""
	}

	description := strings.TrimSpace(i.value.URL)
	if i.selected {
		return lipgloss.NewStyle().Foreground(constant.InputBorderColor).Render(description)
	}

	return description
}

type chaptersModel struct {
	width  int
	height int

	manga    *source.Manga
	keys     chaptersKeyMap
	list     list.Model
	chapters []*source.Chapter
	selected map[string]bool
	status   string
}

func newChaptersModel(manga *source.Manga, chapters []*source.Chapter) chaptersModel {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(true)

	m := chaptersModel{
		manga:    manga,
		keys:     newChaptersKeyMap(),
		list:     l,
		chapters: chapters,
		selected: make(map[string]bool),
	}
	m.syncListItems()
	return m
}

func (m *chaptersModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.list.SetSize(m.panelContentWidth(), m.listHeight())
}

func (m chaptersModel) Update(msg tea.Msg) (chaptersModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Back):
			return m, func() tea.Msg { return goBackMsg{} }
		case key.Matches(msg, m.keys.Toggle):
			m.toggleSelectionAt(m.list.Index())
			return m, nil
		case key.Matches(msg, m.keys.Download):
			chapters := m.chaptersForDownload()
			if len(chapters) == 0 {
				return m, nil
			}
			return m, func() tea.Msg {
				return downloadRequestedMsg{Manga: m.manga, Chapters: chapters}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m chaptersModel) View() string {
	contentWidth := m.panelContentWidth()
	contentHeight := m.panelContentHeight()

	footer := lipgloss.NewStyle().
		Width(contentWidth).
		Padding(0, 1).
		Foreground(constant.MutedColor).
		Render(m.footerText())

	footerHeight := lipgloss.Height(footer)
	listHeight := max(1, contentHeight-footerHeight)
	m.list.SetSize(contentWidth, listHeight)

	inner := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().
			Width(contentWidth).
			Height(listHeight).
			Render(m.list.View()),
		footer,
	)

	return lipgloss.NewStyle().
		Width(contentWidth).
		Height(contentHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(constant.OuterBorderColor).
		Render(inner)
}

func (m chaptersModel) panelWidth() int {
	return max(1, m.width)
}

func (m chaptersModel) panelHeight() int {
	return max(1, m.height)
}

func (m chaptersModel) panelContentWidth() int {
	return max(1, m.panelWidth()-2)
}

func (m chaptersModel) panelContentHeight() int {
	return max(1, m.panelHeight()-2)
}

func (m chaptersModel) listHeight() int {
	return max(1, m.panelContentHeight()-1)
}

func (m chaptersModel) footerText() string {
	parts := []string{}
	if m.manga == nil || strings.TrimSpace(m.manga.Title) == "" {
		parts = append(parts, "Chapters")
	} else {
		parts = append(parts, fmt.Sprintf("Chapters for %q", strings.TrimSpace(m.manga.Title)))
	}

	selectedCount := m.selectedCount()
	if selectedCount > 0 {
		parts = append(parts, fmt.Sprintf("selected: %d", selectedCount))
		parts = append(parts, "enter downloads selected")
	} else {
		parts = append(parts, "enter downloads current")
	}
	parts = append(parts, "space toggles selection")

	if strings.TrimSpace(m.status) != "" {
		parts = append(parts, m.status)
	}

	return strings.Join(parts, " • ")
}

func (m chaptersModel) HelpKeys(global keyMap) help.KeyMap {
	return chaptersHelpKeyMap{
		global: global,
		local:  m.keys,
	}
}

func (m *chaptersModel) setStatus(status string) {
	m.status = strings.TrimSpace(status)
}

func (m *chaptersModel) clearSelection() {
	m.selected = make(map[string]bool)
	m.syncListItems()
}

func (m *chaptersModel) toggleSelectionAt(index int) {
	chapter := m.chapterAt(index)
	if chapter == nil {
		return
	}

	key := chapterSelectionKey(chapter, index)
	m.selected[key] = !m.selected[key]
	if !m.selected[key] {
		delete(m.selected, key)
	}
	m.syncListItems()
}

func (m chaptersModel) chaptersForDownload() []*source.Chapter {
	if m.selectedCount() == 0 {
		chapter := m.chapterAt(m.list.Index())
		if chapter == nil {
			return nil
		}
		return []*source.Chapter{chapter}
	}

	chapters := make([]*source.Chapter, 0, len(m.selected))
	for idx, chapter := range m.chapters {
		if chapter == nil {
			continue
		}
		if !m.selected[chapterSelectionKey(chapter, idx)] {
			continue
		}
		chapters = append(chapters, chapter)
	}
	return chapters
}

func (m chaptersModel) selectedCount() int {
	return len(m.selected)
}

func (m chaptersModel) chapterAt(index int) *source.Chapter {
	if index < 0 || index >= len(m.chapters) {
		return nil
	}
	return m.chapters[index]
}

func (m *chaptersModel) syncListItems() {
	items := make([]list.Item, 0, len(m.chapters))
	for i, chapter := range m.chapters {
		items = append(items, chapterItem{
			idx:      i,
			value:    chapter,
			selected: m.selected[chapterSelectionKey(chapter, i)],
		})
	}
	m.list.SetItems(items)
}

func chapterSelectionKey(chapter *source.Chapter, idx int) string {
	if chapter == nil {
		return fmt.Sprintf("idx:%d", idx)
	}
	if strings.TrimSpace(chapter.ID) != "" {
		return chapter.ID
	}
	return fmt.Sprintf("idx:%d", idx)
}
