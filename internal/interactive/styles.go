package interactive

import (
	"github.com/charmbracelet/lipgloss"
)

type styles struct {
	brand, heading, accent, muted, selected, status, success, warning, danger lipgloss.Style
	input, panel, key, description, cursor                                    lipgloss.Style
}

func newStyles() styles {
	accent := lipgloss.Color("12")
	return styles{
		brand:       lipgloss.NewStyle().Bold(true).Foreground(accent),
		heading:     lipgloss.NewStyle().Bold(true),
		accent:      lipgloss.NewStyle().Foreground(accent),
		muted:       lipgloss.NewStyle().Faint(true),
		selected:    lipgloss.NewStyle().Bold(true).Foreground(accent),
		status:      lipgloss.NewStyle().Foreground(lipgloss.Color("14")),
		success:     lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true),
		warning:     lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
		danger:      lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true),
		input:       lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(accent).Padding(0, 1),
		panel:       lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("8")).Padding(0, 1),
		key:         lipgloss.NewStyle().Foreground(accent).Bold(true),
		description: lipgloss.NewStyle().Faint(true),
		cursor:      lipgloss.NewStyle().Foreground(accent).Bold(true),
	}
}
