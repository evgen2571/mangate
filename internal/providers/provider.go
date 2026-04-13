package providers

import (
	"net/http"

	"github.com/evgen2571/manga-downloader/internal/config"
	"github.com/evgen2571/manga-downloader/internal/source"
)

type Provider interface {
	Name() string

	Search(string) ([]*source.Manga, error)
	Chapters(*source.Manga) ([]*source.Chapter, error)
	Pages(*source.Chapter) ([]*source.Page, error)
	Cover(*source.Manga) (string, error)
}

type Factory func(cfg config.Config, client *http.Client) (Provider, error)
