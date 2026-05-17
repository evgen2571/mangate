package converter

import (
	"archive/zip"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/evgen2571/mangate/internal/config"
)

func TestConvertChapterPlainKeepsProviderFileTypes(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Download.Dir = t.TempDir()
	cfg.Download.Type = config.DownloadTypePlain

	sourceDir := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	writePNG(t, filepath.Join(sourceDir, "0001.png"), color.RGBA{R: 255, A: 255})
	writePNG(t, filepath.Join(sourceDir, "0002.png"), color.RGBA{G: 255, A: 255})

	conv := New(cfg)
	if err := conv.ConvertChapter(sourceDir, "My Manga", "001-Intro"); err != nil {
		t.Fatalf("ConvertChapter() error = %v", err)
	}

	targetDir := filepath.Join(cfg.Download.Dir, "My Manga", "001-Intro")
	entries := dirEntries(t, targetDir)
	want := []string{"0001.png", "0002.png"}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("plain output files = %#v, want %#v", entries, want)
	}

	assertPNG(t, filepath.Join(targetDir, "0001.png"))
	assertPNG(t, filepath.Join(targetDir, "0002.png"))

	if _, err := os.Stat(sourceDir); !os.IsNotExist(err) {
		t.Fatalf("sourceDir still exists, stat err = %v", err)
	}
}

func TestConvertChapterCBZWritesArchive(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Download.Dir = t.TempDir()
	cfg.Download.Type = config.DownloadTypeCBZ

	sourceDir := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	writePNG(t, filepath.Join(sourceDir, "0001.png"), color.RGBA{B: 255, A: 255})
	writePNG(t, filepath.Join(sourceDir, "0002.png"), color.RGBA{R: 255, G: 255, A: 255})

	conv := New(cfg)
	if err := conv.ConvertChapter(sourceDir, "My Manga", "001-Intro"); err != nil {
		t.Fatalf("ConvertChapter() error = %v", err)
	}

	archivePath := filepath.Join(cfg.Download.Dir, "My Manga", "001-Intro.cbz")
	entries := zipEntries(t, archivePath)
	want := []string{"0001.png", "0002.png"}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("archive entries = %#v, want %#v", entries, want)
	}

	if _, err := os.Stat(sourceDir); !os.IsNotExist(err) {
		t.Fatalf("sourceDir still exists, stat err = %v", err)
	}
}

func TestConvertChapterPlainKeepsExistingOutputOnFailure(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Download.Dir = t.TempDir()
	cfg.Download.Type = config.DownloadTypePlain

	targetDir := filepath.Join(cfg.Download.Dir, "My Manga", "001-Intro")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	keepPath := filepath.Join(targetDir, "keep.png")
	if err := os.WriteFile(keepPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", keepPath, err)
	}

	sourceDir := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	brokenPath := filepath.Join(sourceDir, "0001.png")
	if err := os.Symlink(filepath.Join(sourceDir, "missing.png"), brokenPath); err != nil {
		t.Fatalf("Symlink(%q) error = %v", brokenPath, err)
	}

	conv := New(cfg)
	if err := conv.ConvertChapter(sourceDir, "My Manga", "001-Intro"); err == nil {
		t.Fatal("ConvertChapter() error = nil, want non-nil")
	}

	data, err := os.ReadFile(keepPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", keepPath, err)
	}
	if string(data) != "old" {
		t.Fatalf("keep.png = %q, want %q", string(data), "old")
	}
}

func TestConvertChapterUnsupportedTypeReturnsError(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Download.Dir = t.TempDir()
	cfg.Download.Type = "rar"

	sourceDir := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writePNG(t, filepath.Join(sourceDir, "0001.png"), color.RGBA{R: 255, A: 255})

	conv := New(cfg)
	if err := conv.ConvertChapter(sourceDir, "My Manga", "001-Intro"); err == nil {
		t.Fatal("ConvertChapter() error = nil, want non-nil")
	}
}

func writePNG(t *testing.T, path string, fill color.Color) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create(%q) error = %v", path, err)
	}
	defer f.Close()

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.Set(x, y, fill)
		}
	}

	if err := png.Encode(f, img); err != nil {
		t.Fatalf("png.Encode(%q) error = %v", path, err)
	}
}

func assertPNG(t *testing.T, path string) {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open(%q) error = %v", path, err)
	}
	defer f.Close()

	if _, err := png.Decode(f); err != nil {
		t.Fatalf("png.Decode(%q) error = %v", path, err)
	}
}

func dirEntries(t *testing.T, path string) []string {
	t.Helper()

	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", path, err)
	}

	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.Name())
	}
	sort.Strings(out)
	return out
}

func zipEntries(t *testing.T, path string) []string {
	t.Helper()

	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("zip.OpenReader(%q) error = %v", path, err)
	}
	defer zr.Close()

	out := make([]string, 0, len(zr.File))
	for _, file := range zr.File {
		out = append(out, file.Name)
	}
	sort.Strings(out)
	return out
}
