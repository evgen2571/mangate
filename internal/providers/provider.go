package providers

import (
	"context"
	"net/http"

	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/source"
)

type Provider interface {
	Name() string
	Info() source.ProviderInfo

	Search(context.Context, string) ([]*source.Manga, error)
	Title(context.Context, string) (*source.Manga, error)
	Chapters(context.Context, *source.Manga) ([]*source.Chapter, error)
	Pages(context.Context, *source.Chapter) ([]*source.Page, error)
	Cover(context.Context, *source.Manga) (string, error)
}

type Factory func(cfg config.Config, client *http.Client) (Provider, error)

// BrowseProvider is an optional provider capability for catalog-scale title
// discovery. It deliberately sits beside Provider so integrations that only
// support search remain source compatible.
type BrowseProvider interface {
	BrowseManga(context.Context, source.BrowseRequest) (source.BrowsePage, error)
}
