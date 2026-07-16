package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/config"
)

func TestArchiveConvertJSONCreatesCBZWithoutProvider(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "0001.jpg"), []byte{0xff, 0xd8, 0xff, 0xd9}, 0o644); err != nil {
		t.Fatal(err)
	}
	application, err := app.New(config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCmd(application)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--json", "--format", "cbz", "archive", "convert", source})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var response struct {
		Operation string `json:"operation"`
		Status    string `json:"status"`
		Data      struct {
			OutputPath string   `json:"outputPath"`
			Format     string   `json:"format"`
			Warnings   []string `json:"warnings"`
			Validation struct {
				Complete bool `json:"complete"`
			} `json:"validation"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("JSON output = %q: %v", stdout.String(), err)
	}
	if response.Operation != "archive.convert" || response.Status != "success" || response.Data.Format != "cbz" || !response.Data.Validation.Complete {
		t.Fatalf("response = %#v", response)
	}
	if filepath.Ext(response.Data.OutputPath) != ".cbz" {
		t.Fatalf("output path = %q", response.Data.OutputPath)
	}
	if len(response.Data.Warnings) != 1 || !strings.Contains(response.Data.Warnings[0], "identity cannot be confirmed") {
		t.Fatalf("warnings = %#v", response.Data.Warnings)
	}
}

func TestArchiveConvertDryRunDoesNotCreateArchive(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "0001.jpg"), []byte{0xff, 0xd8, 0xff, 0xd9}, 0o644); err != nil {
		t.Fatal(err)
	}
	application, err := app.New(config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCmd(application)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--json", "--format", "zip", "archive", "convert", "--dry-run", "--remove-source", source})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var response struct {
		Operation string `json:"operation"`
		Data      struct {
			OutputPath   string `json:"outputPath"`
			RemoveSource bool   `json:"removeSource"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Operation != "archive.convert.plan" || !response.Data.RemoveSource || filepath.Ext(response.Data.OutputPath) != ".zip" {
		t.Fatalf("response = %#v", response)
	}
	if _, err := os.Stat(response.Data.OutputPath); !os.IsNotExist(err) {
		t.Fatalf("dry run created archive: %v", err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("dry run removed source: %v", err)
	}
}

func TestArchiveConvertRequiresYesForDestructiveOperations(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "remove source", args: []string{"--format", "cbz", "archive", "convert", "--remove-source", "chapter"}, want: "removing the source"},
		{name: "replace", args: []string{"--format", "cbz", "--existing-files", "replace", "archive", "convert", "chapter"}, want: "replacing an existing"},
	} {
		t.Run(test.name, func(t *testing.T) {
			application, err := app.New(config.DefaultConfig())
			if err != nil {
				t.Fatal(err)
			}
			cmd := NewRootCmd(application)
			cmd.SetArgs(test.args)
			err = cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), test.want) || !strings.Contains(err.Error(), "--yes") {
				t.Fatalf("Execute() error = %v, want acknowledgement error", err)
			}
		})
	}
}

func TestArchiveInspectJSONIncludesStoredMetadata(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "0001.jpg"), []byte{0xff, 0xd8, 0xff, 0xd9}, 0o644); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(t.TempDir(), "chapter.cbz")
	if _, err := archive.CreateFromDirectory(archive.Options{
		Format:     archive.FormatCBZ,
		SourceDir:  source,
		OutputPath: archivePath,
		Metadata:   archive.Metadata{Title: "Example", ChapterNumber: "1", ExpectedPages: 1},
	}); err != nil {
		t.Fatalf("CreateFromDirectory() error = %v", err)
	}
	application, err := app.New(config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	inspect := NewRootCmd(application)
	var stdout bytes.Buffer
	inspect.SetOut(&stdout)
	inspect.SetArgs([]string{"--json", "archive", "inspect", archivePath})
	if err := inspect.Execute(); err != nil {
		t.Fatalf("inspect Execute() error = %v", err)
	}
	var response struct {
		Data struct {
			IdentityConfirmed bool `json:"identityConfirmed"`
			Metadata          struct {
				ExpectedPages int    `json:"expectedPages"`
				Completion    string `json:"completion"`
			} `json:"metadata"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("JSON output = %q: %v", stdout.String(), err)
	}
	if response.Data.IdentityConfirmed || response.Data.Metadata.ExpectedPages != 1 || response.Data.Metadata.Completion != "complete" {
		t.Fatalf("metadata = %#v", response.Data.Metadata)
	}
}
