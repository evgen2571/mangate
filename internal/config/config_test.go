package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfigUsesParallelChapterDownloads(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Concurrency.ChapterDownloads != 6 {
		t.Fatalf("ChapterDownloads = %d, want 6", cfg.Concurrency.ChapterDownloads)
	}
}

func TestLoadReturnsDefaultsWhenConfigFileMissing(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv(envConfigPath, configPath)

	cfg, gotPath, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if gotPath != configPath {
		t.Fatalf("Load() path = %q, want %q", gotPath, configPath)
	}

	want := DefaultConfig()
	if cfg != want {
		t.Fatalf("Load() cfg = %#v, want %#v", cfg, want)
	}
}

func TestLoadMergesOverridesFromConfigFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv(envConfigPath, configPath)

	const content = `{
  "language": "ru",
  "http": {
    "timeout": "45s"
  },
  "download": {
    "dir": "/tmp/custom-downloads",
    "type": "cbz"
  },
  "concurrency": {
    "pageDownloads": 3
  },
  "dirs": {
    "temp": "/tmp/custom-temp"
  }
}`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, _, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Language != "ru" {
		t.Fatalf("Language = %q, want %q", cfg.Language, "ru")
	}
	if cfg.HTTP.Timeout != 45*time.Second {
		t.Fatalf("HTTP.Timeout = %v, want %v", cfg.HTTP.Timeout, 45*time.Second)
	}
	if cfg.Download.Dir != "/tmp/custom-downloads" {
		t.Fatalf("Download.Dir = %q, want %q", cfg.Download.Dir, "/tmp/custom-downloads")
	}
	if cfg.Download.Type != "cbz" {
		t.Fatalf("Download.Type = %q, want %q", cfg.Download.Type, "cbz")
	}
	if cfg.Concurrency.PageDownloads != 3 {
		t.Fatalf("Concurrency.PageDownloads = %d, want %d", cfg.Concurrency.PageDownloads, 3)
	}
	if cfg.Dirs.Temp != "/tmp/custom-temp" {
		t.Fatalf("Dirs.Temp = %q, want %q", cfg.Dirs.Temp, "/tmp/custom-temp")
	}

	defaults := DefaultConfig()
	if cfg.Provider != defaults.Provider {
		t.Fatalf("Provider = %q, want default %q", cfg.Provider, defaults.Provider)
	}
	if cfg.Dirs.Cache != defaults.Dirs.Cache {
		t.Fatalf("Dirs.Cache = %q, want default %q", cfg.Dirs.Cache, defaults.Dirs.Cache)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")

	cfg := DefaultConfig()
	cfg.Language = "ja"
	cfg.HTTP.Timeout = 90 * time.Second
	cfg.Download.Type = "zip"
	cfg.Concurrency.ChapterDownloads = 4

	if err := Save(configPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(data), "imageType") {
		t.Fatalf("saved config unexpectedly contains imageType: %s", string(data))
	}

	t.Setenv(envConfigPath, configPath)
	got, gotPath, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if gotPath != configPath {
		t.Fatalf("Load() path = %q, want %q", gotPath, configPath)
	}
	if got != cfg {
		t.Fatalf("Load() cfg = %#v, want %#v", got, cfg)
	}
}

func TestValidateRejectsInvalidConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Concurrency.PageDownloads = 0

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want non-nil")
	}
}

func TestValidateRejectsInvalidMangaDexURL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Providers.MangaDex.BaseURL = "not a url"

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want non-nil")
	}
}
