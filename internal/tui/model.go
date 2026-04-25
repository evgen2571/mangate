package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/source"
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
	app *app.App

	state                    state
	previousState            state
	pendingFullMangaDownload *source.Manga
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

func New(a *app.App) tea.Model {
	h := help.New()
	h.ShowAll = false

	return &model{
		app:    a,
		state:  stateSearch,
		keys:   newKeyMap(),
		help:   h,
		search: newSearchModel(),
		config: newConfigModel(a.Cfg),
	}
}

func (m model) Init() tea.Cmd {
	return m.search.Init()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeActiveModel()

		if m.state == stateResults {
			return m, m.reloadSelectedCoverCmd()
		}
		return m, nil

	case searchSubmittedMsg:
		m.loading = newLoadingModel("Searching manga", msg.Query)
		m.state = stateLoading
		m.resizeActiveModel()
		return m, tea.Batch(m.loading.spinner.Tick, m.searchMangaCmd(msg.Query))

	case searchSucceededMsg:
		m.results = newResultsModel(msg.Query, msg.Results)
		m.state = stateResults
		m.resizeActiveModel()

		selected := m.results.selectedManga()
		if selected == nil {
			return m, nil
		}

		w, h := m.results.coverBodySize()
		m.results.setCoverLoading(selected.ID)
		return m, tea.Batch(
			m.results.coverSpinner.Tick,
			m.loadCoverCmd(selected, w, h),
		)

	case coverLoadRequestedMsg:
		selected := m.results.selectedManga()
		if selected == nil || selected.ID != msg.MangaID {
			return m, nil
		}

		w, h := m.results.coverBodySize()
		m.results.setCoverLoading(selected.ID)
		return m, tea.Batch(
			m.results.coverSpinner.Tick,
			m.loadCoverCmd(selected, w, h),
		)

	case coverLoadedMsg:
		m.results.setCoverLoaded(msg.MangaID, msg.Path, msg.Render)
		return m, nil

	case coverFailedMsg:
		m.results.setCoverFailed(msg.MangaID, msg.Err)
		return m, nil

	case goBackMsg:
		if m.state == stateConfig {
			m.state = m.previousState
			m.resizeActiveModel()
			return m, nil
		}
		if m.state == stateChapters || m.state == stateDownloading {
			m.state = stateResults
		} else {
			m.state = stateSearch
		}
		m.resizeActiveModel()
		return m, nil

	case chaptersOpenRequestedMsg:
		if msg.Manga == nil {
			return m, nil
		}
		m.pendingFullMangaDownload = nil

		m.loading = newLoadingModel("Loading chapters", msg.Manga.Title)
		m.state = stateLoading
		m.resizeActiveModel()
		return m, tea.Batch(m.loading.spinner.Tick, m.loadChaptersCmd(msg.Manga))

	case fullMangaDownloadRequestedMsg:
		if msg.Manga == nil {
			return m, nil
		}
		m.pendingFullMangaDownload = msg.Manga

		m.loading = newLoadingModel("Loading chapters", msg.Manga.Title)
		m.state = stateLoading
		m.resizeActiveModel()
		return m, tea.Batch(m.loading.spinner.Tick, m.loadChaptersCmd(msg.Manga))

	case chaptersLoadedMsg:
		if msg.Manga != nil {
			msg.Manga.Chapters = msg.Chapters
			msg.Manga.Metadata.ChapterCount = nonNilChapterCount(msg.Chapters)
		}
		m.chapters = newChaptersModel(msg.Manga, msg.Chapters)
		if m.pendingFullMangaDownload == msg.Manga {
			m.pendingFullMangaDownload = nil
			chapters := nonNilChapters(msg.Chapters)
			if msg.Manga != nil && len(chapters) > 0 {
				progressCh := make(chan tea.Msg, 1024)
				m.downloading = newDownloadingModel("Downloading pages", downloadDetailText(chapters), progressCh)
				m.state = stateDownloading
				m.resizeActiveModel()
				return m, tea.Batch(m.downloading.waitForMsgCmd(), m.downloadChaptersCmd(msg.Manga, chapters, progressCh))
			}
			m.chapters.setStatus("no chapters to download")
		}
		m.state = stateChapters
		m.resizeActiveModel()
		return m, nil

	case chaptersFailedMsg:
		if m.pendingFullMangaDownload == msg.Manga {
			m.pendingFullMangaDownload = nil
		}
		m.state = stateResults
		m.resizeActiveModel()
		return m, nil

	case downloadRequestedMsg:
		if msg.Manga == nil || len(msg.Chapters) == 0 {
			return m, nil
		}

		progressCh := make(chan tea.Msg, 1024)
		m.downloading = newDownloadingModel("Downloading pages", downloadDetailText(msg.Chapters), progressCh)
		m.state = stateDownloading
		m.resizeActiveModel()
		return m, tea.Batch(m.downloading.waitForMsgCmd(), m.downloadChaptersCmd(msg.Manga, msg.Chapters, progressCh))

	case downloadProgressMsg:
		if m.state != stateDownloading {
			return m, nil
		}

		var progressCmd tea.Cmd
		m.downloading, progressCmd = m.downloading.Update(msg)
		return m, tea.Batch(progressCmd, m.downloading.waitForMsgCmd())

	case downloadSucceededMsg:
		m.chapters.clearSelection()
		m.chapters.setStatus(fmt.Sprintf("downloaded %d chapter(s)", len(msg.Chapters)))
		m.state = stateChapters
		m.resizeActiveModel()
		return m, nil

	case downloadFailedMsg:
		m.chapters.setStatus(fmt.Sprintf("download failed: %v", msg.Err))
		m.state = stateChapters
		m.resizeActiveModel()
		return m, nil

	case configApplyRequestedMsg:
		if err := m.app.ApplyConfig(msg.Config); err != nil {
			m.config.setStatus(fmt.Sprintf("apply failed: %v", err))
			return m, nil
		}
		m.config.draft = m.app.Cfg.Clone()
		m.config.syncInput()
		m.config.setStatus("applied for this session")
		return m, nil

	case configSaveRequestedMsg:
		if err := m.app.ApplyConfig(msg.Config); err != nil {
			m.config.setStatus(fmt.Sprintf("apply failed: %v", err))
			return m, nil
		}
		if err := config.Save(m.app.ConfigPath, m.app.Cfg); err != nil {
			m.config.setStatus(fmt.Sprintf("save failed: %v", err))
			return m, nil
		}
		m.config.draft = m.app.Cfg.Clone()
		m.config.syncInput()
		m.config.setStatus("saved and applied")
		return m, nil

	case tea.ResumeMsg:
		if m.state == stateResults {
			return m, m.reloadSelectedCoverCmd()
		}
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Suspend):
			return m, tea.Suspend
		case key.Matches(msg, m.keys.Config) && m.state != stateConfig && m.state != stateDownloading && m.state != stateLoading:
			m.previousState = m.state
			m.config = newConfigModel(m.app.Cfg)
			m.state = stateConfig
			m.resizeActiveModel()
			return m, nil
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
			m.resizeActiveModel()
			if m.state == stateResults {
				return m, m.reloadSelectedCoverCmd()
			}
			return m, nil
		}
	}

	switch m.state {
	case stateSearch:
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		return m, cmd

	case stateLoading:
		var cmd tea.Cmd
		m.loading, cmd = m.loading.Update(msg)
		return m, cmd

	case stateResults:
		var cmd tea.Cmd
		m.results, cmd = m.results.Update(msg)
		return m, cmd

	case stateChapters:
		var cmd tea.Cmd
		m.chapters, cmd = m.chapters.Update(msg)
		return m, cmd

	case stateDownloading:
		var cmd tea.Cmd
		m.downloading, cmd = m.downloading.Update(msg)
		return m, cmd

	case stateConfig:
		var cmd tea.Cmd
		m.config, cmd = m.config.Update(msg)
		return m, cmd
	}

	return m, nil
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

	selected := m.results.selectedManga()
	if selected == nil {
		return nil
	}

	w, h := m.results.coverBodySize()
	m.results.setCoverLoading(selected.ID)
	return tea.Batch(
		m.results.coverSpinner.Tick,
		m.loadCoverCmd(selected, w, h),
	)
}

func downloadDetailText(chapters []*source.Chapter) string {
	count := len(chapters)
	if count == 1 {
		return chapterDisplayName(chapters[0])
	}
	return fmt.Sprintf("%d chapters selected", count)
}

func nonNilChapters(chapters []*source.Chapter) []*source.Chapter {
	result := make([]*source.Chapter, 0, len(chapters))
	for _, chapter := range chapters {
		if chapter == nil {
			continue
		}
		result = append(result, chapter)
	}
	return result
}
