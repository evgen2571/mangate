package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/evgen2571/manga-downloader/internal/source"
)

type mangasListKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Select   key.Binding
	Download key.Binding
	Back     key.Binding
	Quit     key.Binding
}

func newMangasListKeyMap() mangasListKeyMap {
	return mangasListKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "chapters"),
		),
		Download: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "download manga"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

type mangasListModel struct {
	items  []*source.Manga
	cursor int
	offset int
	query  string
	keys   mangasListKeyMap
	styles uiStyles
	err    error
	width  int
	height int
}

func newMangasListModel() mangasListModel {
	return mangasListModel{
		keys:   newMangasListKeyMap(),
		styles: newUIStyles(),
	}
}

func (m mangasListModel) Init() tea.Cmd {
	return nil
}

func (m *mangasListModel) visibleRows() int {
	if m.width <= 0 || m.height <= 0 {
		return 1
	}

	footer := m.styles.Footer.Width(m.width).Render(
		"↑/↓ move • Enter chapters • d download • Esc back • q quit",
	)
	footerH := lipgloss.Height(footer)

	outerY := 1
	availableH := m.height - outerY*2 - footerH - 1
	if availableH < 12 {
		return 1
	}

	frameW, frameH := m.styles.Pane.GetFrameSize()
	_ = frameW

	innerH := max(1, availableH-frameH)

	titleBlock := m.styles.PaneTitle.Render("Results")
	queryBlock := m.styles.Muted.Render(`Query: "` + m.query + `"`)

	middleH := max(1, innerH-lipgloss.Height(titleBlock)-lipgloss.Height(queryBlock))

	rowStyle := m.styles.ListCard
	_, rowFrameH := rowStyle.GetFrameSize()
	rowHeight := max(1, 1+rowFrameH)

	return max(1, middleH/rowHeight)
}

func (m *mangasListModel) clampScroll() {
	if len(m.items) == 0 {
		m.cursor = 0
		m.offset = 0
		return
	}

	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
	}

	visible := m.visibleRows()
	if visible < 1 {
		visible = 1
	}

	maxOffset := max(0, len(m.items)-visible)
	if m.offset < 0 {
		m.offset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}

	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visible {
		m.offset = m.cursor - visible + 1
	}

	if m.offset < 0 {
		m.offset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
}

func (m mangasListModel) Update(msg tea.Msg) (mangasListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Back):
			return m, func() tea.Msg { return backToSearchMsg{} }

		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
			m.clampScroll()
			return m, nil

		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
			m.clampScroll()
			return m, nil

		case key.Matches(msg, m.keys.Select):
			if len(m.items) == 0 {
				return m, nil
			}

			selected := m.items[m.cursor]
			return m, func() tea.Msg {
				return mangaSelectedMsg{manga: selected}
			}

		case key.Matches(msg, m.keys.Download):
			if len(m.items) == 0 {
				return m, nil
			}

			selected := m.items[m.cursor]
			return m, func() tea.Msg {
				return mangaDownloadRequestedMsg{manga: selected}
			}
		}
	}

	return m, nil
}

func (m mangasListModel) selectedManga() *source.Manga {
	if len(m.items) == 0 || m.cursor < 0 || m.cursor >= len(m.items) {
		return nil
	}
	return m.items[m.cursor]
}

func truncateWidth(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxW {
		return s
	}
	if maxW == 1 {
		return "…"
	}

	r := []rune(s)
	for len(r) > 0 {
		candidate := string(r) + "…"
		if lipgloss.Width(candidate) <= maxW {
			return candidate
		}
		r = r[:len(r)-1]
	}

	return "…"
}

func (m mangasListModel) renderListPane(totalW, totalH int) string {
	frameW, frameH := m.styles.Pane.GetFrameSize()
	innerW := max(1, totalW-frameW)
	innerH := max(1, totalH-frameH)

	titleBlock := m.styles.PaneTitle.Render("Results")
	queryBlock := m.styles.Muted.Render(`Query: "` + m.query + `"`)

	if len(m.items) == 0 {
		middleH := max(1, innerH-lipgloss.Height(titleBlock)-lipgloss.Height(queryBlock))
		middle := lipgloss.Place(
			innerW,
			middleH,
			lipgloss.Left,
			lipgloss.Center,
			m.styles.Muted.Render("No manga found."),
		)

		content := lipgloss.JoinVertical(
			lipgloss.Left,
			titleBlock,
			middle,
			queryBlock,
		)

		return m.styles.Pane.Width(innerW).Height(innerH).Render(content)
	}

	rowStyle := m.styles.ListCard
	_, rowFrameH := rowStyle.GetFrameSize()
	rowHeight := max(1, 1+rowFrameH)

	middleH := max(1, innerH-lipgloss.Height(titleBlock)-lipgloss.Height(queryBlock))
	visibleRows := max(1, middleH/rowHeight)

	offset := m.offset
	if offset < 0 {
		offset = 0
	}
	maxOffset := max(0, len(m.items)-visibleRows)
	if offset > maxOffset {
		offset = maxOffset
	}

	start := offset
	end := min(len(m.items), start+visibleRows)

	var listRows []string
	selected := m.cursor

	for i := start; i < end; i++ {
		manga := m.items[i]

		cardStyle := m.styles.ListCard
		titleStyle := m.styles.ListTitle
		marker := " "
		if i == selected {
			cardStyle = m.styles.ListCardActive
			titleStyle = m.styles.ListTitleActive
			marker = "▌"
		}

		cardFrameW, _ := cardStyle.GetFrameSize()
		rowInnerW := max(1, innerW-cardFrameW)

		index := m.styles.Index.Render(
			lipgloss.NewStyle().
				Width(3).
				Align(lipgloss.Right).
				Render(fmt.Sprintf("%02d", i+1)),
		)

		prefix := lipgloss.JoinHorizontal(
			lipgloss.Top,
			index,
			" ",
			marker,
			" ",
		)

		prefixW := lipgloss.Width(prefix)

		plainTitle := strings.ReplaceAll(manga.Title, "\n", " ")
		titleW := max(1, rowInnerW-prefixW-3)
		plainTitle = truncateWidth(plainTitle, titleW)

		row := prefix + titleStyle.Render(plainTitle)
		listRows = append(listRows, cardStyle.Width(rowInnerW).Render(row))
	}

	listContent := lipgloss.JoinVertical(lipgloss.Left, listRows...)

	if end < len(m.items) {
		listContent = lipgloss.JoinVertical(
			lipgloss.Left,
			listContent,
			m.styles.Muted.Render(fmt.Sprintf("... %d more", len(m.items)-end)),
		)
	}

	middle := lipgloss.Place(
		innerW,
		middleH,
		lipgloss.Left,
		lipgloss.Top,
		listContent,
	)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		titleBlock,
		middle,
		queryBlock,
	)

	return m.styles.Pane.Width(innerW).Height(innerH).Render(content)
}

func (m mangasListModel) renderCoverPane(totalW, totalH int) string {
	frameW, frameH := m.styles.CoverBox.GetFrameSize()
	innerW := max(1, totalW-frameW)
	innerH := max(1, totalH-frameH)

	title := m.styles.PaneTitle.Render("Cover")

	body := m.styles.Muted.Render("No cover available")
	if m.selectedManga() != nil {
		body = lipgloss.Place(
			innerW,
			max(1, innerH-lipgloss.Height(title)-1),
			lipgloss.Center,
			lipgloss.Center,
			m.styles.Muted.Render("[ cover preview ]"),
		)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, title, body)
	return m.styles.CoverBox.Width(innerW).Height(innerH).Render(content)
}

func (m mangasListModel) renderMetaPane(totalW, totalH int) string {
	frameW, frameH := m.styles.MetaBox.GetFrameSize()
	innerW := max(1, totalW-frameW)
	innerH := max(1, totalH-frameH)

	selected := m.selectedManga()
	title := m.styles.SectionTitle.Render("Details")

	if selected == nil {
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			m.styles.Muted.Render("No manga selected."),
		)
		return m.styles.MetaBox.Width(innerW).Height(innerH).Render(content)
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		m.styles.Label.Render("Title"),
		selected.Title,
	)

	return m.styles.MetaBox.Width(innerW).Height(innerH).Render(content)
}

func (m mangasListModel) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	m.clampScroll()

	footer := m.styles.Footer.Width(m.width).Render(
		"↑/↓ move • Enter chapters • d download • Esc back • q quit",
	)
	footerH := lipgloss.Height(footer)

	outerX := 1
	outerY := 1
	gap := 1

	availableW := m.width - outerX*2
	availableH := m.height - outerY*2 - footerH - 1
	if availableW < 40 || availableH < 12 {
		return ""
	}

	leftW := availableW / 2
	rightW := availableW - leftW - gap

	coverH := (availableH * 2) / 3
	metaH := availableH - coverH - gap

	leftPane := m.renderListPane(leftW, availableH)
	coverPane := m.renderCoverPane(rightW, coverH)
	metaPane := m.renderMetaPane(rightW, metaH)

	rightPane := lipgloss.JoinVertical(
		lipgloss.Left,
		coverPane,
		lipgloss.NewStyle().Height(gap).Render(""),
		metaPane,
	)

	layout := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftPane,
		lipgloss.NewStyle().Width(gap).Render(""),
		rightPane,
	)

	return lipgloss.NewStyle().
		Margin(outerY, outerX, 0, outerX).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				layout,
				"",
				footer,
			),
		)
}
