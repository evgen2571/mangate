package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/source"
)

type state int

const (
	stateSearch state = iota
	stateLoading
	stateResults
	stateChapters
	stateDownloading
)

type model struct {
	app *app.App

	state  state
	width  int
	height int

	keys keyMap
	help help.Model

	search      searchModel
	loading     loadingModel
	results     resultsModel
	chapters    chaptersModel
	downloading downloadingModel
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

		m.loading = newLoadingModel("Loading chapters", msg.Manga.Title)
		m.state = stateLoading
		m.resizeActiveModel()
		return m, tea.Batch(m.loading.spinner.Tick, m.loadChaptersCmd(msg.Manga))

	case chaptersLoadedMsg:
		m.chapters = newChaptersModel(msg.Manga, msg.Chapters)
		m.state = stateChapters
		m.resizeActiveModel()
		return m, nil

	case chaptersFailedMsg:
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
