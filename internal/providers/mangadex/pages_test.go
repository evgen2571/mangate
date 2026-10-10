package mangadex

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

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

type pageRoundTripFunc func(*http.Request) (*http.Response, error)

func (f pageRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestProviderPagesPacesAtHomeRequests(t *testing.T) {
	// Measure request starts with a virtual clock, independent of network and OS scheduling.
	synctest.Test(t, func(t *testing.T) {
		var mu sync.Mutex
		requestTimes := make([]time.Time, 0, 2)
		provider := newTestProvider(t, "https://api.example", "en")
		provider.atHomeMinInterval = 20 * time.Millisecond
		provider.client = &http.Client{Transport: pageRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			mu.Lock()
			requestTimes = append(requestTimes, time.Now())
			mu.Unlock()
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"baseUrl":"https://at-home.example","chapter":{"hash":"chapter-hash","data":["001.png"]}}`)),
			}, nil
		})}

		var wg sync.WaitGroup
		wg.Add(2)
		for _, chapterID := range []string{"chapter-1", "chapter-2"} {
			go func() {
				defer wg.Done()
				if _, err := provider.Pages(context.Background(), &source.Chapter{ID: chapterID}); err != nil {
					t.Errorf("Pages(%q) error = %v", chapterID, err)
				}
			}()
		}
		wg.Wait()

		if len(requestTimes) != 2 {
			t.Fatalf("request count = %d, want 2", len(requestTimes))
		}
		if spacing := requestTimes[1].Sub(requestTimes[0]); spacing != provider.atHomeMinInterval {
			t.Fatalf("request spacing = %s, want %s", spacing, provider.atHomeMinInterval)
		}
	})
}

func TestPagesRejectsInvalidImageLocations(t *testing.T) {
	for _, body := range []string{
		`{"baseUrl":"https://example.org","chapter":{"hash":"hash","data":[]}}`,
		`{"baseUrl":"https://example.org","chapter":{"hash":"","data":["1.png"]}}`,
		`{"baseUrl":"file:///tmp","chapter":{"hash":"hash","data":["1.png"]}}`,
		`{"baseUrl":"https://example.org","chapter":{"hash":"hash","data":[""]}}`,
		`{"baseUrl":"https://example.org","chapter":{"hash":"hash","data":["../1.png"]}}`,
	} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
			defer server.Close()
			if _, err := newTestProvider(t, server.URL, "en").Pages(context.Background(), &source.Chapter{ID: "chapter"}); err == nil {
				t.Fatal("expected invalid pages response error")
			}
		})
	}
}

func TestAtHomeQueueDoesNotDelayCancellation(t *testing.T) {
	p := newTestProvider(t, "http://127.0.0.1:1", "en")
	p.atHomeMinInterval = 200 * time.Millisecond
	p.lastAtHomeRequest = time.Now()
	waiting := make(chan struct{})
	go func() { defer close(waiting); p.paceAtHomeRequest(context.Background()) }()
	defer func() { <-waiting }()
	time.Sleep(20 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	if err := p.paceAtHomeRequest(ctx); err != context.Canceled {
		t.Fatalf("error = %v", err)
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("canceled request waited %s for another caller", elapsed)
	}
}
