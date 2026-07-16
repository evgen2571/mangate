package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

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
