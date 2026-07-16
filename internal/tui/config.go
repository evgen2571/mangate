package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/constant"
)

type configField int

const (
	configFieldProvider configField = iota
	configFieldLanguage
	configFieldDownloadDir
	configFieldDownloadFormat
	configFieldExistingFiles
	configFieldRetainSource
	configFieldHTTPTimeout
	configFieldPageDownloads
	configFieldChapterDownloads
	configFieldSearchHistoryMax
	configFieldCacheDir
	configFieldCount
)

type configModel struct {
	width  int
	height int

	keys    configKeyMap
	draft   config.Config
	input   textinput.Model
	cursor  int
	editing bool
	status  string
}

func newConfigModel(cfg config.Config) configModel {
	in := textinput.New()
	in.CharLimit = 256
	in.Width = 60
	in.PromptStyle = lipgloss.NewStyle().Foreground(constant.LogoColor)
	in.TextStyle = lipgloss.NewStyle().Foreground(constant.TextColor)
	in.PlaceholderStyle = lipgloss.NewStyle().Foreground(constant.MutedColor)

	m := configModel{
		keys:  newConfigKeyMap(),
		draft: cfg.Clone(),
		input: in,
	}
	m.syncInput()
	return m
}

func (m *configModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.input.Width = max(20, min(80, width-8))
}

func (m configModel) Update(msg tea.Msg) (configModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.editing {
			switch {
			case key.Matches(msg, m.keys.Confirm):
				if err := m.updateDraftFromInput(); err != nil {
					m.status = err.Error()
					return m, nil
				}
				m.editing = false
				m.input.Blur()
				m.status = "draft updated"
				return m, nil
			case key.Matches(msg, m.keys.Back):
				m.editing = false
				m.input.Blur()
				m.syncInput()
				m.status = "edit cancelled"
				return m, nil
			}

			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}

		switch {
		case key.Matches(msg, m.keys.Up):
			m.move(-1)
			return m, nil
		case key.Matches(msg, m.keys.Down):
			m.move(1)
			return m, nil
		case key.Matches(msg, m.keys.Edit):
			m.editing = true
			m.input.Focus()
			m.status = "editing " + m.currentLabel()
			return m, textinput.Blink
		case key.Matches(msg, m.keys.Apply):
			return m, func() tea.Msg { return configApplyRequestedMsg{Config: m.draft} }
		case key.Matches(msg, m.keys.Save):
			return m, func() tea.Msg { return configSaveRequestedMsg{Config: m.draft} }
		case key.Matches(msg, m.keys.Back):
			return m, func() tea.Msg { return goBackMsg{} }
		}
	}

	return m, nil
}

func (m configModel) View() string {
	contentWidth := max(1, m.width-2)
	contentHeight := max(1, m.height-2)

	rows := []string{
		lipgloss.NewStyle().Bold(true).Foreground(constant.LogoColor).Render("Config"),
		lipgloss.NewStyle().Foreground(constant.MutedColor).Render("Edit draft values, apply for this session, or save to config file."),
		"",
	}
	for field := configField(0); field < configFieldCount; field++ {
		prefix := "  "
		if int(field) == m.cursor {
			prefix = lipgloss.NewStyle().Foreground(constant.LogoColor).Render("› ")
		}
		value := m.fieldValue(field)
		line := fmt.Sprintf("%s%-18s %s", prefix, m.fieldLabel(field)+":", value)
		if int(field) == m.cursor {
			line = lipgloss.NewStyle().Bold(true).Render(line)
		}
		rows = append(rows, line)
	}
	rows = append(rows, "")
	if m.editing {
		rows = append(rows, "New value:", m.input.View())
	}
	if strings.TrimSpace(m.status) != "" {
		rows = append(rows, "", lipgloss.NewStyle().Foreground(constant.MutedColor).Render(m.status))
	}

	inner := lipgloss.NewStyle().Width(contentWidth).Height(contentHeight).Padding(0, 1).Render(strings.Join(rows, "\n"))
	return lipgloss.NewStyle().Width(contentWidth).Height(contentHeight).Border(lipgloss.RoundedBorder()).BorderForeground(constant.OuterBorderColor).Render(inner)
}

func (m configModel) HelpKeys(global keyMap) help.KeyMap {
	return configHelpKeyMap{global: global, local: m.keys}
}

func (m *configModel) setStatus(status string) {
	m.status = strings.TrimSpace(status)
}

func (m *configModel) move(delta int) {
	m.cursor = (m.cursor + delta + int(configFieldCount)) % int(configFieldCount)
	m.syncInput()
}

func (m *configModel) syncInput() {
	m.input.SetValue(m.fieldValue(configField(m.cursor)))
	m.input.CursorEnd()
}

func (m configModel) currentLabel() string {
	return m.fieldLabel(configField(m.cursor))
}

func (m configModel) fieldLabel(field configField) string {
	switch field {
	case configFieldProvider:
		return "Provider"
	case configFieldLanguage:
		return "Language"
	case configFieldDownloadDir:
		return "Download dir"
	case configFieldDownloadFormat:
		return "Output format"
	case configFieldExistingFiles:
		return "Existing files"
	case configFieldRetainSource:
		return "Retain source"
	case configFieldHTTPTimeout:
		return "HTTP timeout"
	case configFieldPageDownloads:
		return "Page downloads"
	case configFieldChapterDownloads:
		return "Chapter downloads"
	case configFieldSearchHistoryMax:
		return "Search history max"
	case configFieldCacheDir:
		return "Cache dir"
	default:
		return "Unknown"
	}
}

func (m configModel) fieldValue(field configField) string {
	switch field {
	case configFieldProvider:
		return m.draft.Provider
	case configFieldLanguage:
		return m.draft.Language
	case configFieldDownloadDir:
		return m.draft.Download.Dir
	case configFieldDownloadFormat:
		return m.draft.Download.Format
	case configFieldExistingFiles:
		return m.draft.Download.ExistingFileMode
	case configFieldRetainSource:
		return strconv.FormatBool(m.draft.Download.RetainSource)
	case configFieldHTTPTimeout:
		return m.draft.HTTP.Timeout.String()
	case configFieldPageDownloads:
		return strconv.Itoa(m.draft.Concurrency.PageDownloads)
	case configFieldChapterDownloads:
		return strconv.Itoa(m.draft.Concurrency.ChapterDownloads)
	case configFieldSearchHistoryMax:
		return strconv.Itoa(m.draft.Search.HistoryMax)
	case configFieldCacheDir:
		return m.draft.Dirs.Cache
	default:
		return ""
	}
}

func (m *configModel) updateDraftFromInput() error {
	field := configField(m.cursor)
	value := strings.TrimSpace(m.input.Value())
	if value == "" {
		return fmt.Errorf("%s cannot be empty", strings.ToLower(m.fieldLabel(field)))
	}

	next := m.draft.Clone()
	switch field {
	case configFieldProvider:
		next.Provider = value
	case configFieldLanguage:
		next.Language = value
	case configFieldDownloadDir:
		next.Download.Dir = value
	case configFieldDownloadFormat:
		next.Download.Format = strings.ToLower(value)
	case configFieldExistingFiles:
		next.Download.ExistingFileMode = strings.ToLower(value)
	case configFieldRetainSource:
		retain, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("parse retain source: %w", err)
		}
		next.Download.RetainSource = retain
	case configFieldHTTPTimeout:
		d, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("parse http timeout: %w", err)
		}
		next.HTTP.Timeout = d
	case configFieldPageDownloads:
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse page downloads: %w", err)
		}
		next.Concurrency.PageDownloads = n
	case configFieldChapterDownloads:
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse chapter downloads: %w", err)
		}
		next.Concurrency.ChapterDownloads = n
	case configFieldSearchHistoryMax:
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse search history max: %w", err)
		}
		next.Search.HistoryMax = n
	case configFieldCacheDir:
		next.Dirs.Cache = value
	}

	if err := next.Validate(); err != nil {
		return err
	}
	m.draft = next
	m.syncInput()
	return nil
}
