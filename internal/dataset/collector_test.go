package dataset

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
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
	return source.BrowsePage{Titles: []source.BrowseTitle{{Manga: &source.Manga{ID: "title", Title: "Title", Metadata: source.MangaMetadata{AlternativeTitle: "Alternate Title", Language: "ko", Status: "ongoing", Year: 2020}}, Tags: []string{"Drama"}, AvailableLanguages: []string{"en", "ko"}, CreatedAt: "2020-01-01T00:00:00Z", UpdatedAt: "2021-01-01T00:00:00Z"}}}, nil
}
func (p datasetProvider) Chapters(_ context.Context, _ *source.Manga) ([]*source.Chapter, error) {
	return []*source.Chapter{{ID: "chapter", Index: "1", Title: "One", Language: "en", PageCount: 1}}, nil
}
func (p datasetProvider) Pages(context.Context, *source.Chapter) ([]*source.Page, error) {
	return []*source.Page{{URL: p.pageURL}}, nil
}

var _ providers.Provider = datasetProvider{}
var _ providers.BrowseProvider = datasetProvider{}

type multiChapterDatasetProvider struct {
	datasetProvider
	chapters int
}

func (p multiChapterDatasetProvider) Chapters(_ context.Context, manga *source.Manga) ([]*source.Chapter, error) {
	chapters := make([]*source.Chapter, p.chapters)
	for index := range chapters {
		chapters[index] = &source.Chapter{ID: fmt.Sprintf("chapter-%d", index+1), Index: fmt.Sprintf("%d", index+1), Title: "Chapter", Language: "en", PageCount: 1, From: manga}
	}
	return chapters, nil
}

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
			var sourceMIMEType, extension string
			if err := store.db.QueryRow("SELECT source_mime_type,extension FROM pages WHERE chapter_id='chapter' AND page_index=1").Scan(&sourceMIMEType, &extension); err != nil {
				t.Fatal(err)
			}
			if sourceMIMEType != "image/png" {
				t.Fatalf("source MIME type = %q", sourceMIMEType)
			}
			if want := map[archive.Format]string{archive.FormatJPEG: ".jpeg"}[format]; want != "" && extension != want {
				t.Fatalf("extension = %q, want %q", extension, want)
			}
			if _, err := os.Stat(filepath.Join(root, "manifest.jsonl")); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(root, "summary.json")); err != nil {
				t.Fatal(err)
			}
			summaryData, err := os.ReadFile(filepath.Join(root, "summary.json"))
			if err != nil {
				t.Fatal(err)
			}
			var summary map[string]any
			if err := json.Unmarshal(summaryData, &summary); err != nil {
				t.Fatal(err)
			}
			if _, ok := summary["statistics"].(map[string]any); !ok {
				t.Fatalf("summary has no statistics: %#v", summary)
			}
			providerDir := filepath.Join(root, "data", "fake", "title")
			if _, err := os.Stat(filepath.Join(providerDir, "title.json")); err != nil {
				t.Fatal(err)
			}
			titleData, err := os.ReadFile(filepath.Join(providerDir, "title.json"))
			if err != nil {
				t.Fatal(err)
			}
			var titleMetadata struct {
				AlternativeTitle   string   `json:"alternativeTitle"`
				Tags               []string `json:"tags"`
				AvailableLanguages []string `json:"availableLanguages"`
				ProviderCreatedAt  string   `json:"providerCreatedAt"`
				ProviderUpdatedAt  string   `json:"providerUpdatedAt"`
			}
			if err := json.Unmarshal(titleData, &titleMetadata); err != nil {
				t.Fatal(err)
			}
			if titleMetadata.AlternativeTitle != "Alternate Title" || len(titleMetadata.Tags) != 1 || titleMetadata.Tags[0] != "Drama" || len(titleMetadata.AvailableLanguages) != 2 || titleMetadata.ProviderCreatedAt == "" || titleMetadata.ProviderUpdatedAt == "" {
				t.Fatalf("title metadata = %#v", titleMetadata)
			}
			chapterMetadata := filepath.Join(providerDir, "chapter", "chapter.json")
			if format.IsArchive() {
				chapterMetadata = filepath.Join(providerDir, "chapter"+format.Extension()+".json")
			}
			if _, err := os.Stat(chapterMetadata); err != nil {
				t.Fatal(err)
			}
			verification, err := Verify(context.Background(), store, false)
			if err != nil {
				t.Fatal(err)
			}
			if verification["valid"] != true {
				t.Fatalf("verification=%v", verification)
			}
			if format == archive.FormatDirectory {
				var split string
				if err := store.db.QueryRow("SELECT split FROM titles WHERE id='title'").Scan(&split); err != nil {
					t.Fatal(err)
				}
				if err := Export(context.Background(), store, ExportOptions{Split: split}); err != nil {
					t.Fatal(err)
				}
				verification, err = Verify(context.Background(), store, false)
				if err != nil {
					t.Fatal(err)
				}
				if verification["valid"] != true {
					t.Fatalf("filtered manifest verification=%v", verification)
				}
				if err := os.WriteFile(filepath.Join(root, "manifest.jsonl"), []byte("not-json\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				verification, err = Verify(context.Background(), store, false)
				if err != nil {
					t.Fatal(err)
				}
				if verification["manifestInconsistencies"] == 0 || verification["valid"] != false {
					t.Fatalf("corrupt manifest verification=%v", verification)
				}
			}
		})
	}
}

func TestCollectStopsAtActualByteLimit(t *testing.T) {
	imageBytes := newPNG(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageBytes)
	}))
	defer server.Close()
	root := t.TempDir()
	cfg := DefaultConfig(root, "fake")
	cfg.Sampling.MaxTitles, cfg.Sampling.MaxChaptersPerTitle, cfg.Discovery.CandidatePoolSize = 1, 1, 1
	cfg.Validation.MinimumWidth, cfg.Validation.MinimumHeight = 1, 1
	cfg.Limits.MaxPages, cfg.Limits.MaxBytes = 10, 1
	cfg.Runtime.PageWorkers, cfg.Runtime.ChapterWorkers, cfg.Runtime.TitleWorkers, cfg.Runtime.ValidationWorkers = 1, 1, 1, 1
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	appCfg := config.DefaultConfig()
	appCfg.Download.Dir = root
	appCfg.Concurrency.PageDownloads, appCfg.Concurrency.ChapterDownloads = 1, 1
	service := Service{Store: store, Provider: datasetProvider{pageURL: server.URL}, Downloader: downloader.New(appCfg, server.Client())}
	result, err := service.Collect(context.Background(), cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.StoppingReason != "max_bytes" || result.Counters.ValidPages != 0 {
		t.Fatalf("unexpected limit result: %#v", result)
	}
	if err := Export(context.Background(), store, ExportOptions{IncludeRejected: true}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "manifest.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) == "" {
		t.Fatal("rejected page was not exported")
	}
}

func TestCollectBoundsConcurrentChapterTransfers(t *testing.T) {
	imageBytes := newPNG(t)
	var active, maximum atomic.Int32
	started := make(chan struct{}, 3)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := active.Add(1)
		for {
			observed := maximum.Load()
			if current <= observed || maximum.CompareAndSwap(observed, current) {
				break
			}
		}
		started <- struct{}{}
		<-release
		active.Add(-1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageBytes)
	}))
	defer server.Close()

	root := t.TempDir()
	cfg := DefaultConfig(root, "fake")
	cfg.Sampling.MaxTitles, cfg.Sampling.MaxChaptersPerTitle, cfg.Discovery.CandidatePoolSize = 1, 3, 1
	cfg.Validation.MinimumWidth, cfg.Validation.MinimumHeight = 1, 1
	cfg.Limits.MaxPages = 2
	cfg.Runtime.PageWorkers, cfg.Runtime.ChapterWorkers, cfg.Runtime.TitleWorkers, cfg.Runtime.ValidationWorkers = 2, 2, 1, 1
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	appCfg := config.DefaultConfig()
	appCfg.Download.Dir = root
	appCfg.Concurrency.PageDownloads, appCfg.Concurrency.ChapterDownloads = 2, 2
	service := Service{Store: store, Provider: multiChapterDatasetProvider{datasetProvider: datasetProvider{pageURL: server.URL}, chapters: 3}, Downloader: downloader.New(appCfg, server.Client())}
	result := make(chan struct {
		value CollectResult
		err   error
	}, 1)
	go func() {
		value, err := service.Collect(context.Background(), cfg, false)
		result <- struct {
			value CollectResult
			err   error
		}{value, err}
	}()
	for range 2 {
		<-started
	}
	close(release)
	collected := <-result
	if collected.err != nil {
		t.Fatal(collected.err)
	}
	if maximum.Load() != 2 || collected.value.Counters.ValidPages != 2 || collected.value.StoppingReason != "max_pages" {
		t.Fatalf("maximum transfers=%d, result=%#v", maximum.Load(), collected.value)
	}
}

func TestVerifyRepairResetsAnEntireCorruptArchiveChapter(t *testing.T) {
	imageBytes := newPNG(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageBytes)
	}))
	defer server.Close()
	root := t.TempDir()
	cfg := DefaultConfig(root, "fake")
	cfg.Output.Format = archive.FormatCBZ
	cfg.Sampling.MaxTitles, cfg.Sampling.MaxChaptersPerTitle, cfg.Discovery.CandidatePoolSize = 1, 1, 1
	cfg.Validation.MinimumWidth, cfg.Validation.MinimumHeight = 1, 1
	cfg.Limits.MaxPages = 1
	cfg.Runtime.PageWorkers, cfg.Runtime.ChapterWorkers, cfg.Runtime.TitleWorkers, cfg.Runtime.ValidationWorkers = 1, 1, 1, 1
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	appCfg := config.DefaultConfig()
	appCfg.Download.Dir = root
	appCfg.Download.Format = string(archive.FormatCBZ)
	appCfg.Concurrency.PageDownloads, appCfg.Concurrency.ChapterDownloads = 1, 1
	service := Service{Store: store, Provider: datasetProvider{pageURL: server.URL}, Downloader: downloader.New(appCfg, server.Client())}
	if _, err := service.Collect(context.Background(), cfg, false); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(root, "data", "fake", "title", "chapter.cbz")
	if err := os.WriteFile(archivePath, []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Verify(context.Background(), store, true)
	if err != nil {
		t.Fatal(err)
	}
	if result["invalidPages"] != 1 || result["valid"] != false {
		t.Fatalf("verify result = %#v", result)
	}
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Fatalf("corrupt archive remained: %v", err)
	}
	var pageState, chapterState string
	if err := store.db.QueryRow("SELECT state FROM pages WHERE chapter_id='chapter' AND page_index=1").Scan(&pageState); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow("SELECT state FROM chapters WHERE id='chapter'").Scan(&chapterState); err != nil {
		t.Fatal(err)
	}
	if pageState != "pending" || chapterState != "partial" {
		t.Fatalf("states after repair: page=%q chapter=%q", pageState, chapterState)
	}
	if _, err := service.Collect(context.Background(), cfg, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("archive was not recreated: %v", err)
	}
}

func TestVerifyArchiveEntryRejectsWrongMetadataIdentity(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "0001.png"), newPNG(t), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "chapter.cbz")
	if _, err := archive.CreateFromDirectory(archive.Options{Format: archive.FormatCBZ, SourceDir: source, OutputPath: path, Metadata: archive.Metadata{Provider: "fake", TitleID: "title", ChapterID: "chapter", ExpectedPages: 1, SchemaVersion: "1", Completion: "complete"}}); err != nil {
		t.Fatal(err)
	}
	validation := Validation{MinimumWidth: 1, MinimumHeight: 1, MaximumWidth: 10, MaximumHeight: 10, MaximumDecodedPixels: 100, FullDecode: true, CalculateSHA256: true}
	if err := verifyArchiveEntry(path, "other-title", "other-chapter", "0001.png", "", validation); err == nil {
		t.Fatal("expected archive identity mismatch")
	}
}

func TestVerifyRepairAdoptsCompletedArchiveWithoutPageState(t *testing.T) {
	imageBytes := newPNG(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageBytes)
	}))
	defer server.Close()
	root := t.TempDir()
	cfg := DefaultConfig(root, "fake")
	cfg.Output.Format = archive.FormatZIP
	cfg.Sampling.MaxTitles, cfg.Sampling.MaxChaptersPerTitle, cfg.Discovery.CandidatePoolSize = 1, 1, 1
	cfg.Validation.MinimumWidth, cfg.Validation.MinimumHeight = 1, 1
	cfg.Limits.MaxPages = 1
	cfg.Runtime.PageWorkers, cfg.Runtime.ChapterWorkers, cfg.Runtime.TitleWorkers, cfg.Runtime.ValidationWorkers = 1, 1, 1, 1
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	appCfg := config.DefaultConfig()
	appCfg.Download.Dir = root
	appCfg.Download.Format = string(archive.FormatZIP)
	appCfg.Concurrency.PageDownloads, appCfg.Concurrency.ChapterDownloads = 1, 1
	service := Service{Store: store, Provider: datasetProvider{pageURL: server.URL}, Downloader: downloader.New(appCfg, server.Client())}
	if _, err := service.Collect(context.Background(), cfg, false); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 1 {
		t.Fatalf("requests after initial collection = %d", requests.Load())
	}
	if _, err := store.db.Exec("DELETE FROM pages; UPDATE chapters SET state='partial',output_path=NULL,archive_path=NULL; UPDATE titles SET state='partial'"); err != nil {
		t.Fatal(err)
	}
	result, err := Verify(context.Background(), store, true)
	if err != nil {
		t.Fatal(err)
	}
	if result["adoptedArchives"] != 1 || result["valid"] != true {
		t.Fatalf("verify result = %#v", result)
	}
	var pages int
	var chapterState string
	if err := store.db.QueryRow("SELECT COUNT(*) FROM pages WHERE state='valid'").Scan(&pages); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow("SELECT state FROM chapters WHERE id='chapter'").Scan(&chapterState); err != nil {
		t.Fatal(err)
	}
	if pages != 1 || chapterState != "completed" {
		t.Fatalf("adopted state: pages=%d chapter=%q", pages, chapterState)
	}
	if _, err := service.Collect(context.Background(), cfg, true); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 1 {
		t.Fatalf("resume redownloaded an adopted archive: %d requests", requests.Load())
	}
}

func TestCollectCancellationLeavesClaimResumable(t *testing.T) {
	started := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		<-r.Context().Done()
	}))
	defer server.Close()
	root := t.TempDir()
	cfg := DefaultConfig(root, "fake")
	cfg.Sampling.MaxTitles, cfg.Sampling.MaxChaptersPerTitle, cfg.Discovery.CandidatePoolSize = 1, 1, 1
	cfg.Validation.MinimumWidth, cfg.Validation.MinimumHeight = 1, 1
	cfg.Limits.MaxPages = 1
	cfg.Runtime.PageWorkers, cfg.Runtime.ChapterWorkers, cfg.Runtime.TitleWorkers, cfg.Runtime.ValidationWorkers = 1, 1, 1, 1
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	appCfg := config.DefaultConfig()
	appCfg.Download.Dir = root
	appCfg.Concurrency.PageDownloads, appCfg.Concurrency.ChapterDownloads = 1, 1
	service := Service{Store: store, Provider: datasetProvider{pageURL: server.URL}, Downloader: downloader.New(appCfg, server.Client())}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := service.Collect(ctx, cfg, false)
		result <- err
	}()
	<-started
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("collection error = %v, want context cancellation", err)
	}
	var runState, chapterState string
	if err := store.db.QueryRow("SELECT state FROM dataset_meta WHERE id=1").Scan(&runState); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow("SELECT state FROM chapters WHERE id='chapter'").Scan(&chapterState); err != nil {
		t.Fatal(err)
	}
	if runState != "interrupted" || chapterState != "downloading" {
		t.Fatalf("interrupted state: run=%q chapter=%q", runState, chapterState)
	}
	if _, err := Verify(context.Background(), store, true); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow("SELECT state FROM chapters WHERE id='chapter'").Scan(&chapterState); err != nil {
		t.Fatal(err)
	}
	if chapterState != "partial" {
		t.Fatalf("chapter was not made retryable: %q", chapterState)
	}
}

func TestValidateDownloadedPagesPreservesInputOrder(t *testing.T) {
	directory := t.TempDir()
	first := filepath.Join(directory, "0002.png")
	second := filepath.Join(directory, "0001.png")
	imageBytes := newPNG(t)
	if err := os.WriteFile(first, imageBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, imageBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	validated, err := validateDownloadedPages(context.Background(), Validation{MinimumWidth: 1, MinimumHeight: 1, MaximumWidth: 10, MaximumHeight: 10, MaximumDecodedPixels: 100, FullDecode: true, CalculateSHA256: true}, 2, []downloader.PageDownloadResult{{PageIndex: 2, Path: first}, {PageIndex: 1, Path: second}})
	if err != nil {
		t.Fatal(err)
	}
	if len(validated) != 2 || validated[0].result.PageIndex != 2 || validated[1].result.PageIndex != 1 {
		t.Fatalf("validation order = %#v", validated)
	}
	for _, page := range validated {
		if page.err != nil || page.image.SHA256 == "" {
			t.Fatalf("validation result = %#v", page)
		}
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
