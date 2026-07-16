package tui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/evgen2571/mangate/internal/usecase"
)

func TestProgressSummaryDetailUsesChapterProgressViews(t *testing.T) {
	tests := []struct {
		name     string
		chapters []chapterProgressView
		want     string
	}{
		{
			name: "one active with queued",
			chapters: []chapterProgressView{
				{Name: "Chapter 1", Active: true},
				{Name: "Chapter 2"},
				{Name: "Chapter 3", Completed: true},
			},
			want: "Now: Chapter 1 • queued: 1",
		},
		{
			name: "multiple active with queued",
			chapters: []chapterProgressView{
				{Name: "Chapter 1", Active: true},
				{Name: "Chapter 2", Active: true},
				{Name: "Chapter 3"},
			},
			want: "Active: 2 chapters • queued: 1",
		},
		{
			name: "one active",
			chapters: []chapterProgressView{
				{Name: "Chapter 1", Active: true},
				{Name: "Chapter 2", Completed: true},
			},
			want: "Now: Chapter 1",
		},
		{
			name: "queued only",
			chapters: []chapterProgressView{
				{Name: "Chapter 1"},
				{Name: "Chapter 2"},
			},
			want: "Queued: 2 chapters",
		},
		{
			name:     "finishing",
			chapters: []chapterProgressView{{Name: "Chapter 1", Completed: true}},
			want:     "Finishing download...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := progressSummaryDetail(tt.chapters)
			if got != tt.want {
				t.Fatalf("progressSummaryDetail() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestArchiveChaptersFinalizesCompletedDirectoriesAfterPartialDownload(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Download.Dir = t.TempDir()
	cfg.Download.Format = "cbz"
	a, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	manga := &source.Manga{ID: "title", Title: "Example"}
	chapters := []*source.Chapter{{ID: "one", Index: "1", PageCount: 1}, {ID: "two", Index: "2", PageCount: 2}}
	names := downloader.ChapterDirectoryNames(chapters)
	titleDir := filepath.Join(cfg.Download.Dir, downloader.TitleDirectoryName(manga))
	writeChapterStateForTUI(t, filepath.Join(titleDir, names[0]), 1, true)
	writeChapterStateForTUI(t, filepath.Join(titleDir, names[1]), 2, false)

	m := model{app: a}
	outcomes, err := m.archiveChapters(context.Background(), manga, chapters)
	if err != nil {
		t.Fatalf("archiveChapters() error = %v", err)
	}
	if outcomes[0].Status != "complete" || !strings.HasSuffix(outcomes[0].Path, ".cbz") || outcomes[1].Status != "incomplete" || !strings.HasSuffix(outcomes[1].Path, names[1]) {
		t.Fatalf("outcomes = %#v", outcomes)
	}
	if _, err := os.Stat(outcomes[0].Path); err != nil {
		t.Fatalf("completed chapter archive was not created: %v", err)
	}
	if _, err := os.Stat(outcomes[1].Path + ".cbz"); !os.IsNotExist(err) {
		t.Fatalf("incomplete chapter archive exists: %v", err)
	}
}

func writeChapterStateForTUI(t *testing.T, directory string, expectedPages int, complete bool) {
	t.Helper()
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "0001.jpg"), []byte{0xff, 0xd8, 0xff, 0xd9}, 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := json.Marshal(map[string]any{"expectedPages": expectedPages, "complete": complete})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, ".mangate.json"), state, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadProgressMsgFromUsecaseConvertsToTUIViewData(t *testing.T) {
	progress := usecase.DownloadProgress{
		CompletedChapters: 1,
		TotalChapters:     2,
		CompletedPages:    3,
		TotalPages:        10,
		Chapters: []usecase.ChapterDownloadProgress{
			{
				Name:           "Chapter 1",
				CompletedPages: 3,
				TotalPages:     5,
				Active:         true,
			},
			{
				Name:       "Chapter 2",
				TotalPages: 5,
			},
		},
	}

	msg := downloadProgressMsgFromUsecase(progress)

	if msg.Title != "Downloading pages" {
		t.Fatalf("Title = %q, want %q", msg.Title, "Downloading pages")
	}
	if msg.Detail != "Now: Chapter 1 • queued: 1" {
		t.Fatalf("Detail = %q, want %q", msg.Detail, "Now: Chapter 1 • queued: 1")
	}
	if msg.Status != "Downloaded 1/2 chapters" {
		t.Fatalf("Status = %q, want %q", msg.Status, "Downloaded 1/2 chapters")
	}
	if msg.Completed != 3 || msg.Total != 10 {
		t.Fatalf("Completed/Total = %d/%d, want 3/10", msg.Completed, msg.Total)
	}
	if len(msg.Chapters) != 2 {
		t.Fatalf("len(Chapters) = %d, want 2", len(msg.Chapters))
	}
	first := msg.Chapters[0]
	if first.Name != "Chapter 1" || first.CompletedPages != 3 || first.TotalPages != 5 || !first.Active || first.Completed {
		t.Fatalf("first chapter view = %+v, want active Chapter 1 with 3/5 pages", first)
	}
}
