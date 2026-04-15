package tui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/evgen2571/mangate/internal/source"
)

type chaptersListKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Toggle   key.Binding
	Download key.Binding
	Back     key.Binding
	Quit     key.Binding
}

func newChaptersListKeyMap() chaptersListKeyMap {
	return chaptersListKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Toggle: key.NewBinding(
			key.WithKeys(" ", "x"),
			key.WithHelp("space/x", "select"),
		),
		Download: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "download selected"),
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

type chaptersListModel struct {
	manga    *source.Manga
	items    []*source.Chapter
	cursor   int
	selected map[int]bool
	loading  bool
	err      error
	keys     chaptersListKeyMap
	styles   uiStyles
	width    int
	height   int
}

func newChaptersListModel() chaptersListModel {
	return chaptersListModel{
		keys:     newChaptersListKeyMap(),
		styles:   newUIStyles(),
		selected: make(map[int]bool),
		cursor:   0,
	}
}

func (m chaptersListModel) Init() tea.Cmd {
	return nil
}

func (m chaptersListModel) Update(msg tea.Msg) (chaptersListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Back):
			if m.loading {
				return m, nil
			}
			return m, func() tea.Msg { return backToMangasMsg{} }

		case key.Matches(msg, m.keys.Up):
			if !m.loading && m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case key.Matches(msg, m.keys.Down):
			if !m.loading && m.cursor < len(m.items)-1 {
				m.cursor++
			}
			return m, nil

		case key.Matches(msg, m.keys.Toggle):
			if m.loading || len(m.items) == 0 {
				return m, nil
			}
			m.selected[m.cursor] = !m.selected[m.cursor]
			if !m.selected[m.cursor] {
				delete(m.selected, m.cursor)
			}
			return m, nil

		case key.Matches(msg, m.keys.Download):
			if m.loading || len(m.items) == 0 {
				return m, nil
			}

			selected := m.selectedChapters()
			if len(selected) == 0 {
				selected = []*source.Chapter{m.items[m.cursor]}
			}

			return m, func() tea.Msg {
				return chaptersDownloadRequestedMsg{
					manga:    m.manga,
					chapters: selected,
				}
			}
		}
	}

	return m, nil
}

func (m chaptersListModel) renderPane(totalW, totalH int) string {
	frameW, frameH := m.styles.Pane.GetFrameSize()
	innerW := max(1, totalW-frameW)
	innerH := max(1, totalH-frameH)

	title := m.styles.PaneTitle.Render("Chapters")

	var bottom string
	if m.manga != nil {
		bottom = m.styles.Muted.Render(
			fmt.Sprintf("%s • selected: %d • total: %d",
				sanitizeChapterText(m.manga.Title),
				m.selectedCount(),
				len(m.items),
			),
		)
	} else {
		bottom = m.styles.Muted.Render(
			fmt.Sprintf("Selected: %d • total: %d", m.selectedCount(), len(m.items)),
		)
	}

	headerH := lipgloss.Height(title)
	footerH := lipgloss.Height(bottom)
	middleH := max(1, innerH-headerH-footerH)

	var middle string
	switch {
	case m.loading:
		middle = lipgloss.Place(
			innerW,
			middleH,
			lipgloss.Center,
			lipgloss.Top,
			m.styles.Muted.Render("Loading chapters..."),
		)
	case m.err != nil:
		middle = lipgloss.Place(
			innerW,
			middleH,
			lipgloss.Center,
			lipgloss.Top,
			m.styles.Error.Render("Error: "+m.err.Error()),
		)
	case len(m.items) == 0:
		middle = lipgloss.Place(
			innerW,
			middleH,
			lipgloss.Center,
			lipgloss.Top,
			m.styles.Muted.Render("No chapters found."),
		)
	default:
		middle = m.renderVisibleRows(innerW, middleH)
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		middle,
		bottom,
	)

	return m.styles.Pane.
		Width(totalW).
		Height(totalH).
		Render(content)
}

func (m chaptersListModel) renderVisibleRows(innerW, middleH int) string {
	if len(m.items) == 0 || middleH <= 0 {
		return ""
	}

	rowHeight := 2
	visibleCount := max(1, middleH/rowHeight)
	start, end := visibleWindow(len(m.items), m.cursor, visibleCount)

	rowBoxW := min(innerW, 96)
	rows := make([]string, 0, end-start)

	for i := start; i < end; i++ {
		chapter := m.items[i]

		cardStyle := m.styles.ListCard
		titleStyle := m.styles.ListTitle

		isSelected := m.selected[i]
		isCursor := i == m.cursor

		switch {
		case isSelected && isCursor:
			cardStyle = m.styles.ListCardSelectedActive
			titleStyle = m.styles.ListTitleSelected
		case isSelected:
			cardStyle = m.styles.ListCardSelected
			titleStyle = m.styles.ListTitleSelected
		case isCursor:
			cardStyle = m.styles.ListCardActive
			titleStyle = m.styles.ListTitleActive
		}

		cardFrameW, _ := cardStyle.GetFrameSize()
		rowInnerW := max(1, rowBoxW-cardFrameW)

		cursorMark := " "
		if isCursor {
			cursorMark = "›"
		}

		indexText := formatChapterIndex(chapter)
		indexBlock := m.styles.Index.Render(
			lipgloss.NewStyle().
				Width(8).
				Align(lipgloss.Right).
				Render(indexText),
		)

		checkboxText := "[ ]"
		if isSelected {
			checkboxText = "[x]"
		}
		checkboxBlock := m.styles.Index.Render(
			lipgloss.NewStyle().
				Width(3).
				Align(lipgloss.Left).
				Render(checkboxText),
		)

		leftBlock := lipgloss.JoinHorizontal(
			lipgloss.Top,
			cursorMark,
			" ",
			indexBlock,
		)
		leftW := lipgloss.Width(leftBlock)
		rightW := lipgloss.Width(checkboxBlock)

		titleColW := max(1, rowInnerW-leftW-rightW-4)
		titleText := truncateWidth(sanitizeChapterText(chapter.Title), titleColW)
		titleBlock := titleStyle.
			Width(titleColW).
			Align(lipgloss.Center).
			Render(titleText)

		row := lipgloss.JoinHorizontal(
			lipgloss.Top,
			leftBlock,
			" ",
			titleBlock,
			" ",
			checkboxBlock,
		)

		rows = append(rows, cardStyle.Width(rowInnerW).Render(row))
	}

	listBlock := lipgloss.JoinVertical(lipgloss.Left, rows...)

	return lipgloss.Place(
		innerW,
		middleH,
		lipgloss.Center,
		lipgloss.Top,
		listBlock,
	)
}

func visibleWindow(total, cursor, size int) (int, int) {
	if total <= 0 || size <= 0 {
		return 0, 0
	}
	if size >= total {
		return 0, total
	}

	start := cursor - size/2
	if start < 0 {
		start = 0
	}

	end := start + size
	if end > total {
		end = total
		start = end - size
		if start < 0 {
			start = 0
		}
	}

	return start, end
}

func formatChapterIndex(chapter *source.Chapter) string {
	if chapter == nil {
		return "-"
	}

	index := strings.TrimSpace(sanitizeChapterText(fmt.Sprint(chapter.Index)))
	if index == "" {
		return "-"
	}

	return index
}

func sanitizeChapterText(s string) string {
	replacer := strings.NewReplacer(
		"\x1b", "",
		"\u001b", "",
		"\n", " ",
		"\r", " ",
		"\t", " ",
	)

	s = replacer.Replace(s)

	ansiCSI := regexp.MustCompile(`\[[0-9;]*[A-Za-z]`)
	s = ansiCSI.ReplaceAllString(s, "")

	brokenANSI := regexp.MustCompile(`\[[0-9;]*m`)
	s = brokenANSI.ReplaceAllString(s, "")

	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

func (m chaptersListModel) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	footer := m.styles.Footer.Width(m.width).Render(
		"↑/↓ move • space select • Enter download • Esc back • q quit",
	)
	footerH := lipgloss.Height(footer)

	outerX := 1
	outerY := 1

	availableW := m.width - outerX*2
	availableH := m.height - outerY*2 - footerH - 1
	if availableW < 40 || availableH < 12 {
		return ""
	}

	pane := m.renderPane(availableW, availableH)

	return lipgloss.NewStyle().
		Margin(outerY, outerX, 0, outerX).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				pane,
				"",
				footer,
			),
		)
}

func (m chaptersListModel) selectedCount() int {
	return len(m.selected)
}

func (m chaptersListModel) selectedChapters() []*source.Chapter {
	out := make([]*source.Chapter, 0, len(m.selected))
	for i, ch := range m.items {
		if m.selected[i] {
			out = append(out, ch)
		}
	}
	return out
}
