package app

import (
	"context"
	"fmt"

	"github.com/evgen2571/mangate/internal/cache"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/evgen2571/mangate/internal/usecase"
)

func (a *App) UseCases() usecase.Service {
	return usecase.New(usecase.Deps{
		ProviderResolver: providerResolver{app: a},
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

	return r.app.Provider()
}

type coverCache struct {
	cache *cache.Cache
}

func (c coverCache) Get(ctx context.Context, provider usecase.CoverProvider, manga *source.Manga) (string, error) {
	if c.cache == nil {
		return "", fmt.Errorf("load cover: cache is not configured")
	}

	return c.cache.Get(ctx, provider, manga)
}
