package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/source"
)

func TestModelFullMangaDownloadOpensFormatSelectionAfterMatchingChaptersLoad(t *testing.T) {
	manga := &source.Manga{ID: "manga-a", Title: "Manga A"}
	chapters := []*source.Chapter{
		{ID: "chapter-a", Index: "1"},
		nil,
		{ID: "chapter-b", Index: "2"},
	}
	a, err := app.New(config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	m := model{
		app:                      a,
		state:                    stateLoading,
		pendingFullMangaDownload: manga,
	}

	updated, cmd := m.Update(chaptersLoadedMsg{Manga: manga, Chapters: chapters})
	got, ok := updated.(model)
	if !ok {
		t.Fatalf("Update() returned %T, want model", updated)
	}

	if got.state != stateFormat {
		t.Fatalf("state = %v, want stateFormat", got.state)
	}
	if got.pendingFullMangaDownload != nil {
		t.Fatalf("pendingFullMangaDownload = %#v, want nil", got.pendingFullMangaDownload)
	}
	if cmd != nil {
		t.Fatalf("Update() returned unexpected command")
	}
	if len(got.confirm.chapters) != 2 {
		t.Fatalf("confirmation chapters = %#v", got.confirm.chapters)
	}
}

func TestModelSearchFailureReturnsToEditableSearch(t *testing.T) {
	m := model{state: stateLoading, search: newSearchModel(nil)}
	updated, cmd := m.Update(searchFailedMsg{Err: errors.New("provider unavailable")})
	got, ok := updated.(model)
	if !ok {
		t.Fatalf("Update() returned %T, want model", updated)
	}
	if got.state != stateSearch || cmd != nil {
		t.Fatalf("state = %v, command = %v", got.state, cmd)
	}
	if got.search.status == "" || got.search.input.Focused() == false {
		t.Fatalf("search = %#v, want visible editable error", got.search)
	}
}

func TestTUISanitizationDoesNotChangeProviderIdentifiers(t *testing.T) {
	manga := &source.Manga{ID: "id\x1b[2J", Title: "title\nnext", Metadata: source.MangaMetadata{Description: map[string]string{"en": "body\ttext"}}}
	chapter := &source.Chapter{ID: "chapter\x1b[2J", Title: "part\nnext"}
	if !strings.Contains(manga.ID, "\x1b") || !strings.Contains(chapter.ID, "\x1b") {
		t.Fatalf("provider identifiers were changed: %#v %#v", manga, chapter)
	}
	if strings.Contains(resultItem{value: manga}.Title(), "\n") || strings.Contains(chapter.DisplayName(), "\n") {
		t.Fatalf("display helpers leaked controls")
	}
}

func TestCompletionModelReportsArchivePaths(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Download.Dir = t.TempDir()
	cfg.Download.Format = "cbz"
	a, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	manga := &source.Manga{ID: "title", Title: "Example"}
	chapters := []*source.Chapter{{ID: "one", Index: "1"}, {ID: "two", Index: "2"}}
	completion := newCompletionModel(a, manga, chapters, nil)
	if !completion.success || len(completion.paths) != 2 || !strings.HasSuffix(completion.paths[0], ".cbz") {
		t.Fatalf("completion = %#v", completion)
	}
}

func TestCompletionModelReportsPerChapterOutcomes(t *testing.T) {
	completion := completionModel{
		success: false,
		summary: "Download finished: 1 completed, 1 skipped/reused, 2 failed or incomplete.",
		outcomes: []chapterOutcome{
			{Name: "One", Status: "complete", Path: "one.cbz"},
			{Name: "Two", Status: "skipped", Path: "two.cbz"},
			{Name: "Three", Status: "incomplete", Path: "three"},
			{Name: "Four", Status: "archive_failed", Path: "four.cbz"},
		},
		paths: []string{"one.cbz", "two.cbz", "three", "four.cbz"},
	}
	view := completion.View()
	for _, want := range []string{"Completed: 1", "Skipped/reused: 1", "Failed or incomplete: 2", "Archive failures: 1", "[incomplete] three", "[archive_failed] four.cbz"} {
		if !strings.Contains(view, want) {
			t.Fatalf("completion view = %q, want %q", view, want)
		}
	}
}

func TestQuitCancelsAnActiveDownloadBeforeLeavingTheTUI(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := model{state: stateDownloading, downloadCancel: cancel, keys: newKeyMap()}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	got := updated.(model)
	if cmd != nil || got.downloadCancel != nil || got.state != stateDownloading || got.downloading.status != "Cancelling download..." {
		t.Fatalf("model after cancel = %#v, command = %v", got, cmd)
	}
	select {
	case <-ctx.Done():
	default:
		t.Fatal("download context was not cancelled")
	}
}

func TestNewWithContextKeepsCallerLifecycleContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, err := app.New(config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	created := NewWithContext(a, ctx)
	m, ok := created.(*model)
	if !ok || m.baseContext != ctx {
		t.Fatalf("NewWithContext() = %#v, want model with caller context", created)
	}
}

func TestOutputPathStepAppliesPathAndPreservesBackNavigation(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Download.Dir = t.TempDir()
	a, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	manga := &source.Manga{ID: "title", Title: "Example"}
	chapters := []*source.Chapter{{ID: "one", Index: "1"}}
	m := model{app: a, state: stateFormat, format: newFormatModel("cbz"), confirm: newConfirmModel(cfg, manga, chapters, "cbz"), output: newOutputModel(cfg.Download.Dir)}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(model)
	if got.state != stateOutput {
		t.Fatalf("state after format confirmation = %v, want stateOutput", got.state)
	}
	outputDir := filepath.Join(t.TempDir(), "nested", "library")
	updated, _ = got.Update(outputPathSelectedMsg{Path: outputDir})
	got = updated.(model)
	if got.state != stateConfirm || got.app.Cfg.Download.Dir != outputDir || got.confirm.output != outputDir {
		t.Fatalf("output confirmation = %#v", got)
	}
	updated, _ = got.Update(goBackMsg{})
	got = updated.(model)
	if got.state != stateOutput {
		t.Fatalf("state after confirmation back = %v, want stateOutput", got.state)
	}
	updated, _ = got.Update(goBackMsg{})
	got = updated.(model)
	if got.state != stateFormat {
		t.Fatalf("state after output back = %v, want stateFormat", got.state)
	}
}

func TestLocalChapterStatusesRecognizesCompleteDirectoriesAndArchives(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Download.Dir = t.TempDir()
	manga := &source.Manga{ID: "title", Title: "Example"}
	chapters := []*source.Chapter{{ID: "directory", Index: "1"}, {ID: "archive", Index: "2"}, {ID: "missing", Index: "3"}}
	names := downloader.ChapterDirectoryNames(chapters)
	titleDir := filepath.Join(cfg.Download.Dir, downloader.TitleDirectoryName(manga))
	writeChapterStateForTUI(t, filepath.Join(titleDir, names[0]), 1, true)
	cfg.Download.Format = "cbz"
	if err := os.MkdirAll(titleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(titleDir, names[1]+".cbz"), []byte("archive"), 0o644); err != nil {
		t.Fatal(err)
	}
	statuses := localChapterStatuses(cfg, manga, chapters)
	if statuses["directory"] != "complete" || statuses["archive"] != "archive" || statuses["missing"] != "missing" {
		t.Fatalf("local statuses = %#v", statuses)
	}
}

func TestConfirmationPlanShowsArchivePathsAndExistingOutputs(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Download.Dir = t.TempDir()
	cfg.Download.ExistingFileMode = "skip"
	cfg.Download.RetainSource = false
	manga := &source.Manga{ID: "title", Title: "Example"}
	chapters := []*source.Chapter{{ID: "one", Index: "1", PageCount: 3}, {ID: "two", Index: "2"}}
	plan := newConfirmModel(cfg, manga, chapters, "cbz")
	if len(plan.plannedPaths) != 2 || !strings.HasSuffix(plan.plannedPaths[0], ".cbz") || plan.expectedPages != 3 || plan.unknownPageCounts != 1 {
		t.Fatalf("confirmation plan = %#v", plan)
	}
	if err := os.MkdirAll(filepath.Dir(plan.plannedPaths[0]), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.plannedPaths[0], []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan = newConfirmModel(cfg, manga, chapters, "cbz")
	if len(plan.existingPaths) != 1 || !strings.Contains(plan.View(), "Source pages: removed") || !strings.Contains(plan.View(), "Existing outputs: 1") {
		t.Fatalf("confirmation view = %q", plan.View())
	}
}

func TestApplyingConfigRefreshesConfirmationPlan(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Download.Dir = t.TempDir()
	a, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	manga := &source.Manga{ID: "title", Title: "Example"}
	chapters := []*source.Chapter{{ID: "one", Index: "1"}}
	m := model{app: a, state: stateConfig, previousState: stateConfirm, format: newFormatModel("cbz"), confirm: newConfirmModel(cfg, manga, chapters, "cbz"), config: newConfigModel(cfg)}
	updatedCfg := cfg.Clone()
	updatedCfg.Download.Dir = t.TempDir()
	updated, _ := m.Update(configApplyRequestedMsg{Config: updatedCfg})
	got := updated.(model)
	if got.confirm.output != updatedCfg.Download.Dir || !strings.HasPrefix(got.confirm.plannedPaths[0], updatedCfg.Download.Dir) {
		t.Fatalf("confirmation plan was not refreshed: %#v", got.confirm)
	}
}

func TestModelFullMangaDownloadIgnoresStaleChaptersLoad(t *testing.T) {
	pending := &source.Manga{ID: "manga-pending", Title: "Pending"}
	stale := &source.Manga{ID: "manga-stale", Title: "Stale"}
	m := model{
		state:                    stateLoading,
		pendingFullMangaDownload: pending,
	}

	updated, cmd := m.Update(chaptersLoadedMsg{
		Manga:    stale,
		Chapters: []*source.Chapter{{ID: "stale-chapter", Index: "1"}},
	})
	got, ok := updated.(model)
	if !ok {
		t.Fatalf("Update() returned %T, want model", updated)
	}

	if got.state != stateChapters {
		t.Fatalf("state = %v, want stateChapters", got.state)
	}
	if got.pendingFullMangaDownload != pending {
		t.Fatalf("pendingFullMangaDownload = %#v, want pending manga", got.pendingFullMangaDownload)
	}
	if cmd != nil {
		t.Fatalf("Update() returned unexpected command for stale chapters load")
	}
}

func TestModelFullMangaDownloadShowsStatusWhenNoChaptersLoad(t *testing.T) {
	manga := &source.Manga{ID: "manga-a", Title: "Manga A"}
	m := model{
		state:                    stateLoading,
		pendingFullMangaDownload: manga,
	}

	updated, cmd := m.Update(chaptersLoadedMsg{Manga: manga, Chapters: []*source.Chapter{nil}})
	got, ok := updated.(model)
	if !ok {
		t.Fatalf("Update() returned %T, want model", updated)
	}

	if got.state != stateChapters {
		t.Fatalf("state = %v, want stateChapters", got.state)
	}
	if got.pendingFullMangaDownload != nil {
		t.Fatalf("pendingFullMangaDownload = %#v, want nil", got.pendingFullMangaDownload)
	}
	if got.chapters.status != "no chapters to download" {
		t.Fatalf("chapters status = %q, want %q", got.chapters.status, "no chapters to download")
	}
	if cmd != nil {
		t.Fatalf("Update() returned unexpected command")
	}
}

func TestModelPlainChaptersOpenClearsPendingFullDownload(t *testing.T) {
	manga := &source.Manga{ID: "manga-a", Title: "Manga A"}
	pending := &source.Manga{ID: "pending", Title: "Pending"}
	m := model{pendingFullMangaDownload: pending}

	updated, _ := m.Update(chaptersOpenRequestedMsg{Manga: manga})
	got, ok := updated.(model)
	if !ok {
		t.Fatalf("Update() returned %T, want model", updated)
	}

	if got.pendingFullMangaDownload != nil {
		t.Fatalf("pendingFullMangaDownload = %#v, want nil", got.pendingFullMangaDownload)
	}
	if got.state != stateLoading {
		t.Fatalf("state = %v, want stateLoading", got.state)
	}
}

func TestModelConfigSaveUsesAppFacadeStatus(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Dirs.Cache = t.TempDir()
	cfg.Download.Dir = t.TempDir()

	a, err := app.New(cfg)
	if err != nil {
		t.Fatalf("app.New() error = %v", err)
	}
	a.ConfigPath = filepath.Join(t.TempDir(), "config.json")

	updatedCfg := cfg
	updatedCfg.Language = "ru"
	m := model{app: a, config: newConfigModel(cfg)}

	updated, cmd := m.Update(configSaveRequestedMsg{Config: updatedCfg})
	got, ok := updated.(model)
	if !ok {
		t.Fatalf("Update() returned %T, want model", updated)
	}
	if cmd != nil {
		t.Fatalf("Update() returned unexpected command")
	}
	if got.config.status != "saved and applied" {
		t.Fatalf("config status = %q, want %q", got.config.status, "saved and applied")
	}
	if got.config.draft != updatedCfg {
		t.Fatalf("draft config = %#v, want %#v", got.config.draft, updatedCfg)
	}
}

func TestModelConfigSaveShowsApplyAndSaveFailuresFromAppFacade(t *testing.T) {
	cfg := config.DefaultConfig()
	a, err := app.New(cfg)
	if err != nil {
		t.Fatalf("app.New() error = %v", err)
	}
	a.ConfigPath = filepath.Join(t.TempDir(), "config.json")

	invalid := cfg
	invalid.Provider = ""
	m := model{app: a, config: newConfigModel(cfg)}

	updated, _ := m.Update(configSaveRequestedMsg{Config: invalid})
	got, ok := updated.(model)
	if !ok {
		t.Fatalf("Update() returned %T, want model", updated)
	}
	if got.config.status != "apply failed: provider cannot be empty" {
		t.Fatalf("apply failure status = %q", got.config.status)
	}

	a.ConfigPath = "  "
	validUpdate := cfg
	validUpdate.Language = "ru"
	m = model{app: a, config: newConfigModel(cfg)}

	updated, _ = m.Update(configSaveRequestedMsg{Config: validUpdate})
	got, ok = updated.(model)
	if !ok {
		t.Fatalf("Update() returned %T, want model", updated)
	}
	if got.config.status != "save failed: config path cannot be empty" {
		t.Fatalf("save failure status = %q", got.config.status)
	}
}
