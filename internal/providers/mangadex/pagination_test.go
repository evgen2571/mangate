package mangadex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/evgen2571/mangate/internal/source"
)

func TestSearchRetrievesAllResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if offset != 0 && offset != 100 {
			t.Errorf("unexpected offset %d", offset)
		}
		count := 100
		if offset == 100 {
			count = 1
		}
		data := make([]map[string]any, count)
		for i := range data {
			data[i] = map[string]any{"id": fmt.Sprintf("manga-%d", offset+i), "attributes": map[string]any{"title": map[string]string{"en": "Manga"}}}
		}
		json.NewEncoder(w).Encode(map[string]any{"result": "ok", "data": data, "offset": offset, "limit": 100, "total": 101})
	}))
	defer server.Close()
	got, err := newTestProvider(t, server.URL, "en").Search(context.Background(), "manga")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 101 || got[100].ID != "manga-100" {
		t.Fatalf("got %d results, want 101", len(got))
	}
}

func TestCollectionsRejectTruncatedPagination(t *testing.T) {
	for _, name := range []string{"search", "chapters", "browse"} {
		for _, tc := range []struct{ name, body string }{
			{"empty before total", `{"result":"ok","data":[],"offset":0,"total":10}`},
			{"wrong offset", `{"result":"ok","data":[],"offset":50,"total":50}`},
			{"missing collection", `{"result":"ok","chapter":{}}`},
			{"missing total with data", `{"result":"ok","data":[{"id":"item"}],"offset":0}`},
			{"total smaller than page", `{"result":"ok","data":[{"id":"one"},{"id":"two"}],"offset":0,"total":1}`},
		} {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tc.body) }))
				defer server.Close()
				if err := providerOperations()[name](context.Background(), newTestProvider(t, server.URL, "en")); err == nil {
					t.Fatal("expected invalid pagination error")
				}
			})
		}
	}
}

func TestSearchReportsResultWindowLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"result":"ok","data":[{"id":"manga"}],"offset":0,"total":10001}`)
	}))
	defer server.Close()
	_, err := newTestProvider(t, server.URL, "en").Search(context.Background(), "manga")
	if err == nil || !strings.Contains(err.Error(), "narrow") {
		t.Fatalf("error = %v, want instruction to narrow query", err)
	}
}

func TestMissingChapterNumberStaysUnknown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"result":"ok","data":[{"id":"special","attributes":{"chapter":null,"title":"Special"}}],"total":1}`)
	}))
	defer server.Close()
	chapters, err := newTestProvider(t, server.URL, "en").Chapters(context.Background(), &source.Manga{ID: "manga"})
	if err != nil {
		t.Fatal(err)
	}
	if len(chapters) != 1 || chapters[0].Index != "" || chapters[0].DisplayName() != "Special" {
		t.Fatalf("chapters = %#v", chapters)
	}
}
