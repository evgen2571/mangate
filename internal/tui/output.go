package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evgen2571/mangate/internal/constant"
)

func outputPathWarning(path string) (string, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." || path == "" {
		return "", fmt.Errorf("output root cannot be empty")
	}
	probe := path
	for {
		info, err := os.Stat(probe)
		if err == nil {
			if probe == path && !info.IsDir() {
				return "", fmt.Errorf("output root is an existing file")
			}
			if info.Mode().Perm()&0o222 == 0 {
				return fmt.Sprintf("nearest existing directory %s is not writable", probe), nil
			}
			return "", nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect output root: %w", err)
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return "", fmt.Errorf("output root has no accessible parent")
		}
		probe = parent
	}
}

type outputModel struct {
	width  int
	height int
	input  textinput.Model
	status string
}

func newOutputModel(path string) outputModel {
	input := textinput.New()
	input.SetValue(strings.TrimSpace(path))
	input.CursorEnd()
	input.Focus()
	input.Prompt = "Output root: "
	input.CharLimit = 512
	input.Width = 64
	input.PromptStyle = lipgloss.NewStyle().Foreground(constant.LogoColor)
	input.TextStyle = lipgloss.NewStyle().Foreground(constant.TextColor)
	return outputModel{input: input}
}

func (m *outputModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.input.Width = max(20, min(100, width-8))
}

func (m outputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m outputModel) Update(msg tea.Msg) (outputModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			path := strings.TrimSpace(m.input.Value())
			if path == "" {
				m.status = "output root cannot be empty"
				return m, nil
			}
			return m, func() tea.Msg { return outputPathSelectedMsg{Path: path} }
		case "esc":
			return m, func() tea.Msg { return goBackMsg{} }
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m outputModel) View() string {
	contentWidth := max(1, m.width-2)
	contentHeight := max(1, m.height-2)
	lines := []string{
		"Choose output location",
		"Downloads and per-chapter archives will be created under this root.",
		"",
		m.input.View(),
		"",
		"enter: continue  esc: change format  ctrl+g: edit full config",
	}
	if m.status != "" {
		lines = append(lines, "", fmt.Sprintf("Error: %s", m.status))
	}
	return lipgloss.NewStyle().
		Width(contentWidth).
		Height(contentHeight).
		Padding(1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(constant.OuterBorderColor).
		Render(strings.Join(lines, "\n"))
}

func (m outputModel) HelpKeys(global keyMap) help.KeyMap {
	return outputHelpKeyMap{global: global}
}

type outputHelpKeyMap struct {
	global keyMap
}

func (k outputHelpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "continue")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		k.global.Config,
		k.global.Help,
		k.global.Quit,
	}
}

func (k outputHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "continue")), key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")), k.global.Config, k.global.Help, k.global.Suspend, k.global.Quit}}
}
