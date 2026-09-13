package interactive

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/evgen2571/mangate/internal/util"
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
	m.input.Width = m.inputContentWidth()
	m.input.SetValue(m.input.Value())
	m.resultsList.SetSize(m.resultListWidth(), m.mainHeight())
	m.doneViewport.Width, m.doneViewport.Height = w-2, max(1, m.mainHeight()-2)
	m.doneViewport.SetContent(lipgloss.NewStyle().Width(w - 2).Render(m.completion))
	m.progressBar.Width = max(12, min(52, w-8))
}

// inputContentWidth is the stable inner width shared by every framed input.
// The border and its horizontal padding are deliberately excluded so typing
// never changes the outer field dimensions.
func (m *model) inputContentWidth() int {
	return max(1, m.contentWidth()-4-lipgloss.Width(m.input.Prompt))
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
		body = m.help.FullHelpView([][]key.Binding{m.bindings()})
		footer = "esc close help"
	}
	status := ""
	if m.status != "" {
		status = s.status.Render(util.SanitizeTerminalText(m.status))
	}
	if m.height > 0 && m.height < 24 {
		header = strings.Split(header, "\n")[0]
		parts := []string{header, body}
		if status != "" {
			parts = append(parts, ansi.Truncate(status, w, "…"))
		}
		parts = append(parts, ansi.Truncate(footer, w, "…"))
		content := lipgloss.JoinVertical(lipgloss.Left, parts...)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, lipgloss.NewStyle().Width(w).Render(content))
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
