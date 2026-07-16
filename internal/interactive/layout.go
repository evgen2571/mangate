package interactive

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

const (
	minWidth        = 48
	minHeight       = 14
	maxContentWidth = 92
)

func (m *model) contentWidth() int {
	if m.width <= 0 {
		return 72
	}
	w := m.width - 6
	if w > maxContentWidth {
		w = maxContentWidth
	}
	return max(24, w)
}

func (m *model) mainHeight() int {
	if m.height <= 0 {
		return 12
	}
	return max(3, m.height-10)
}

func (m *model) resize() {
	w := m.contentWidth()
	m.input.Width = max(12, w-6)
	m.resultsList.SetSize(w, m.mainHeight())
	m.doneViewport.Width, m.doneViewport.Height = w-2, m.mainHeight()
	m.progressBar.Width = max(12, min(52, w-8))
}

func (m *model) frame(header, body string) string {
	if m.width > 0 && (m.width < minWidth || m.height < minHeight) {
		message := "Terminal is too small\n\nResize to at least " + itoa(minWidth) + " × " + itoa(minHeight) + ".\nCurrent: " + itoa(m.width) + " × " + itoa(m.height) + "\n\nq quit"
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, message)
	}
	s := newStyles()
	w := m.contentWidth()
	footer := m.help.ShortHelpView(m.bindings())
	if m.showHelp {
		footer = m.help.FullHelpView([][]key.Binding{m.bindings()}) + "\n\n" + s.muted.Render("esc close help")
	}
	status := ""
	if m.status != "" {
		status = s.status.Render(m.status)
	}
	content := lipgloss.JoinVertical(lipgloss.Left, header, "", body, "", status, "", s.description.Render(footer))
	if m.width <= 0 || m.height <= 0 {
		return content + "\n"
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, lipgloss.NewStyle().Width(w).Render(content))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func itoa(value int) string { return fmt.Sprintf("%d", value) }
