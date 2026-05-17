package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	DownloadTypePlain = "plain"
	DownloadTypeCBZ   = "cbz"
	DownloadTypeZIP   = "zip"
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
			Type: DownloadTypePlain,
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
