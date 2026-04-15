package tui

import (
	"fmt"
	"strings"

	"charm.land/glamour/v2"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
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

	query    string
	keys     resultsKeyMap
	list     list.Model
	metadata viewport.Model
	results  []*source.Manga
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

	return resultsModel{
		query:       query,
		keys:        newResultsKeyMap(),
		list:        l,
		metadata:    vp,
		results:     results,
		initialized: true,
	}
}

func (m *resultsModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	if !m.initialized {
		return
	}

	availableHeight := max(8, height-2)
	gap := 1
	availableWidth := max(40, width-gap)

	leftOuterWidth := availableWidth / 2
	rightOuterWidth := availableWidth - leftOuterWidth

	leftContentWidth := max(1, leftOuterWidth-2)
	rightContentWidth := max(1, rightOuterWidth-2)

	leftOuterHeight := availableHeight
	rightTopOuterHeight := availableHeight / 2
	rightBottomOuterHeight := availableHeight - rightTopOuterHeight

	leftContentHeight := max(1, leftOuterHeight-2)
	bottomContentHeight := max(1, rightBottomOuterHeight-2)

	footerHeight := 1
	listHeight := max(1, leftContentHeight-footerHeight)
	m.list.SetSize(leftContentWidth, listHeight)

	// metadata panel content height includes title + spacer + viewport body
	metadataBodyHeight := max(1, bottomContentHeight-2)
	m.metadata.Width = rightContentWidth
	m.metadata.Height = metadataBodyHeight

	m.syncMetadataViewport()
}

func (m resultsModel) Update(msg tea.Msg) (resultsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Back):
			return m, func() tea.Msg { return goBackMsg{} }

		case key.Matches(msg, m.keys.MetaUp):
			m.metadata.LineUp(5)
			return m, nil

		case key.Matches(msg, m.keys.MetaDown):
			m.metadata.LineDown(5)
			return m, nil
		}
	}

	prevIndex := m.list.Index()

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	if m.list.Index() != prevIndex {
		m.syncMetadataViewport()
	}

	return m, cmd
}

func (m resultsModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading results UI..."
	}

	availableHeight := max(8, m.height-2)

	gap := 1

	// Split total OUTER width between left and right columns.
	availableWidth := max(40, m.width-gap)
	leftOuterWidth := availableWidth / 2
	rightOuterWidth := availableWidth - leftOuterWidth

	leftContentWidth := max(1, leftOuterWidth-2)
	rightContentWidth := max(1, rightOuterWidth-2)

	// Split total OUTER height so left panel and right column have identical total height.
	leftOuterHeight := availableHeight
	rightTopOuterHeight := availableHeight / 2
	rightBottomOuterHeight := availableHeight - rightTopOuterHeight

	leftContentHeight := max(1, leftOuterHeight-2)
	topContentHeight := max(1, rightTopOuterHeight-2)
	bottomContentHeight := max(1, rightBottomOuterHeight-2)

	footer := lipgloss.NewStyle().
		Width(leftContentWidth).
		Padding(0, 1).
		Foreground(constant.MutedColor).
		Render(fmt.Sprintf("Results for %q", m.query))

	footerHeight := lipgloss.Height(footer)
	listHeight := max(1, leftContentHeight-footerHeight)

	m.list.SetSize(leftContentWidth, listHeight)

	leftInner := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().
			Width(leftContentWidth).
			Height(listHeight).
			Render(m.list.View()),
		footer,
	)

	leftPanel := lipgloss.NewStyle().
		Width(leftContentWidth).
		Height(leftContentHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(constant.OuterBorderColor).
		Render(leftInner)

	coverPanel := lipgloss.NewStyle().
		Width(rightContentWidth).
		Height(topContentHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(constant.InputBorderColor).
		Render(m.coverView())

	metadataPanel := lipgloss.NewStyle().
		Width(rightContentWidth).
		Height(bottomContentHeight).
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
		lipgloss.NewStyle().Width(gap).Render(""),
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

	body := lipgloss.NewStyle().
		Foreground(constant.MutedColor).
		Render("[ cover here ]")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		lipgloss.PlaceHorizontal(max(0, m.rightPanelInnerWidth()), lipgloss.Center, body),
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
		"",
		"Description:",
		"",
	}

	return strings.Join(header, "\n") + m.renderMarkdown(desc)
}

func (m resultsModel) rightPanelInnerWidth() int {
	gap := 1
	availableWidth := max(40, m.width-gap)
	leftOuterWidth := availableWidth / 2
	rightOuterWidth := availableWidth - leftOuterWidth

	return max(1, rightOuterWidth-2)
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
