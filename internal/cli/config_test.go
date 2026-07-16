package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/config"
)

func TestConfigShowsEffectiveFormatAndOutput(t *testing.T) {
	cfg := config.DefaultConfig()
	a, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCmd(a)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--format", "cbz", "--output", "./library", "config"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	text := output.String()
	if !strings.Contains(text, "Format: cbz") || !strings.Contains(text, "Output: ./library") {
		t.Fatalf("output = %q", text)
	}
}

func TestConfigJSONUsesStableLowerCamelCaseNames(t *testing.T) {
	a, err := app.New(config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCmd(a)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--json", "config"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var response map[string]any
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	data := response["data"].(map[string]any)
	configuration := data["config"].(map[string]any)
	if _, exists := configuration["provider"]; !exists {
		t.Fatalf("configuration = %#v, want provider key", configuration)
	}
	if _, exists := configuration["Provider"]; exists {
		t.Fatalf("configuration contains unstable Provider key: %#v", configuration)
	}
}

func TestTUIRejectsNonInteractiveMode(t *testing.T) {
	a, err := app.New(config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCmd(a)
	cmd.SetArgs([]string{"--non-interactive", "tui"})
	err = cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--non-interactive") {
		t.Fatalf("Execute() error = %v, want non-interactive rejection", err)
	}
}

func TestTUIRejectsConflictingColorFlagsBeforeOpeningTerminal(t *testing.T) {
	a, err := app.New(config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCmd(a)
	cmd.SetArgs([]string{"--color", "--no-color", "tui"})
	err = cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("Execute() error = %v, want color conflict", err)
	}
}

func TestMissingTitleArgumentExplainsStableReference(t *testing.T) {
	a, err := app.New(config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCmd(a)
	cmd.SetArgs([]string{"download"})
	err = cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "stable <title-id>") || !strings.Contains(err.Error(), "mangate download") {
		t.Fatalf("Execute() error = %v, want actionable title reference error", err)
	}
}
