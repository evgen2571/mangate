package mangadex

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/evgen2571/mangate/internal/config"
)

func TestRandomMangaUsesNativeEndpointAndRetry(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/manga/random" {
			t.Errorf("path=%s", r.URL.Path)
		}
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"result":"ok","data":{"id":"random-id","attributes":{"title":{"en":"Random"},"originalLanguage":"ko"}}}`))
	}))
	defer server.Close()
	cfg := config.DefaultConfig()
	cfg.Providers.MangaDex.BaseURL = server.URL
	p, err := New(cfg, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	manga, err := p.RandomManga(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || manga.ID != "random-id" || manga.Metadata.Language != "ko" {
		t.Fatalf("calls=%d manga=%+v", calls, manga)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := p.RandomManga(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("pace cancellation=%v", err)
	}
}
func TestRandomMangaRejectsMalformedPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"result":"ok","data":{"attributes":{"title":{"en":"Random"}}}}`))
	}))
	defer server.Close()
	cfg := config.DefaultConfig()
	cfg.Providers.MangaDex.BaseURL = server.URL
	p, _ := New(cfg, server.Client())
	if _, err := p.RandomManga(context.Background()); err == nil {
		t.Fatal("accepted missing id")
	}
}
