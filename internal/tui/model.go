package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/evgen2571/mangate/internal/util"
)

type state int

const (
	stateSearch state = iota
	stateLoading
	stateResults
	stateChapters
	stateDownloading
	stateConfig
	stateFormat
	stateOutput
	stateConfirm
	stateCompletion
)

type model struct {
	app         *app.App
	baseContext context.Context

	state                    state
	previousState            state
	pendingFullMangaDownload *source.Manga
	downloadCancel           context.CancelFunc
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
	format      formatModel
	output      outputModel
	confirm     confirmModel
	completion  completionModel
}

func New(a *app.App) tea.Model {
	return NewWithContext(a, context.Background())
}

// NewWithContext creates the TUI using the caller's lifecycle context. Active
// downloads inherit it, so process interruption cancels provider work too.
func NewWithContext(a *app.App, baseContext context.Context) tea.Model {
	if baseContext == nil {
		baseContext = context.Background()
	}
	h := help.New()
	h.ShowAll = false

	searchHistory := []string(nil)
	if a != nil {
		if history, err := a.SearchHistory(); err == nil {
			searchHistory = history
		}
	}

	model := &model{
		app:         a,
		baseContext: baseContext,
		state:       stateSearch,
		keys:        newKeyMap(),
		help:        h,
		search:      newSearchModel(searchHistory),
		config:      newConfigModel(a.Cfg),
	}
	model.search.SetProvider(a.Cfg.Provider)
	return model
}

// NewWithSearchResults opens the TUI at an already-resolved search result
// list. It lets direct CLI search hand off to interactive selection without
// repeating the provider request.
func NewWithSearchResults(a *app.App, baseContext context.Context, query string, results []*source.Manga) tea.Model {
	m := NewWithContext(a, baseContext).(*model)
	m.search.input.SetValue(query)
	m.search.input.CursorEnd()
	m.results = newResultsModel(query, a.Cfg.Provider, results)
	m.state = stateResults
	return m
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
		if m.app != nil {
			if err := m.app.AddSearchQuery(msg.Query); err == nil {
				if history, err := m.app.SearchHistory(); err == nil {
					m.search.SetHistory(history)
				}
			}
		}
		m.loading = newLoadingModel("Searching manga", msg.Query)
		m.state = stateLoading
		m.resizeActiveModel()
		return m, tea.Batch(m.loading.spinner.Tick, m.searchMangaCmd(msg.Query))

	case searchSucceededMsg:
		m.results = newResultsModel(msg.Query, m.app.Cfg.Provider, msg.Results)
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

	case searchFailedMsg:
		m.search.SetStatus(fmt.Sprintf("Search failed: %v. Edit the query and try again.", msg.Err))
		m.state = stateSearch
		m.resizeActiveModel()
		return m, nil

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
		if m.state == stateConfirm {
			m.state = stateOutput
		} else if m.state == stateOutput {
			m.state = stateFormat
		} else if m.state == stateFormat {
			m.state = stateChapters
		} else if m.state == stateChapters || m.state == stateDownloading {
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

		m.loading = newLoadingModel("Loading chapters", util.SanitizeTerminalText(msg.Manga.Title))
		m.state = stateLoading
		m.resizeActiveModel()
		return m, tea.Batch(m.loading.spinner.Tick, m.loadChaptersCmd(msg.Manga))

	case fullMangaDownloadRequestedMsg:
		if msg.Manga == nil {
			return m, nil
		}
		m.pendingFullMangaDownload = msg.Manga

		m.loading = newLoadingModel("Loading chapters", util.SanitizeTerminalText(msg.Manga.Title))
		m.state = stateLoading
		m.resizeActiveModel()
		return m, tea.Batch(m.loading.spinner.Tick, m.loadChaptersCmd(msg.Manga))

	case chaptersLoadedMsg:
		if msg.Manga != nil {
			msg.Manga.Chapters = msg.Chapters
			msg.Manga.Metadata.ChapterCount = nonNilChapterCount(msg.Chapters)
		}
		m.chapters = newChaptersModel(msg.Manga, msg.Chapters)
		if m.app != nil {
			m.chapters.setLocalStatuses(localChapterStatuses(m.app.Cfg, msg.Manga, msg.Chapters))
		}
		if m.pendingFullMangaDownload == msg.Manga {
			m.pendingFullMangaDownload = nil
			chapters := nonNilChapters(msg.Chapters)
			if msg.Manga != nil && len(chapters) > 0 {
				m.openFormatSelection(msg.Manga, chapters)
				m.resizeActiveModel()
				return m, nil
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
		if msg.Err != nil {
			m.results.setStatus(fmt.Sprintf("Could not load chapters: %v", msg.Err))
		}
		m.resizeActiveModel()
		return m, nil

	case downloadRequestedMsg:
		if msg.Manga == nil || len(msg.Chapters) == 0 {
			return m, nil
		}
		m.openFormatSelection(msg.Manga, msg.Chapters)
		m.resizeActiveModel()
		return m, nil

	case outputPathSelectedMsg:
		path := strings.TrimSpace(msg.Path)
		if path == "" {
			m.output.status = "output root cannot be empty"
			return m, nil
		}
		cfg := m.app.Cfg.Clone()
		cfg.Download.Dir = filepath.Clean(path)
		if err := m.app.ApplyConfig(cfg); err != nil {
			m.output.status = fmt.Sprintf("apply output root: %v", err)
			return m, nil
		}
		m.confirm = newConfirmModel(m.app.Cfg, m.confirm.manga, m.confirm.chapters, m.format.selected())
		m.state = stateConfirm
		m.resizeActiveModel()
		return m, nil

	case downloadConfirmedMsg:
		if msg.Manga == nil || len(msg.Chapters) == 0 {
			return m, nil
		}
		cfg := m.app.Cfg.Clone()
		cfg.Download.Format = string(m.format.selected())
		if err := m.app.ApplyConfig(cfg); err != nil {
			m.chapters.setStatus(fmt.Sprintf("apply format: %v", err))
			m.state = stateChapters
			return m, nil
		}
		ctx, cancel := context.WithCancel(m.baseContext)
		m.downloadCancel = cancel
		progressCh := make(chan tea.Msg, 1024)
		m.downloading = newDownloadingModel("Downloading pages", downloadDetailText(msg.Chapters), progressCh)
		m.state = stateDownloading
		m.resizeActiveModel()
		return m, tea.Batch(m.downloading.waitForMsgCmd(), m.downloadChaptersCmd(ctx, msg.Manga, msg.Chapters, progressCh))

	case downloadProgressMsg:
		if m.state != stateDownloading {
			return m, nil
		}

		var progressCmd tea.Cmd
		m.downloading, progressCmd = m.downloading.Update(msg)
		return m, tea.Batch(progressCmd, m.downloading.waitForMsgCmd())

	case downloadSucceededMsg:
		m.downloadCancel = nil
		m.chapters.clearSelection()
		m.chapters.setStatus(fmt.Sprintf("downloaded %d chapter(s)", len(msg.Chapters)))
		m.completion = newCompletionModelWithOutcomes(m.app, msg.Manga, msg.Chapters, msg.Outcomes, nil)
		m.state = stateCompletion
		m.resizeActiveModel()
		return m, nil

	case downloadFailedMsg:
		m.downloadCancel = nil
		if msg.Cancelled {
			m.chapters.setStatus("download cancelled")
		} else {
			m.chapters.setStatus(fmt.Sprintf("download failed: %v", msg.Err))
		}
		m.completion = newCompletionModelWithOutcomes(m.app, msg.Manga, msg.Chapters, msg.Outcomes, msg.Err)
		m.completion.cancelled = msg.Cancelled
		if msg.Cancelled {
			m.completion.error = "Download cancelled; completed data has been kept."
		}
		m.state = stateCompletion
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
		if m.previousState == stateConfirm {
			m.confirm = newConfirmModel(m.app.Cfg, m.confirm.manga, m.confirm.chapters, m.format.selected())
		}
		return m, nil

	case configSaveRequestedMsg:
		if err := m.app.ApplyAndSaveConfig(msg.Config); err != nil {
			m.config.setStatus(err.Error())
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
			if m.state == stateDownloading && m.downloadCancel != nil {
				m.downloadCancel()
				m.downloadCancel = nil
				m.downloading.status = "Cancelling download..."
				return m, nil
			}
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
		if m.state == stateFormat {
			switch msg.String() {
			case "up", "k":
				m.format.move(-1)
				return m, nil
			case "down", "j":
				m.format.move(1)
				return m, nil
			case "enter":
				m.output = newOutputModel(m.app.Cfg.Download.Dir)
				m.state = stateOutput
				m.resizeActiveModel()
				return m, m.output.Init()
			case "esc", "backspace":
				m.state = stateChapters
				m.resizeActiveModel()
				return m, nil
			}
		}
		if m.state == stateConfirm {
			switch msg.String() {
			case "enter":
				return m, func() tea.Msg { return downloadConfirmedMsg{Manga: m.confirm.manga, Chapters: m.confirm.chapters} }
			case "esc", "backspace":
				m.state = stateOutput
				m.resizeActiveModel()
				return m, nil
			}
		}
		if m.state == stateCompletion {
			switch msg.String() {
			case "enter", "esc", "backspace":
				m.state = stateChapters
				m.resizeActiveModel()
				return m, nil
			}
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

	case stateOutput:
		var cmd tea.Cmd
		m.output, cmd = m.output.Update(msg)
		return m, cmd

	case stateFormat, stateConfirm:
		return m, nil

	case stateCompletion:
		return m, nil
	}

	return m, nil
}

func (m *model) openFormatSelection(manga *source.Manga, chapters []*source.Chapter) {
	m.format = newFormatModel(m.app.Cfg.Download.Format)
	m.pendingFullMangaDownload = nil
	m.confirm = newConfirmModel(m.app.Cfg, manga, chapters, m.format.selected())
	m.output = newOutputModel(m.app.Cfg.Download.Dir)
	m.state = stateFormat
}

func newCompletionModel(application *app.App, manga *source.Manga, chapters []*source.Chapter, operationErr error) completionModel {
	return newCompletionModelWithOutcomes(application, manga, chapters, nil, operationErr)
}

func newCompletionModelWithOutcomes(application *app.App, manga *source.Manga, chapters []*source.Chapter, outcomes []chapterOutcome, operationErr error) completionModel {
	completion := completionModel{success: operationErr == nil}
	if application == nil {
		completion.summary = "Download finished."
		return completion
	}
	format, err := archive.ParseFormat(application.Cfg.Download.Format)
	if err != nil {
		format = archive.FormatDirectory
	}
	if manga == nil {
		if operationErr != nil {
			completion.error = operationErr.Error()
		}
		completion.summary = "Download finished."
		return completion
	}
	names := downloader.ChapterDirectoryNames(chapters)
	root := filepath.Join(application.Cfg.Download.Dir, downloader.TitleDirectoryName(manga))
	if len(outcomes) == 0 {
		outcomes = make([]chapterOutcome, len(chapters))
		for index, chapter := range chapters {
			path := filepath.Join(root, names[index])
			if format != archive.FormatDirectory {
				path += format.Extension()
			}
			name := "Unknown chapter"
			if chapter != nil {
				name = chapter.DisplayName()
			}
			outcomes[index] = chapterOutcome{Name: name, Status: "complete", Path: path}
		}
	}
	completion.outcomes = outcomes
	for _, outcome := range outcomes {
		if outcome.Path != "" {
			completion.paths = append(completion.paths, outcome.Path)
		}
	}
	completed, skipped, incomplete, archiveFailures := completionCounts(outcomes)
	if operationErr != nil {
		completion.summary = fmt.Sprintf("Download finished: %d completed, %d skipped/reused, %d failed or incomplete.", completed, skipped, incomplete)
		if archiveFailures > 0 {
			completion.summary += fmt.Sprintf(" %d archive failure(s).", archiveFailures)
		}
		completion.error = operationErr.Error()
		return completion
	}
	completion.summary = completionSummary(completed+skipped, string(format))
	return completion
}

func (m model) View() string {
	if m.width > 0 && m.height > 0 && (m.width < 40 || m.height < 12) {
		return "Terminal is too small for Mangate. Resize to at least 40 columns by 12 rows.\n"
	}
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
	case stateFormat:
		body = m.format.View()
	case stateOutput:
		body = m.output.View()
	case stateConfirm:
		body = m.confirm.View()
	case stateCompletion:
		body = m.completion.View()
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
	case stateFormat, stateConfirm:
		return m.chapters.HelpKeys(m.keys)
	case stateOutput:
		return m.output.HelpKeys(m.keys)
	case stateCompletion:
		return m.chapters.HelpKeys(m.keys)
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
	case stateFormat:
		m.format.SetSize(m.width, bodyHeight)
	case stateOutput:
		m.output.SetSize(m.width, bodyHeight)
	case stateConfirm:
		m.confirm.SetSize(m.width, bodyHeight)
	case stateCompletion:
		m.completion.SetSize(m.width, bodyHeight)
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
		return chapters[0].LogName()
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

func localChapterStatuses(cfg config.Config, manga *source.Manga, chapters []*source.Chapter) map[string]string {
	statuses := make(map[string]string, len(chapters))
	if manga == nil {
		return statuses
	}
	format, err := archive.ParseFormat(cfg.Download.Format)
	if err != nil {
		format = archive.FormatDirectory
	}
	names := downloader.ChapterDirectoryNames(chapters)
	titleDir := filepath.Join(cfg.Download.Dir, downloader.TitleDirectoryName(manga))
	for index, chapter := range chapters {
		key := chapterSelectionKey(chapter, index)
		if chapter == nil {
			statuses[key] = "missing"
			continue
		}
		directory := filepath.Join(titleDir, names[index])
		if format != archive.FormatDirectory {
			if info, statErr := os.Stat(directory + format.Extension()); statErr == nil && !info.IsDir() && info.Size() > 0 {
				statuses[key] = "archive"
				continue
			}
		}
		if chapterDirectoryComplete(directory) {
			statuses[key] = "complete"
			continue
		}
		if _, statErr := os.Stat(filepath.Join(directory, ".mangate.json")); statErr == nil {
			statuses[key] = "incomplete"
			continue
		}
		statuses[key] = "missing"
	}
	return statuses
}
