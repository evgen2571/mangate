package app

import (
	"github.com/evgen2571/mangate/internal/acquisition"
	"github.com/evgen2571/mangate/internal/providers"
)

func (a *App) Acquisition(provider providers.Provider) acquisition.Service {
	return acquisition.Service{Config: a.Cfg.Clone(), Provider: provider, Downloader: a.Downloader}
}
