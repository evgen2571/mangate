package app

import (
	"fmt"
	"net/http"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/providers"
	"github.com/evgen2571/mangate/internal/tui"
)

type App struct {
	Cfg        config.Config
	Client     *http.Client
	Registry   *providers.Registry
	Downloader *downloader.Downloader
}

func New(cfg config.Config) (*App, error) {
	client := &http.Client{Timeout: cfg.HTTP.Timeout}
	registry := providers.NewDefaultRegistry()

	return &App{
		Cfg:        cfg,
		Client:     client,
		Registry:   registry,
		Downloader: downloader.New(cfg, client),
	}, nil
}

func (a *App) Run() error {
	p := tea.NewProgram(tui.New())
	_, err := p.Run()
	return err
}

func (a *App) InitDirs() error {
	dirs := []string{
		a.Cfg.Download.Dir,
		a.Cfg.Dirs.Cache,
		a.Cfg.Dirs.Temp,
	}

	for _, dir := range dirs {
		if dir == "" {
			return fmt.Errorf("directory path cannot be empty")
		}

		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %q: %w", dir, err)
		}
	}

	return nil
}
