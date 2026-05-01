package usecase

import (
	"context"

	"github.com/evgen2571/mangate/internal/source"
)

type ProviderResolver interface {
	Provider() (Provider, error)
}

type Provider interface {
	Name() string
	Search(context.Context, string) ([]*source.Manga, error)
	Chapters(context.Context, *source.Manga) ([]*source.Chapter, error)
	Pages(context.Context, *source.Chapter) ([]*source.Page, error)
	Cover(context.Context, *source.Manga) (string, error)
}

type CoverCache interface {
	Get(context.Context, Provider, *source.Manga) (string, error)
}

type PageLoader func(context.Context, *source.Chapter) ([]*source.Page, error)

type MangaDownloader interface {
	DownloadManga(context.Context, *source.Manga, PageLoader, func(DownloadProgress)) error
}

type ChapterDownloadProgress struct {
	Name           string
	CompletedPages int
	TotalPages     int
	Active         bool
	Completed      bool
}

type DownloadProgress struct {
	CompletedPages    int
	TotalPages        int
	CompletedChapters int
	TotalChapters     int
	Chapters          []ChapterDownloadProgress
}
