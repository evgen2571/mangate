package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/spf13/cobra"
)

func TestSelectChaptersUsesNumericRangeAndStableIDs(t *testing.T) {
	chapters := []*source.Chapter{
		{ID: "one", Index: "1"},
		{ID: "two", Index: "2"},
		{ID: "ten", Index: "10"},
	}
	selected, err := selectChapters(chapters, chapterSelection{Range: "2-10"})
	if err != nil {
		t.Fatalf("selectChapters() error = %v", err)
	}
	if len(selected) != 2 || selected[0].ID != "two" || selected[1].ID != "ten" {
		t.Fatalf("selected = %#v, want chapters two and ten", selected)
	}

	selected, err = selectChapters(chapters, chapterSelection{IDs: []string{"ten"}})
	if err != nil || len(selected) != 1 || selected[0].ID != "ten" {
		t.Fatalf("stable ID selection = %#v, %v", selected, err)
	}
}

func TestReportDownloadResultWritesPartialJSONAndPreservesExitCode(t *testing.T) {
	var output bytes.Buffer
	root := newJSONTestCommand(&output)
	record := downloadRecord{Chapters: []chapterDownload{{ID: "complete", Status: "complete"}, {ID: "failed", Status: "incomplete"}}}
	err := reportDownloadResult(root, &record, errors.New("download page failed"))
	var reported *ReportedError
	if !errors.As(err, &reported) || reported.Code != 5 {
		t.Fatalf("error = %#v, want partial reported error", err)
	}
	var response envelope
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatalf("JSON output = %q: %v", output.String(), err)
	}
	if response.Status != "partial" || record.Status != "partial" {
		t.Fatalf("status = %q, record = %#v", response.Status, record)
	}
}

func newJSONTestCommand(output *bytes.Buffer) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetOut(output)
	cmd.Flags().Bool("json", true, "")
	return cmd
}

func TestReportDownloadResultUsesArchiveExitCodeWhenNothingCompletes(t *testing.T) {
	root := newJSONTestCommand(&bytes.Buffer{})
	record := downloadRecord{Chapters: []chapterDownload{{ID: "failed", Status: "archive_failed"}}}
	err := reportDownloadResult(root, &record, errors.New("create archive: validation failed"))
	var reported *ReportedError
	if !errors.As(err, &reported) || reported.Code != 8 {
		t.Fatalf("error = %#v, want archive reported error", err)
	}
}

func TestWriteDownloadSummaryDistinguishesOutcomesAndPaths(t *testing.T) {
	var output bytes.Buffer
	writeDownloadSummary(&output, downloadRecord{Chapters: []chapterDownload{
		{ID: "completed", Status: "complete", OutputPath: "pages", ExpectedPages: 2},
		{ID: "reused", Status: "skipped", OutputPath: "source", ArchivePath: "archive.cbz", ExpectedPages: 3},
		{ID: "failed", Status: "archive_failed", OutputPath: "failed-pages", ArchivePath: "failed.cbz", ExpectedPages: 4},
	}})
	for _, want := range []string{"Completed: 1", "Skipped/reused: 1", "Failed or incomplete: 1", "Archive failures: 1", "Expected pages: 9", "Reused pages: 3", "[skipped] archive.cbz", "[archive_failed] failed.cbz"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("summary = %q, want %q", output.String(), want)
		}
	}
}

func TestDownloadConfirmationRequirementOnlyGatesBroadOrDestructiveOperations(t *testing.T) {
	chapters := make([]*source.Chapter, broadDownloadChapterThreshold)
	for index := range chapters {
		chapters[index] = &source.Chapter{ID: strconv.Itoa(index)}
	}
	tests := []struct {
		name         string
		chapters     []*source.Chapter
		format       archive.Format
		existingMode string
		want         string
	}{
		{name: "single directory", chapters: chapters[:1], format: archive.FormatDirectory, existingMode: "skip"},
		{name: "broad", chapters: chapters, format: archive.FormatDirectory, existingMode: "skip", want: "broad operation"},
		{name: "replace", chapters: chapters[:1], format: archive.FormatCBZ, existingMode: "replace", want: "replacing"},
		{name: "archive removes temporary source", chapters: chapters[:1], format: archive.FormatCBZ, existingMode: "skip", want: "removing temporary"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := downloadConfirmationRequirement(test.chapters, test.format, test.existingMode)
			if test.want == "" && got != "" {
				t.Fatalf("requirement = %q, want none", got)
			}
			if test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("requirement = %q, want %q", got, test.want)
			}
		})
	}
}

func TestWriteDownloadPreflightShowsEffectiveOperation(t *testing.T) {
	var output bytes.Buffer
	writeDownloadPreflight(&output, &source.Manga{Title: "Example"}, "provider", []*source.Chapter{{ID: "one"}}, archive.FormatCBZ, "./library", "skip")
	for _, want := range []string{"Title: Example", "Provider: provider", "Chapters: 1 selected", "Format: cbz", "Source pages: removed after archive validation"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("preflight = %q, want %q", output.String(), want)
		}
	}
}

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

func TestReusableArchiveSelectionSkipsValidatedMatchingArchives(t *testing.T) {
	directory := t.TempDir()
	archivePath := filepath.Join(directory, "chapter.cbz")
	result, err := archive.CreateFromDirectory(archive.Options{Format: archive.FormatCBZ, SourceDir: writeArchiveSource(t), OutputPath: archivePath, Metadata: archive.Metadata{TitleID: "title", ChapterID: "chapter"}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Validation.Complete {
		t.Fatal("test archive is incomplete")
	}
	record := downloadRecord{Chapters: []chapterDownload{{ID: "chapter", ArchivePath: archivePath, Status: "pending"}}}
	selection := []*source.Chapter{{ID: "chapter"}}
	pending, err := reusableArchiveSelection(&record, selection, &source.Manga{ID: "title"})
	if err != nil || len(pending) != 0 || record.Chapters[0].Status != "skipped" || record.Chapters[0].Validation == nil {
		t.Fatalf("pending = %#v, record = %#v", pending, record)
	}
}

func TestReusableArchiveSelectionRejectsMismatchedExistingArchive(t *testing.T) {
	directory := t.TempDir()
	archivePath := filepath.Join(directory, "chapter.cbz")
	if _, err := archive.CreateFromDirectory(archive.Options{Format: archive.FormatCBZ, SourceDir: writeArchiveSource(t), OutputPath: archivePath, Metadata: archive.Metadata{TitleID: "other-title", ChapterID: "other-chapter"}}); err != nil {
		t.Fatal(err)
	}
	record := downloadRecord{Chapters: []chapterDownload{{ID: "chapter", ArchivePath: archivePath, Status: "pending"}}}
	_, err := reusableArchiveSelection(&record, []*source.Chapter{{ID: "chapter"}}, &source.Manga{ID: "title"})
	if err == nil || !strings.Contains(err.Error(), "different chapter") {
		t.Fatalf("reusableArchiveSelection() error = %v, want collision", err)
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

func TestWriteHumanEscapesTerminalControlText(t *testing.T) {
	var output bytes.Buffer
	writeHuman(&output, "Title: %s\n", "Bad\x1b[2J\nTitle")
	if strings.Contains(output.String(), "\x1b") || strings.Contains(output.String(), "\nTitle\n") {
		t.Fatalf("writeHuman() output = %q, want control text escaped", output.String())
	}
}

func TestSelectChaptersRejectsAmbiguousNumber(t *testing.T) {
	_, err := selectChapters([]*source.Chapter{{ID: "a", Index: "1"}, {ID: "b", Index: "1"}}, chapterSelection{Numbers: []string{"1"}})
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("selectChapters() error = %v, want ambiguity error", err)
	}
}

func TestSelectChaptersRejectsConflictingSelectorsBeforeMatching(t *testing.T) {
	tests := []chapterSelection{
		{Latest: true, All: true},
		{All: true, IDs: []string{"chapter"}},
		{Numbers: []string{"1"}, Range: "1-2"},
	}
	for _, selection := range tests {
		_, err := selectChapters(nil, selection)
		if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
			t.Fatalf("selectChapters(%#v) error = %v, want conflict", selection, err)
		}
	}
}
