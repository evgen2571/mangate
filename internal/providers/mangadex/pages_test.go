package mangadex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/evgen2571/mangate/internal/source"
)

func TestProviderPagesUsesAtHomeBaseURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/at-home/server/chapter-id" {
			t.Fatalf("request path = %q, want %q", r.URL.Path, "/at-home/server/chapter-id")
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"baseUrl": "https://at-home.example",
			"chapter": map[string]any{
				"hash": "chapter-hash",
				"data": []string{"001.png", "002.jpg"},
			},
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	provider := newTestProvider(t, server.URL, "en")
	pages, err := provider.Pages(context.Background(), &source.Chapter{ID: "chapter-id"})
	if err != nil {
		t.Fatalf("Pages() error = %v", err)
	}

	wantURLs := []string{
		"https://at-home.example/data/chapter-hash/001.png",
		"https://at-home.example/data/chapter-hash/002.jpg",
	}
	if len(pages) != len(wantURLs) {
		t.Fatalf("len(pages) = %d, want %d", len(pages), len(wantURLs))
	}
	for idx, want := range wantURLs {
		if pages[idx].URL != want {
			t.Fatalf("pages[%d].URL = %q, want %q", idx, pages[idx].URL, want)
		}
	}
}
