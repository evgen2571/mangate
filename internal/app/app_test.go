package app

import (
	"testing"
	"time"

	"github.com/evgen2571/mangate/internal/config"
)

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
