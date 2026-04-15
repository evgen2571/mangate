package providers

import (
	"context"
	"net/http"

	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/source"
)

type Provider interface {
	Name() string

	Search(context.Context, string) ([]*source.Manga, error)
	Chapters(context.Context, *source.Manga) ([]*source.Chapter, error)
	Pages(context.Context, *source.Chapter) ([]*source.Page, error)
	Cover(context.Context, *source.Manga) (string, error)
}

type Factory func(cfg config.Config, client *http.Client) (Provider, error)
