package usecase

import (
	"context"
	"fmt"
	"net/http"

	"github.com/evgen2571/mangate/internal/cache"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/providers"
	"github.com/evgen2571/mangate/internal/source"
)

type Deps struct {
	Cfg        config.Config
	Client     *http.Client
	Registry   *providers.Registry
	Downloader *downloader.Downloader
	Cache      *cache.Cache
}

type Service struct {
	deps Deps
}

func New(deps Deps) Service {
	return Service{deps: deps}
}

func (s Service) SearchManga(ctx context.Context, query string) ([]*source.Manga, error) {
	provider, err := s.provider()
	if err != nil {
		return nil, err
	}

	ctx, cancel := s.withTimeout(ctx)
	defer cancel()

	return provider.Search(ctx, query)
}

func (s Service) Chapters(ctx context.Context, manga *source.Manga) ([]*source.Chapter, error) {
	if manga == nil {
		return nil, fmt.Errorf("load chapters: nil manga")
	}

	provider, err := s.provider()
	if err != nil {
		return nil, err
	}

	ctx, cancel := s.withTimeout(ctx)
	defer cancel()

	return provider.Chapters(ctx, manga)
}

func (s Service) CoverPath(ctx context.Context, manga *source.Manga) (string, error) {
	if manga == nil {
		return "", fmt.Errorf("load cover: nil manga")
	}
	if s.deps.Cache == nil {
		return "", fmt.Errorf("load cover: cache is not configured")
	}

	provider, err := s.provider()
	if err != nil {
		return "", err
	}

	ctx, cancel := s.withTimeout(ctx)
	defer cancel()

	return s.deps.Cache.Get(ctx, provider, manga)
}

func (s Service) DownloadChapters(ctx context.Context, manga *source.Manga, chapters []*source.Chapter, notify func(downloader.DownloadProgress)) error {
	if s.deps.Downloader == nil {
		return fmt.Errorf("download chapters: downloader is not configured")
	}

	provider, err := s.provider()
	if err != nil {
		return err
	}

	downloadManga, err := buildDownloadManga(manga, chapters)
	if err != nil {
		return err
	}

	pageLoader := downloader.PageLoader(func(loaderCtx context.Context, chapter *source.Chapter) ([]*source.Page, error) {
		requestCtx, cancel := s.withTimeout(loaderCtx)
		defer cancel()

		pages, err := provider.Pages(requestCtx, chapter)
		if err != nil {
			return nil, fmt.Errorf("load pages for %s: %w", chapter.LogName(), err)
		}

		return pages, nil
	})

	return s.deps.Downloader.DownloadMangaWithProgressAndPageLoader(ctxOrBackground(ctx), downloadManga, pageLoader, notify)
}

func (s Service) provider() (providers.Provider, error) {
	if s.deps.Registry == nil {
		return nil, fmt.Errorf("provider: registry is not configured")
	}
	if s.deps.Client == nil {
		return nil, fmt.Errorf("provider: http client is not configured")
	}

	return s.deps.Registry.New(s.deps.Cfg.Provider, s.deps.Cfg, s.deps.Client)
}

func (s Service) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx = ctxOrBackground(ctx)
	if s.deps.Cfg.HTTP.Timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, s.deps.Cfg.HTTP.Timeout)
}

func ctxOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func buildDownloadManga(manga *source.Manga, chapters []*source.Chapter) (*source.Manga, error) {
	if manga == nil {
		return nil, fmt.Errorf("download chapters: nil manga")
	}
	if len(chapters) == 0 {
		return nil, fmt.Errorf("download chapters: no chapters selected")
	}

	downloadManga := &source.Manga{
		ID:       manga.ID,
		URL:      manga.URL,
		Title:    manga.Title,
		Cover:    manga.Cover,
		Metadata: manga.Metadata,
		Chapters: make([]*source.Chapter, 0, len(chapters)),
	}

	for _, chapter := range chapters {
		if chapter == nil {
			return nil, fmt.Errorf("download chapters: selected chapter is nil")
		}

		chapterCopy := *chapter
		chapterCopy.From = downloadManga
		chapterCopy.Pages = nil
		downloadManga.Chapters = append(downloadManga.Chapters, &chapterCopy)
	}

	return downloadManga, nil
}
