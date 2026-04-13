package cmd

import (
	"log"
	"net/http"

	"github.com/evgen2571/manga-downloader/internal/config"
	"github.com/evgen2571/manga-downloader/internal/downloader"
	"github.com/evgen2571/manga-downloader/internal/providers"
)

type App struct {
	Cfg        config.Config
	Client     *http.Client
	Downloader *downloader.Downloader
	Provider   providers.Provider
}

func main() {
	cfg := config.DefaultConfig()
	client := &http.Client{Timeout: cfg.HTTP.Timeout}
	registry := providers.NewDefaultRegistry()

	provider, err := registry.New(cfg.Provider, cfg, client)
	if err != nil {
		log.Fatal(err)
	}

	app := App{
		Cfg:        cfg,
		Client:     client,
		Downloader: downloader.New(cfg),
		Provider:   provider,
	}

	app.Run()
}
