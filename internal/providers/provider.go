package providers

import (
	"github.com/evgen2571/manga-downloader/internal/providers/mangadex"
	"github.com/evgen2571/manga-downloader/internal/sources"
)

type provider interface {
	GetManga(string) ([]*sources.Manga, error)
	GetChapters(*sources.Manga) ([]*sources.Chapter, error)
	GetPages(*sources.Chapter) ([]*sources.Page, error)
}

var Providers = map[string]provider{
	"mangadex": &mangadex.MangaDex{
		BaseURL:        "https://api.mangadex.org/",
		UploadsBaseURL: "https://uploads.mangadex.org/",
	},
}
