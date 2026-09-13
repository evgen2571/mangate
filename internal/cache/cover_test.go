package cache

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/source"
)

type coverProvider struct{ url string }

func (p coverProvider) Name() string                                         { return "fixture" }
func (p coverProvider) Cover(context.Context, *source.Manga) (string, error) { return p.url, nil }

func TestCoverRejectsEmptyAndNonImageResponses(t *testing.T) {
	for _, tc := range []struct{ name, body, contentType string }{
		{"empty", "", "image/png"}, {"html", "<html>error</html>", "text/html"}, {"invalid image", "not an image", "image/png"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.Write([]byte(tc.body))
			}))
			defer server.Close()
			cfg := config.DefaultConfig()
			cfg.Dirs.Cache = t.TempDir()
			cache := New(cfg, server.Client())
			_, err := cache.Get(context.Background(), coverProvider{server.URL + "/cover.png"}, &source.Manga{ID: "manga"})
			if err == nil {
				t.Fatal("accepted invalid cover")
			}
			filepath.WalkDir(cfg.Dirs.Cache, func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !entry.IsDir() {
					t.Errorf("invalid cached file remains: %s", path)
				}
				return nil
			})
		})
	}
}

func TestCoverWaiterCanCancelWithoutCancelingOwner(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-release; w.WriteHeader(503) }))
	defer server.Close()
	cfg := config.DefaultConfig()
	cfg.Dirs.Cache = t.TempDir()
	cache := New(cfg, server.Client())
	provider := coverProvider{server.URL + "/cover.png"}
	manga := &source.Manga{ID: "manga"}
	owner := make(chan error, 1)
	go func() { _, err := cache.Get(context.Background(), provider, manga); owner <- err }()
	<-started
	waiter := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	go func() { _, err := cache.Get(ctx, provider, manga); waiter <- err }()
	select {
	case err := <-waiter:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("error = %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("waiter ignored its cancellation")
	}
	close(release)
	if err := <-owner; err == nil {
		t.Error("owner should report HTTP failure")
	}
}

func TestCoverRepairsInvalidCacheAndReusesValidImage(t *testing.T) {
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	downloads := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloads++
		w.Header().Set("Content-Type", "image/png")
		w.Write(data.Bytes())
	}))
	defer server.Close()
	cfg := config.DefaultConfig()
	cfg.Dirs.Cache = t.TempDir()
	cache := New(cfg, server.Client())
	provider := coverProvider{server.URL + "/cover.png?token=abc"}
	manga := &source.Manga{ID: "manga"}
	path := cache.coverPath(provider.Name(), manga.ID, provider.url)
	if strings.ContainsAny(filepath.Base(path), "?&=") {
		t.Fatalf("cache filename contains URL query: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("broken"), 0644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		got, err := cache.Get(context.Background(), provider, manga)
		if err != nil || got != path {
			t.Fatalf("Get = %q, %v", got, err)
		}
	}
	if downloads != 1 {
		t.Fatalf("downloads = %d, want one repair then cache hit", downloads)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, data.Bytes()) {
		t.Fatalf("cache was not repaired: %v", err)
	}
}
