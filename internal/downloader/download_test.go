package downloader

import (
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

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

	pagePath := filepath.Join(cfg.Download.Dir, "My-Manga", "1-Intro", "0001.png")
	f, err := os.Open(pagePath)
	if err != nil {
		t.Fatalf("Open(%q) error = %v", pagePath, err)
	}
	defer f.Close()

	if _, err := png.Decode(f); err != nil {
		t.Fatalf("png.Decode(%q) error = %v", pagePath, err)
	}
}

func TestDetectPageExtensionUsesAnyImageContentType(t *testing.T) {
	ext := detectPageExtension("image/tiff", "https://example.com/page")
	if ext != ".tiff" && ext != ".tif" {
		t.Fatalf("detectPageExtension() = %q, want .tiff or .tif", ext)
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
