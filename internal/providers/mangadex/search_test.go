package mangadex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProviderSearchPopulatesDescriptionMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("title"); got != "test manga" {
			t.Fatalf("title query = %q, want %q", got, "test manga")
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"result":   "ok",
			"response": "collection",
			"limit":    100,
			"offset":   0,
			"total":    1,
			"data": []map[string]any{
				{
					"id": "manga-id",
					"attributes": map[string]any{
						"title": map[string]string{
							"en": "Test Manga",
						},
						"description": map[string]string{
							"en": "English description",
							"ru": "Russian description",
						},
					},
				},
			},
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	provider := newTestProvider(t, server.URL, "en")
	mangas, err := provider.Search(context.Background(), "test manga")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(mangas) != 1 {
		t.Fatalf("len(mangas) = %d, want 1", len(mangas))
	}

	got := mangas[0].Metadata.Description
	if got["en"] != "English description" || got["ru"] != "Russian description" {
		t.Fatalf("Metadata.Description = %#v, want English and Russian descriptions", got)
	}
}
