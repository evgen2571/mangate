package tui

import (
	"fmt"
	"strings"

	"charm.land/glamour/v2"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evgen2571/mangate/internal/constant"
	"github.com/evgen2571/mangate/internal/source"
)

type resultItem struct {
	idx   int
	value *source.Manga
}

type coverState struct {
	Loading bool
	Path    string
	Render  string
	Err     error
}

func (i resultItem) FilterValue() string {
	if i.value == nil {
		return ""
	}
	return i.value.Title
}

func (i resultItem) Title() string {
	if i.value == nil || strings.TrimSpace(i.value.Title) == "" {
		return fmt.Sprintf("Unknown #%d", i.idx+1)
	}
	return strings.TrimSpace(i.value.Title)
}

func (i resultItem) Description() string {
	if i.value == nil {
		return ""
	}
	return strings.TrimSpace(i.value.URL)
}

type resultsModel struct {
	width       int
	height      int
	initialized bool

	query        string
	keys         resultsKeyMap
	list         list.Model
	metadata     viewport.Model
	coverSpinner spinner.Model
	covers       map[string]coverState
	results      []*source.Manga
}

type resultsLayout struct {
	leftContentWidth    int
	rightContentWidth   int
	leftContentHeight   int
	topContentHeight    int
	bottomContentHeight int
}

func newResultsModel(query string, results []*source.Manga) resultsModel {
	items := make([]list.Item, 0, len(results))
	for i, r := range results {
		items = append(items, resultItem{
			idx:   i,
			value: r,
		})
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(true)

	vp := viewport.New(0, 0)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(constant.LogoColor)

	return resultsModel{
		query:        query,
		keys:         newResultsKeyMap(),
		list:         l,
		metadata:     vp,
		coverSpinner: s,
		covers:       make(map[string]coverState),
		results:      results,
		initialized:  true,
	}
}

func (m resultsModel) layout() resultsLayout {
	availableHeight := max(8, m.height-2)
	gap := 1
	availableWidth := max(40, m.width-gap)

	// Give more space to the cover than to the list.
	leftOuterWidth := max(24, availableWidth*38/100)
	rightOuterWidth := availableWidth - leftOuterWidth

	leftContentWidth := max(1, leftOuterWidth-2)
	rightContentWidth := max(1, rightOuterWidth-2)

	leftOuterHeight := availableHeight
	rightTopOuterHeight := max(8, availableHeight*72/100)
	rightBottomOuterHeight := availableHeight - rightTopOuterHeight

	leftContentHeight := max(1, leftOuterHeight-2)
	topContentHeight := max(1, rightTopOuterHeight-2)
	bottomContentHeight := max(1, rightBottomOuterHeight-2)

	return resultsLayout{
		leftContentWidth:    leftContentWidth,
		rightContentWidth:   rightContentWidth,
		leftContentHeight:   leftContentHeight,
		topContentHeight:    topContentHeight,
		bottomContentHeight: bottomContentHeight,
	}
}

func (m *resultsModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	if !m.initialized {
		return
	}

	l := m.layout()

	footerHeight := 1
	listHeight := max(1, l.leftContentHeight-footerHeight)
	m.list.SetSize(l.leftContentWidth, listHeight)

	metadataBodyHeight := max(1, l.bottomContentHeight-2)
	m.metadata.Width = l.rightContentWidth
	m.metadata.Height = metadataBodyHeight

	m.syncMetadataViewport()
}

func (m resultsModel) Update(msg tea.Msg) (resultsModel, tea.Cmd) {
	var cmds []tea.Cmd

	if m.isCoverLoading() {
		var spinCmd tea.Cmd
		m.coverSpinner, spinCmd = m.coverSpinner.Update(msg)
		if spinCmd != nil {
			cmds = append(cmds, spinCmd)
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Back):
			return m, tea.Batch(append(cmds, func() tea.Msg { return goBackMsg{} })...)

		case key.Matches(msg, m.keys.Select):
			selected := m.selectedManga()
			if selected == nil {
				return m, tea.Batch(cmds...)
			}

			return m, tea.Batch(append(cmds, func() tea.Msg {
				return chaptersOpenRequestedMsg{Manga: selected}
			})...)

		case key.Matches(msg, m.keys.Download):
			selected := m.selectedManga()
			if selected == nil {
				return m, tea.Batch(cmds...)
			}

			return m, tea.Batch(append(cmds, func() tea.Msg {
				return fullMangaDownloadRequestedMsg{Manga: selected}
			})...)

		case key.Matches(msg, m.keys.MetaUp):
			m.metadata.LineUp(5)
			return m, tea.Batch(cmds...)

		case key.Matches(msg, m.keys.MetaDown):
			m.metadata.LineDown(5)
			return m, tea.Batch(cmds...)
		}
	}

	prevIndex := m.list.Index()

	var listCmd tea.Cmd
	m.list, listCmd = m.list.Update(msg)
	if listCmd != nil {
		cmds = append(cmds, listCmd)
	}

	if m.list.Index() != prevIndex {
		m.syncMetadataViewport()

		selected := m.selectedManga()
		if selected != nil {
			cmds = append(cmds, func() tea.Msg {
				return coverLoadRequestedMsg{MangaID: selected.ID}
			})
		}
	}

	return m, tea.Batch(cmds...)
}

func (m resultsModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading results UI..."
	}

	l := m.layout()

	footer := lipgloss.NewStyle().
		Width(l.leftContentWidth).
		Padding(0, 1).
		Foreground(constant.MutedColor).
		Render(fmt.Sprintf("Results for %q", m.query))

	footerHeight := lipgloss.Height(footer)
	listHeight := max(1, l.leftContentHeight-footerHeight)

	m.list.SetSize(l.leftContentWidth, listHeight)

	leftInner := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().
			Width(l.leftContentWidth).
			Height(listHeight).
			Render(m.list.View()),
		footer,
	)

	leftPanel := lipgloss.NewStyle().
		Width(l.leftContentWidth).
		Height(l.leftContentHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(constant.OuterBorderColor).
		Render(leftInner)

	coverPanel := lipgloss.NewStyle().
		Width(l.rightContentWidth).
		Height(l.topContentHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(constant.InputBorderColor).
		Render(m.coverView())

	metadataPanel := lipgloss.NewStyle().
		Width(l.rightContentWidth).
		Height(l.bottomContentHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(constant.InputBorderColor).
		Render(m.metadataView())

	rightColumn := lipgloss.JoinVertical(
		lipgloss.Left,
		coverPanel,
		metadataPanel,
	)

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftPanel,
		lipgloss.NewStyle().Width(1).Render(""),
		rightColumn,
	)
}

func (m resultsModel) selectedItem() *resultItem {
	item, ok := m.list.SelectedItem().(resultItem)
	if !ok {
		return nil
	}
	return &item
}

func (m resultsModel) coverView() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Padding(0, 1).
		Foreground(constant.LogoColor).
		Render("Cover")

	bodyWidth, bodyHeight := m.coverBodySize()
	selected := m.selectedManga()

	var body string

	switch {
	case selected == nil:
		body = renderCoverPlaceholder(bodyWidth, bodyHeight, "No cover")

	case m.covers[selected.ID].Loading:
		body = m.coverLoadingView(bodyWidth, bodyHeight)

	case m.covers[selected.ID].Err != nil:
		body = renderCoverPlaceholder(bodyWidth, bodyHeight, "Cover unavailable")

	case strings.TrimSpace(m.covers[selected.ID].Render) == "":
		body = renderCoverPlaceholder(bodyWidth, bodyHeight, "No cover")

	default:
		body = lipgloss.Place(
			bodyWidth,
			bodyHeight,
			lipgloss.Center,
			lipgloss.Center,
			m.covers[selected.ID].Render,
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		body,
	)
}

func (m resultsModel) metadataView() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Padding(0, 1).
		Foreground(constant.LogoColor).
		Render("Metadata")

	body := lipgloss.NewStyle().
		Foreground(constant.TextColor).
		Render(m.metadata.View())

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		body,
	)
}

func (m *resultsModel) syncMetadataViewport() {
	m.metadata.SetContent(m.metadataContent())
	m.metadata.GotoTop()
}

func (m resultsModel) metadataContent() string {
	item := m.selectedItem()
	if item == nil || item.value == nil {
		return "No result selected"
	}

	desc := ""
	if item.value.Metadata.Description != nil {
		if en, ok := item.value.Metadata.Description["en"]; ok {
			desc = strings.TrimSpace(en)
		} else {
			for _, v := range item.value.Metadata.Description {
				desc = strings.TrimSpace(v)
				if desc != "" {
					break
				}
			}
		}
	}

	header := []string{
		fmt.Sprintf("Title: %s", item.value.Title),
		fmt.Sprintf("ID: %s", item.value.ID),
		fmt.Sprintf("URL: %s", item.value.URL),
	}
	if item.value.Metadata.ChapterCount > 0 {
		header = append(header, fmt.Sprintf("Chapters: %d", item.value.Metadata.ChapterCount))
	}
	header = append(header,
		"",
		"Description:",
		"",
	)

	return strings.Join(header, "\n") + m.renderMarkdown(desc)
}

func (m resultsModel) rightPanelInnerWidth() int {
	return m.layout().rightContentWidth
}

func (m resultsModel) renderMarkdown(input string) string {
	if strings.TrimSpace(input) == "" {
		return "No description"
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithWordWrap(max(20, m.rightPanelInnerWidth()-2)),
	)
	if err != nil {
		return input
	}

	out, err := r.Render(input)
	if err != nil {
		return input
	}

	return out
}

func (m resultsModel) selectedManga() *source.Manga {
	item := m.selectedItem()
	if item == nil {
		return nil
	}
	return item.value
}

func (m resultsModel) coverBodySize() (int, int) {
	l := m.layout()

	// title + blank line
	bodyHeight := max(1, l.topContentHeight-2)

	return l.rightContentWidth, bodyHeight
}

func (m *resultsModel) setCoverLoading(mangaID string) {
	prev := m.covers[mangaID]
	prev.Loading = true
	prev.Err = nil
	m.covers[mangaID] = prev

	m.coverSpinner = spinner.New()
	m.coverSpinner.Spinner = spinner.Dot
	m.coverSpinner.Style = lipgloss.NewStyle().Foreground(constant.LogoColor)
}

func (m *resultsModel) setCoverLoaded(mangaID, path, render string) {
	m.covers[mangaID] = coverState{
		Loading: false,
		Path:    path,
		Render:  render,
		Err:     nil,
	}
}

func (m *resultsModel) setCoverFailed(mangaID string, err error) {
	prev := m.covers[mangaID]
	prev.Loading = false
	prev.Err = err
	m.covers[mangaID] = prev
}

func (m resultsModel) isCoverLoading() bool {
	selected := m.selectedManga()
	if selected == nil {
		return false
	}

	state, ok := m.covers[selected.ID]
	return ok && state.Loading
}

func (m resultsModel) coverLoadingView(width, height int) string {
	body := fmt.Sprintf("%s Loading cover...", m.coverSpinner.View())

	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(constant.TextColor).
		Render(body)
}
