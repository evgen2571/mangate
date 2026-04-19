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
	idx   int
	value *source.Chapter
}

func (i chapterItem) FilterValue() string {
	if i.value == nil {
		return ""
	}

	return strings.TrimSpace(i.value.Title)
}

func (i chapterItem) Title() string {
	if i.value == nil {
		return fmt.Sprintf("Unknown chapter #%d", i.idx+1)
	}

	index := strings.TrimSpace(i.value.Index)
	title := strings.TrimSpace(i.value.Title)

	switch {
	case index != "" && title != "":
		return fmt.Sprintf("Chapter %s - %s", index, title)
	case title != "":
		return title
	case index != "":
		return fmt.Sprintf("Chapter %s", index)
	default:
		return fmt.Sprintf("Unknown chapter #%d", i.idx+1)
	}
}

func (i chapterItem) Description() string {
	if i.value == nil {
		return ""
	}

	return strings.TrimSpace(i.value.URL)
}

type chaptersModel struct {
	width  int
	height int

	manga *source.Manga
	keys  chaptersKeyMap
	list  list.Model
}

func newChaptersModel(manga *source.Manga, chapters []*source.Chapter) chaptersModel {
	items := make([]list.Item, 0, len(chapters))
	for i, chapter := range chapters {
		items = append(items, chapterItem{
			idx:   i,
			value: chapter,
		})
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(true)

	return chaptersModel{
		manga: manga,
		keys:  newChaptersKeyMap(),
		list:  l,
	}
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
	if m.manga == nil || strings.TrimSpace(m.manga.Title) == "" {
		return "Chapters"
	}

	return fmt.Sprintf("Chapters for %q", strings.TrimSpace(m.manga.Title))
}

func (m chaptersModel) HelpKeys(global keyMap) help.KeyMap {
	return chaptersHelpKeyMap{
		global: global,
		local:  m.keys,
	}
}
