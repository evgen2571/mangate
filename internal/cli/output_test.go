package cli

import (
	"bytes"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/dataset"
)

func TestErrorDiagnosticUsesStableCategoryAndExitCode(t *testing.T) {
	if got, want := ErrorDiagnostic(errors.New("title cannot be empty")), "error category: invalid_input; exit code: 2"; got != want {
		t.Fatalf("ErrorDiagnostic() = %q, want %q", got, want)
	}
}

func TestEffectiveDatasetConfigLetsExplicitFalseOverrideFile(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "collection.json")
	if err := os.WriteFile(configPath, []byte(`{"version":1,"datasetId":"test","provider":"mangadex","output":{"directory":"`+filepath.Join(root, "data")+`","format":"directory","existingFiles":"skip"},"discovery":{"candidatePoolSize":1,"orderDirection":"desc"},"sampling":{"maxTitles":1,"keepDuplicateChapterReleases":true},"limits":{},"validation":{"minimumWidth":1,"minimumHeight":1,"maximumWidth":2,"maximumHeight":2,"maximumDecodedPixels":4},"splits":{},"runtime":{"titleWorkers":1,"chapterWorkers":1,"pageWorkers":1,"validationWorkers":1,"retryLimit":0}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := app.New(config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	cmd := newDatasetCollectCmd(a)
	if err := cmd.Flags().Set("collection-config", configPath); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("keep-duplicate-releases", "false"); err != nil {
		t.Fatal(err)
	}
	cfg, err := effectiveDatasetConfig(cmd, a, datasetFlags{configPath: configPath, candidatePool: -1, maxTitles: -1, maxChapters: -1, maxPages: -1, maxFailures: -1, seed: math.MinInt64})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Sampling.KeepDuplicateChapterReleases {
		t.Fatal("explicit false did not override collection config")
	}
}

func TestErrorDiagnosticRecognizesEmptySearches(t *testing.T) {
	if got, want := ErrorDiagnostic(errors.New("no results found for \"missing\"")), "error category: no_results; exit code: 1"; got != want {
		t.Fatalf("ErrorDiagnostic() = %q, want %q", got, want)
	}
}

func TestErrorCategoryRecognizesDatasetFailures(t *testing.T) {
	for message, want := range map[string]string{
		"dataset configuration mismatch: existing format directory": "configuration_mismatch",
		"dataset already exists; use --resume":                      "dataset_exists",
		"dataset database: corrupt":                                 "database",
		"verification found corrupt output":                         "verification",
	} {
		if got := ErrorCategory(message); got != want {
			t.Fatalf("ErrorCategory(%q) = %q, want %q", message, got, want)
		}
	}
}

func TestParseDatasetBytesUsesLongestUnitSuffix(t *testing.T) {
	for value, want := range map[string]int64{"500GiB": 500 << 30, "20MiB": 20 << 20, "1TiB": 1 << 40, "10MB": 10_000_000} {
		got, err := parseDatasetBytes(value)
		if err != nil || got != want {
			t.Fatalf("parseDatasetBytes(%q) = %d, %v; want %d", value, got, err, want)
		}
	}
	if _, err := parseDatasetBytes("500GIBB"); err == nil {
		t.Fatal("expected invalid unit")
	}
}

func TestWriteDatasetPlanHumanIncludesCollectionContext(t *testing.T) {
	cfg := dataset.DefaultConfig("/tmp/set", "fake")
	cfg.Limits.MaxBytes = 123
	var output bytes.Buffer
	writeDatasetPlanHuman(&output, "Dataset plan", cfg, dataset.Plan{Candidates: 4, Titles: 2, Chapters: 3, EstimatedPages: 9, SplitCounts: map[string]int{"train": 1, "validation": 1}, Warnings: []string{"no chapters for one title"}}, true)
	for _, want := range []string{"Byte limit: 123", "Title strategy: stratified", "Chapter strategy: uniform", "Splits: train=1 validation=1 test=0", "Warnings: no chapters for one title", "Confirmation required: yes"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("plan output missing %q:\n%s", want, output.String())
		}
	}
}
