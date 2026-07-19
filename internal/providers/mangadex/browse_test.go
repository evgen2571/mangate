package mangadex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/evgen2571/mangate/internal/source"
)

func TestBrowseMangaBuildsBoundedFilteredRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("limit") != "25" || query.Get("offset") != "50" {
			t.Fatalf("pagination query = %q", r.URL.RawQuery)
		}
		if got := query["originalLanguage[]"]; len(got) != 1 || got[0] != "ko" {
			t.Fatalf("original languages = %#v", got)
		}
		if got := query["availableTranslatedLanguage[]"]; len(got) != 1 || got[0] != "en" {
			t.Fatalf("chapter languages = %#v", got)
		}
		if got := query["status[]"]; len(got) != 1 || got[0] != "ongoing" {
			t.Fatalf("statuses = %#v", got)
		}
		if got := query["contentRating[]"]; len(got) != 1 || got[0] != "safe" {
			t.Fatalf("content ratings = %#v", got)
		}
		if got := query["includedTags[]"]; len(got) != 1 || got[0] != "include-tag" {
			t.Fatalf("included tags = %#v", got)
		}
		if got := query["excludedTags[]"]; len(got) != 1 || got[0] != "exclude-tag" {
			t.Fatalf("excluded tags = %#v", got)
		}
		if query.Get("order[updatedAt]") != "desc" {
			t.Fatalf("order = %q", query.Get("order[updatedAt]"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"offset": 50, "total": 100, "data": []map[string]any{{"id": "valid", "attributes": map[string]any{"title": map[string]string{"en": "Valid"}, "originalLanguage": "ko", "availableTranslatedLanguages": []string{"en"}, "tags": []map[string]any{{"attributes": map[string]any{"name": map[string]string{"en": "Drama"}}}}}}, {"attributes": map[string]any{"title": map[string]string{"en": "Missing ID"}}}}})
	}))
	defer server.Close()
	provider := newTestProvider(t, server.URL, "en")
	page, err := provider.BrowseManga(context.Background(), source.BrowseRequest{Limit: 25, Offset: 50, OriginalLanguages: []string{"ko"}, ChapterLanguages: []string{"en"}, Statuses: []string{"ongoing"}, ContentRatings: []string{"safe"}, IncludedTags: []string{"include-tag"}, ExcludedTags: []string{"exclude-tag"}, OrderBy: "updatedAt", OrderDirection: "desc"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Titles) != 1 || page.Titles[0].Manga.ID != "valid" || page.NextOffset != 52 || !page.HasMore {
		t.Fatalf("unexpected browse page: %#v", page)
	}
	if page.Titles[0].Tags[0] != "Drama" {
		t.Fatalf("tags = %#v", page.Titles[0].Tags)
	}
}

func TestBrowseMangaHonorsCancellation(t *testing.T) {
	provider := newTestProvider(t, "http://127.0.0.1:1", "en")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := provider.BrowseManga(ctx, source.BrowseRequest{Limit: 1}); err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestBrowseMangaRetriesTransientServerFailure(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"offset": 0, "total": 0, "data": []any{}})
	}))
	defer server.Close()
	provider := newTestProvider(t, server.URL, "en")
	page, err := provider.BrowseManga(context.Background(), source.BrowseRequest{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || page.HasMore {
		t.Fatalf("attempts=%d page=%#v", attempts, page)
	}
}

func TestBrowseMangaRetriesRateLimitResponse(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"offset": 0, "total": 0, "data": []any{}})
	}))
	defer server.Close()
	provider := newTestProvider(t, server.URL, "en")
	if _, err := provider.BrowseManga(context.Background(), source.BrowseRequest{Limit: 1}); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("attempts=%d, want 2", attempts)
	}
}
