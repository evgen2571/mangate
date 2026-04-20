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
	ConfigPath string
	Client     *http.Client
	Registry   *providers.Registry
	Downloader *downloader.Downloader
	Cache      *cache.Cache
}

func New(cfg config.Config) (*App, error) {
	a := &App{
		Registry: providers.NewDefaultRegistry(),
	}
	if err := a.ApplyConfig(cfg); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *App) ApplyConfig(cfg config.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if a.Client != nil && a.Downloader != nil && a.Cache != nil && a.Cfg == cfg {
		return nil
	}

	var applyErr error
	if a.Downloader != nil {
		applyErr = errors.Join(applyErr, a.Downloader.Close())
	}
	if a.Registry == nil {
		a.Registry = providers.NewDefaultRegistry()
	}

	client := &http.Client{Timeout: cfg.HTTP.Timeout}
	a.Cfg = cfg
	a.Client = client
	a.Downloader = downloader.New(cfg, client)
	a.Cache = cache.New(cfg, client)

	return applyErr
}

func (a *App) Close() error {
	var closeErr error

	if a.Downloader != nil {
		closeErr = errors.Join(closeErr, a.Downloader.Close())
	}

	return closeErr
}
