package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/evgen2571/mangate/internal/app"
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
			OutputPath string `json:"outputPath"`
			Format     string `json:"format"`
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
}
