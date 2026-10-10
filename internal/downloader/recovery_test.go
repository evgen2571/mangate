package downloader

import (
	"context"
	"errors"
	"image/color"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/source"
)

func TestQueuedPageDownloadCancelsPromptly(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Concurrency.PageDownloads = 1
	d := New(cfg, http.DefaultClient)
	d.pageDownloads <- struct{}{}
	released := make(chan struct{})
	go func() { time.Sleep(200 * time.Millisecond); <-d.pageDownloads; close(released) }()
	defer func() { <-released }()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, _, err := d.downloadPage(ctx, &source.Page{URL: "http://127.0.0.1:1/page"}, filepath.Join(t.TempDir(), "page"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v", err)
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Fatal("canceled page waited for an unrelated download slot")
	}
}

func TestChapterDestinationsRemainDistinctAfterSanitizing(t *testing.T) {
	data := mustPNGBytes(t, color.RGBA{A: 255})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(data)
	}))
	defer server.Close()
	cfg := config.DefaultConfig()
	cfg.Download.Dir = t.TempDir()
	manga := &source.Manga{ID: "manga", Title: "Title"}
	for i, title := range []string{"A/B", "AB", "ab"} {
		manga.Chapters = append(manga.Chapters, &source.Chapter{ID: string(rune('a' + i)), Title: title, From: manga, Pages: []*source.Page{{URL: server.URL}}})
	}
	d := New(cfg, server.Client())
	if err := d.DownloadManga(manga); err != nil {
		t.Fatal(err)
	}
	names := ChapterDirectoryNames(manga.Chapters)
	seen := map[string]bool{}
	for _, name := range names {
		if seen[strings.ToLower(name)] {
			t.Fatalf("colliding destination %q", name)
		}
		seen[strings.ToLower(name)] = true
		if _, err := os.Stat(filepath.Join(cfg.Download.Dir, TitleDirectoryName(manga), name, "0001.png")); err != nil {
			t.Fatalf("reported destination does not exist: %v", err)
		}
	}
}

func TestFailedPageIdentifiesDestinationAndChapter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) }))
	defer server.Close()
	for _, directory := range []string{"library", `library\chapters`} {
		t.Run(directory, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Download.Dir = filepath.Join(t.TempDir(), directory)
			chapter := &source.Chapter{ID: "chapter-id", Index: "7", From: &source.Manga{ID: "manga-id", Title: "Title"}, Pages: []*source.Page{{URL: server.URL}}}
			err := New(cfg, server.Client()).DownloadChapter(chapter)
			if err == nil {
				t.Fatal("expected failure")
			}
			destination := filepath.Join(cfg.Download.Dir, TitleDirectoryName(chapter.From), "Chapter-7")
			for _, want := range []string{"manga-id", "chapter-id", "page 1", strconv.Quote(destination)} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q lacks %q", err, want)
				}
			}
		})
	}
}

func TestFailedReplacementPreservesExistingPage(t *testing.T) {
	for _, stable := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "stable"}[stable], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) }))
			defer server.Close()
			cfg := config.DefaultConfig()
			cfg.Download.Dir = t.TempDir()
			cfg.Download.ExistingFileMode = "replace"
			chapter := &source.Chapter{ID: "chapter", Index: "1", From: &source.Manga{ID: "manga", Title: "Title"}, Pages: []*source.Page{{URL: server.URL}}}
			directory := filepath.Join(cfg.Download.Dir, TitleDirectoryName(chapter.From), "Chapter-1")
			if err := os.MkdirAll(directory, 0755); err != nil {
				t.Fatal(err)
			}
			page := filepath.Join(directory, "0001.png")
			data := mustPNGBytes(t, color.RGBA{A: 255})
			if err := os.WriteFile(page, data, 0644); err != nil {
				t.Fatal(err)
			}
			d := New(cfg, server.Client())
			var err error
			if stable {
				_, err = d.DownloadChapterTo(context.Background(), chapter, directory, nil)
			} else {
				err = d.DownloadChapter(chapter)
			}
			if err == nil {
				t.Fatal("expected failed replacement")
			}
			got, err := os.ReadFile(page)
			if err != nil || string(got) != string(data) {
				t.Fatalf("existing page was lost: %v", err)
			}
		})
	}
}

func TestPageRetriesTransientServerFailure(t *testing.T) {
	attempts := 0
	data := mustPNGBytes(t, color.RGBA{A: 255})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(503)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(data)
	}))
	defer server.Close()
	cfg := config.DefaultConfig()
	cfg.Download.Dir = t.TempDir()
	d := New(cfg, server.Client())
	got, _, err := d.downloadPage(context.Background(), &source.Page{URL: server.URL}, filepath.Join(cfg.Download.Dir, "0001"))
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d", attempts)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatal(err)
	}
}

func TestPageReuseAndReplacementTreatPathsLiterally(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Title [Special]")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(dir, "0001")
	if err := os.WriteFile(base+".png", []byte("old image"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !existingPage(base) || existingPagePath(base) != base+".png" {
		t.Fatal("existing image in a bracketed directory was not found")
	}
	if err := os.WriteFile(base+".jpg", []byte("new image"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := removeExistingPage(base, base+".jpg"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(base + ".png"); !os.IsNotExist(err) {
		t.Fatalf("old page still exists: %v", err)
	}
	if _, err := os.Stat(base + ".jpg"); err != nil {
		t.Fatalf("replacement missing: %v", err)
	}
}
