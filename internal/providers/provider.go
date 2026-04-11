package providers

import (
	"fmt"

	"github.com/evgen2571/manga-downloader/internal/config"
	"github.com/evgen2571/manga-downloader/internal/providers/mangadex"
	"github.com/evgen2571/manga-downloader/internal/sources"
)

type provider interface {
	GetManga(string) ([]*sources.Manga, error)
	GetChapters(*sources.Manga) ([]*sources.Chapter, error)
	GetPages(*sources.Chapter) ([]*sources.Page, error)
}

var providers = map[string]provider{
	"mangadex": &mangadex.MangaDex{
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
