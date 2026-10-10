package acquisition

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/providers/mangadex"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/evgen2571/mangate/internal/testprovider"
)

func fixtureService(t testing.TB, format string) (Service, *testprovider.Fixture) {
	t.Helper()
	f := testprovider.New(t)
	cfg := config.DefaultConfig()
	cfg.Download.Dir = t.TempDir()
	cfg.Download.Format = format
	cfg.Concurrency.PageDownloads = 1
	cfg.Providers.MangaDex.BaseURL = f.URL
	provider, err := mangadex.New(cfg, f.Client())
	if err != nil {
		t.Fatal(err)
	}
	return Service{Config: cfg, Provider: provider, Downloader: downloader.New(cfg, f.Client())}, f
}

func selected(t testing.TB, s Service) (*source.Manga, []*source.Chapter) {
	t.Helper()
	manga, err := s.Provider.Title(context.Background(), "title-id")
	if err != nil {
		t.Fatal(err)
	}
	chapters, err := s.Provider.Chapters(context.Background(), manga)
	if err != nil {
		t.Fatal(err)
	}
	return manga, chapters
}

func TestExecutionDestinationsShareRecoveryPolicies(t *testing.T) {
	for _, exact := range []bool{false, true} {
		for _, format := range []string{"directory", "png", "jpeg"} {
			t.Run(format+map[bool]string{false: "/ordinary", true: "/exact"}[exact], func(t *testing.T) {
				s, f := fixtureService(t, format)
				manga, chapters := selected(t, s)
				destination := ""
				if exact {
					destination = filepath.Join(s.Config.Download.Dir, "exact")
				}
				f.FailedPage.Store(2)
				partial, err := s.Execute(context.Background(), manga, chapters, destination, nil)
				if err == nil || partial.Status != "partial" || len(partial.Chapters[0].ImagePaths) != 1 {
					t.Fatalf("partial=%#v err=%v", partial, err)
				}
				if len(chapters[0].Pages) != 0 {
					t.Fatal("mutated selected chapter")
				}
				f.FailedPage.Store(0)
				calls := f.ImageCalls.Load()
				recovered, err := s.Execute(context.Background(), manga, chapters, destination, nil)
				if err != nil || recovered.Status != "complete" || len(recovered.Chapters[0].ImagePaths) != 2 {
					t.Fatalf("recovered=%#v err=%v", recovered, err)
				}
				if f.ImageCalls.Load()-calls != 1 {
					t.Fatal("downloaded recovered page again")
				}
				metadata := filepath.Join(recovered.Chapters[0].OutputPath, ".mangate.json")
				before, err := os.ReadFile(metadata)
				if err != nil {
					t.Fatal(err)
				}
				s.Config.Download.ExistingFileMode = "fail"
				s.Downloader = downloader.New(s.Config, f.Client())
				rejected, err := s.Execute(context.Background(), manga, chapters, destination, nil)
				if err == nil || rejected.Status != "failed" || len(rejected.Chapters[0].ImagePaths) != 0 {
					t.Fatalf("rejected=%#v err=%v", rejected, err)
				}
				after, err := os.ReadFile(metadata)
				if err != nil || string(before) != string(after) {
					t.Fatal("preflight rejection changed chapter state")
				}
			})
		}
	}
}

func TestExecutionCancellationRetainsPartialResults(t *testing.T) {
	for _, exact := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "exact"}[exact], func(t *testing.T) {
			s, _ := fixtureService(t, "directory")
			manga, chapters := selected(t, s)
			destination := ""
			if exact {
				destination = filepath.Join(s.Config.Download.Dir, "exact")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result, err := s.Execute(ctx, manga, chapters, destination, func(p downloader.DownloadProgress) {
				if p.CompletedPages == 1 {
					cancel()
				}
			})
			if !errors.Is(err, context.Canceled) || result.Status != "cancelled" || len(result.Chapters[0].ImagePaths) != 1 {
				t.Fatalf("result=%#v err=%v", result, err)
			}
		})
	}
}

func TestPlanReplaySharesTransferAndValidatesManifest(t *testing.T) {
	s, f := fixtureService(t, "jpeg")
	plan, err := PlanRandom(context.Background(), s.Provider, SelectionPolicy{Provider: "mangadex", MinPages: 2, MaxAttempts: 1}, filepath.Join(s.Config.Download.Dir, "exact"), archive.FormatJPEG, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.ExpiredPage.Store(2)
	var completed []int
	result, err := s.ExecutePlan(context.Background(), *plan, func(p ProgressEvent) {
		if p.Status == "progress" {
			completed = append(completed, p.CompletedPages)
		}
	})
	if err != nil || result.Status != "complete" || result.Manifest == nil {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if !reflect.DeepEqual(completed, []int{1, 2}) {
		t.Fatalf("progress=%v", completed)
	}
	if err := result.Manifest.Verify(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Config.Download.Dir, "manifest.json")
	if err := result.Manifest.Write(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := loaded.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentChaptersKeepNamesWhenArchiveIsReused(t *testing.T) {
	s, f := fixtureService(t, "cbz")
	manga, chapters := selected(t, s)
	other := *chapters[0]
	other.ID = "chapter-other"
	chapters = append(chapters, &other)
	// Initially download only the first release; a larger selection must retain its name.
	first, err := s.Execute(context.Background(), manga, chapters[:1], "", nil)
	if err != nil {
		t.Fatal(err)
	}
	calls := f.ImageCalls.Load()
	var last downloader.DownloadProgress
	result, err := s.Execute(context.Background(), manga, chapters, "", func(p downloader.DownloadProgress) {
		if p.CompletedPages < last.CompletedPages || p.CompletedPages > p.TotalPages {
			t.Errorf("non-monotonic progress=%#v last=%#v", p, last)
		}
		last = p
	})
	if err != nil || result.Status != "complete" || result.Chapters[0].Status != "skipped" || result.Chapters[1].Status != "complete" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if result.Chapters[0].ArchivePath != first.Chapters[0].ArchivePath || result.Chapters[0].ArchivePath == result.Chapters[1].ArchivePath {
		t.Fatal("selection changed archive destinations")
	}
	if f.ImageCalls.Load()-calls != 2 || last.CompletedChapters != 2 || last.CompletedPages != 4 {
		t.Fatalf("calls=%d progress=%#v", f.ImageCalls.Load()-calls, last)
	}
	for _, chapter := range result.Chapters {
		inspection, err := archive.Inspect(chapter.ArchivePath)
		if err != nil || !inspection.Complete || inspection.ChapterID != chapter.ID {
			t.Fatalf("inspection=%#v err=%v", inspection, err)
		}
		if _, err := os.Stat(chapter.OutputPath); !os.IsNotExist(err) {
			t.Fatal("archive source retained")
		}
	}
}

func BenchmarkExecute(b *testing.B) {
	for _, exact := range []bool{false, true} {
		b.Run(map[bool]string{false: "ordinary", true: "exact"}[exact], func(b *testing.B) {
			s, f := fixtureService(b, "directory")
			manga, chapters := selected(b, s)
			s.Config.Provider = "fixture"
			s.Provider = &fixtureProvider{manga: manga, chapters: chapters, pages: []*source.Page{{URL: f.URL + "/data/hash/1.png"}, {URL: f.URL + "/data/hash/2.png"}}}
			s.Config.Download.ExistingFileMode = "replace"
			s.Downloader = downloader.New(s.Config, f.Client())
			destination := ""
			if exact {
				destination = filepath.Join(s.Config.Download.Dir, "exact")
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := s.Execute(context.Background(), manga, chapters, destination, nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestExecutePlanRejectsInvalidSelectionBeforeBuildingResults(t *testing.T) {
	s, f := fixtureService(t, "directory")
	_, err := s.ExecutePlan(context.Background(), DownloadPlan{}, nil)
	if err == nil || f.ImageCalls.Load() != 0 {
		t.Fatalf("err=%v image calls=%d", err, f.ImageCalls.Load())
	}
}

func TestPlanReplayPreservesPartialChapterStatus(t *testing.T) {
	s, f := fixtureService(t, "directory")
	plan, err := PlanRandom(context.Background(), s.Provider, SelectionPolicy{Provider: "mangadex", MinPages: 2, MaxAttempts: 1}, filepath.Join(s.Config.Download.Dir, "exact"), archive.FormatDirectory, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.FailedPage.Store(2)
	result, err := s.ExecutePlan(context.Background(), *plan, nil)
	if err == nil || result.Status != "partial" || result.Chapters[0].Status != "partial" || result.Manifest.Status != "partial" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}
