package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evgen2571/mangate/internal/constant"
)

type chapterProgressView struct {
	Name           string
	CompletedPages int
	TotalPages     int
	Active         bool
	Completed      bool
}

type downloadingModel struct {
	width  int
	height int

	title       string
	detail      string
	status      string
	completed   int
	total       int
	chapters    []chapterProgressView
	progressBar progress.Model
	progressCh  chan tea.Msg
}

func newDownloadingModel(title, detail string, progressCh chan tea.Msg) downloadingModel {
	bar := progress.New(progress.WithDefaultGradient())
	bar.Full = '█'
	bar.Empty = '░'
	bar.ShowPercentage = true
	bar.Width = 40

	return downloadingModel{
		title:       title,
		detail:      detail,
		status:      "Preparing download...",
		progressBar: bar,
		progressCh:  progressCh,
	}
}

func (m *downloadingModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.progressBar.Width = max(12, m.panelContentWidth()-2)
}

func (m downloadingModel) Init() tea.Cmd {
	return nil
}

func (m downloadingModel) Update(msg tea.Msg) (downloadingModel, tea.Cmd) {
	switch msg := msg.(type) {
	case downloadProgressMsg:
		m.title = msg.Title
		m.detail = msg.Detail
		m.status = msg.Status
		m.completed = msg.Completed
		m.total = msg.Total
		m.chapters = msg.Chapters

		percent := 0.0
		if msg.Total > 0 {
			percent = float64(msg.Completed) / float64(msg.Total)
		}

		cmd := m.progressBar.SetPercent(percent)
		return m, cmd

	case progress.FrameMsg:
		var cmd tea.Cmd
		progressModel, progressCmd := m.progressBar.Update(msg)
		m.progressBar = progressModel.(progress.Model)
		cmd = progressCmd
		return m, cmd
	}

	return m, nil
}

func (m downloadingModel) View() string {
	contentWidth := m.panelContentWidth()
	contentHeight := m.panelContentHeight()

	title := lipgloss.NewStyle().
		Width(contentWidth).
		Bold(true).
		Foreground(constant.LogoColor).
		Render(truncateText(m.title, contentWidth))

	detail := lipgloss.NewStyle().
		Width(contentWidth).
		Foreground(constant.TextColor).
		Render(truncateText(m.detail, contentWidth))

	status := lipgloss.NewStyle().
		Width(contentWidth).
		Foreground(constant.MutedColor).
		Render(truncateText(m.progressText(), contentWidth))

	bar := lipgloss.NewStyle().
		Width(contentWidth).
		Render(m.progressBar.View())

	chapterListHeight := max(3, contentHeight-6)
	chapterList := lipgloss.NewStyle().
		Width(contentWidth).
		Height(chapterListHeight).
		Foreground(constant.TextColor).
		Render(m.chapterProgressList(contentWidth, chapterListHeight))

	inner := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		detail,
		"",
		bar,
		status,
		"",
		chapterList,
	)

	panel := lipgloss.NewStyle().
		Width(contentWidth).
		Height(contentHeight).
		Padding(1, 3).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(constant.OuterBorderColor).
		Render(inner)

	if m.width == 0 || m.height == 0 {
		return panel
	}

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		panel,
	)
}

func (m downloadingModel) progressText() string {
	parts := []string{}
	if strings.TrimSpace(m.status) != "" {
		parts = append(parts, m.status)
	}
	if m.total > 0 {
		parts = append(parts, fmt.Sprintf("%d/%d pages", m.completed, m.total))
	}
	active := m.activeChapterNames()
	if len(active) > 0 {
		parts = append(parts, "active: "+strings.Join(active, ", "))
	}
	return strings.Join(parts, " • ")
}

func (m downloadingModel) activeChapterNames() []string {
	names := make([]string, 0)
	for _, chapter := range m.chapters {
		if chapter.Active {
			names = append(names, chapter.Name)
		}
	}
	return names
}

func (m downloadingModel) chapterProgressList(width, height int) string {
	if len(m.chapters) == 0 || width <= 0 || height <= 0 {
		return ""
	}

	visible := m.visibleChapters(height)
	nameWidth := max(8, width-14)
	lines := make([]string, 0, len(visible)+1)

	for _, chapter := range visible {
		marker := lipgloss.NewStyle().Foreground(constant.MutedColor).Render("○")
		nameStyle := lipgloss.NewStyle().Foreground(constant.TextColor)

		switch {
		case chapter.Completed:
			marker = lipgloss.NewStyle().Foreground(constant.LogoColor).Bold(true).Render("✓")
			nameStyle = lipgloss.NewStyle().Foreground(constant.LogoColor).Bold(true)
		case chapter.Active:
			marker = lipgloss.NewStyle().Foreground(constant.InputBorderColor).Bold(true).Render("●")
			nameStyle = lipgloss.NewStyle().Foreground(constant.InputBorderColor).Bold(true)
		}

		name := truncateText(chapter.Name, nameWidth)
		progressText := fmt.Sprintf("(%d/%d)", chapter.CompletedPages, chapter.TotalPages)
		line := fmt.Sprintf("%s %s %s", marker, nameStyle.Render(name), progressText)
		lines = append(lines, truncateText(line, width))
	}

	hidden := len(m.chapters) - len(visible)
	if hidden > 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(constant.MutedColor).Render(fmt.Sprintf("... and %d more chapter(s)", hidden)))
	}

	body := strings.Join(lines, "\n")
	return lipgloss.NewStyle().Width(width).Height(height).Render(body)
}

func (m downloadingModel) visibleChapters(height int) []chapterProgressView {
	if len(m.chapters) <= height {
		return m.chapters
	}

	visibleSlots := max(1, height-1)
	visible := make([]chapterProgressView, 0, visibleSlots)

	for _, chapter := range m.chapters {
		if chapter.Active {
			visible = append(visible, chapter)
			if len(visible) == visibleSlots {
				return visible
			}
		}
	}

	for _, chapter := range m.chapters {
		if chapter.Active {
			continue
		}
		visible = append(visible, chapter)
		if len(visible) == visibleSlots {
			break
		}
	}

	return visible
}

func (m downloadingModel) panelWidth() int {
	if m.width == 0 {
		return 72
	}
	return max(44, min(84, m.width-6))
}

func (m downloadingModel) panelHeight() int {
	if m.height == 0 {
		return 16
	}
	return max(14, min(20, m.height-2))
}

func (m downloadingModel) panelContentWidth() int {
	return max(1, m.panelWidth()-8)
}

func (m downloadingModel) panelContentHeight() int {
	return max(1, m.panelHeight()-4)
}

func (m downloadingModel) HelpKeys(global keyMap) help.KeyMap {
	return loadingHelpKeyMap{global: global}
}

func (m downloadingModel) waitForMsgCmd() tea.Cmd {
	if m.progressCh == nil {
		return nil
	}

	return func() tea.Msg {
		msg, ok := <-m.progressCh
		if !ok {
			return nil
		}
		return msg
	}
}

func truncateText(s string, width int) string {
	s = strings.TrimSpace(s)
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width <= 3 {
		return strings.Repeat(".", width)
	}

	runes := []rune(s)
	if len(runes) <= width {
		return s
	}

	return string(runes[:width-3]) + "..."
}
