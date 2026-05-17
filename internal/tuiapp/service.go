package tuiapp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/evgen2571/mangate/internal/usecase"
)

type runtime interface {
	search(context.Context, string) ([]*source.Manga, error)
	searchHistory() ([]string, error)
	addSearchQuery(string) error
	chapters(context.Context, *source.Manga) ([]*source.Chapter, error)
	coverPath(context.Context, *source.Manga) (string, error)
	download(context.Context, *source.Manga, []*source.Chapter, func(usecase.DownloadProgress)) error
	config() config.Config
	applyConfig(config.Config) error
	applyAndSaveConfig(config.Config) error
}

type service struct {
	runtime runtime
}

func New(a *app.App) Service {
	return service{runtime: appRuntime{app: a}}
}

func (s service) Search(ctx context.Context, query string) ([]SearchResult, error) {
	results, err := s.runtime.search(ctx, query)
	if err != nil {
		return nil, err
	}

	cfg := s.runtime.config()
	mapped := make([]SearchResult, 0, len(results))
	for _, manga := range results {
		if manga == nil {
			continue
		}
		mapped = append(mapped, mapSearchResult(manga, cfg.Language))
	}
	_ = s.runtime.addSearchQuery(query)
	return mapped, nil
}

func (s service) SearchHistory(context.Context) ([]string, error) {
	return s.runtime.searchHistory()
}

func (s service) LoadChapters(ctx context.Context, selected SearchResult) (MangaDetails, []ChapterItem, error) {
	manga := &source.Manga{
		ID:    selected.ID,
		Title: selected.Title,
		URL:   selected.URL,
	}

	chapters, err := s.runtime.chapters(ctx, manga)
	if err != nil {
		return MangaDetails{}, nil, err
	}

	items := make([]ChapterItem, 0, len(chapters))
	count := 0
	for idx, chapter := range chapters {
		if chapter == nil {
			continue
		}
		count++
		items = append(items, mapChapterItem(chapter, idx))
	}

	return MangaDetails{
		ID:           selected.ID,
		Title:        selected.Title,
		URL:          selected.URL,
		SummaryMD:    selected.SummaryMD,
		ChapterCount: count,
	}, items, nil
}

func (s service) LoadCover(ctx context.Context, selected SearchResult) (CoverResult, error) {
	path, err := s.runtime.coverPath(ctx, &source.Manga{
		ID:    selected.ID,
		Title: selected.Title,
		URL:   selected.URL,
	})
	if err != nil {
		return CoverResult{}, err
	}

	return CoverResult{
		MangaID: selected.ID,
		Path:    path,
	}, nil
}

func (s service) Download(ctx context.Context, req DownloadRequest, notify func(DownloadProgress)) error {
	manga := &source.Manga{
		ID:    req.Manga.ID,
		Title: req.Manga.Title,
		URL:   req.Manga.URL,
	}

	chapters := make([]*source.Chapter, 0, len(req.Chapters))
	for _, chapter := range req.Chapters {
		chapters = append(chapters, &source.Chapter{
			ID:    chapter.ID,
			Index: chapter.Index,
			Title: chapter.Title,
			URL:   chapter.URL,
		})
	}

	return s.runtime.download(ctx, manga, chapters, func(progress usecase.DownloadProgress) {
		if notify == nil {
			return
		}
		notify(mapDownloadProgress(progress))
	})
}

func (s service) Config() ConfigState {
	return mapConfigState(s.runtime.config())
}

func (s service) ApplyConfig(_ context.Context, state ConfigState) (ConfigState, error) {
	cfg := toConfig(state, s.runtime.config())
	if err := s.runtime.applyConfig(cfg); err != nil {
		return ConfigState{}, err
	}
	return mapConfigState(s.runtime.config()), nil
}

func (s service) SaveConfig(_ context.Context, state ConfigState) (ConfigState, error) {
	cfg := toConfig(state, s.runtime.config())
	if err := s.runtime.applyAndSaveConfig(cfg); err != nil {
		return mapConfigState(s.runtime.config()), err
	}
	return mapConfigState(s.runtime.config()), nil
}

func mapSearchResult(manga *source.Manga, language string) SearchResult {
	return SearchResult{
		ID:           manga.ID,
		Title:        manga.Title,
		URL:          manga.URL,
		SummaryMD:    summaryMarkdown(manga.Metadata.Description, language),
		ChapterCount: manga.Metadata.ChapterCount,
	}
}

func mapChapterItem(chapter *source.Chapter, idx int) ChapterItem {
	return ChapterItem{
		ID:          chapter.ID,
		Index:       chapter.Index,
		Title:       chapter.Title,
		DisplayText: chapter.DisplayTitle(idx),
		URL:         chapter.URL,
	}
}

func mapDownloadProgress(progress usecase.DownloadProgress) DownloadProgress {
	chapters := make([]ChapterProgress, 0, len(progress.Chapters))
	for _, chapter := range progress.Chapters {
		chapters = append(chapters, ChapterProgress{
			Name:           chapter.Name,
			CompletedPages: chapter.CompletedPages,
			TotalPages:     chapter.TotalPages,
			Active:         chapter.Active,
			Completed:      chapter.Completed,
		})
	}

	return DownloadProgress{
		CompletedPages:    progress.CompletedPages,
		TotalPages:        progress.TotalPages,
		CompletedChapters: progress.CompletedChapters,
		TotalChapters:     progress.TotalChapters,
		Chapters:          chapters,
	}
}

func mapConfigState(cfg config.Config) ConfigState {
	return ConfigState{
		Provider:           cfg.Provider,
		Language:           cfg.Language,
		HTTPTimeout:        cfg.HTTP.Timeout,
		DownloadDir:        cfg.Download.Dir,
		DownloadType:       cfg.Download.Type,
		PageDownloads:      cfg.Concurrency.PageDownloads,
		ChapterDownloads:   cfg.Concurrency.ChapterDownloads,
		SearchHistoryMax:   cfg.Search.HistoryMax,
		CacheDir:           cfg.Dirs.Cache,
		TempDir:            cfg.Dirs.Temp,
		MangaDexSiteURL:    cfg.Providers.MangaDex.SiteURL,
		MangaDexBaseURL:    cfg.Providers.MangaDex.BaseURL,
		MangaDexUploadsURL: cfg.Providers.MangaDex.UploadsURL,
	}
}

func toConfig(state ConfigState, base config.Config) config.Config {
	next := base
	next.Provider = state.Provider
	next.Language = state.Language
	next.HTTP.Timeout = state.HTTPTimeout
	next.Download.Dir = state.DownloadDir
	next.Download.Type = state.DownloadType
	next.Concurrency.PageDownloads = state.PageDownloads
	next.Concurrency.ChapterDownloads = state.ChapterDownloads
	next.Search.HistoryMax = state.SearchHistoryMax
	next.Dirs.Cache = state.CacheDir
	next.Dirs.Temp = state.TempDir
	next.Providers.MangaDex.SiteURL = state.MangaDexSiteURL
	next.Providers.MangaDex.BaseURL = state.MangaDexBaseURL
	next.Providers.MangaDex.UploadsURL = state.MangaDexUploadsURL
	return next
}

func summaryMarkdown(descriptions map[string]string, language string) string {
	if len(descriptions) == 0 {
		return ""
	}

	for _, key := range []string{strings.TrimSpace(language), "en"} {
		if key == "" {
			continue
		}
		if desc := strings.TrimSpace(descriptions[key]); desc != "" {
			return desc
		}
	}

	keys := make([]string, 0, len(descriptions))
	for key := range descriptions {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if desc := strings.TrimSpace(descriptions[key]); desc != "" {
			return desc
		}
	}

	return ""
}

type appRuntime struct {
	app *app.App
}

func (r appRuntime) search(ctx context.Context, query string) ([]*source.Manga, error) {
	if r.app == nil {
		return nil, fmt.Errorf("tuiapp: app is not configured")
	}
	return r.app.UseCases().SearchManga(ctx, query)
}

func (r appRuntime) searchHistory() ([]string, error) {
	if r.app == nil {
		return nil, fmt.Errorf("tuiapp: app is not configured")
	}
	return r.app.SearchHistory()
}

func (r appRuntime) addSearchQuery(query string) error {
	if r.app == nil {
		return fmt.Errorf("tuiapp: app is not configured")
	}
	return r.app.AddSearchQuery(query)
}

func (r appRuntime) chapters(ctx context.Context, manga *source.Manga) ([]*source.Chapter, error) {
	if r.app == nil {
		return nil, fmt.Errorf("tuiapp: app is not configured")
	}
	return r.app.UseCases().Chapters(ctx, manga)
}

func (r appRuntime) coverPath(ctx context.Context, manga *source.Manga) (string, error) {
	if r.app == nil {
		return "", fmt.Errorf("tuiapp: app is not configured")
	}
	return r.app.UseCases().CoverPath(ctx, manga)
}

func (r appRuntime) download(ctx context.Context, manga *source.Manga, chapters []*source.Chapter, notify func(usecase.DownloadProgress)) error {
	if r.app == nil {
		return fmt.Errorf("tuiapp: app is not configured")
	}
	return r.app.UseCases().DownloadChapters(ctx, manga, chapters, notify)
}

func (r appRuntime) config() config.Config {
	if r.app == nil {
		return config.Config{}
	}
	return r.app.Cfg
}

func (r appRuntime) applyConfig(cfg config.Config) error {
	if r.app == nil {
		return fmt.Errorf("tuiapp: app is not configured")
	}
	return r.app.ApplyConfig(cfg)
}

func (r appRuntime) applyAndSaveConfig(cfg config.Config) error {
	if r.app == nil {
		return fmt.Errorf("tuiapp: app is not configured")
	}
	return r.app.ApplyAndSaveConfig(cfg)
}
