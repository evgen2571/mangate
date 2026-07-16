package app

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/providers"
)

func TestNewUsesDefaultRegistry(t *testing.T) {
	cfg := config.DefaultConfig()

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if a.Registry == nil {
		t.Fatal("Registry is nil, want default registry")
	}
}

func TestNewWithRegistryUsesCustomRegistry(t *testing.T) {
	cfg := config.DefaultConfig()
	registry := providers.NewRegistry()

	a, err := New(cfg, WithRegistry(registry))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if a.Registry != registry {
		t.Fatal("Registry was not set to custom registry")
	}
}

func TestNewWithRegistryRejectsNilRegistry(t *testing.T) {
	cfg := config.DefaultConfig()

	a, err := New(cfg, WithRegistry(nil))
	if err == nil || err.Error() != "registry cannot be nil" {
		t.Fatalf("New() error = %v, want nil registry error", err)
	}
	if a != nil {
		t.Fatalf("New() app = %#v, want nil app", a)
	}
}

func TestApplyConfigRebuildsRuntimeDependencies(t *testing.T) {
	cfg := config.DefaultConfig()

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	oldClient := a.Client
	oldDownloader := a.Downloader
	oldCache := a.Cache

	updated := cfg
	updated.HTTP.Timeout = 42 * time.Second
	updated.Download.Dir = t.TempDir()

	if err := a.ApplyConfig(updated); err != nil {
		t.Fatalf("ApplyConfig() error = %v", err)
	}

	if a.Cfg != updated {
		t.Fatalf("Cfg = %#v, want %#v", a.Cfg, updated)
	}
	if a.Client == oldClient {
		t.Fatal("Client pointer did not change")
	}
	if a.Client.Timeout != updated.HTTP.Timeout {
		t.Fatalf("Client.Timeout = %v, want %v", a.Client.Timeout, updated.HTTP.Timeout)
	}
	if a.Downloader == oldDownloader {
		t.Fatal("Downloader pointer did not change")
	}
	if a.Cache == oldCache {
		t.Fatal("Cache pointer did not change")
	}
}

func TestApplyConfigSkipsRebuildWhenConfigUnchanged(t *testing.T) {
	cfg := config.DefaultConfig()

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	oldClient := a.Client
	oldDownloader := a.Downloader
	oldCache := a.Cache

	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatalf("ApplyConfig() error = %v", err)
	}

	if a.Client != oldClient {
		t.Fatal("Client pointer changed for unchanged config")
	}
	if a.Downloader != oldDownloader {
		t.Fatal("Downloader pointer changed for unchanged config")
	}
	if a.Cache != oldCache {
		t.Fatal("Cache pointer changed for unchanged config")
	}
}

func TestSearchHistoryDelegatesToCache(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Dirs.Cache = t.TempDir()
	cfg.Search.HistoryMax = 2

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	for _, query := range []string{"puniru", "one piece", "puniru"} {
		if err := a.AddSearchQuery(query); err != nil {
			t.Fatalf("AddSearchQuery(%q) error = %v", query, err)
		}
	}

	got, err := a.SearchHistory()
	if err != nil {
		t.Fatalf("SearchHistory() error = %v", err)
	}
	want := []string{"puniru", "one piece"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SearchHistory() = %#v, want %#v", got, want)
	}
}

func TestSearchHistoryFacadeHandlesMissingAppOrCache(t *testing.T) {
	var nilApp *App
	if got, err := nilApp.SearchHistory(); err != nil || got != nil {
		t.Fatalf("nil app SearchHistory() = %#v, %v; want nil, nil", got, err)
	}
	if err := nilApp.AddSearchQuery("ignored"); err != nil {
		t.Fatalf("nil app AddSearchQuery() error = %v", err)
	}

	a := &App{}
	if got, err := a.SearchHistory(); err != nil || got != nil {
		t.Fatalf("nil cache SearchHistory() = %#v, %v; want nil, nil", got, err)
	}
	if err := a.AddSearchQuery("ignored"); err != nil {
		t.Fatalf("nil cache AddSearchQuery() error = %v", err)
	}
}

func TestApplyAndSaveConfigPersistsAppliedConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Dirs.Cache = t.TempDir()
	cfg.Download.Dir = t.TempDir()

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	a.ConfigPath = filepath.Join(t.TempDir(), "config.json")

	updated := cfg
	updated.Language = "ru"
	updated.HTTP.Timeout = 42 * time.Second

	if err := a.ApplyAndSaveConfig(updated); err != nil {
		t.Fatalf("ApplyAndSaveConfig() error = %v", err)
	}
	if a.Cfg != updated {
		t.Fatalf("app config = %#v, want %#v", a.Cfg, updated)
	}

	loaded, err := config.LoadFromPath(a.ConfigPath)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}
	if loaded != updated {
		t.Fatalf("saved config = %#v, want %#v", loaded, updated)
	}
}

func TestApplyAndSaveConfigReturnsApplyFailure(t *testing.T) {
	cfg := config.DefaultConfig()
	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	a.ConfigPath = filepath.Join(t.TempDir(), "config.json")

	invalid := cfg
	invalid.Provider = ""

	err = a.ApplyAndSaveConfig(invalid)
	if err == nil || err.Error() != "apply failed: provider cannot be empty" {
		t.Fatalf("ApplyAndSaveConfig() error = %v, want apply failure", err)
	}
	if a.Cfg != cfg {
		t.Fatalf("app config changed after apply failure")
	}
}

func TestApplyAndSaveConfigReturnsSaveFailureForEmptyPathAfterApplying(t *testing.T) {
	cfg := config.DefaultConfig()
	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	a.ConfigPath = "  "

	updated := cfg
	updated.Language = "ru"

	err = a.ApplyAndSaveConfig(updated)
	if err == nil || err.Error() != "save failed: config path cannot be empty" {
		t.Fatalf("ApplyAndSaveConfig() error = %v, want empty-path save failure", err)
	}
	if a.Cfg != updated {
		t.Fatalf("app config was not applied before save failure")
	}
}
