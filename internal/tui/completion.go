package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/evgen2571/mangate/internal/constant"
	"github.com/evgen2571/mangate/internal/util"
)

type completionModel struct {
	width    int
	height   int
	success  bool
	summary  string
	paths    []string
	outcomes []chapterOutcome
	error    string
}

func (m *completionModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m completionModel) View() string {
	title := "Download incomplete"
	if m.success {
		title = "Download complete"
	}
	lines := []string{title, "", util.SanitizeTerminalText(m.summary)}
	if m.error != "" {
		lines = append(lines, "", "Error: "+util.SanitizeTerminalText(m.error))
	}
	if len(m.outcomes) > 0 {
		completed, skipped, incomplete, archiveFailures := completionCounts(m.outcomes)
		lines = append(lines, fmt.Sprintf("Completed: %d", completed), fmt.Sprintf("Skipped/reused: %d", skipped), fmt.Sprintf("Failed or incomplete: %d", incomplete), fmt.Sprintf("Archive failures: %d", archiveFailures))
	}
	if len(m.paths) > 0 {
		lines = append(lines, "", "Outputs:")
		for index, path := range m.paths {
			status := "complete"
			if index < len(m.outcomes) {
				status = m.outcomes[index].Status
			}
			lines = append(lines, fmt.Sprintf("  [%s] %s", util.SanitizeTerminalText(status), util.SanitizeTerminalText(path)))
		}
	}
	lines = append(lines, "", "enter or esc: return to chapters   q: exit")
	return lipgloss.NewStyle().
		Width(max(1, m.width-2)).
		Height(max(1, m.height-2)).
		Padding(1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(constant.OuterBorderColor).
		Render(strings.Join(lines, "\n"))
}

func completionSummary(count int, format string) string {
	if count == 1 {
		return fmt.Sprintf("1 chapter completed as %s.", format)
	}
	return fmt.Sprintf("%d chapters completed as %s.", count, format)
}

func completionCounts(outcomes []chapterOutcome) (completed, skipped, incomplete, archiveFailures int) {
	for _, outcome := range outcomes {
		switch outcome.Status {
		case "complete":
			completed++
		case "skipped":
			skipped++
		case "archive_failed":
			archiveFailures++
			incomplete++
		default:
			incomplete++
		}
	}
	return completed, skipped, incomplete, archiveFailures
}
