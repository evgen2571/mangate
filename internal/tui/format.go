package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/constant"
)

type formatModel struct {
	formats []archive.Format
	index   int
	width   int
	height  int
}

func newFormatModel(current string) formatModel {
	formats := []archive.Format{archive.FormatDirectory, archive.FormatCBZ, archive.FormatZIP}
	model := formatModel{formats: formats}
	for index, format := range formats {
		if string(format) == strings.ToLower(strings.TrimSpace(current)) {
			model.index = index
			break
		}
	}
	return model
}

func (m *formatModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *formatModel) move(offset int) {
	m.index = (m.index + offset + len(m.formats)) % len(m.formats)
}

func (m formatModel) selected() archive.Format {
	return m.formats[m.index]
}

func (m formatModel) View() string {
	lines := []string{"Choose output format"}
	for index, format := range m.formats {
		marker := "  "
		if index == m.index {
			marker = "> "
		}
		lines = append(lines, marker+formatLabel(format))
	}
	lines = append(lines, "", "j/k or arrows: move  enter: continue  esc: back")
	return lipgloss.NewStyle().
		Width(max(1, m.width-2)).
		Height(max(1, m.height-2)).
		Padding(1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(constant.OuterBorderColor).
		Render(strings.Join(lines, "\n"))
}

func formatLabel(format archive.Format) string {
	switch format {
	case archive.FormatDirectory:
		return "Directory  Keep pages as separate image files"
	case archive.FormatCBZ:
		return "CBZ        One comic-book archive per chapter"
	case archive.FormatZIP:
		return "ZIP        One general-purpose archive per chapter"
	default:
		return fmt.Sprintf("%s", format)
	}
}
