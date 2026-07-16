// Package interactive provides Mangate's deliberately plain terminal interface.
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

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/evgen2571/mangate/internal/usecase"
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
	status        string
	query         string
	results       []*source.Manga
	resultCursor  int
	chapters      []*source.Chapter
	chapterCursor int
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
	configCursor  int
	configEditing bool
	draft         config.Config
}

type searchDone struct {
	query   string
	results []*source.Manga
	err     error
}
type chaptersDone struct {
	manga    *source.Manga
	chapters []*source.Chapter
	err      error
	all      bool
}
type downloadProgress struct {
	completed, total, completedChapters, totalChapters int
	active                                             string
}
type downloadDone struct {
	err                        error
	completed, skipped, failed int
	paths                      []string
}

func New(a *app.App) tea.Model { return NewWithContext(a, context.Background()) }

func NewWithContext(a *app.App, ctx context.Context) tea.Model {
	if ctx == nil {
		ctx = context.Background()
	}
	m := &model{app: a, ctx: ctx, screen: searchScreen, selected: map[int]bool{}, rangeAnchor: -1, format: archive.FormatDirectory}
	if a != nil {
		m.format, _ = archive.ParseFormat(a.Cfg.Download.Format)
	}
	m.resetInput("search: ", "title")
	return m
}

func NewWithSearchResults(a *app.App, ctx context.Context, query string, results []*source.Manga) tea.Model {
	m := NewWithContext(a, ctx).(*model)
	m.query, m.results, m.screen = query, results, resultsScreen
	return m
}

func (m *model) Init() tea.Cmd { return textinput.Blink }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.Width = max(16, msg.Width-12)
		return m, nil
	case searchDone:
		if msg.err != nil {
			m.status = "search: " + msg.err.Error()
			m.screen = searchScreen
			return m, nil
		}
		m.query, m.results, m.resultCursor, m.status, m.screen = msg.query, msg.results, 0, "", resultsScreen
		return m, nil
	case chaptersDone:
		if msg.err != nil {
			m.status = "chapters: " + msg.err.Error()
			m.screen = resultsScreen
			return m, nil
		}
		m.manga, m.chapters, m.chapterCursor, m.selected, m.chapterFilter, m.filtering, m.rangeAnchor = msg.manga, msg.chapters, 0, map[int]bool{}, "", false, -1
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
		m.completion = fmt.Sprintf("complete: %d  reused: %d  failed: %d", msg.completed, msg.skipped, msg.failed)
		if msg.err != nil {
			m.completion += "\n" + util.SanitizeTerminalText(msg.err.Error())
		}
		if len(msg.paths) > 0 {
			m.completion += "\n" + strings.Join(msg.paths, "\n")
		}
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			if m.screen == workingScreen && m.cancel != nil {
				m.cancel()
				m.status = "cancelling"
				return m, nil
			}
			return m, tea.Quit
		}
		if msg.String() == "ctrl+g" && m.screen != workingScreen {
			m.previous, m.screen, m.draft, m.configCursor, m.configEditing = m.screen, configScreen, m.app.Cfg.Clone(), 0, false
			return m, nil
		}
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
		if key, ok := msg.(tea.KeyMsg); ok && (key.String() == "enter" || key.String() == "esc") {
			m.screen = chaptersScreen
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
		m.status = "searching " + q
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
		return m, nil
	}
	switch key.String() {
	case "j", "down":
		m.resultCursor = move(m.resultCursor, 1, len(m.results))
	case "k", "up":
		m.resultCursor = move(m.resultCursor, -1, len(m.results))
	case "esc", "backspace":
		m.screen = searchScreen
		m.resetInput("search: ", "title")
	case "enter", "f":
		if len(m.results) == 0 || m.results[m.resultCursor] == nil {
			return m, nil
		}
		manga, all := m.results[m.resultCursor], key.String() == "f"
		m.status = "loading chapters"
		return m, func() tea.Msg {
			chapters, err := m.app.UseCases().Chapters(nil, manga)
			return chaptersDone{manga, chapters, err, all}
		}
	}
	return m, nil
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
		m.resetInput("filter: ", "chapter, language, local:complete")
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
		m.resetInput("output: ", m.app.Cfg.Download.Dir)
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
		m.resetInput("value: ", m.configValue())
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

func (m *model) View() string {
	lines := []string{"mangate"}
	switch m.screen {
	case searchScreen:
		lines = append(lines, "", m.input.View(), "", "enter search  ctrl+g config  q quit")
	case resultsScreen:
		lines = append(lines, "", "results: "+m.query)
		for i, r := range m.results {
			mark := " "
			if i == m.resultCursor {
				mark = ">"
			}
			title := "unknown"
			if r != nil {
				title = util.SanitizeTerminalText(r.Title)
			}
			lines = append(lines, fmt.Sprintf("%s %s", mark, title))
		}
		lines = append(lines, "", "j/k move  enter chapters  f all chapters  esc back")
	case chaptersScreen:
		lines = append(lines, "", util.SanitizeTerminalText(m.manga.Title))
		for position, index := range m.visibleChapters() {
			mark := " "
			if position == m.chapterCursor {
				mark = ">"
			}
			check := "[ ]"
			if m.selected[index] {
				check = "[x]"
			}
			lines = append(lines, fmt.Sprintf("%s %s %s", mark, check, m.chapterLabel(index)))
		}
		if m.filtering {
			lines = append(lines, "", m.input.View())
		}
		lines = append(lines, "", fmt.Sprintf("%d selected. space toggle  a all  d clear  l latest  r range  / filter  enter continue", len(m.selected)))
	case formatScreen:
		lines = append(lines, "", "format")
		for _, f := range []archive.Format{archive.FormatDirectory, archive.FormatCBZ, archive.FormatZIP} {
			mark := " "
			if f == m.format {
				mark = ">"
			}
			lines = append(lines, fmt.Sprintf("%s %s", mark, f))
		}
		lines = append(lines, "", "j/k choose  enter continue  esc back")
	case outputScreen:
		lines = append(lines, "", m.input.View(), "", "enter continue  esc back")
	case reviewScreen:
		lines = append(lines, "", fmt.Sprintf("%s", util.SanitizeTerminalText(m.manga.Title)), fmt.Sprintf("chapters: %d", len(m.selectedChapters())), "format: "+string(m.format), "output: "+m.app.Cfg.Download.Dir, "", "enter download  esc back")
	case configScreen:
		lines = append(lines, "", "config")
		for i, label := range configLabels {
			mark := " "
			if i == m.configCursor {
				mark = ">"
			}
			lines = append(lines, fmt.Sprintf("%s %s: %s", mark, label, m.configValueAt(i)))
		}
		if m.configEditing {
			lines = append(lines, "", m.input.View())
		}
		lines = append(lines, "", "enter edit  a apply  s save  esc back")
	case workingScreen:
		lines = append(lines, "", fmt.Sprintf("downloading %d/%d pages", m.progress.completed, m.progress.total), fmt.Sprintf("chapters %d/%d", m.progress.completedChapters, m.progress.totalChapters), m.progress.active, "", "q cancel")
	case doneScreen:
		lines = append(lines, "", m.completion, "", "enter back to chapters")
	}
	if m.status != "" {
		lines = append(lines, "", m.status)
	}
	return strings.Join(lines, "\n") + "\n"
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

func (m *model) waitForProgress() tea.Cmd {
	return func() tea.Msg { return <-m.progressCh }
}

func (m *model) download(ctx context.Context, chapters []*source.Chapter, progressCh chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		go func() {
			var completed, skipped, failed int
			paths := []string{}
			err := m.app.UseCases().DownloadChapters(ctx, m.manga, chapters, func(p usecase.DownloadProgress) {
				active := ""
				for _, c := range p.Chapters {
					if c.Active {
						active = c.Name
						break
					}
				}
				progressCh <- downloadProgress{p.CompletedPages, p.TotalPages, p.CompletedChapters, p.TotalChapters, active}
			})
			if err == nil {
				for _, chapter := range chapters {
					path := filepath.Join(m.app.Cfg.Download.Dir, downloader.TitleDirectoryName(m.manga), m.chapterDirectory(chapter))
					if m.format != archive.FormatDirectory {
						archivePath := path + m.format.Extension()
						result, archiveErr := archive.CreateFromDirectoryContext(ctx, archive.Options{Format: m.format, SourceDir: path, OutputPath: archivePath, ExistingFileMode: archive.ExistingFileMode(m.app.Cfg.Download.ExistingFileMode), RemoveSource: !m.app.Cfg.Download.RetainSource, Metadata: archive.Metadata{Provider: m.app.Cfg.Provider, TitleID: m.manga.ID, Title: m.manga.Title, ChapterID: chapter.ID, ChapterNumber: chapter.Index, ChapterTitle: chapter.Title, ExpectedPages: chapter.PageCount}})
						if archiveErr != nil {
							failed++
							err = errors.Join(err, archiveErr)
							continue
						}
						path = archivePath
						if result.Status == archive.StatusSkipped {
							skipped++
							paths = append(paths, path)
							continue
						}
					}
					completed++
					paths = append(paths, path)
				}
			}
			progressCh <- downloadDone{err, completed, skipped, failed, paths}
			close(progressCh)
		}()
		return nil
	}
}

func (m *model) chapterDirectory(chapter *source.Chapter) string {
	for index, candidate := range m.chapters {
		if candidate == chapter {
			return downloader.ChapterDirectoryNames(m.chapters)[index]
		}
	}
	return downloader.ChapterDirectoryNames([]*source.Chapter{chapter})[0]
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
