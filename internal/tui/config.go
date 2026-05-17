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
	"github.com/evgen2571/mangate/internal/tuiapp"
)

type configField int

const (
	configFieldProvider configField = iota
	configFieldLanguage
	configFieldDownloadDir
	configFieldDownloadType
	configFieldHTTPTimeout
	configFieldPageDownloads
	configFieldChapterDownloads
	configFieldSearchHistoryMax
	configFieldCacheDir
	configFieldTempDir
	configFieldCount
)

type configModel struct {
	width  int
	height int

	keys    configKeyMap
	draft   tuiapp.ConfigState
	input   textinput.Model
	cursor  int
	editing bool
	status  string
}

func newConfigModel(state tuiapp.ConfigState) configModel {
	in := textinput.New()
	in.CharLimit = 256
	in.Width = 60
	in.PromptStyle = lipgloss.NewStyle().Foreground(constant.LogoColor)
	in.TextStyle = lipgloss.NewStyle().Foreground(constant.TextColor)
	in.PlaceholderStyle = lipgloss.NewStyle().Foreground(constant.MutedColor)

	m := configModel{
		keys:  newConfigKeyMap(),
		draft: state,
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
	case configFieldDownloadType:
		return "Download type"
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
	case configFieldTempDir:
		return "Temp dir"
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
		return m.draft.DownloadDir
	case configFieldDownloadType:
		return m.draft.DownloadType
	case configFieldHTTPTimeout:
		return m.draft.HTTPTimeout.String()
	case configFieldPageDownloads:
		return strconv.Itoa(m.draft.PageDownloads)
	case configFieldChapterDownloads:
		return strconv.Itoa(m.draft.ChapterDownloads)
	case configFieldSearchHistoryMax:
		return strconv.Itoa(m.draft.SearchHistoryMax)
	case configFieldCacheDir:
		return m.draft.CacheDir
	case configFieldTempDir:
		return m.draft.TempDir
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

	next := m.draft
	switch field {
	case configFieldProvider:
		next.Provider = value
	case configFieldLanguage:
		next.Language = value
	case configFieldDownloadDir:
		next.DownloadDir = value
	case configFieldDownloadType:
		next.DownloadType = value
	case configFieldHTTPTimeout:
		d, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("parse http timeout: %w", err)
		}
		next.HTTPTimeout = d
	case configFieldPageDownloads:
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse page downloads: %w", err)
		}
		next.PageDownloads = n
	case configFieldChapterDownloads:
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse chapter downloads: %w", err)
		}
		next.ChapterDownloads = n
	case configFieldSearchHistoryMax:
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse search history max: %w", err)
		}
		next.SearchHistoryMax = n
	case configFieldCacheDir:
		next.CacheDir = value
	case configFieldTempDir:
		next.TempDir = value
	}

	if err := configFromState(next).Validate(); err != nil {
		return err
	}
	m.draft = next
	m.syncInput()
	return nil
}

func (m *configModel) loadFromState(state tuiapp.ConfigState) {
	m.draft = state
	m.syncInput()
}

func currentConfigState(svc tuiapp.Service) tuiapp.ConfigState {
	if svc == nil {
		return configStateFromConfig(config.DefaultConfig())
	}
	return svc.Config()
}

func configStateFromConfig(cfg config.Config) tuiapp.ConfigState {
	return tuiapp.ConfigState{
		Provider:           cfg.Provider,
		Language:           cfg.Language,
		HTTPTimeout:        cfg.HTTP.Timeout,
		DownloadDir:        cfg.Download.Dir,
		DownloadType:       cfg.Download.Type,
		PageDownloads:      cfg.Concurrency.PageDownloads,
		ChapterDownloads:   cfg.Concurrency.ChapterDownloads,
		SearchHistoryMax:   cfg.Search.HistoryMax,
		CacheDir:           cfg.Dirs.Cache,
		TempDir:            cfg.Dirs.Temp,
		MangaDexSiteURL:    cfg.Providers.MangaDex.SiteURL,
		MangaDexBaseURL:    cfg.Providers.MangaDex.BaseURL,
		MangaDexUploadsURL: cfg.Providers.MangaDex.UploadsURL,
	}
}

func configFromState(state tuiapp.ConfigState) config.Config {
	cfg := config.DefaultConfig()
	cfg.Provider = state.Provider
	cfg.Language = state.Language
	cfg.HTTP.Timeout = state.HTTPTimeout
	cfg.Download.Dir = state.DownloadDir
	cfg.Download.Type = state.DownloadType
	cfg.Concurrency.PageDownloads = state.PageDownloads
	cfg.Concurrency.ChapterDownloads = state.ChapterDownloads
	cfg.Search.HistoryMax = state.SearchHistoryMax
	cfg.Dirs.Cache = state.CacheDir
	cfg.Dirs.Temp = state.TempDir
	cfg.Providers.MangaDex.SiteURL = state.MangaDexSiteURL
	cfg.Providers.MangaDex.BaseURL = state.MangaDexBaseURL
	cfg.Providers.MangaDex.UploadsURL = state.MangaDexUploadsURL
	return cfg
}
