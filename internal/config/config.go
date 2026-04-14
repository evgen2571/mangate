package config

import (
	"os"
	"path/filepath"
	"time"

	"github.com/evgen2571/mangate/internal/constant"
)

type Config struct {
	Provider string
	Language string

	Providers struct {
		MangaDex struct {
			SiteURL    string
			BaseURL    string
			UploadsURL string
		}
	}

	HTTP struct {
		Timeout time.Duration
	}

	Download struct {
		Dir       string
		Type      string
		ImageType string
	}

	Concurrency struct {
		PageFetches      int
		PageDownloads    int
		ChapterDownloads int
	}

	Dirs struct {
		Cache string
		Temp  string
	}
}

func DefaultConfig() Config {
	var cfg Config

	cfg.Provider = "mangadex"
	cfg.Language = "en"

	cfg.Providers.MangaDex.SiteURL = "https://mangadex.org"
	cfg.Providers.MangaDex.BaseURL = "https://api.mangadex.org"
	cfg.Providers.MangaDex.UploadsURL = "https://uploads.mangadex.org"

	cfg.HTTP.Timeout = 30 * time.Second

	cfg.Download.Dir = "./downloads"
	cfg.Download.Type = "plain"
	cfg.Download.ImageType = "jpg"

	cfg.Concurrency.PageFetches = 8
	cfg.Concurrency.PageDownloads = 8
	cfg.Concurrency.ChapterDownloads = 2

	cfg.Dirs.Cache = defaultCacheDir()
	cfg.Dirs.Temp = defaultTempDir()

	return cfg
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
