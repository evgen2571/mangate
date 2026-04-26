package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/evgen2571/mangate/internal/constant"
)

const (
	envConfigPath         = "MANGATE_CONFIG"
	defaultConfigFileName = "config.json"
)

type Config struct {
	Provider string
	Language string

	Providers   ProvidersConfig
	HTTP        HTTPConfig
	Download    DownloadConfig
	Concurrency ConcurrencyConfig
	Search      SearchConfig
	Dirs        DirsConfig
}

type ProvidersConfig struct {
	MangaDex MangaDexConfig
}

type MangaDexConfig struct {
	SiteURL    string
	BaseURL    string
	UploadsURL string
}

type HTTPConfig struct {
	Timeout time.Duration
}

type DownloadConfig struct {
	Dir  string
	Type string
}

type ConcurrencyConfig struct {
	PageDownloads    int
	ChapterDownloads int
}

type SearchConfig struct {
	HistoryMax int
}

type DirsConfig struct {
	Cache string
	Temp  string
}

type fileConfig struct {
	Provider    *string                `json:"provider,omitempty"`
	Language    *string                `json:"language,omitempty"`
	Providers   *fileProvidersConfig   `json:"providers,omitempty"`
	HTTP        *fileHTTPConfig        `json:"http,omitempty"`
	Download    *fileDownloadConfig    `json:"download,omitempty"`
	Concurrency *fileConcurrencyConfig `json:"concurrency,omitempty"`
	Search      *fileSearchConfig      `json:"search,omitempty"`
	Dirs        *fileDirsConfig        `json:"dirs,omitempty"`
}

type fileProvidersConfig struct {
	MangaDex *fileMangaDexConfig `json:"mangadex,omitempty"`
}

type fileMangaDexConfig struct {
	SiteURL    *string `json:"siteUrl,omitempty"`
	BaseURL    *string `json:"baseUrl,omitempty"`
	UploadsURL *string `json:"uploadsUrl,omitempty"`
}

type fileHTTPConfig struct {
	Timeout *string `json:"timeout,omitempty"`
}

type fileDownloadConfig struct {
	Dir  *string `json:"dir,omitempty"`
	Type *string `json:"type,omitempty"`
}

type fileConcurrencyConfig struct {
	PageDownloads    *int `json:"pageDownloads,omitempty"`
	ChapterDownloads *int `json:"chapterDownloads,omitempty"`
}

type fileSearchConfig struct {
	HistoryMax *int `json:"historyMax,omitempty"`
}

type fileDirsConfig struct {
	Cache *string `json:"cache,omitempty"`
	Temp  *string `json:"temp,omitempty"`
}

func DefaultConfig() Config {
	return Config{
		Provider: "mangadex",
		Language: "en",
		Providers: ProvidersConfig{
			MangaDex: MangaDexConfig{
				SiteURL:    "https://mangadex.org",
				BaseURL:    "https://api.mangadex.org",
				UploadsURL: "https://uploads.mangadex.org",
			},
		},
		HTTP: HTTPConfig{
			Timeout: 30 * time.Second,
		},
		Download: DownloadConfig{
			Dir:  defaultDownloadDir(),
			Type: "plain",
		},
		Concurrency: ConcurrencyConfig{
			PageDownloads:    8,
			ChapterDownloads: 6,
		},
		Search: SearchConfig{
			HistoryMax: 100,
		},
		Dirs: DirsConfig{
			Cache: defaultCacheDir(),
			Temp:  defaultTempDir(),
		},
	}
}

func (c Config) Clone() Config {
	return c
}

func (c Config) Validate() error {
	switch {
	case strings.TrimSpace(c.Provider) == "":
		return fmt.Errorf("provider cannot be empty")
	case strings.TrimSpace(c.Language) == "":
		return fmt.Errorf("language cannot be empty")
	case c.HTTP.Timeout <= 0:
		return fmt.Errorf("http timeout must be > 0")
	case strings.TrimSpace(c.Download.Dir) == "":
		return fmt.Errorf("download dir cannot be empty")
	case strings.TrimSpace(c.Download.Type) == "":
		return fmt.Errorf("download type cannot be empty")
	case c.Concurrency.PageDownloads <= 0:
		return fmt.Errorf("page-downloads must be > 0")
	case c.Concurrency.ChapterDownloads <= 0:
		return fmt.Errorf("chapter-downloads must be > 0")
	case c.Search.HistoryMax < 0:
		return fmt.Errorf("search history max must be >= 0")
	case strings.TrimSpace(c.Dirs.Cache) == "":
		return fmt.Errorf("cache dir cannot be empty")
	case strings.TrimSpace(c.Dirs.Temp) == "":
		return fmt.Errorf("temp dir cannot be empty")
	}

	if err := validateHTTPURL("mangadex site url", c.Providers.MangaDex.SiteURL); err != nil {
		return err
	}
	if err := validateHTTPURL("mangadex base url", c.Providers.MangaDex.BaseURL); err != nil {
		return err
	}
	if err := validateHTTPURL("mangadex uploads url", c.Providers.MangaDex.UploadsURL); err != nil {
		return err
	}

	return nil
}

func DefaultConfigPath() string {
	if path := strings.TrimSpace(os.Getenv(envConfigPath)); path != "" {
		return path
	}

	root, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", "."+constant.ProjectName, defaultConfigFileName)
	}

	return filepath.Join(root, constant.ProjectName, defaultConfigFileName)
}

func Load() (Config, string, error) {
	path := DefaultConfigPath()
	cfg, err := LoadFromPath(path)
	return cfg, path, err
}

func LoadFromPath(path string) (Config, error) {
	cfg := DefaultConfig()
	if strings.TrimSpace(path) == "" {
		return cfg, fmt.Errorf("config path cannot be empty")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config file %q: %w", path, err)
	}

	var raw fileConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return cfg, fmt.Errorf("decode config file %q: %w", path, err)
	}

	if err := raw.applyTo(&cfg); err != nil {
		return cfg, fmt.Errorf("apply config file %q: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("validate config file %q: %w", path, err)
	}

	return cfg, nil
}

func Save(path string, cfg Config) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("config path cannot be empty")
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory %q: %w", filepath.Dir(path), err)
	}

	data, err := json.MarshalIndent(newFileConfig(cfg), "", "  ")
	if err != nil {
		return fmt.Errorf("encode config file %q: %w", path, err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config file %q: %w", path, err)
	}

	return nil
}

func (f fileConfig) applyTo(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("nil config")
	}
	if f.Provider != nil {
		cfg.Provider = *f.Provider
	}
	if f.Language != nil {
		cfg.Language = *f.Language
	}
	if f.Providers != nil && f.Providers.MangaDex != nil {
		md := f.Providers.MangaDex
		if md.SiteURL != nil {
			cfg.Providers.MangaDex.SiteURL = *md.SiteURL
		}
		if md.BaseURL != nil {
			cfg.Providers.MangaDex.BaseURL = *md.BaseURL
		}
		if md.UploadsURL != nil {
			cfg.Providers.MangaDex.UploadsURL = *md.UploadsURL
		}
	}
	if f.HTTP != nil && f.HTTP.Timeout != nil {
		timeout, err := time.ParseDuration(*f.HTTP.Timeout)
		if err != nil {
			return fmt.Errorf("parse http timeout %q: %w", *f.HTTP.Timeout, err)
		}
		cfg.HTTP.Timeout = timeout
	}
	if f.Download != nil {
		if f.Download.Dir != nil {
			cfg.Download.Dir = *f.Download.Dir
		}
		if f.Download.Type != nil {
			cfg.Download.Type = *f.Download.Type
		}
	}
	if f.Concurrency != nil {
		if f.Concurrency.PageDownloads != nil {
			cfg.Concurrency.PageDownloads = *f.Concurrency.PageDownloads
		}
		if f.Concurrency.ChapterDownloads != nil {
			cfg.Concurrency.ChapterDownloads = *f.Concurrency.ChapterDownloads
		}
	}
	if f.Search != nil && f.Search.HistoryMax != nil {
		cfg.Search.HistoryMax = *f.Search.HistoryMax
	}
	if f.Dirs != nil {
		if f.Dirs.Cache != nil {
			cfg.Dirs.Cache = *f.Dirs.Cache
		}
		if f.Dirs.Temp != nil {
			cfg.Dirs.Temp = *f.Dirs.Temp
		}
	}

	return nil
}

func newFileConfig(cfg Config) fileConfig {
	return fileConfig{
		Provider: stringPtr(cfg.Provider),
		Language: stringPtr(cfg.Language),
		Providers: &fileProvidersConfig{
			MangaDex: &fileMangaDexConfig{
				SiteURL:    stringPtr(cfg.Providers.MangaDex.SiteURL),
				BaseURL:    stringPtr(cfg.Providers.MangaDex.BaseURL),
				UploadsURL: stringPtr(cfg.Providers.MangaDex.UploadsURL),
			},
		},
		HTTP: &fileHTTPConfig{
			Timeout: stringPtr(cfg.HTTP.Timeout.String()),
		},
		Download: &fileDownloadConfig{
			Dir:  stringPtr(cfg.Download.Dir),
			Type: stringPtr(cfg.Download.Type),
		},
		Concurrency: &fileConcurrencyConfig{
			PageDownloads:    intPtr(cfg.Concurrency.PageDownloads),
			ChapterDownloads: intPtr(cfg.Concurrency.ChapterDownloads),
		},
		Search: &fileSearchConfig{
			HistoryMax: intPtr(cfg.Search.HistoryMax),
		},
		Dirs: &fileDirsConfig{
			Cache: stringPtr(cfg.Dirs.Cache),
			Temp:  stringPtr(cfg.Dirs.Temp),
		},
	}
}

func stringPtr(v string) *string {
	return &v
}

func intPtr(v int) *int {
	return &v
}

func validateHTTPURL(fieldName, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s is invalid: %w", fieldName, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s must use http or https", fieldName)
	}
	if parsed.Host == "" {
		return fmt.Errorf("%s must include a host", fieldName)
	}

	return nil
}

func defaultDownloadDir() string {
	root, err := os.UserHomeDir()
	if err != nil {
		return "./downloads"
	}

	return filepath.Join(root, "downloads", constant.ProjectName)
}

func defaultCacheDir() string {
	root, err := os.UserCacheDir()
	if err != nil {
		return "./.cache"
	}
	return filepath.Join(root, constant.ProjectName)
}

func defaultTempDir() string {
	return filepath.Join(os.TempDir(), constant.ProjectName)
}
