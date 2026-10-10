package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/evgen2571/mangate/internal/acquisition"
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
	record := acquisition.Result{Chapters: []acquisition.ChapterResult{{ID: "complete", Status: "complete"}, {ID: "failed", Status: "incomplete"}}}
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
	record := acquisition.Result{Chapters: []acquisition.ChapterResult{{ID: "failed", Status: "archive_failed"}}}
	err := reportDownloadResult(root, &record, errors.New("create archive: validation failed"))
	var reported *ReportedError
	if !errors.As(err, &reported) || reported.Code != 8 {
		t.Fatalf("error = %#v, want archive reported error", err)
	}
}

func TestWriteDownloadSummaryDistinguishesOutcomesAndPaths(t *testing.T) {
	var output bytes.Buffer
	writeDownloadSummary(&output, acquisition.Result{Chapters: []acquisition.ChapterResult{
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

func TestReportDownloadCancellationHasDedicatedStatus(t *testing.T) {
	record := acquisition.Result{Chapters: []acquisition.ChapterResult{{ID: "chapter", Status: "incomplete"}}}
	err := reportDownloadResult(newJSONTestCommand(&bytes.Buffer{}), &record, context.Canceled)
	var reported *ReportedError
	if !errors.As(err, &reported) || reported.Code != 7 || record.Status != "cancelled" || record.Chapters[0].Status != "cancelled" {
		t.Fatalf("record = %#v, error = %v", record, err)
	}
}
