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
	"github.com/evgen2571/mangate/internal/tuiapp"
)

type chapterItem struct {
	idx      int
	value    tuiapp.ChapterItem
	selected bool
}

func (i chapterItem) FilterValue() string {
	if strings.TrimSpace(i.value.DisplayText) != "" {
		return i.value.DisplayText
	}
	if strings.TrimSpace(i.value.Title) != "" {
		return i.value.Title
	}
	return strings.TrimSpace(i.value.Index)
}

func (i chapterItem) Title() string {
	text := strings.TrimSpace(i.value.DisplayText)
	if text == "" {
		text = strings.TrimSpace(i.value.Title)
	}
	if text == "" {
		text = fmt.Sprintf("Chapter %d", i.idx+1)
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
	if !isChapterItemSet(i.value) {
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

	manga    tuiapp.MangaDetails
	keys     chaptersKeyMap
	list     list.Model
	chapters []tuiapp.ChapterItem
	selected map[string]bool
	status   string
}

func newChaptersModel(manga tuiapp.MangaDetails, chapters []tuiapp.ChapterItem) chaptersModel {
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
		case key.Matches(msg, m.keys.SelectAll):
			if m.allChaptersSelected() {
				m.deselectAll()
				m.setStatus("cleared selection")
				return m, nil
			}
			m.selectAll()
			m.setStatus("selected all chapters")
			return m, nil
		case key.Matches(msg, m.keys.DeselectAll):
			m.deselectAll()
			m.setStatus("cleared selection")
			return m, nil
		case key.Matches(msg, m.keys.Download):
			chapters := m.chaptersForDownload()
			if len(chapters) == 0 {
				m.setStatus("no chapter selected")
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
	chapterCount := m.chapterCount()
	if strings.TrimSpace(m.manga.Title) == "" {
		parts = append(parts, "Chapters")
	} else {
		parts = append(parts, fmt.Sprintf("Chapters for %q", strings.TrimSpace(m.manga.Title)))
	}
	parts = append(parts, fmt.Sprintf("chapters: %d", chapterCount))

	selectedCount := m.selectedCount()
	if selectedCount > 0 {
		parts = append(parts, fmt.Sprintf("selected: %d", selectedCount))
		parts = append(parts, "enter: selected")
	} else {
		parts = append(parts, "enter: current")
	}
	parts = append(parts, "space: toggle")

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
	m.deselectAll()
}

func (m *chaptersModel) selectAll() {
	if m.allChaptersSelected() {
		m.deselectAll()
		return
	}

	m.selected = make(map[string]bool)
	for idx, chapter := range m.chapters {
		if !isChapterItemSet(chapter) {
			continue
		}
		m.selected[chapterSelectionKey(chapter, idx)] = true
	}
	m.syncListItems()
}

func (m *chaptersModel) deselectAll() {
	m.selected = make(map[string]bool)
	m.syncListItems()
}

func (m *chaptersModel) toggleSelectionAt(index int) {
	chapter := m.chapterAt(index)
	if !isChapterItemSet(chapter) {
		return
	}

	key := chapterSelectionKey(chapter, index)
	m.selected[key] = !m.selected[key]
	if !m.selected[key] {
		delete(m.selected, key)
	}
	m.syncListItems()
}

func (m chaptersModel) chaptersForDownload() []tuiapp.ChapterItem {
	if m.selectedCount() == 0 {
		chapter := m.chapterAt(m.list.Index())
		if !isChapterItemSet(chapter) {
			return nil
		}
		return []tuiapp.ChapterItem{chapter}
	}

	chapters := make([]tuiapp.ChapterItem, 0, len(m.selected))
	for idx, chapter := range m.chapters {
		if !isChapterItemSet(chapter) {
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

func (m chaptersModel) allChaptersSelected() bool {
	if m.chapterCount() == 0 {
		return false
	}

	for idx, chapter := range m.chapters {
		if !isChapterItemSet(chapter) {
			continue
		}
		if !m.selected[chapterSelectionKey(chapter, idx)] {
			return false
		}
	}
	return true
}

func (m chaptersModel) chapterCount() int {
	count := 0
	for _, chapter := range m.chapters {
		if isChapterItemSet(chapter) {
			count++
		}
	}
	return count
}

func (m chaptersModel) chapterAt(index int) tuiapp.ChapterItem {
	if index < 0 || index >= len(m.chapters) {
		return tuiapp.ChapterItem{}
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

func downloadDetailText(chapters []tuiapp.ChapterItem) string {
	count := len(chapters)
	if count == 1 {
		chapter := chapters[0]
		for _, text := range []string{chapter.DisplayText, chapter.Title, chapter.Index, chapter.ID} {
			if text != "" {
				return text
			}
		}
		return "1 chapter selected"
	}
	return fmt.Sprintf("%d chapters selected", count)
}

func nonNilChapters(chapters []tuiapp.ChapterItem) []tuiapp.ChapterItem {
	result := make([]tuiapp.ChapterItem, 0, len(chapters))
	for _, chapter := range chapters {
		if !isChapterItemSet(chapter) {
			continue
		}
		result = append(result, chapter)
	}
	return result
}

func chapterSelectionKey(chapter tuiapp.ChapterItem, idx int) string {
	if strings.TrimSpace(chapter.ID) != "" {
		return chapter.ID
	}
	if strings.TrimSpace(chapter.Index) != "" {
		return fmt.Sprintf("%s#%d", chapter.Index, idx)
	}
	return fmt.Sprintf("idx:%d", idx)
}

func isChapterItemSet(chapter tuiapp.ChapterItem) bool {
	return strings.TrimSpace(chapter.ID) != "" ||
		strings.TrimSpace(chapter.Index) != "" ||
		strings.TrimSpace(chapter.Title) != "" ||
		strings.TrimSpace(chapter.DisplayText) != "" ||
		strings.TrimSpace(chapter.URL) != ""
}
