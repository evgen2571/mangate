package dataset

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/source"
)

type datasetProvider struct{ pageURL string }

func (p datasetProvider) Name() string { return "fake" }
func (p datasetProvider) Info() source.ProviderInfo {
	return source.ProviderInfo{ID: "fake", DownloadPermitted: true}
}
func (p datasetProvider) Search(context.Context, string) ([]*source.Manga, error) { return nil, nil }
func (p datasetProvider) Title(context.Context, string) (*source.Manga, error)    { return nil, nil }
func (p datasetProvider) Cover(context.Context, *source.Manga) (string, error)    { return "", nil }
func (p datasetProvider) BrowseManga(context.Context, source.BrowseRequest) (source.BrowsePage, error) {
	return source.BrowsePage{Titles: []source.BrowseTitle{{Manga: &source.Manga{ID: "provider-title-id", Title: "My Manga: The Return", Metadata: source.MangaMetadata{Language: "ko"}}}}}, nil
}
func (p datasetProvider) Chapters(context.Context, *source.Manga) ([]*source.Chapter, error) {
	return []*source.Chapter{{ID: "provider-chapter-id", Index: "12.5", Language: "en", PageCount: 1}}, nil
}
func (p datasetProvider) Pages(context.Context, *source.Chapter) ([]*source.Page, error) {
	return []*source.Page{{URL: p.pageURL}}, nil
}

func TestCollectUsesReadableTitleAndChapterDirectories(t *testing.T) {
	imageBytes := newPNG(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageBytes)
	}))
	defer server.Close()
	root := t.TempDir()
	cfg := DefaultConfig(root, "fake")
	cfg.Sampling.MaxTitles = 1
	cfg.Sampling.MaxChaptersPerTitle = 1
	cfg.Discovery.CandidatePoolSize = 1
	cfg.Limits.MaxPages = 1
	cfg.Validation.MinimumWidth = 1
	cfg.Validation.MinimumHeight = 1
	cfg.Runtime.TitleWorkers = 1
	cfg.Runtime.ChapterWorkers = 1
	cfg.Runtime.PageWorkers = 1
	cfg.Runtime.ValidationWorkers = 1
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	appCfg := config.DefaultConfig()
	appCfg.Download.Dir = root
	service := Service{Store: store, Provider: datasetProvider{pageURL: server.URL}, Downloader: downloader.New(appCfg, server.Client())}
	if _, err := service.Collect(context.Background(), cfg, false); err != nil {
		t.Fatal(err)
	}
	chapter := filepath.Join(root, "data", "my-manga-the-return", "chapter-12.5")
	entries, err := os.ReadDir(chapter)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("chapter entries = %v", entries)
	}
	if entries[0].Name() == "chapter.json" || entries[0].Name() == "title.json" {
		t.Fatalf("metadata leaked into data: %s", entries[0].Name())
	}
	if _, err := os.Stat(filepath.Join(root, "data", "fake")); !os.IsNotExist(err) {
		t.Fatalf("provider/id layout exists: %v", err)
	}
	if result, err := Verify(context.Background(), store, false); err != nil || result["valid"] != true {
		t.Fatalf("verify = %#v, %v", result, err)
	}
}
func newPNG(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

var _ color.Color
