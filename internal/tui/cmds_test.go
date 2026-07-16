package tui

import (
	"testing"

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
