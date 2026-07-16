package tui

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/config"
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
