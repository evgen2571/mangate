package providers

import (
	"fmt"

	"github.com/evgen2571/manga-downloader/internal/config"
	"github.com/evgen2571/manga-downloader/internal/providers/mangadex"
	"github.com/evgen2571/manga-downloader/internal/source"
)

type provider interface {
	GetManga(string) ([]*source.Manga, error)
	GetChapters(*source.Manga) ([]*source.Chapter, error)
	GetPages(*source.Chapter) ([]*source.Page, error)
}

var providers = map[string]provider{
	"mangadex": &mangadex.MangaDex{
		SiteURL:        "https://mangadex.org/",
		BaseURL:        "https://api.mangadex.org/",
		UploadsBaseURL: "https://uploads.mangadex.org/",
	},
}

var Provider = providers[config.Provider]

func UpdateProvider(newProvider string) error {
	provider, exists := providers[newProvider]
	if !exists {
		return fmt.Errorf("provider '%s' not found", newProvider)
	}

	Provider = provider
	return nil
}
