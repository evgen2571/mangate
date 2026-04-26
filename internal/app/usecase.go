package app

import "github.com/evgen2571/mangate/internal/usecase"

func (a *App) UseCases() usecase.Service {
	return usecase.New(usecase.Deps{
		Cfg:        a.Cfg,
		Client:     a.Client,
		Registry:   a.Registry,
		Downloader: a.Downloader,
		Cache:      a.Cache,
	})
}
