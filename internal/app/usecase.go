package app

import (
	"context"
	"fmt"

	"github.com/evgen2571/mangate/internal/cache"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/evgen2571/mangate/internal/usecase"
)

func (a *App) UseCases() usecase.Service {
	return usecase.New(usecase.Deps{
		ProviderResolver: providerResolver{app: a},
		Downloader:       mangaDownloader{downloader: a.Downloader},
		Cache:            coverCache{cache: a.Cache},
		Timeout:          a.Cfg.HTTP.Timeout,
	})
}

type providerResolver struct {
	app *App
}

func (r providerResolver) Provider() (usecase.Provider, error) {
	if r.app == nil {
		return nil, fmt.Errorf("provider: app is not configured")
	}
	if r.app.Registry == nil {
		return nil, fmt.Errorf("provider: registry is not configured")
	}
	if r.app.Client == nil {
		return nil, fmt.Errorf("provider: http client is not configured")
	}

	return r.app.Registry.New(r.app.Cfg.Provider, r.app.Cfg, r.app.Client)
}

type coverCache struct {
	cache *cache.Cache
}

func (c coverCache) Get(ctx context.Context, provider usecase.Provider, manga *source.Manga) (string, error) {
	if c.cache == nil {
		return "", fmt.Errorf("load cover: cache is not configured")
	}

	return c.cache.Get(ctx, provider, manga)
}

type mangaDownloader struct {
	downloader *downloader.Downloader
}

func (d mangaDownloader) DownloadManga(ctx context.Context, manga *source.Manga, pageLoader usecase.PageLoader, notify func(usecase.DownloadProgress)) error {
	if d.downloader == nil {
		return fmt.Errorf("download chapters: downloader is not configured")
	}

	return d.downloader.DownloadMangaWithProgressAndPageLoader(
		ctx,
		manga,
		downloader.PageLoader(pageLoader),
		func(progress downloader.DownloadProgress) {
			if notify == nil {
				return
			}
			notify(toUsecaseDownloadProgress(progress))
		},
	)
}

func toUsecaseDownloadProgress(progress downloader.DownloadProgress) usecase.DownloadProgress {
	chapters := make([]usecase.ChapterDownloadProgress, 0, len(progress.Chapters))
	for _, chapter := range progress.Chapters {
		chapters = append(chapters, usecase.ChapterDownloadProgress{
			Name:           chapter.Name,
			CompletedPages: chapter.CompletedPages,
			TotalPages:     chapter.TotalPages,
			Active:         chapter.Active,
			Completed:      chapter.Completed,
		})
	}

	return usecase.DownloadProgress{
		CompletedPages:    progress.CompletedPages,
		TotalPages:        progress.TotalPages,
		CompletedChapters: progress.CompletedChapters,
		TotalChapters:     progress.TotalChapters,
		Chapters:          chapters,
	}
}
