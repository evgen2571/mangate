package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/config"
)

func TestDiagnosticsJSONReportsLocalCapabilities(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Download.Dir = t.TempDir()
	cfg.Dirs.Cache = t.TempDir()
	application, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCmd(application)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--json", "diagnostics"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var response struct {
		Operation string `json:"operation"`
		Data      struct {
			Provider sourceProvider `json:"provider"`
			Formats  []string       `json:"supportedFormats"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("JSON output = %q: %v", stdout.String(), err)
	}
	if response.Operation != "diagnostics" || response.Data.Provider.ID != "mangadex" || !containsString(response.Data.Formats, "cbz") {
		t.Fatalf("response = %#v", response)
	}
}

type sourceProvider struct {
	ID string `json:"id"`
}

func TestDiagnosticsDoesNotRequireExistingDirectories(t *testing.T) {
	record := inspectPath(t.TempDir() + "/not-created")
	if record.Exists || record.Error != "" || !strings.Contains(formatPathDiagnostic(record), "will be created") {
		t.Fatalf("path diagnostic = %#v", record)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
