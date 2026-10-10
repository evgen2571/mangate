package acquisition

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/source"
)

func TestChapterRecordsUseDownloaderChapterDirectoryNames(t *testing.T) {
	title := &source.Manga{ID: "title-id", Title: "Example"}
	chapters := []*source.Chapter{
		{ID: "chapter-a", Title: "Special"},
		{ID: "chapter-b", Index: "1"},
		{ID: "chapter-c", Index: "1"},
	}

	records := chapterRecords(t.TempDir(), title, chapters, archive.FormatCBZ, "pending")
	for index, name := range []string{"Title-Special", "Chapter-1", "Chapter-1-chapter-c"} {
		if filepath.Base(records[index].OutputPath) != name {
			t.Fatalf("chapter %d output path = %q, want directory %q", index, records[index].OutputPath, name)
		}
		if records[index].ArchivePath != records[index].OutputPath+".cbz" {
			t.Fatalf("chapter %d archive path = %q, want source path with .cbz", index, records[index].ArchivePath)
		}
	}
}

func TestMarkReusableArchivesSkipsValidatedMatchingArchives(t *testing.T) {
	directory := t.TempDir()
	archivePath := filepath.Join(directory, "chapter.cbz")
	result, err := archive.CreateFromDirectory(archive.Options{Format: archive.FormatCBZ, SourceDir: writeArchiveSource(t), OutputPath: archivePath, Metadata: archive.Metadata{TitleID: "title", ChapterID: "chapter"}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Validation.Complete {
		t.Fatal("test archive is incomplete")
	}
	record := Result{Title: &source.Manga{ID: "title"}, Chapters: []ChapterResult{{ID: "chapter", ArchivePath: archivePath, Status: "pending"}}}
	err = markReusableArchives(&record)
	if err != nil || record.Chapters[0].Status != "skipped" || record.Chapters[0].Validation == nil {
		t.Fatalf("record = %#v, error = %v", record, err)
	}
}

func TestMarkReusableArchivesRejectsMismatchedExistingArchive(t *testing.T) {
	directory := t.TempDir()
	archivePath := filepath.Join(directory, "chapter.cbz")
	if _, err := archive.CreateFromDirectory(archive.Options{Format: archive.FormatCBZ, SourceDir: writeArchiveSource(t), OutputPath: archivePath, Metadata: archive.Metadata{TitleID: "other-title", ChapterID: "other-chapter"}}); err != nil {
		t.Fatal(err)
	}
	record := Result{Title: &source.Manga{ID: "title"}, Chapters: []ChapterResult{{ID: "chapter", ArchivePath: archivePath, Status: "pending"}}}
	err := markReusableArchives(&record)
	if err == nil || !strings.Contains(err.Error(), "different chapter") {
		t.Fatalf("markReusableArchives() error = %v, want collision", err)
	}
}

func TestChapterRecordDoesNotTrustCompletionWithoutImages(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, ".mangate.json"), []byte(`{"complete":true,"expectedPages":2}`), 0644); err != nil {
		t.Fatal(err)
	}
	record := Result{Chapters: []ChapterResult{{ID: "chapter", OutputPath: directory}}}
	if err := updateChapterRecordStates(&record); err == nil || record.Chapters[0].Status == "complete" {
		t.Fatalf("record = %#v, error = %v", record, err)
	}
}

func TestChapterRecordKeepsOrderedPathsAfterPartialFailure(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, ".mangate.json"), []byte(`{"complete":false,"expectedPages":3}`), 0644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"0001.png", "0003.png"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("image"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	record := Result{Chapters: []ChapterResult{{ID: "chapter", OutputPath: directory}}}
	if err := updateChapterRecordStates(&record); err == nil {
		t.Fatal("expected partial acquisition error")
	}
	paths := record.Chapters[0].ImagePaths
	if len(paths) != 2 || filepath.Base(paths[0]) != "0001.png" || filepath.Base(paths[1]) != "0003.png" || record.Chapters[0].Status == "complete" || record.Chapters[0].ExpectedPages != 3 {
		t.Fatalf("record = %#v", record)
	}
}

func TestIncompleteStateIsNeverSuccessful(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, ".mangate.json"), []byte(`{"complete":false,"expectedPages":2}`), 0644); err != nil {
		t.Fatal(err)
	}
	record := Result{Chapters: []ChapterResult{{ID: "chapter", OutputPath: directory}}}
	if err := updateChapterRecordStates(&record); err == nil {
		t.Fatal("incomplete chapter must produce an acquisition error")
	}
}

func TestChapterRecordRejectsForeignCompletionState(t *testing.T) {
	directory := t.TempDir()
	state := []byte(`{"complete":true,"expectedPages":1,"titleId":"title","chapterId":"other","provider":"mangadex"}`)
	if err := os.WriteFile(filepath.Join(directory, ".mangate.json"), state, 0644); err != nil {
		t.Fatal(err)
	}
	record := Result{Provider: "mangadex", Title: &source.Manga{ID: "title"}, Chapters: []ChapterResult{{ID: "chapter", OutputPath: directory, ImagePaths: []string{"old"}}}}
	if err := updateChapterRecordStates(&record); err == nil {
		t.Fatal("expected identity rejection")
	}
	if record.Chapters[0].Status == "complete" || len(record.Chapters[0].ImagePaths) != 0 {
		t.Fatalf("foreign files exposed: %#v", record)
	}
}

func writeArchiveSource(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "0001.jpg"), []byte{0xff, 0xd8, 0xff, 0xd9}, 0o644); err != nil {
		t.Fatal(err)
	}
	return directory
}

func TestBuildDownloadMangaLeavesPagesForLazyLoading(t *testing.T) {
	manga := &source.Manga{ID: "manga-id", Title: "My Manga"}
	chapter := &source.Chapter{
		ID:        "chapter-1",
		Index:     "1",
		Title:     "Intro",
		PageCount: 23,
		Pages:     []*source.Page{{URL: "https://example.com/page.png"}},
	}

	downloadManga, err := buildDownloadManga(manga, []*source.Chapter{chapter})
	if err != nil {
		t.Fatalf("buildDownloadManga() error = %v", err)
	}
	if len(downloadManga.Chapters) != 1 {
		t.Fatalf("len(downloadManga.Chapters) = %d, want 1", len(downloadManga.Chapters))
	}

	downloadChapter := downloadManga.Chapters[0]
	if len(downloadChapter.Pages) != 0 {
		t.Fatalf("len(downloadChapter.Pages) = %d, want 0 for lazy loading", len(downloadChapter.Pages))
	}
	if downloadChapter.PageCount != 23 {
		t.Fatalf("download chapter PageCount = %d, want 23", downloadChapter.PageCount)
	}
	if downloadChapter == chapter {
		t.Fatalf("download chapter reuses original pointer")
	}
	if downloadChapter.From != downloadManga {
		t.Fatalf("download chapter parent = %p, want download manga %p", downloadChapter.From, downloadManga)
	}
	if len(chapter.Pages) != 1 {
		t.Fatalf("original chapter pages were mutated")
	}
}
