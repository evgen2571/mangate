package mangadex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"

	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/source"
)

func TestProviderChaptersRequestsConfiguredLanguage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		languages := r.URL.Query()["translatedLanguage[]"]
		if !reflect.DeepEqual(languages, []string{"ru"}) {
			t.Fatalf("translatedLanguage[] query = %#v, want %#v", languages, []string{"ru"})
		}

		writeMangaDexChaptersResponse(t, w, 0, 500, 0, nil)
	}))
	defer server.Close()

	provider := newTestProvider(t, server.URL, "ru")
	_, err := provider.Chapters(context.Background(), &source.Manga{ID: "manga-id"})
	if err != nil {
		t.Fatalf("Chapters() error = %v", err)
	}
}

func TestProviderChaptersFetchesAllPages(t *testing.T) {
	requestedOffsets := make([]int, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
		if err != nil {
			t.Fatalf("offset query = %q, want integer: %v", r.URL.Query().Get("offset"), err)
		}
		requestedOffsets = append(requestedOffsets, offset)

		switch offset {
		case 0:
			writeMangaDexChaptersResponse(t, w, 0, 500, 501, []testMangaDexChapter{
				{ID: "chapter-2", Chapter: "2", Title: "Second", Language: "en"},
			})
		case 500:
			writeMangaDexChaptersResponse(t, w, 500, 500, 501, []testMangaDexChapter{
				{ID: "chapter-1", Chapter: "1", Title: "First", Language: "en"},
			})
		default:
			t.Fatalf("unexpected offset %d", offset)
		}
	}))
	defer server.Close()

	provider := newTestProvider(t, server.URL, "en")
	chapters, err := provider.Chapters(context.Background(), &source.Manga{ID: "manga-id"})
	if err != nil {
		t.Fatalf("Chapters() error = %v", err)
	}

	if !reflect.DeepEqual(requestedOffsets, []int{0, 500}) {
		t.Fatalf("requested offsets = %#v, want %#v", requestedOffsets, []int{0, 500})
	}

	gotIDs := make([]string, 0, len(chapters))
	for _, chapter := range chapters {
		gotIDs = append(gotIDs, chapter.ID)
	}
	if !reflect.DeepEqual(gotIDs, []string{"chapter-1", "chapter-2"}) {
		t.Fatalf("chapter IDs = %#v, want %#v", gotIDs, []string{"chapter-1", "chapter-2"})
	}
}

func TestProviderChaptersIncludesPageCount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeMangaDexChaptersResponse(t, w, 0, 500, 1, []testMangaDexChapter{
			{ID: "chapter-1", Chapter: "1", Title: "First", Language: "en", Pages: 23},
		})
	}))
	defer server.Close()

	provider := newTestProvider(t, server.URL, "en")
	chapters, err := provider.Chapters(context.Background(), &source.Manga{ID: "manga-id"})
	if err != nil {
		t.Fatalf("Chapters() error = %v", err)
	}
	if len(chapters) != 1 {
		t.Fatalf("len(chapters) = %d, want 1", len(chapters))
	}
	if chapters[0].PageCount != 23 {
		t.Fatalf("chapter PageCount = %d, want 23", chapters[0].PageCount)
	}
	if chapters[0].Language != "en" {
		t.Fatalf("chapter Language = %q, want en", chapters[0].Language)
	}
}

func newTestProvider(t *testing.T, baseURL, language string) *Provider {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.Language = language
	cfg.Providers.MangaDex.BaseURL = baseURL
	cfg.Providers.MangaDex.SiteURL = baseURL
	cfg.Providers.MangaDex.UploadsURL = baseURL

	provider, err := New(cfg, http.DefaultClient)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return provider
}

type testMangaDexChapter struct {
	ID       string
	Chapter  string
	Title    string
	Language string
	Pages    int
}

func writeMangaDexChaptersResponse(t *testing.T, w http.ResponseWriter, offset, limit, total int, chapters []testMangaDexChapter) {
	t.Helper()

	data := make([]map[string]any, 0, len(chapters))
	for _, chapter := range chapters {
		data = append(data, map[string]any{
			"id": chapter.ID,
			"attributes": map[string]any{
				"chapter":            chapter.Chapter,
				"title":              chapter.Title,
				"pages":              chapter.Pages,
				"translatedLanguage": chapter.Language,
			},
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"result":   "ok",
		"response": "collection",
		"limit":    limit,
		"offset":   offset,
		"total":    total,
		"data":     data,
	}); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
