package tuiapp

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/providers"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/evgen2571/mangate/internal/usecase"
)

func TestServiceSearchMapsResultsAndHistory(t *testing.T) {
	cfg := testConfig(t)

	provider := &fakeProvider{
		searchResults: []*source.Manga{{
			ID:    "manga-1",
			Title: "Dandadan",
			URL:   "https://example.com/manga-1",
			Metadata: source.MangaMetadata{
				Description: map[string]string{
					"en": "Occult chaos.",
					"ru": "Паранормальный хаос.",
				},
				ChapterCount: 182,
			},
		}},
	}

	a, err := app.New(cfg, app.WithRegistry(withFakeProviderRegistry(provider)))
	if err != nil {
		t.Fatalf("app.New() error = %v", err)
	}

	svc := New(a)
	results, err := svc.Search(context.Background(), "dandadan")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	wantResults := []SearchResult{{
		ID:           "manga-1",
		Title:        "Dandadan",
		URL:          "https://example.com/manga-1",
		SummaryMD:    "Occult chaos.",
		ChapterCount: 182,
	}}
	if !reflect.DeepEqual(results, wantResults) {
		t.Fatalf("Search() = %#v, want %#v", results, wantResults)
	}

	history, err := svc.SearchHistory(context.Background())
	if err != nil {
		t.Fatalf("SearchHistory() error = %v", err)
	}
	if !reflect.DeepEqual(history, []string{"dandadan"}) {
		t.Fatalf("SearchHistory() = %#v, want %#v", history, []string{"dandadan"})
	}
}

func TestServiceSearchReturnsResultsWhenHistoryPersistenceFails(t *testing.T) {
	runtime := &fakeRuntime{
		cfg: config.DefaultConfig(),
		searchFn: func(context.Context, string) ([]*source.Manga, error) {
			return []*source.Manga{{
				ID:    "manga-1",
				Title: "Dandadan",
				URL:   "https://example.com/manga-1",
			}}, nil
		},
		addSearchFn: func(string) error {
			return errors.New("history write failed")
		},
	}

	svc := service{runtime: runtime}
	results, err := svc.Search(context.Background(), "dandadan")
	if err != nil {
		t.Fatalf("Search() error = %v, want nil", err)
	}

	want := []SearchResult{{
		ID:    "manga-1",
		Title: "Dandadan",
		URL:   "https://example.com/manga-1",
	}}
	if !reflect.DeepEqual(results, want) {
		t.Fatalf("Search() = %#v, want %#v", results, want)
	}
}

func TestServiceLoadChaptersMapsChapterItems(t *testing.T) {
	cfg := testConfig(t)

	provider := &fakeProvider{
		chapters: []*source.Chapter{
			{ID: "chapter-1", Index: "1", Title: "Start", URL: "https://example.com/c1"},
			nil,
			{ID: "chapter-2", Title: "Bonus", URL: "https://example.com/c2"},
		},
	}

	a, err := app.New(cfg, app.WithRegistry(withFakeProviderRegistry(provider)))
	if err != nil {
		t.Fatalf("app.New() error = %v", err)
	}

	svc := New(a)
	selected := SearchResult{
		ID:           "manga-1",
		Title:        "Dandadan",
		URL:          "https://example.com/manga-1",
		SummaryMD:    "Occult chaos.",
		ChapterCount: 999,
	}

	details, chapters, err := svc.LoadChapters(context.Background(), selected)
	if err != nil {
		t.Fatalf("LoadChapters() error = %v", err)
	}

	if provider.chaptersManga == nil {
		t.Fatal("provider.chaptersManga = nil, want request manga")
	}
	if provider.chaptersManga.ID != selected.ID || provider.chaptersManga.Title != selected.Title || provider.chaptersManga.URL != selected.URL {
		t.Fatalf("provider manga = %#v, want selected result fields", provider.chaptersManga)
	}

	wantDetails := MangaDetails{
		ID:           selected.ID,
		Title:        selected.Title,
		URL:          selected.URL,
		SummaryMD:    selected.SummaryMD,
		ChapterCount: 2,
	}
	if details != wantDetails {
		t.Fatalf("details = %#v, want %#v", details, wantDetails)
	}

	wantChapters := []ChapterItem{
		{ID: "chapter-1", Index: "1", Title: "Start", DisplayText: "Chapter 1 - Start", URL: "https://example.com/c1"},
		{ID: "chapter-2", Index: "", Title: "Bonus", DisplayText: "Bonus", URL: "https://example.com/c2"},
	}
	if !reflect.DeepEqual(chapters, wantChapters) {
		t.Fatalf("chapters = %#v, want %#v", chapters, wantChapters)
	}
}

func TestServiceDownloadMapsRequestAndProgress(t *testing.T) {
	runtime := &fakeRuntime{
		cfg: config.DefaultConfig(),
		downloadFn: func(_ context.Context, manga *source.Manga, chapters []*source.Chapter, notify func(usecase.DownloadProgress)) error {
			if manga.ID != "manga-1" || manga.Title != "Dandadan" || manga.URL != "https://example.com/manga-1" {
				t.Fatalf("download manga = %#v, want mapped request manga", manga)
			}

			wantChapters := []*source.Chapter{
				{ID: "chapter-1", Index: "1", Title: "Start", URL: "https://example.com/c1"},
				{ID: "chapter-2", Index: "2", Title: "Next", URL: "https://example.com/c2"},
			}
			if !reflect.DeepEqual(chapters, wantChapters) {
				t.Fatalf("download chapters = %#v, want %#v", chapters, wantChapters)
			}

			notify(usecase.DownloadProgress{
				CompletedPages:    3,
				TotalPages:        5,
				CompletedChapters: 1,
				TotalChapters:     2,
				Chapters: []usecase.ChapterDownloadProgress{
					{Name: "Chapter 1", CompletedPages: 3, TotalPages: 5, Active: true},
					{Name: "Chapter 2", TotalPages: 7},
				},
			})
			return nil
		},
	}

	svc := service{runtime: runtime}
	req := DownloadRequest{
		Manga: MangaDetails{
			ID:    "manga-1",
			Title: "Dandadan",
			URL:   "https://example.com/manga-1",
		},
		Chapters: []ChapterItem{
			{ID: "chapter-1", Index: "1", Title: "Start", URL: "https://example.com/c1"},
			{ID: "chapter-2", Index: "2", Title: "Next", URL: "https://example.com/c2"},
		},
	}

	var got DownloadProgress
	err := svc.Download(context.Background(), req, func(progress DownloadProgress) {
		got = progress
	})
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}

	want := DownloadProgress{
		CompletedPages:    3,
		TotalPages:        5,
		CompletedChapters: 1,
		TotalChapters:     2,
		Chapters: []ChapterProgress{
			{Name: "Chapter 1", CompletedPages: 3, TotalPages: 5, Active: true},
			{Name: "Chapter 2", TotalPages: 7},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("progress = %#v, want %#v", got, want)
	}
}

func TestServiceApplyAndSaveConfigReturnsUpdatedState(t *testing.T) {
	cfg := testConfig(t)

	a, err := app.New(cfg)
	if err != nil {
		t.Fatalf("app.New() error = %v", err)
	}
	a.ConfigPath = filepath.Join(t.TempDir(), "config.json")

	svc := New(a)
	next := svc.Config()
	next.Language = "ru"
	next.HTTPTimeout = 42 * time.Second
	next.SearchHistoryMax = 77

	applied, err := svc.ApplyConfig(context.Background(), next)
	if err != nil {
		t.Fatalf("ApplyConfig() error = %v", err)
	}
	if applied.Language != "ru" || applied.HTTPTimeout != 42*time.Second || applied.SearchHistoryMax != 77 {
		t.Fatalf("ApplyConfig() = %#v, want updated state", applied)
	}
	if a.Cfg.Language != "ru" || a.Cfg.HTTP.Timeout != 42*time.Second || a.Cfg.Search.HistoryMax != 77 {
		t.Fatalf("app config after ApplyConfig = %#v, want updated runtime config", a.Cfg)
	}

	next.CacheDir = t.TempDir()
	saved, err := svc.SaveConfig(context.Background(), next)
	if err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
	if saved.CacheDir != next.CacheDir {
		t.Fatalf("SaveConfig() = %#v, want CacheDir %q", saved, next.CacheDir)
	}

	loaded, err := config.LoadFromPath(a.ConfigPath)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}
	if loaded.Language != "ru" || loaded.HTTP.Timeout != 42*time.Second || loaded.Search.HistoryMax != 77 || loaded.Dirs.Cache != next.CacheDir {
		t.Fatalf("saved config = %#v, want persisted updates", loaded)
	}
}

func TestServiceSaveConfigReturnsAppliedStateOnSaveFailure(t *testing.T) {
	saveErr := errors.New("disk full")
	runtime := &fakeRuntime{
		cfg: config.DefaultConfig(),
		saveFn: func(config.Config) error {
			return saveErr
		},
	}

	svc := service{runtime: runtime}
	next := svc.Config()
	next.Language = "ru"
	next.HTTPTimeout = 45 * time.Second

	saved, err := svc.SaveConfig(context.Background(), next)
	if !errors.Is(err, saveErr) {
		t.Fatalf("SaveConfig() error = %v, want %v", err, saveErr)
	}
	if saved.Language != "ru" || saved.HTTPTimeout != 45*time.Second {
		t.Fatalf("SaveConfig() state = %#v, want applied state", saved)
	}
	if runtime.cfg.Language != "ru" || runtime.cfg.HTTP.Timeout != 45*time.Second {
		t.Fatalf("runtime config after SaveConfig() = %#v, want applied config", runtime.cfg)
	}
}

func testConfig(t *testing.T) config.Config {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.Dirs.Cache = t.TempDir()
	cfg.Dirs.Temp = t.TempDir()
	cfg.Download.Dir = t.TempDir()
	return cfg
}

func withFakeProviderRegistry(provider providers.Provider) *providers.Registry {
	registry := providers.NewRegistry()
	registry.Register("mangadex", func(config.Config, *http.Client) (providers.Provider, error) {
		return provider, nil
	})
	return registry
}

type fakeProvider struct {
	searchResults []*source.Manga
	chapters      []*source.Chapter
	chaptersManga *source.Manga
}

func (p *fakeProvider) Name() string {
	return "fake"
}

func (p *fakeProvider) Search(context.Context, string) ([]*source.Manga, error) {
	return p.searchResults, nil
}

func (p *fakeProvider) Chapters(_ context.Context, manga *source.Manga) ([]*source.Chapter, error) {
	p.chaptersManga = manga
	return p.chapters, nil
}

func (p *fakeProvider) Pages(context.Context, *source.Chapter) ([]*source.Page, error) {
	return nil, nil
}

func (p *fakeProvider) Cover(context.Context, *source.Manga) (string, error) {
	return "", nil
}

type fakeRuntime struct {
	cfg          config.Config
	searchFn     func(context.Context, string) ([]*source.Manga, error)
	searchHistFn func() ([]string, error)
	addSearchFn  func(string) error
	chaptersFn   func(context.Context, *source.Manga) ([]*source.Chapter, error)
	coverFn      func(context.Context, *source.Manga) (string, error)
	downloadFn   func(context.Context, *source.Manga, []*source.Chapter, func(usecase.DownloadProgress)) error
	applyFn      func(config.Config) error
	saveFn       func(config.Config) error
}

func (r *fakeRuntime) search(ctx context.Context, query string) ([]*source.Manga, error) {
	return r.searchFn(ctx, query)
}

func (r *fakeRuntime) searchHistory() ([]string, error) {
	return r.searchHistFn()
}

func (r *fakeRuntime) addSearchQuery(query string) error {
	return r.addSearchFn(query)
}

func (r *fakeRuntime) chapters(ctx context.Context, manga *source.Manga) ([]*source.Chapter, error) {
	return r.chaptersFn(ctx, manga)
}

func (r *fakeRuntime) coverPath(ctx context.Context, manga *source.Manga) (string, error) {
	return r.coverFn(ctx, manga)
}

func (r *fakeRuntime) download(ctx context.Context, manga *source.Manga, chapters []*source.Chapter, notify func(usecase.DownloadProgress)) error {
	return r.downloadFn(ctx, manga, chapters, notify)
}

func (r *fakeRuntime) config() config.Config {
	return r.cfg
}

func (r *fakeRuntime) applyConfig(cfg config.Config) error {
	r.cfg = cfg
	if r.applyFn != nil {
		return r.applyFn(cfg)
	}
	return nil
}

func (r *fakeRuntime) applyAndSaveConfig(cfg config.Config) error {
	r.cfg = cfg
	if r.saveFn != nil {
		return r.saveFn(cfg)
	}
	return nil
}
