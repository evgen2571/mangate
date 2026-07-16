package downloader

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/source"
)

func TestDownloadChapterPlainKeepsDownloadedPageType(t *testing.T) {
	pngBytes := mustPNGBytes(t, color.RGBA{R: 255, A: 255})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Download.Dir = t.TempDir()
	cfg.Concurrency.PageDownloads = 1

	d := New(cfg, server.Client())

	manga := &source.Manga{Title: "My Manga"}
	chapter := &source.Chapter{
		Index: "1",
		Title: "Intro",
		From:  manga,
		Pages: []*source.Page{{URL: server.URL + "/page?token=abc"}},
	}

	if err := d.DownloadChapter(chapter); err != nil {
		t.Fatalf("DownloadChapter() error = %v", err)
	}

	pagePath := filepath.Join(cfg.Download.Dir, "My-Manga", "Chapter-1-Intro", "0001.png")
	f, err := os.Open(pagePath)
	if err != nil {
		t.Fatalf("Open(%q) error = %v", pagePath, err)
	}
	defer f.Close()

	if _, err := png.Decode(f); err != nil {
		t.Fatalf("png.Decode(%q) error = %v", pagePath, err)
	}
}

func TestDownloadChapterKeepsCompletedPagesAndMarksPartialState(t *testing.T) {
	pngBytes := mustPNGBytes(t, color.RGBA{R: 255, G: 255, A: 255})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/broken.png" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Download.Dir = t.TempDir()
	cfg.Concurrency.PageDownloads = 1
	d := New(cfg, server.Client())
	manga := &source.Manga{ID: "manga-1", Title: "Partial Manga"}
	chapter := &source.Chapter{ID: "chapter-1", Index: "1", From: manga, Pages: []*source.Page{{URL: server.URL + "/good.png"}, {URL: server.URL + "/broken.png"}}}

	if err := d.DownloadChapter(chapter); err == nil {
		t.Fatal("DownloadChapter() error = nil, want failed page")
	}
	chapterDir := filepath.Join(cfg.Download.Dir, "Partial-Manga-manga-1", "Chapter-1")
	if _, err := os.Stat(filepath.Join(chapterDir, "0001.png")); err != nil {
		t.Fatalf("completed page was not preserved: %v", err)
	}
	stateData, err := os.ReadFile(filepath.Join(chapterDir, ".mangate.json"))
	if err != nil {
		t.Fatalf("read incomplete state: %v", err)
	}
	var state struct {
		Complete bool   `json:"complete"`
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal(stateData, &state); err != nil {
		t.Fatalf("decode state: %v", err)
	}
	if state.Complete {
		t.Fatal("incomplete chapter was marked complete")
	}
	if state.Provider != cfg.Provider {
		t.Fatalf("state provider = %q, want %q", state.Provider, cfg.Provider)
	}
}

func TestDownloadMangaWithPageLoaderLoadsMissingPages(t *testing.T) {
	pngBytes := mustPNGBytes(t, color.RGBA{G: 255, A: 255})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Download.Dir = t.TempDir()
	cfg.Concurrency.PageDownloads = 1
	cfg.Concurrency.ChapterDownloads = 1

	d := New(cfg, server.Client())

	manga := &source.Manga{Title: "Lazy Manga"}
	manga.Chapters = []*source.Chapter{
		{ID: "chapter-1", Index: "1", Title: "One", From: manga},
		{ID: "chapter-2", Index: "2", Title: "Two", From: manga},
	}

	loaded := make([]string, 0, len(manga.Chapters))
	loader := func(_ context.Context, chapter *source.Chapter) ([]*source.Page, error) {
		loaded = append(loaded, chapter.ID)
		return []*source.Page{{URL: server.URL + "/" + chapter.ID + ".png"}}, nil
	}

	var finalProgress DownloadProgress
	if err := d.DownloadMangaWithProgressAndPageLoader(context.Background(), manga, loader, func(progress DownloadProgress) {
		finalProgress = progress
	}); err != nil {
		t.Fatalf("DownloadMangaWithProgressAndPageLoader() error = %v", err)
	}

	if !reflect.DeepEqual(loaded, []string{"chapter-1", "chapter-2"}) {
		t.Fatalf("loaded chapters = %#v, want %#v", loaded, []string{"chapter-1", "chapter-2"})
	}

	for _, chapterName := range []string{"Chapter-1-One", "Chapter-2-Two"} {
		pagePath := filepath.Join(cfg.Download.Dir, "Lazy-Manga", chapterName, "0001.png")
		if _, err := os.Stat(pagePath); err != nil {
			t.Fatalf("Stat(%q) error = %v", pagePath, err)
		}
	}

	if finalProgress.TotalPages != 2 || finalProgress.CompletedPages != 2 {
		t.Fatalf("final progress pages = %d/%d, want 2/2", finalProgress.CompletedPages, finalProgress.TotalPages)
	}
}

func TestDownloadMangaReportsChaptersActiveBeforeLazyPageLoadingCompletes(t *testing.T) {
	pngBytes := mustPNGBytes(t, color.RGBA{R: 255, B: 255, A: 255})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Download.Dir = t.TempDir()
	cfg.Concurrency.ChapterDownloads = 5

	d := New(cfg, server.Client())

	manga := &source.Manga{Title: "Lazy Active Manga"}
	for idx := 1; idx <= 5; idx++ {
		manga.Chapters = append(manga.Chapters, &source.Chapter{
			ID:        "chapter-" + strconv.Itoa(idx),
			Index:     strconv.Itoa(idx),
			From:      manga,
			PageCount: 1,
		})
	}

	loaderStarted := make(chan struct{}, len(manga.Chapters))
	releaseLoader := make(chan struct{})
	loader := func(_ context.Context, chapter *source.Chapter) ([]*source.Page, error) {
		loaderStarted <- struct{}{}
		<-releaseLoader
		return []*source.Page{{URL: server.URL + "/" + chapter.ID + ".png"}}, nil
	}

	progressCh := make(chan DownloadProgress, 32)
	done := make(chan error, 1)
	go func() {
		done <- d.DownloadMangaWithProgressAndPageLoader(context.Background(), manga, loader, func(progress DownloadProgress) {
			progressCh <- progress
		})
	}()

	for range manga.Chapters {
		select {
		case <-loaderStarted:
		case <-time.After(time.Second):
			close(releaseLoader)
			t.Fatalf("timed out waiting for all chapter workers to enter lazy page loading")
		}
	}

	observedAllActive := false
	deadline := time.After(time.Second)
	for !observedAllActive {
		select {
		case progress := <-progressCh:
			active := 0
			for _, chapter := range progress.Chapters {
				if chapter.Active {
					active++
				}
			}
			observedAllActive = active == 5
		case err := <-done:
			t.Fatalf("download finished before reporting active lazy-loading chapters: %v", err)
		case <-deadline:
			close(releaseLoader)
			t.Fatalf("timed out waiting for progress with 5 active chapters")
		}
	}

	close(releaseLoader)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("DownloadMangaWithProgressAndPageLoader() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for download to finish")
	}
}

func TestDownloadMangaUsesGlobalPageDownloadLimit(t *testing.T) {
	pngBytes := mustPNGBytes(t, color.RGBA{B: 255, A: 255})
	var active atomic.Int32
	var maxActive atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := active.Add(1)
		for {
			max := maxActive.Load()
			if current <= max || maxActive.CompareAndSwap(max, current) {
				break
			}
		}
		defer active.Add(-1)

		time.Sleep(25 * time.Millisecond)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Download.Dir = t.TempDir()
	cfg.Concurrency.PageDownloads = 1
	cfg.Concurrency.ChapterDownloads = 4

	d := New(cfg, server.Client())

	manga := &source.Manga{Title: "Limited Manga"}
	for idx := 1; idx <= 4; idx++ {
		chapter := &source.Chapter{
			Index: strconv.Itoa(idx),
			From:  manga,
			Pages: []*source.Page{{URL: server.URL + "/page.png"}},
		}
		manga.Chapters = append(manga.Chapters, chapter)
	}

	if err := d.DownloadManga(manga); err != nil {
		t.Fatalf("DownloadManga() error = %v", err)
	}

	if got := maxActive.Load(); got != 1 {
		t.Fatalf("max concurrent page downloads = %d, want 1", got)
	}
}

func TestDownloadMangaDisambiguatesDuplicateChapterDirectoryNames(t *testing.T) {
	pngBytes := mustPNGBytes(t, color.RGBA{R: 128, G: 128, A: 255})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Download.Dir = t.TempDir()
	cfg.Concurrency.PageDownloads = 2
	cfg.Concurrency.ChapterDownloads = 2

	d := New(cfg, server.Client())

	manga := &source.Manga{Title: "Duplicate Manga"}
	manga.Chapters = []*source.Chapter{
		{ID: "chapter-a", Index: "1", From: manga, Pages: []*source.Page{{URL: server.URL + "/a.png"}}},
		{ID: "chapter-b", Index: "1", From: manga, Pages: []*source.Page{{URL: server.URL + "/b.png"}}},
	}

	if err := d.DownloadManga(manga); err != nil {
		t.Fatalf("DownloadManga() error = %v", err)
	}

	wantDirs := []string{"Chapter-1", "Chapter-1-chapter-b"}
	for _, dir := range wantDirs {
		pagePath := filepath.Join(cfg.Download.Dir, "Duplicate-Manga", dir, "0001.png")
		if _, err := os.Stat(pagePath); err != nil {
			t.Fatalf("Stat(%q) error = %v", pagePath, err)
		}
	}
}

func TestDownloadMangaRefreshesLazyPagesAfterForbiddenImage(t *testing.T) {
	pngBytes := mustPNGBytes(t, color.RGBA{R: 64, B: 200, A: 255})
	var staleAttempts atomic.Int32
	var freshAttempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/stale.png":
			staleAttempts.Add(1)
			w.WriteHeader(http.StatusForbidden)
		case "/fresh.png":
			freshAttempts.Add(1)
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngBytes)
		default:
			t.Fatalf("unexpected request path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Download.Dir = t.TempDir()
	cfg.Concurrency.PageDownloads = 1
	cfg.Concurrency.ChapterDownloads = 1

	d := New(cfg, server.Client())

	manga := &source.Manga{Title: "Refresh Manga"}
	chapter := &source.Chapter{ID: "chapter-1", Index: "1", From: manga, PageCount: 1}
	manga.Chapters = []*source.Chapter{chapter}

	var loads atomic.Int32
	loader := func(_ context.Context, chapter *source.Chapter) ([]*source.Page, error) {
		if loads.Add(1) == 1 {
			return []*source.Page{{URL: server.URL + "/stale.png"}}, nil
		}
		return []*source.Page{{URL: server.URL + "/fresh.png"}}, nil
	}

	if err := d.DownloadMangaWithProgressAndPageLoader(context.Background(), manga, loader, nil); err != nil {
		t.Fatalf("DownloadMangaWithProgressAndPageLoader() error = %v", err)
	}

	if got := loads.Load(); got != 2 {
		t.Fatalf("page loads = %d, want 2", got)
	}
	if got := staleAttempts.Load(); got != 1 {
		t.Fatalf("stale attempts = %d, want 1", got)
	}
	if got := freshAttempts.Load(); got != 1 {
		t.Fatalf("fresh attempts = %d, want 1", got)
	}

	pagePath := filepath.Join(cfg.Download.Dir, "Refresh-Manga", "Chapter-1", "0001.png")
	if _, err := os.Stat(pagePath); err != nil {
		t.Fatalf("Stat(%q) error = %v", pagePath, err)
	}
}

func TestDownloadPageRetriesTooManyRequests(t *testing.T) {
	pngBytes := mustPNGBytes(t, color.RGBA{R: 255, G: 255, A: 255})
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		if attempt == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Download.Dir = t.TempDir()
	cfg.Concurrency.PageDownloads = 1

	d := New(cfg, server.Client())

	manga := &source.Manga{Title: "Retry Manga"}
	chapter := &source.Chapter{
		Index: "1",
		From:  manga,
		Pages: []*source.Page{{URL: server.URL + "/page.png"}},
	}

	if err := d.DownloadChapter(chapter); err != nil {
		t.Fatalf("DownloadChapter() error = %v", err)
	}

	if got := attempts.Load(); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
}

func TestDetectPageExtensionUsesAnyImageContentType(t *testing.T) {
	ext := detectPageExtension("image/tiff", "https://example.com/page")
	if ext != ".tiff" && ext != ".tif" {
		t.Fatalf("detectPageExtension() = %q, want .tiff or .tif", ext)
	}
}

func TestChapterDirBaseName(t *testing.T) {
	tests := []struct {
		name    string
		chapter *source.Chapter
		want    string
	}{
		{name: "nil chapter", chapter: nil, want: "unknown-chapter"},
		{name: "index and title", chapter: &source.Chapter{Index: " 1 ", Title: " Intro "}, want: "Chapter-1-Intro"},
		{name: "index only", chapter: &source.Chapter{Index: " 2 "}, want: "Chapter-2"},
		{name: "title only is prefixed", chapter: &source.Chapter{Title: " Special "}, want: "Title-Special"},
		{name: "title only avoids indexed chapter collision", chapter: &source.Chapter{Title: "1"}, want: "Title-1"},
		{name: "empty chapter", chapter: &source.Chapter{}, want: "unknown-chapter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := chapterDirBaseName(tt.chapter); got != tt.want {
				t.Fatalf("chapterDirBaseName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func mustPNGBytes(t *testing.T, fill color.Color) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.Set(x, y, fill)
		}
	}

	file, err := os.CreateTemp(t.TempDir(), "image-*.png")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	path := file.Name()
	defer os.Remove(path)

	if err := png.Encode(file, img); err != nil {
		file.Close()
		t.Fatalf("png.Encode() error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return data
}
