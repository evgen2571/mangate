package mangadex

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/evgen2571/mangate/internal/source"
)

func providerOperations() map[string]func(context.Context, *Provider) error {
	return map[string]func(context.Context, *Provider) error{
		"search": func(ctx context.Context, p *Provider) error { _, err := p.Search(ctx, "test"); return err },
		"title":  func(ctx context.Context, p *Provider) error { _, err := p.Title(ctx, "title"); return err },
		"chapters": func(ctx context.Context, p *Provider) error {
			_, err := p.Chapters(ctx, &source.Manga{ID: "title"})
			return err
		},
		"cover": func(ctx context.Context, p *Provider) error {
			_, err := p.Cover(ctx, &source.Manga{ID: "title"})
			return err
		},
		"pages": func(ctx context.Context, p *Provider) error {
			_, err := p.Pages(ctx, &source.Chapter{ID: "chapter"})
			return err
		},
		"browse": func(ctx context.Context, p *Provider) error {
			_, err := p.BrowseManga(ctx, source.BrowseRequest{})
			return err
		},
	}
}

func successfulResponse(path string) string {
	switch path {
	case "/manga/title":
		return `{"result":"ok","data":{"id":"title"}}`
	case "/at-home/server/chapter":
		return `{"result":"ok","baseUrl":"https://example.org","chapter":{"hash":"hash","data":["1.png"]}}`
	case "/cover":
		return `{"result":"ok","data":[{"id":"cover","attributes":{"fileName":"cover.jpg"}}]}`
	default:
		return `{"result":"ok","data":[],"offset":0,"total":0}`
	}
}

func TestAllOperationsValidateResponses(t *testing.T) {
	for name, operation := range providerOperations() {
		for _, tc := range []struct {
			name, body, want string
			status           int
		}{
			{"api error", `{"result":"error","errors":[{"detail":"provider rejected request"}]}`, "provider rejected request", 200},
			{"empty", "", "decode", 200},
			{"malformed", "{", "decode", 200},
			{"null", "null", "response", 200},
			{"missing fields", "{}", "response", 200},
			{"trailing data", `{"result":"ok","data":[]} {}`, "decode", 200},
			{"status", `{"errors":[{"detail":"denied"}]}`, "403", 403},
		} {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); fmt.Fprint(w, tc.body) }))
				defer server.Close()
				p := newTestProvider(t, server.URL, "en")
				p.atHomeMinInterval = 0
				err := operation(context.Background(), p)
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("error = %v, want %q", err, tc.want)
				}
			})
		}
	}
}

func TestAllOperationsRetryTransientResponses(t *testing.T) {
	for name, operation := range providerOperations() {
		for _, status := range []int{429, 500, 502, 503, 504} {
			t.Run(fmt.Sprintf("%s/%d", name, status), func(t *testing.T) {
				attempts := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					attempts++
					if attempts == 1 {
						w.Header().Set("Retry-After", "0")
						w.WriteHeader(status)
						return
					}
					fmt.Fprint(w, successfulResponse(r.URL.Path))
				}))
				defer server.Close()
				p := newTestProvider(t, server.URL, "en")
				p.atHomeMinInterval = 0
				if err := operation(context.Background(), p); err != nil {
					t.Fatal(err)
				}
				if attempts != 2 {
					t.Fatalf("attempts = %d", attempts)
				}
			})
		}
	}
}

func TestAllOperationsHonorTimeout(t *testing.T) {
	for name, operation := range providerOperations() {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
			defer server.Close()
			p := newTestProvider(t, server.URL, "en")
			p.atHomeMinInterval = 0
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			if err := operation(ctx, p); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestAllOperationsBoundRetriesAndCancelWaits(t *testing.T) {
	for name, operation := range providerOperations() {
		t.Run(name, func(t *testing.T) {
			attempts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts++
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(503)
			}))
			defer server.Close()
			p := newTestProvider(t, server.URL, "en")
			p.atHomeMinInterval = 0
			p.atHomeMinInterval = 0
			if err := operation(context.Background(), p); err == nil {
				t.Fatal("expected exhausted retries error")
			}
			if attempts != 4 {
				t.Fatalf("attempts = %d, want 4", attempts)
			}
		})
		t.Run(name+"/cancel wait", func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Retry-After", "60"); w.WriteHeader(429) }))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			if err := operation(ctx, newTestProvider(t, server.URL, "en")); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestDiscoveryFailureClassification(t *testing.T) {
	for _, status := range []int{401, 403, 404, 410, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(status)
			}))
			defer server.Close()
			p := newTestProvider(t, server.URL, "en")
			_, err := p.Chapters(context.Background(), &source.Manga{ID: "title"})
			if err == nil || errors.Is(err, source.ErrCandidateUnavailable) != (status == 404 || status == 410) {
				t.Fatalf("status=%d err=%v", status, err)
			}
		})
	}
}
