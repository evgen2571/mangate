package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evgen2571/mangate/internal/tuiapp"
)

func TestSearchSubmittedUsesTUIAppServiceResultsAndHistory(t *testing.T) {
	svc := fakeTUIService{
		searchResults: []tuiapp.SearchResult{{
			ID:           "manga-a",
			Title:        "Manga A",
			URL:          "https://example.com/manga-a",
			SummaryMD:    "A test summary.",
			ChapterCount: 5,
		}},
		history: []string{"query"},
	}
	m := model{svc: svc}

	updated, cmd := m.Update(searchSubmittedMsg{Query: "query"})
	gotModel, ok := updated.(model)
	if !ok {
		t.Fatalf("Update() returned %T, want model", updated)
	}
	if gotModel.state != stateLoading {
		t.Fatalf("state = %v, want stateLoading", gotModel.state)
	}
	if cmd == nil {
		t.Fatal("Update() command = nil, want search command")
	}

	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		msg = nil
		for _, batchCmd := range batch {
			if batchCmd == nil {
				continue
			}
			if candidate, ok := batchCmd().(searchSucceededMsg); ok {
				msg = candidate
				break
			}
		}
	}

	got, ok := msg.(searchSucceededMsg)
	if !ok {
		t.Fatalf("command returned %T, want searchSucceededMsg", msg)
	}
	if got.Query != "query" {
		t.Fatalf("Query = %q, want %q", got.Query, "query")
	}
	if len(got.Results) != 1 || got.Results[0].ID != "manga-a" || got.Results[0].SummaryMD != "A test summary." {
		t.Fatalf("Results = %#v, want mapped tuiapp results", got.Results)
	}
	if len(got.History) != 1 || got.History[0] != "query" {
		t.Fatalf("History = %#v, want updated service history", got.History)
	}
}

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

func TestDownloadProgressMsgFromTUIAppConvertsToTUIViewData(t *testing.T) {
	progress := tuiapp.DownloadProgress{
		CompletedChapters: 1,
		TotalChapters:     2,
		CompletedPages:    3,
		TotalPages:        10,
		Chapters: []tuiapp.ChapterProgress{
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

	msg := downloadProgressMsgFromTUIApp(progress)

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
