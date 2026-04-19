package app

import (
	"errors"
	"net/http"

	"github.com/evgen2571/mangate/internal/cache"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/providers"
)

type App struct {
	Cfg        config.Config
	Client     *http.Client
	Registry   *providers.Registry
	Downloader *downloader.Downloader
	Cache      *cache.Cache
}

func New(cfg config.Config) (*App, error) {
	client := &http.Client{Timeout: cfg.HTTP.Timeout}
	registry := providers.NewDefaultRegistry()

	return &App{
		Cfg:        cfg,
		Client:     client,
		Registry:   registry,
		Downloader: downloader.New(cfg, client),
		Cache:      cache.New(cfg, client),
	}, nil
}

func (a *App) Close() error {
	var closeErr error

	if a.Downloader != nil {
		closeErr = errors.Join(closeErr, a.Downloader.Close())
	}

	return closeErr
}
