package interactive

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/source"
)

func TestArchiveUsesNamesFromSelectedChapterSlice(t *testing.T) {
	allChapters := []*source.Chapter{
		{ID: "chapter-a", Index: "1"},
		{ID: "chapter-b", Index: "1"},
	}
	selected := allChapters[1:]

	paths := chapterDownloadPaths("/downloads", &source.Manga{ID: "title-id", Title: "Example"}, selected)
	if got, want := paths[0], filepath.Join("/downloads", "Example-title-id", "Chapter-1"); got != want {
		t.Fatalf("selected chapter path = %q, want %q", got, want)
	}
}

func TestDownloadCommandReturnsCompletionWithoutProgressConsumer(t *testing.T) {
	m := testModel(t)
	m.manga = &source.Manga{ID: "title", Title: "Title"}
	messages := make(chan tea.Msg)
	result := m.download(context.Background(), nil, messages)()
	if result == nil {
		t.Fatal("download command returned before its worker completed")
	}
	done, ok := result.(downloadDone)
	if !ok || done.err == nil {
		t.Fatalf("completion = %#v", result)
	}
}

func TestPartialDownloadReportsAndArchivesCompletedChapters(t *testing.T) {
	for _, format := range []archive.Format{archive.FormatDirectory, archive.FormatCBZ} {
		t.Run(string(format), func(t *testing.T) {
			var page bytes.Buffer
			if err := png.Encode(&page, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/at-home/server/good":
					fmt.Fprintf(w, `{"baseUrl":"http://%s","chapter":{"hash":"hash","data":["1.png"]}}`, r.Host)
				case "/data/hash/1.png":
					w.Header().Set("Content-Type", "image/png")
					w.Write(page.Bytes())
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			m := testModel(t)
			cfg := m.app.Cfg
			cfg.Download.Dir = t.TempDir()
			cfg.Download.Format = string(format)
			cfg.Providers.MangaDex.BaseURL = server.URL
			if err := m.app.ApplyConfig(cfg); err != nil {
				t.Fatal(err)
			}
			m.format = format
			m.integration = format == archive.FormatDirectory
			m.manga = &source.Manga{ID: "title", Title: "Fixture"}
			chapters := []*source.Chapter{{ID: "good", Index: "1", PageCount: 1}, {ID: "bad", Index: "2", PageCount: 1}}
			result := m.download(context.Background(), chapters, make(chan tea.Msg))().(downloadDone)
			if result.err == nil || result.completed != 1 || result.failed != 1 || len(result.paths) != 1 {
				t.Fatalf("result = %#v", result)
			}
			if format == archive.FormatDirectory && (len(result.results) != 2 || result.results[0].Status != "success" || len(result.results[0].ImagePaths) != 1 || result.results[1].Status != "failed" || len(result.results[1].ImagePaths) != 0) {
				t.Fatalf("integration chapters = %#v", result.results)
			}
			if format.IsArchive() {
				inspection, err := archive.Inspect(result.paths[0])
				if err != nil || !inspection.Complete || inspection.PageCount != 1 {
					t.Fatalf("archive = %#v, %v", inspection, err)
				}
			} else if _, err := os.Stat(filepath.Join(result.paths[0], "0001.png")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTUIReusesCompleteArchivesWithoutNetworkOrSourceDirectories(t *testing.T) {
	m := testModel(t)
	cfg := m.app.Cfg
	cfg.Download.Dir = t.TempDir()
	cfg.Download.Format = "cbz"
	cfg.Providers.MangaDex.BaseURL = "http://127.0.0.1:1"
	if err := m.app.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	m.format = archive.FormatCBZ
	m.manga = &source.Manga{ID: "title", Title: "Fixture"}
	chapters := []*source.Chapter{{ID: "chapter", Index: "1", PageCount: 1}}
	directory := chapterDownloadPaths(cfg.Download.Dir, m.manga, chapters)[0]
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	var page bytes.Buffer
	if err := png.Encode(&page, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "0001.png"), page.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := archive.CreateFromDirectoryContext(context.Background(), archive.Options{Format: archive.FormatCBZ, SourceDir: directory, OutputPath: directory + ".cbz", RemoveSource: true, Metadata: archive.Metadata{TitleID: "title", ChapterID: "chapter", ExpectedPages: 1}})
	if err != nil {
		t.Fatal(err)
	}
	result := m.download(context.Background(), chapters, make(chan tea.Msg))().(downloadDone)
	if result.err != nil || result.skipped != 1 || result.failed != 0 {
		t.Fatalf("reuse = %#v", result)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("source directory recreated: %v", err)
	}
	chapters = append(chapters, &source.Chapter{ID: "new-release", Index: "1", PageCount: 1})
	mixed := m.download(context.Background(), chapters, make(chan tea.Msg))().(downloadDone)
	if mixed.err == nil || mixed.skipped != 1 || mixed.failed != 1 || mixed.completed != 0 {
		t.Fatalf("mixed reuse and failed download = %#v", mixed)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("mixed selection recreated reused source directory: %v", err)
	}
}
