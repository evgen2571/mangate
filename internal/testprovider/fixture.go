// Package testprovider supplies a local MangaDex fixture for acquisition and interface tests.
package testprovider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

type Fixture struct {
	*httptest.Server
	Image       []byte
	FailedPage  atomic.Int32
	ExpiredPage atomic.Int32
	ImageCalls  atomic.Int32
}

func New(t testing.TB) *Fixture {
	t.Helper()
	var page bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 3, 4))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&page, img); err != nil {
		t.Fatal(err)
	}
	title := map[string]any{"id": "title-id", "attributes": map[string]any{"title": map[string]string{"en": "Fixture Manga"}, "description": map[string]string{"en": "Offline integration fixture"}, "originalLanguage": "en"}}
	f := &Fixture{Image: page.Bytes()}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/manga/":
			data := []any{title}
			if r.URL.Query().Get("title") == "empty" {
				data = []any{}
			}
			json.NewEncoder(w).Encode(map[string]any{"result": "ok", "data": data, "limit": 100, "offset": 0, "total": len(data)})
		case r.URL.Path == "/manga/title-id" || r.URL.Path == "/manga/random":
			json.NewEncoder(w).Encode(map[string]any{"result": "ok", "data": title})
		case r.URL.Path == "/manga/title-id/feed":
			fmt.Fprint(w, `{"result":"ok","data":[{"id":"chapter-id","attributes":{"chapter":"1","title":"Opening / scene","pages":2,"translatedLanguage":"en"}}],"limit":500,"offset":0,"total":1}`)
		case r.URL.Path == "/at-home/server/chapter-id" || r.URL.Path == "/at-home/server/chapter-other":
			fmt.Fprintf(w, `{"result":"ok","baseUrl":"http://%s","chapter":{"hash":"hash","data":["1.png","2.png"]}}`, r.Host)
		case r.URL.Path == "/data/hash/1.png" || r.URL.Path == "/data/hash/2.png":
			index := int32(1)
			if strings.HasSuffix(r.URL.Path, "2.png") {
				index = 2
			}
			f.ImageCalls.Add(1)
			if f.FailedPage.Load() == index {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if f.ExpiredPage.CompareAndSwap(index, 0) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "image/png")
			w.Write(page.Bytes())
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.Close)
	return f
}
