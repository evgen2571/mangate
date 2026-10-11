package mangadex

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/evgen2571/mangate/internal/source"
)

func TestDescriptionAcrossDiscoveryEndpoints(t *testing.T) {
	for _, description := range []struct {
		name, field, expected string
		invalid               bool
	}{
		{"object", `,"description":{"en":"A story"}`, "A story", false},
		{"empty array", `,"description":[ ]`, "", false},
		{"null", `,"description":null`, "", false},
		{"missing", "", "", false},
		{"nonempty array", `,"description":["text"]`, "", true},
		{"string", `,"description":"text"`, "", true},
		{"boolean", `,"description":true`, "", true},
		{"number", `,"description":3`, "", true},
		{"nested object", `,"description":{"en":{}}`, "", true},
		{"null value", `,"description":{"en":null}`, "", true},
		{"unrelated schema error", `,"year":"invalid"`, "", true},
	} {
		t.Run(description.name, func(t *testing.T) {
			for _, operation := range []string{"search", "browse", "title", "random"} {
				t.Run(operation, func(t *testing.T) {
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						item := fmt.Sprintf(`{"id":"title-id","attributes":{"title":{"en":"Title"}%s}}`, description.field)
						if operation == "search" || operation == "browse" {
							fmt.Fprintf(w, `{"result":"ok","data":[%s],"limit":100,"offset":0,"total":1}`, item)
						} else {
							fmt.Fprintf(w, `{"result":"ok","data":%s}`, item)
						}
					}))
					defer server.Close()
					p := newTestProvider(t, server.URL, "en")
					var manga *source.Manga
					var err error
					switch operation {
					case "search":
						var titles []*source.Manga
						titles, err = p.Search(context.Background(), "Title")
						if err == nil {
							manga = titles[0]
						}
					case "browse":
						var page source.BrowsePage
						page, err = p.BrowseManga(context.Background(), source.BrowseRequest{})
						if err == nil {
							manga = page.Titles[0].Manga
						}
					case "title":
						manga, err = p.Title(context.Background(), "title-id")
					case "random":
						manga, err = p.RandomManga(context.Background())
					}
					if description.invalid {
						if err == nil || !strings.Contains(err.Error(), "decode") {
							t.Fatalf("expected decode error, got %v", err)
						}
					} else if err != nil || manga.Metadata.Description["en"] != description.expected {
						t.Fatalf("manga=%+v err=%v", manga, err)
					}
				})
			}
		})
	}
}
