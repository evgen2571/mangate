package providers

import (
	"github.com/evgen2571/manga-downloader/internal/providers/mangadex"
	"github.com/evgen2571/manga-downloader/internal/sources"
)

type Provider interface {
	GetProviderObject() Provider
	GetManga(string) ([]*sources.Manga, error)
	GetChapters(string) ([]*sources.Chapter, error)
}

type ProviderSource interface {
	toSource(string) *sources.Source
}

var Providers = map[string]Provider{
	"MangaDex": mangadex.GetProviderObject(),
}
