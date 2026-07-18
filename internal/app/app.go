package app

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/evgen2571/mangate/internal/cache"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/dataset"
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
	appliedCfg config.Config
	hasApplied bool
}

// DatasetService creates a run-scoped downloader. It keeps a dataset's output
// and concurrency choices out of the user's saved application configuration
// while still sharing this application's HTTP client and provider registry.
func (a *App) DatasetService(collection dataset.Config) (dataset.Service, error) {
	if a == nil || a.Client == nil {
		return dataset.Service{}, fmt.Errorf("dataset service: app is not configured")
	}
	cfg := a.Cfg.Clone()
	cfg.Provider = collection.Provider
	cfg.Download.Dir = collection.Output.Directory
	cfg.Download.Format = string(collection.Output.Format)
	cfg.Download.ExistingFileMode = string(collection.Output.ExistingFiles)
	cfg.Concurrency.PageDownloads = collection.Runtime.PageWorkers
	cfg.Concurrency.ChapterDownloads = collection.Runtime.ChapterWorkers
	if err := cfg.Validate(); err != nil {
		return dataset.Service{}, fmt.Errorf("dataset service configuration: %w", err)
	}
	provider, err := a.Registry.New(cfg.Provider, cfg, a.Client)
	if err != nil {
		return dataset.Service{}, err
	}
	store, err := dataset.Open(collection.Output.Directory)
	if err != nil {
		return dataset.Service{}, err
	}
	return dataset.Service{Store: store, Provider: provider, Downloader: downloader.New(cfg, a.Client)}, nil
}

type Option func(*App) error

func WithRegistry(registry *providers.Registry) Option {
	return func(a *App) error {
		if registry == nil {
			return fmt.Errorf("registry cannot be nil")
		}
		a.Registry = registry
		return nil
	}
}

func New(cfg config.Config, opts ...Option) (*App, error) {
	a := &App{
		Registry: providers.NewDefaultRegistry(),
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(a); err != nil {
			return nil, err
		}
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
	if a.hasApplied && a.Client != nil && a.Downloader != nil && a.Cache != nil && a.appliedCfg == cfg {
		return nil
	}

	if a.Registry == nil {
		a.Registry = providers.NewDefaultRegistry()
	}

	client := &http.Client{Timeout: cfg.HTTP.Timeout}
	a.Cfg = cfg
	a.Client = client
	a.Downloader = downloader.New(cfg, client)
	a.Cache = cache.New(cfg, client)
	a.appliedCfg = cfg
	a.hasApplied = true

	return nil
}

func (a *App) ApplyAndSaveConfig(cfg config.Config) error {
	if a == nil {
		return fmt.Errorf("apply failed: app is nil")
	}
	if err := a.ApplyConfig(cfg); err != nil {
		return fmt.Errorf("apply failed: %w", err)
	}
	if strings.TrimSpace(a.ConfigPath) == "" {
		return fmt.Errorf("save failed: config path cannot be empty")
	}
	if err := config.Save(a.ConfigPath, a.Cfg); err != nil {
		return fmt.Errorf("save failed: %w", err)
	}
	return nil
}

func (a *App) SearchHistory() ([]string, error) {
	if a == nil || a.Cache == nil {
		return nil, nil
	}
	return a.Cache.SearchHistory()
}

// Provider resolves the configured provider for non-interactive callers.
// Callers can inspect its declared capabilities before making a request.
func (a *App) Provider() (providers.Provider, error) {
	if a == nil || a.Registry == nil || a.Client == nil {
		return nil, fmt.Errorf("provider: app is not configured")
	}
	return a.Registry.New(a.Cfg.Provider, a.Cfg, a.Client)
}

func (a *App) AddSearchQuery(query string) error {
	if a == nil || a.Cache == nil {
		return nil
	}
	return a.Cache.AddSearchQuery(query)
}
