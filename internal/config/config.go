package config

import (
	"os"
	"path/filepath"
	"time"

	"github.com/evgen2571/manga-downloader/internal/constant"
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
		Dir  string
		Type string
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
	return Config{
		Provider: "mangadex",
		Language: "en",

		Providers: struct {
			MangaDex struct {
				SiteURL    string
				BaseURL    string
				UploadsURL string
			}
		}{
			MangaDex: struct {
				SiteURL    string
				BaseURL    string
				UploadsURL string
			}{
				SiteURL:    "https://mangadex.org/",
				BaseURL:    "https://api.mangadex.org/",
				UploadsURL: "https://uploads.mangadex.org/",
			},
		},

		HTTP: struct{ Timeout time.Duration }{
			Timeout: 5 * time.Second,
		},

		Download: struct {
			Dir  string
			Type string
		}{
			Dir:  "downloads",
			Type: "plain",
		},

		Concurrency: struct {
			PageFetches      int
			PageDownloads    int
			ChapterDownloads int
		}{
			PageFetches:      4,
			PageDownloads:    8,
			ChapterDownloads: 1,
		},

		Dirs: struct {
			Cache string
			Temp  string
		}{
			Cache: defaultCacheDir(),
			Temp:  defaultTempDir(),
		},
	}
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
