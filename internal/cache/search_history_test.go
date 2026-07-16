package cache

import (
	"net/http"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/evgen2571/mangate/internal/config"
)

func TestSearchHistoryAddDeduplicatesAndLimits(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Dirs.Cache = t.TempDir()
	cfg.Search.HistoryMax = 3

	c := New(cfg, http.DefaultClient)
	for _, query := range []string{"one", "two", "three", "two", "four", "  "} {
		if err := c.AddSearchQuery(query); err != nil {
			t.Fatalf("AddSearchQuery(%q) error = %v", query, err)
		}
	}

	got, err := c.SearchHistory()
	if err != nil {
		t.Fatalf("SearchHistory() error = %v", err)
	}
	want := []string{"four", "two", "three"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SearchHistory() = %#v, want %#v", got, want)
	}
}

func TestSearchHistoryPersistsInCacheDirectory(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Dirs.Cache = t.TempDir()
	cfg.Search.HistoryMax = 10

	c := New(cfg, http.DefaultClient)
	if err := c.AddSearchQuery("puniru"); err != nil {
		t.Fatalf("AddSearchQuery() error = %v", err)
	}

	newCache := New(cfg, http.DefaultClient)
	got, err := newCache.SearchHistory()
	if err != nil {
		t.Fatalf("SearchHistory() error = %v", err)
	}
	want := []string{"puniru"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SearchHistory() = %#v, want %#v", got, want)
	}

	if path := c.searchHistoryPath(); path != filepath.Join(cfg.Dirs.Cache, "search-history.json") {
		t.Fatalf("searchHistoryPath() = %q, want cache-local search-history.json", path)
	}
}

func TestSearchHistoryDisabledWhenMaxIsZero(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Dirs.Cache = t.TempDir()
	cfg.Search.HistoryMax = 0

	c := New(cfg, http.DefaultClient)
	if err := c.AddSearchQuery("ignored"); err != nil {
		t.Fatalf("AddSearchQuery() error = %v", err)
	}

	got, err := c.SearchHistory()
	if err != nil {
		t.Fatalf("SearchHistory() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("SearchHistory() = %#v, want empty", got)
	}
}
