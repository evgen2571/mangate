package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/evgen2571/mangate/internal/tuiapp"
)

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
		return m.handleSearchSubmitted(msg)

	case searchSucceededMsg:
		m.search.SetHistory(msg.History)
		m.search.setStatus("")
		m.results = newResultsModel(msg.Query, msg.Results)
		m.state = stateResults
		m.resizeActiveModel()

		selected, ok := m.results.selectedResult()
		if !ok {
			return m, nil
		}

		w, h := m.results.coverBodySize()
		m.results.setCoverLoading(selected.ID)
		return m, tea.Batch(
			m.results.coverSpinner.Tick,
			m.loadCoverCmd(selected, w, h),
		)

	case searchFailedMsg:
		m.search.setStatus(fmt.Sprintf("search failed: %v", msg.Err))
		m.state = stateSearch
		m.resizeActiveModel()
		return m, nil

	case coverLoadRequestedMsg:
		selected, ok := m.results.selectedResult()
		if !ok || selected.ID != msg.MangaID {
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
		if msg.Result.ID == "" {
			return m, nil
		}
		m.pendingFullMangaDownload = tuiapp.MangaDetails{}

		m.loading = newLoadingModel("Loading chapters", msg.Result.Title)
		m.state = stateLoading
		m.resizeActiveModel()
		return m, tea.Batch(m.loading.spinner.Tick, m.loadChaptersCmd(msg.Result))

	case fullMangaDownloadRequestedMsg:
		if msg.Result.ID == "" {
			return m, nil
		}
		m.pendingFullMangaDownload = mangaDetailsFromSearchResult(msg.Result)

		m.loading = newLoadingModel("Loading chapters", msg.Result.Title)
		m.state = stateLoading
		m.resizeActiveModel()
		return m, tea.Batch(m.loading.spinner.Tick, m.loadChaptersCmd(msg.Result))

	case chaptersLoadedMsg:
		m.chapters = newChaptersModel(msg.Manga, msg.Chapters)
		if m.pendingFullMangaDownload.ID != "" && m.pendingFullMangaDownload.ID == msg.Manga.ID {
			m.pendingFullMangaDownload = tuiapp.MangaDetails{}
			chapters := nonNilChapters(msg.Chapters)
			if len(chapters) > 0 {
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
		if m.pendingFullMangaDownload.ID != "" && m.pendingFullMangaDownload.ID == msg.MangaID {
			m.pendingFullMangaDownload = tuiapp.MangaDetails{}
		}
		m.state = stateResults
		m.resizeActiveModel()
		return m, nil

	case downloadRequestedMsg:
		if msg.Manga.ID == "" || len(msg.Chapters) == 0 {
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
		return m.applyConfig(msg.Config)

	case configSaveRequestedMsg:
		return m.saveConfig(msg.Config)

	case tea.ResumeMsg:
		if m.state == stateResults {
			return m, m.reloadSelectedCoverCmd()
		}
		return m, nil

	case tea.KeyMsg:
		if updated, cmd, handled := m.handleRootKeyMsg(msg); handled {
			return updated, cmd
		}
	}

	return m.routeActiveModelUpdate(msg)
}

func (m model) handleRootKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit, true
	case key.Matches(msg, m.keys.Suspend):
		return m, tea.Suspend, true
	case key.Matches(msg, m.keys.Config) && m.state != stateConfig && m.state != stateDownloading && m.state != stateLoading:
		m.previousState = m.state
		m.config = newConfigModel(currentConfigState(m.svc))
		m.state = stateConfig
		m.resizeActiveModel()
		return m, nil, true
	case key.Matches(msg, m.keys.Help):
		m.help.ShowAll = !m.help.ShowAll
		m.resizeActiveModel()
		if m.state == stateResults {
			return m, m.reloadSelectedCoverCmd(), true
		}
		return m, nil, true
	default:
		return m, nil, false
	}
}

func (m model) handleSearchSubmitted(msg searchSubmittedMsg) (tea.Model, tea.Cmd) {
	m.loading = newLoadingModel("Searching manga", msg.Query)
	m.state = stateLoading
	m.resizeActiveModel()

	return m, tea.Batch(
		m.loading.spinner.Tick,
		func() tea.Msg {
			results, err := m.svc.Search(nil, msg.Query)
			if err != nil {
				return searchFailedMsg{Err: err}
			}

			history, _ := m.svc.SearchHistory(nil)
			return searchSucceededMsg{
				Query:   msg.Query,
				Results: results,
				History: history,
			}
		},
	)
}

func (m model) routeActiveModelUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	default:
		return m, nil
	}
}

func (m model) applyConfig(state tuiapp.ConfigState) (tea.Model, tea.Cmd) {
	next, err := m.svc.ApplyConfig(nil, state)
	if err != nil {
		m.config.setStatus(fmt.Sprintf("apply failed: %v", err))
		return m, nil
	}
	m.config.loadFromState(next)
	m.config.setStatus("applied for this session")
	return m, nil
}

func (m model) saveConfig(state tuiapp.ConfigState) (tea.Model, tea.Cmd) {
	next, err := m.svc.SaveConfig(nil, state)
	if err != nil {
		m.config.setStatus(err.Error())
		return m, nil
	}
	m.config.loadFromState(next)
	m.config.setStatus("saved and applied")
	return m, nil
}
