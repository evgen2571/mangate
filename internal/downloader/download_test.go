package downloader

import (
	"context"
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
	"github.com/evgen2571/mangate/internal/constant"
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
	cfg.Dirs.Temp = t.TempDir()
	cfg.Download.Type = constant.FormatPlain
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

func TestDownloadMangaWithPageLoaderLoadsMissingPages(t *testing.T) {
	pngBytes := mustPNGBytes(t, color.RGBA{G: 255, A: 255})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Download.Dir = t.TempDir()
	cfg.Dirs.Temp = t.TempDir()
	cfg.Download.Type = constant.FormatPlain
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
	cfg.Dirs.Temp = t.TempDir()
	cfg.Download.Type = constant.FormatPlain
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
	cfg.Dirs.Temp = t.TempDir()
	cfg.Download.Type = constant.FormatPlain
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

func TestChapterDirNameUsesConsistentPrefix(t *testing.T) {
	chapter := &source.Chapter{Index: "1", Title: "Intro"}
	if got := chapterDirName(chapter); got != "Chapter-1-Intro" {
		t.Fatalf("chapterDirName() = %q, want %q", got, "Chapter-1-Intro")
	}
}

func TestChapterDirNamePrefixesTitleOnlyChapter(t *testing.T) {
	chapter := &source.Chapter{Title: "Special"}
	if got := chapterDirName(chapter); got != "Title-Special" {
		t.Fatalf("chapterDirName() = %q, want %q", got, "Title-Special")
	}
}

func TestChapterDirNameAvoidsIndexedChapterCollisionForTitleOnlyChapter(t *testing.T) {
	chapter := &source.Chapter{Title: "1"}
	if got := chapterDirName(chapter); got != "Title-1" {
		t.Fatalf("chapterDirName() = %q, want %q", got, "Title-1")
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
