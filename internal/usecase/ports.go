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
	Cover(context.Context, *source.Manga) (string, error)
}

type CoverProvider interface {
	Name() string
	Cover(context.Context, *source.Manga) (string, error)
}

type CoverCache interface {
	Get(context.Context, CoverProvider, *source.Manga) (string, error)
}
