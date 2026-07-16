package cli

import (
	"bytes"
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
