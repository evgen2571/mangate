// Package interactive provides Mangate's keyboard-first terminal interface.
package interactive

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/evgen2571/mangate/internal/util"
)

type screen uint8

const (
	searchScreen screen = iota
	resultsScreen
	chaptersScreen
	formatScreen
	outputScreen
	reviewScreen
	configScreen
	workingScreen
	doneScreen
)

type model struct {
	app           *app.App
	ctx           context.Context
	screen        screen
	previous      screen
	width, height int

	input         textinput.Model
	resultsList   list.Model
	spinner       spinner.Model
	progressBar   progress.Model
	help          help.Model
	doneViewport  viewport.Model
	showHelp      bool
	loading       bool
	status        string
	query         string
	results       []*source.Manga
	resultCursor  int
	chapters      []*source.Chapter
	chapterCursor int
	chapterOffset int
	selected      map[int]bool
	chapterFilter string
	filtering     bool
	rangeAnchor   int
	format        archive.Format
	manga         *source.Manga
	cancel        context.CancelFunc
	progress      downloadProgress
	progressCh    chan tea.Msg
	completion    string
	doneErr       error
	doneCompleted int
	doneFailed    int
	configCursor  int
	configEditing bool
	draft         config.Config
}

func New(a *app.App) tea.Model { return NewWithContext(a, context.Background()) }

func NewWithContext(a *app.App, ctx context.Context) tea.Model {
	if ctx == nil {
		ctx = context.Background()
	}
	m := &model{app: a, ctx: ctx, screen: searchScreen, selected: map[int]bool{}, rangeAnchor: -1, format: archive.FormatDirectory}
	m.spinner = spinner.New(spinner.WithSpinner(spinner.Dot))
	m.progressBar = progress.New(progress.WithDefaultGradient())
	m.progressBar.ShowPercentage = true
	m.help = help.New()
	m.doneViewport = viewport.New(1, 1)
	if a != nil {
		m.format, _ = archive.ParseFormat(a.Cfg.Download.Format)
	}
	m.resetInput("Search: ", "")
	m.newResultsList(nil)
	return m
}

func NewWithSearchResults(a *app.App, ctx context.Context, query string, results []*source.Manga) tea.Model {
	m := NewWithContext(a, ctx).(*model)
	m.query, m.results, m.screen = query, results, resultsScreen
	m.newResultsList(results)
	return m
}

func (m *model) Init() tea.Cmd { return tea.Batch(textinput.Blink, m.spinner.Tick) }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil
	case searchDone:
		if msg.err != nil {
			m.status = "search: " + msg.err.Error()
			m.screen = searchScreen
			return m, nil
		}
		m.query, m.results, m.resultCursor, m.status, m.screen, m.loading = msg.query, msg.results, 0, "", resultsScreen, false
		m.newResultsList(msg.results)
		return m, nil
	case chaptersDone:
		if msg.err != nil {
			m.status = "chapters: " + msg.err.Error()
			m.screen = resultsScreen
			return m, nil
		}
		m.manga, m.chapters, m.chapterCursor, m.chapterOffset, m.selected, m.chapterFilter, m.filtering, m.rangeAnchor, m.loading = msg.manga, msg.chapters, 0, 0, map[int]bool{}, "", false, -1, false
		if msg.all {
			for i, chapter := range m.chapters {
				if chapter != nil {
					m.selected[i] = true
				}
			}
			m.openFormat()
		} else {
			m.screen, m.status = chaptersScreen, ""
		}
		return m, nil
	case downloadProgress:
		m.progress = msg
		return m, m.waitForProgress()
	case downloadDone:
		m.cancel = nil
		m.screen = doneScreen
		m.loading = false
		m.doneErr, m.doneCompleted, m.doneFailed = msg.err, msg.completed, msg.failed
		m.completion = fmt.Sprintf("complete: %d  reused: %d  failed: %d", msg.completed, msg.skipped, msg.failed)
		if msg.err != nil {
			m.completion += "\n" + util.SanitizeTerminalText(msg.err.Error())
		}
		if len(msg.paths) > 0 {
			m.completion += "\n" + strings.Join(msg.paths, "\n")
		}
		m.doneViewport.SetContent(m.completion)
		m.resize()
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "?" && m.screen != workingScreen {
			m.showHelp = !m.showHelp
			return m, nil
		}
		if m.showHelp && msg.String() == "esc" {
			m.showHelp = false
			return m, nil
		}
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			if m.screen == workingScreen && m.cancel != nil {
				m.cancel()
				m.status = "cancelling"
				return m, nil
			}
			return m, tea.Quit
		}
		if m.width > 0 && (m.width < minWidth || m.height < minHeight) {
			return m, nil
		}
		if msg.String() == "ctrl+g" && m.screen != workingScreen {
			m.previous, m.screen, m.draft, m.configCursor, m.configEditing = m.screen, configScreen, m.app.Cfg.Clone(), 0, false
			return m, nil
		}
	}
	if m.loading || m.screen == workingScreen {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	switch m.screen {
	case searchScreen:
		return m.updateSearch(msg)
	case resultsScreen:
		return m.updateResults(msg)
	case chaptersScreen:
		return m.updateChapters(msg)
	case formatScreen:
		return m.updateFormat(msg)
	case outputScreen:
		return m.updateOutput(msg)
	case reviewScreen:
		return m.updateReview(msg)
	case configScreen:
		return m.updateConfig(msg)
	case doneScreen:
		if key, ok := msg.(tea.KeyMsg); ok {
			if key.String() == "enter" || key.String() == "esc" {
				m.screen = chaptersScreen
				return m, nil
			}
			var cmd tea.Cmd
			m.doneViewport, cmd = m.doneViewport.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m *model) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
		q := strings.TrimSpace(m.input.Value())
		if q == "" {
			return m, nil
		}
		m.status, m.loading = "Searching...", true
		return m, func() tea.Msg {
			results, err := m.app.UseCases().SearchManga(nil, q)
			if err == nil {
				_ = m.app.AddSearchQuery(q)
			}
			return searchDone{q, results, err}
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *model) updateResults(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		m.resultsList, cmd = m.resultsList.Update(msg)
		return m, cmd
	}
	switch key.String() {
	case "j", "down":
		m.resultsList.CursorDown()
	case "k", "up":
		m.resultsList.CursorUp()
	case "esc", "backspace":
		m.screen = searchScreen
		m.resetInput("Search: ", m.query)
	case "enter", "f":
		item, ok := m.resultsList.SelectedItem().(resultItem)
		if !ok || item.manga == nil {
			return m, nil
		}
		manga, all := item.manga, key.String() == "f"
		m.status, m.loading = "Loading chapters...", true
		return m, func() tea.Msg {
			chapters, err := m.app.UseCases().Chapters(nil, manga)
			return chaptersDone{manga, chapters, err, all}
		}
	}
	var cmd tea.Cmd
	m.resultsList, cmd = m.resultsList.Update(msg)
	return m, cmd
}

func (m *model) updateChapters(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.filtering {
		if key.String() == "enter" {
			m.chapterFilter, m.filtering = strings.TrimSpace(m.input.Value()), false
			return m, nil
		}
		if key.String() == "esc" {
			m.filtering = false
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	visible := m.visibleChapters()
	switch key.String() {
	case "j", "down":
		m.chapterCursor = move(m.chapterCursor, 1, len(visible))
	case "k", "up":
		m.chapterCursor = move(m.chapterCursor, -1, len(visible))
	case "space":
		if len(visible) > 0 {
			i := visible[m.chapterCursor]
			m.selected[i] = !m.selected[i]
			if m.selected[i] {
				m.rangeAnchor = i
			}
			if !m.selected[i] {
				delete(m.selected, i)
			}
		}
	case "a":
		for _, i := range visible {
			m.selected[i] = true
		}
	case "d":
		m.selected = map[int]bool{}
	case "l":
		m.selected = map[int]bool{}
		if len(visible) > 0 {
			m.selected[visible[len(visible)-1]] = true
			m.rangeAnchor = visible[len(visible)-1]
		}
	case "r":
		if len(visible) > 0 {
			current := visible[m.chapterCursor]
			start := -1
			for position, index := range visible {
				if index == m.rangeAnchor {
					start = position
				}
				if index == current && start >= 0 {
					end := position
					if start > end {
						start, end = end, start
					}
					for _, selected := range visible[start : end+1] {
						m.selected[selected] = true
					}
					break
				}
			}
			m.rangeAnchor = current
		}
	case "/":
		m.filtering = true
		m.resetInput("Filter: ", m.chapterFilter)
	case "esc", "backspace":
		m.screen = resultsScreen
	case "enter":
		if len(visible) == 0 {
			return m, nil
		}
		if len(m.selected) == 0 {
			m.selected[visible[m.chapterCursor]] = true
		}
		m.openFormat()
	}
	return m, nil
}

func (m *model) updateFormat(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	formats := []archive.Format{archive.FormatDirectory, archive.FormatCBZ, archive.FormatZIP}
	index := 0
	for i, f := range formats {
		if f == m.format {
			index = i
		}
	}
	switch key.String() {
	case "j", "down":
		m.format = formats[(index+1)%len(formats)]
	case "k", "up":
		m.format = formats[(index+len(formats)-1)%len(formats)]
	case "enter":
		m.screen = outputScreen
		m.resetInput("Output: ", m.app.Cfg.Download.Dir)
	case "esc":
		m.screen = chaptersScreen
	}
	return m, nil
}

func (m *model) updateOutput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if key.String() == "esc" {
			m.screen = formatScreen
			return m, nil
		}
		if key.String() == "enter" {
			path := strings.TrimSpace(m.input.Value())
			if err := validOutput(path); err != nil {
				m.status = err.Error()
				return m, nil
			}
			cfg := m.app.Cfg.Clone()
			cfg.Download.Dir = filepath.Clean(path)
			if err := m.app.ApplyConfig(cfg); err != nil {
				m.status = err.Error()
				return m, nil
			}
			m.screen = reviewScreen
			return m, nil
		}
		m.status = ""
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *model) updateReview(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if key.String() == "esc" {
		m.screen = outputScreen
		return m, nil
	}
	if key.String() != "enter" {
		return m, nil
	}
	cfg := m.app.Cfg.Clone()
	cfg.Download.Format = string(m.format)
	if err := m.app.ApplyConfig(cfg); err != nil {
		m.status = err.Error()
		m.screen = chaptersScreen
		return m, nil
	}
	chapters := m.selectedChapters()
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancel, m.screen = cancel, workingScreen
	m.progressCh = make(chan tea.Msg, 32)
	return m, tea.Batch(m.waitForProgress(), m.download(ctx, chapters, m.progressCh))
}

func (m *model) updateConfig(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.configEditing {
		if key.String() == "enter" {
			if err := m.setConfigValue(m.input.Value()); err != nil {
				m.status = err.Error()
				return m, nil
			}
			m.configEditing = false
			return m, nil
		}
		if key.String() == "esc" {
			m.configEditing = false
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	switch key.String() {
	case "j", "down":
		m.configCursor = move(m.configCursor, 1, len(configLabels))
	case "k", "up":
		m.configCursor = move(m.configCursor, -1, len(configLabels))
	case "enter":
		m.configEditing = true
		m.resetInput("Value: ", m.configValue())
	case "a":
		if err := m.app.ApplyConfig(m.draft); err != nil {
			m.status = err.Error()
		} else {
			m.status = "applied"
		}
	case "s":
		if err := m.app.ApplyAndSaveConfig(m.draft); err != nil {
			m.status = err.Error()
		} else {
			m.status = "saved"
		}
	case "esc":
		m.screen = m.previous
	}
	return m, nil
}

func (m *model) openFormat() {
	m.screen = formatScreen
	m.format, _ = archive.ParseFormat(m.app.Cfg.Download.Format)
}
func (m *model) resetInput(prompt, value string) {
	m.input = textinput.New()
	m.input.Prompt = prompt
	m.input.SetValue(value)
	m.input.CursorEnd()
	m.input.Focus()
	m.input.Width = max(16, m.width-12)
}
func move(value, delta, length int) int {
	if length <= 0 {
		return 0
	}
	return (value + delta + length) % length
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m *model) visibleChapters() []int {
	needle := strings.ToLower(strings.TrimSpace(m.chapterFilter))
	out := []int{}
	for i, c := range m.chapters {
		if c == nil {
			continue
		}
		text := strings.ToLower(c.DisplayName() + " " + c.Language + " " + c.ID)
		if needle == "" || strings.Contains(text, needle) {
			out = append(out, i)
		}
	}
	if m.chapterCursor >= len(out) {
		m.chapterCursor = max(0, len(out)-1)
	}
	return out
}
func (m *model) chapterLabel(index int) string {
	c := m.chapters[index]
	return util.SanitizeTerminalText(fmt.Sprintf("%s  %s  %d pages", c.DisplayName(), c.Language, c.PageCount))
}
func (m *model) selectedChapters() []*source.Chapter {
	out := []*source.Chapter{}
	for i, c := range m.chapters {
		if m.selected[i] && c != nil {
			out = append(out, c)
		}
	}
	return out
}

func validOutput(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("output cannot be empty")
	}
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return errors.New("output is an existing file")
	}
	return nil
}

var configLabels = []string{"provider", "language", "output", "format", "existing files", "retain source", "http timeout", "page downloads", "chapter downloads", "history max", "cache"}

func (m *model) configValue() string { return m.configValueAt(m.configCursor) }
func (m *model) configValueAt(i int) string {
	c := m.draft
	switch i {
	case 0:
		return c.Provider
	case 1:
		return c.Language
	case 2:
		return c.Download.Dir
	case 3:
		return c.Download.Format
	case 4:
		return c.Download.ExistingFileMode
	case 5:
		return strconv.FormatBool(c.Download.RetainSource)
	case 6:
		return c.HTTP.Timeout.String()
	case 7:
		return strconv.Itoa(c.Concurrency.PageDownloads)
	case 8:
		return strconv.Itoa(c.Concurrency.ChapterDownloads)
	case 9:
		return strconv.Itoa(c.Search.HistoryMax)
	default:
		return c.Dirs.Cache
	}
}
func (m *model) setConfigValue(raw string) error {
	value := strings.TrimSpace(raw)
	if value == "" {
		return errors.New("value cannot be empty")
	}
	next := m.draft.Clone()
	switch m.configCursor {
	case 0:
		next.Provider = value
	case 1:
		next.Language = value
	case 2:
		next.Download.Dir = value
	case 3:
		next.Download.Format = value
	case 4:
		next.Download.ExistingFileMode = value
	case 5:
		v, e := strconv.ParseBool(value)
		if e != nil {
			return e
		}
		next.Download.RetainSource = v
	case 6:
		v, e := time.ParseDuration(value)
		if e != nil {
			return e
		}
		next.HTTP.Timeout = v
	case 7:
		v, e := strconv.Atoi(value)
		if e != nil {
			return e
		}
		next.Concurrency.PageDownloads = v
	case 8:
		v, e := strconv.Atoi(value)
		if e != nil {
			return e
		}
		next.Concurrency.ChapterDownloads = v
	case 9:
		v, e := strconv.Atoi(value)
		if e != nil {
			return e
		}
		next.Search.HistoryMax = v
	case 10:
		next.Dirs.Cache = value
	}
	if err := next.Validate(); err != nil {
		return err
	}
	m.draft = next
	return nil
}
