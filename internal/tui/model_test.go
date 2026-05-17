package tui

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/evgen2571/mangate/internal/tuiapp"
)

func TestModelFullMangaDownloadStartsDownloadAfterMatchingChaptersLoad(t *testing.T) {
	manga := &source.Manga{ID: "manga-a", Title: "Manga A"}
	chapters := []*source.Chapter{
		{ID: "chapter-a", Index: "1"},
		nil,
		{ID: "chapter-b", Index: "2"},
	}
	m := model{
		state:                    stateLoading,
		pendingFullMangaDownload: manga.ID,
	}

	updated, cmd := m.Update(chaptersLoadedMsg{Manga: manga, Chapters: chapters})
	got, ok := updated.(model)
	if !ok {
		t.Fatalf("Update() returned %T, want model", updated)
	}

	if got.state != stateDownloading {
		t.Fatalf("state = %v, want stateDownloading", got.state)
	}
	if got.pendingFullMangaDownload != "" {
		t.Fatalf("pendingFullMangaDownload = %#v, want nil", got.pendingFullMangaDownload)
	}
	if cmd == nil {
		t.Fatalf("Update() returned nil command")
	}
	if got.downloading.detail != "2 chapters selected" {
		t.Fatalf("downloading detail = %q, want %q", got.downloading.detail, "2 chapters selected")
	}
}

func TestModelFullMangaDownloadIgnoresStaleChaptersLoad(t *testing.T) {
	pending := &source.Manga{ID: "manga-pending", Title: "Pending"}
	stale := &source.Manga{ID: "manga-stale", Title: "Stale"}
	m := model{
		state:                    stateLoading,
		pendingFullMangaDownload: pending.ID,
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
	if got.pendingFullMangaDownload != pending.ID {
		t.Fatalf("pendingFullMangaDownload = %#v, want pending manga id %q", got.pendingFullMangaDownload, pending.ID)
	}
	if cmd != nil {
		t.Fatalf("Update() returned unexpected command for stale chapters load")
	}
}

func TestModelFullMangaDownloadShowsStatusWhenNoChaptersLoad(t *testing.T) {
	manga := &source.Manga{ID: "manga-a", Title: "Manga A"}
	m := model{
		state:                    stateLoading,
		pendingFullMangaDownload: manga.ID,
	}

	updated, cmd := m.Update(chaptersLoadedMsg{Manga: manga, Chapters: []*source.Chapter{nil}})
	got, ok := updated.(model)
	if !ok {
		t.Fatalf("Update() returned %T, want model", updated)
	}

	if got.state != stateChapters {
		t.Fatalf("state = %v, want stateChapters", got.state)
	}
	if got.pendingFullMangaDownload != "" {
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
	m := model{pendingFullMangaDownload: pending.ID}

	updated, _ := m.Update(chaptersOpenRequestedMsg{Result: tuiapp.SearchResult{ID: manga.ID, Title: manga.Title, URL: manga.URL}})
	got, ok := updated.(model)
	if !ok {
		t.Fatalf("Update() returned %T, want model", updated)
	}

	if got.pendingFullMangaDownload != "" {
		t.Fatalf("pendingFullMangaDownload = %#v, want nil", got.pendingFullMangaDownload)
	}
	if got.state != stateLoading {
		t.Fatalf("state = %v, want stateLoading", got.state)
	}
}

func TestNewModelLoadsSearchHistoryFromService(t *testing.T) {
	svc := fakeTUIService{history: []string{"Dandadan", "Puniru"}}

	got, ok := newModel(nil, svc).(*model)
	if !ok {
		t.Fatalf("newModel() returned %T, want *model", newModel(nil, svc))
	}

	if len(got.search.history) != 2 || got.search.history[0] != "Dandadan" || got.search.history[1] != "Puniru" {
		t.Fatalf("search history = %#v, want service history", got.search.history)
	}
}

func TestModelConfigSaveUsesAppFacadeStatus(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Dirs.Cache = t.TempDir()
	cfg.Dirs.Temp = t.TempDir()
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

type fakeTUIService struct {
	searchResults []tuiapp.SearchResult
	searchErr     error
	history       []string
	historyErr    error
}

func (f fakeTUIService) Search(context.Context, string) ([]tuiapp.SearchResult, error) {
	return f.searchResults, f.searchErr
}

func (f fakeTUIService) SearchHistory(context.Context) ([]string, error) {
	return f.history, f.historyErr
}

func (f fakeTUIService) LoadChapters(context.Context, tuiapp.SearchResult) (tuiapp.MangaDetails, []tuiapp.ChapterItem, error) {
	return tuiapp.MangaDetails{}, nil, nil
}

func (f fakeTUIService) LoadCover(context.Context, tuiapp.SearchResult, tuiapp.CoverSize) (tuiapp.CoverResult, error) {
	return tuiapp.CoverResult{}, nil
}

func (f fakeTUIService) Download(context.Context, tuiapp.DownloadRequest, func(tuiapp.DownloadProgress)) error {
	return nil
}

func (f fakeTUIService) Config() tuiapp.ConfigState {
	return tuiapp.ConfigState{}
}

func (f fakeTUIService) ApplyConfig(context.Context, tuiapp.ConfigState) (tuiapp.ConfigState, error) {
	return tuiapp.ConfigState{}, nil
}

func (f fakeTUIService) SaveConfig(context.Context, tuiapp.ConfigState) (tuiapp.ConfigState, error) {
	return tuiapp.ConfigState{}, nil
}
