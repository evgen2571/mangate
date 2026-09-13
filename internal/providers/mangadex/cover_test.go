package mangadex

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/evgen2571/mangate/internal/source"
)

func TestCoverValidatesAndEscapesFilename(t *testing.T) {
	for _, tc := range []struct {
		filename string
		valid    bool
	}{
		{"", false}, {"../cover.jpg", false}, {"cover image.jpg", true},
	} {
		t.Run(tc.filename, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("manga[]") != "manga&other" {
					t.Errorf("query = %s", r.URL.RawQuery)
				}
				fmt.Fprintf(w, `{"result":"ok","data":[{"id":"cover","attributes":{"fileName":%q}}]}`, tc.filename)
			}))
			defer server.Close()
			got, err := newTestProvider(t, server.URL, "en").Cover(context.Background(), &source.Manga{ID: "manga&other"})
			if !tc.valid {
				if err == nil {
					t.Fatalf("accepted invalid cover URL %q", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if want := server.URL + "/covers/manga&other/cover%20image.jpg"; got != want {
				t.Fatalf("URL = %q, want %q", got, want)
			}
		})
	}
}
