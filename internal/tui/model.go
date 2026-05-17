package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/tuiapp"
)

type state int

const (
	stateSearch state = iota
	stateLoading
	stateResults
	stateChapters
	stateDownloading
	stateConfig
)

type model struct {
	svc tuiapp.Service

	state                    state
	previousState            state
	pendingFullMangaDownload tuiapp.MangaDetails
	width                    int
	height                   int

	keys keyMap
	help help.Model

	search      searchModel
	loading     loadingModel
	results     resultsModel
	chapters    chaptersModel
	downloading downloadingModel
	config      configModel
}

func New(svc tuiapp.Service) tea.Model {
	return newModel(svc)
}

func newModel(svc tuiapp.Service) tea.Model {
	h := help.New()
	h.ShowAll = false

	searchHistory := []string(nil)
	if svc != nil {
		if history, err := svc.SearchHistory(nil); err == nil {
			searchHistory = history
		}
	}
	configDraft := currentConfigState(svc)

	return &model{
		svc:    svc,
		state:  stateSearch,
		keys:   newKeyMap(),
		help:   h,
		search: newSearchModel(searchHistory),
		config: newConfigModel(configDraft),
	}
}

func (m model) Init() tea.Cmd {
	return m.search.Init()
}

func (m model) View() string {
	var body string

	switch m.state {
	case stateSearch:
		body = m.search.View()
	case stateLoading:
		body = m.loading.View()
	case stateResults:
		body = m.results.View()
	case stateChapters:
		body = m.chapters.View()
	case stateDownloading:
		body = m.downloading.View()
	case stateConfig:
		body = m.config.View()
	default:
		body = ""
	}

	helpView := m.help.View(m.currentHelp())

	return lipgloss.JoinVertical(
		lipgloss.Left,
		body,
		"",
		helpView,
	)
}

func (m model) currentHelp() help.KeyMap {
	switch m.state {
	case stateSearch:
		return m.search.HelpKeys(m.keys)
	case stateLoading:
		return m.loading.HelpKeys(m.keys)
	case stateResults:
		return m.results.HelpKeys(m.keys)
	case stateChapters:
		return m.chapters.HelpKeys(m.keys)
	case stateDownloading:
		return m.downloading.HelpKeys(m.keys)
	case stateConfig:
		return m.config.HelpKeys(m.keys)
	default:
		return m.search.HelpKeys(m.keys)
	}
}

func (m *model) resizeActiveModel() {
	if m.width == 0 || m.height == 0 {
		return
	}

	helpView := m.help.View(m.currentHelp())
	helpHeight := lipgloss.Height(helpView)

	bodyHeight := max(1, m.height-helpHeight-1)

	switch m.state {
	case stateSearch:
		m.search.SetSize(m.width, bodyHeight)
	case stateLoading:
		m.loading.SetSize(m.width, bodyHeight)
	case stateResults:
		m.results.SetSize(m.width, bodyHeight)
	case stateChapters:
		m.chapters.SetSize(m.width, bodyHeight)
	case stateDownloading:
		m.downloading.SetSize(m.width, bodyHeight)
	case stateConfig:
		m.config.SetSize(m.width, bodyHeight)
	}
}

func (m model) reloadSelectedCoverCmd() tea.Cmd {
	if m.state != stateResults {
		return nil
	}

	selected, ok := m.results.selectedResult()
	if !ok {
		return nil
	}

	w, h := m.results.coverBodySize()
	m.results.setCoverLoading(selected.ID)
	return tea.Batch(
		m.results.coverSpinner.Tick,
		m.loadCoverCmd(selected, w, h),
	)
}

func downloadDetailText(chapters []tuiapp.ChapterItem) string {
	count := len(chapters)
	if count == 1 {
		chapter := chapters[0]
		for _, text := range []string{chapter.DisplayText, chapter.Title, chapter.Index, chapter.ID} {
			if text != "" {
				return text
			}
		}
		return "1 chapter selected"
	}
	return fmt.Sprintf("%d chapters selected", count)
}

func nonNilChapters(chapters []tuiapp.ChapterItem) []tuiapp.ChapterItem {
	result := make([]tuiapp.ChapterItem, 0, len(chapters))
	for _, chapter := range chapters {
		if !isChapterItemSet(chapter) {
			continue
		}
		result = append(result, chapter)
	}
	return result
}

func mangaDetailsFromSearchResult(result tuiapp.SearchResult) tuiapp.MangaDetails {
	return tuiapp.MangaDetails{
		ID:           result.ID,
		Title:        result.Title,
		URL:          result.URL,
		SummaryMD:    result.SummaryMD,
		ChapterCount: result.ChapterCount,
	}
}

func currentConfigState(svc tuiapp.Service) tuiapp.ConfigState {
	if svc == nil {
		return configStateFromConfig(config.DefaultConfig())
	}
	return svc.Config()
}
