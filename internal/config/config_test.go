package config

import (
	"encoding/json"
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

func TestDefaultConfigKeepsSearchHistory(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Search.HistoryMax != 100 {
		t.Fatalf("Search.HistoryMax = %d, want 100", cfg.Search.HistoryMax)
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
    "dir": "/tmp/custom-downloads"
  },
  "concurrency": {
    "pageDownloads": 3
  },
  "search": {
    "historyMax": 25
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
	if cfg.Concurrency.PageDownloads != 3 {
		t.Fatalf("Concurrency.PageDownloads = %d, want %d", cfg.Concurrency.PageDownloads, 3)
	}
	if cfg.Search.HistoryMax != 25 {
		t.Fatalf("Search.HistoryMax = %d, want %d", cfg.Search.HistoryMax, 25)
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

func TestLoadRejectsLegacyDownloadType(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte(`{"download":{"type":"cbz"}}`), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	_, err := LoadFromPath(configPath)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("LoadFromPath() error = %v, want unknown-field rejection", err)
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

func TestLoadRejectsUnknownAndNullConfiguration(t *testing.T) {
	for _, body := range []string{`null`, `{"download":{"formatt":"cbz"}}`, `{"http":{"timout":"1s"}}`} {
		t.Run(body, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(body), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadFromPath(path); err == nil {
				t.Fatal("accepted malformed configuration")
			}
		})
	}
}

func TestSerializedConfigurationCanBeLoaded(t *testing.T) {
	cfg := DefaultConfig()
	cfg.HTTP.Timeout = 45 * time.Second
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadFromPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != cfg {
		t.Fatalf("configuration changed on round trip: %#v", got)
	}
}

func TestLoadNormalizesConfigurationEnums(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"provider":" MangaDex ","language":" EN ","download":{"format":" CBZ ","existingFileMode":" SKIP "}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFromPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != "mangadex" || cfg.Language != "en" || cfg.Download.Format != "cbz" || cfg.Download.ExistingFileMode != "skip" {
		t.Fatalf("configuration was not normalized: %+v", cfg)
	}
}

func TestSaveReplacementAndFailureCleanup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := DefaultConfig()
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Download.Format = " CBZ "
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := LoadFromPath(path)
	if err != nil || got.Download.Format != "cbz" {
		t.Fatalf("replacement = %q, %v", got.Download.Format, err)
	}
	blocked := filepath.Join(dir, "directory")
	if err := os.Mkdir(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Save(blocked, cfg); err == nil {
		t.Fatal("saving over a directory succeeded")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("temporary file leaked: %v, %v", entries, err)
	}
}
