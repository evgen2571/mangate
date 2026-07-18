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

	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/providers"
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
	return source.BrowsePage{Titles: []source.BrowseTitle{{Manga: &source.Manga{ID: "title", Title: "Title", Metadata: source.MangaMetadata{Language: "ko", Status: "ongoing", Year: 2020}}}}}, nil
}
func (p datasetProvider) Chapters(_ context.Context, manga *source.Manga) ([]*source.Chapter, error) {
	return []*source.Chapter{{ID: "chapter", Index: "1", Title: "One", Language: "en", PageCount: 1, From: manga}}, nil
}
func (p datasetProvider) Pages(context.Context, *source.Chapter) ([]*source.Page, error) {
	return []*source.Page{{URL: p.pageURL}}, nil
}

var _ providers.Provider = datasetProvider{}
var _ providers.BrowseProvider = datasetProvider{}

func TestCollectsEveryFormatAndExportsManifest(t *testing.T) {
	imageBytes := newPNG(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageBytes)
	}))
	defer server.Close()
	for _, format := range []archive.Format{archive.FormatDirectory, archive.FormatPNG, archive.FormatJPEG, archive.FormatCBZ, archive.FormatZIP} {
		t.Run(string(format), func(t *testing.T) {
			root := t.TempDir()
			cfg := DefaultConfig(root, "fake")
			cfg.Output.Format = format
			cfg.Sampling.MaxTitles = 1
			cfg.Sampling.MaxChaptersPerTitle = 1
			cfg.Discovery.CandidatePoolSize = 1
			cfg.Validation.MinimumWidth = 1
			cfg.Validation.MinimumHeight = 1
			cfg.Limits.MaxPages = 1
			cfg.Runtime.PageWorkers = 1
			cfg.Runtime.ChapterWorkers = 1
			cfg.Runtime.TitleWorkers = 1
			cfg.Runtime.ValidationWorkers = 1
			store, err := Open(root)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			appCfg := config.DefaultConfig()
			appCfg.Download.Dir = root
			appCfg.Download.Format = string(format)
			appCfg.Concurrency.PageDownloads = 1
			appCfg.Concurrency.ChapterDownloads = 1
			service := Service{Store: store, Provider: datasetProvider{pageURL: server.URL}, Downloader: downloader.New(appCfg, server.Client())}
			result, err := service.Collect(context.Background(), cfg, false)
			if err != nil {
				t.Fatal(err)
			}
			if result.Counters.ValidPages != 1 {
				t.Fatalf("valid pages=%d", result.Counters.ValidPages)
			}
			if _, err := os.Stat(filepath.Join(root, "manifest.jsonl")); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(root, "summary.json")); err != nil {
				t.Fatal(err)
			}
			verification, err := Verify(context.Background(), store, false)
			if err != nil {
				t.Fatal(err)
			}
			if verification["valid"] != true {
				t.Fatalf("verification=%v", verification)
			}
		})
	}
}
func newPNG(t *testing.T) []byte {
	t.Helper()
	var data bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}
