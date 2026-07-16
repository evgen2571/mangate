package mangadex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProviderSearchPopulatesMetadata(t *testing.T) {
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
						"altTitles": []map[string]string{{"ja": "テスト漫画"}, {"en": "Test Manga Alternative"}},
						"description": map[string]string{
							"en": "English description",
							"ru": "Russian description",
						},
						"status":           "ongoing",
						"contentRating":    "safe",
						"originalLanguage": "ja",
						"year":             2024,
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

	got := mangas[0].Metadata
	if got.Description["en"] != "English description" || got.Description["ru"] != "Russian description" {
		t.Fatalf("Metadata.Description = %#v, want English and Russian descriptions", got.Description)
	}
	if got.AlternativeTitle != "Test Manga Alternative" || got.Status != "ongoing" || got.ContentType != "safe" || got.Language != "ja" || got.Year != 2024 {
		t.Fatalf("Metadata = %#v, want mapped title metadata", got)
	}
}

func TestLocalizedValueIsDeterministic(t *testing.T) {
	values := map[string]string{"fr": "French", "de": "German"}
	if got := localizedValue(values, "ja"); got != "German" {
		t.Fatalf("localizedValue() = %q, want %q", got, "German")
	}
}
