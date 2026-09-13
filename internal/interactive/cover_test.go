package interactive

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/source"
)

func TestResultsLoadCoverAndShowMetadata(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			var data bytes.Buffer
			if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 4, 6))); err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if fail {
					http.Error(w, "unavailable", http.StatusNotFound)
					return
				}
				if r.URL.Path == "/cover" {
					fmt.Fprint(w, `{"result":"ok","data":[{"id":"cover","attributes":{"fileName":"cover.png"}}]}`)
					return
				}
				w.Header().Set("Content-Type", "image/png")
				w.Write(data.Bytes())
			}))
			defer server.Close()
			cfg := config.DefaultConfig()
			cfg.Dirs.Cache = t.TempDir()
			cfg.Providers.MangaDex.BaseURL = server.URL
			cfg.Providers.MangaDex.UploadsURL = server.URL
			a, err := app.New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			m := NewWithContext(a, context.Background()).(*model)
			resize(t, m, 100, 30)
			manga := &source.Manga{ID: "manga", Title: "Test title", Metadata: source.MangaMetadata{Description: map[string]string{"en": "A readable synopsis"}, Status: "ongoing"}}
			_, cmd := m.Update(searchDone{query: "Test", results: []*source.Manga{manga}})
			if cmd == nil {
				t.Fatal("selecting the first search result did not load its cover")
			}
			if !strings.Contains(ansi.Strip(m.View()), "Loading cover") {
				t.Fatal("missing cover loading state")
			}
			m.Update(cmd())
			view := ansi.Strip(m.View())
			for _, want := range []string{"A readable synopsis", "ongoing"} {
				if !strings.Contains(view, want) {
					t.Errorf("metadata %q missing from view", want)
				}
			}
			if fail {
				if !strings.Contains(view, "Cover unavailable") {
					t.Fatalf("failure missing: %s", view)
				}
			} else if !strings.Contains(view, "▀") {
				t.Fatalf("image preview missing: %s", view)
			}
		})
	}
}

func TestStaleCoverResponseCannotReplaceSelectedTitle(t *testing.T) {
	m := testModel(t)
	m.cover = coverState{id: "second", request: 2, loading: true}
	old := image.NewRGBA(image.Rect(0, 0, 2, 2))
	m.Update(coverDone{request: 1, image: old})
	if !m.cover.loading || m.cover.image != nil {
		t.Fatal("stale result overwrote current cover state")
	}
	m.Update(coverDone{request: 2, image: old})
	if m.cover.loading || m.cover.image != old {
		t.Fatal("current result was not applied")
	}
}

func TestChangingCoverSelectionCancelsPreviousRequest(t *testing.T) {
	m := testModel(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.cover = coverState{id: "first", request: 1, loading: true, cancel: cancel}
	m.newResultsList([]*source.Manga{{ID: "second", Title: "Second"}})
	cmd := m.loadSelectedCover()
	if ctx.Err() != context.Canceled {
		t.Fatal("previous request was not canceled")
	}
	if cmd == nil || m.cover.id != "second" || !m.cover.loading {
		t.Fatal("new selection was not loaded")
	}
	m.clearCover()
}
