package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/config"
)

func TestCompletionGeneratesScriptsForSupportedShells(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		t.Run(shell, func(t *testing.T) {
			application, err := app.New(config.DefaultConfig())
			if err != nil {
				t.Fatal(err)
			}
			cmd := NewRootCmd(application)
			var stdout bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetArgs([]string{"completion", shell})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !strings.Contains(stdout.String(), "mangate") {
				t.Fatalf("completion script does not target mangate: %q", stdout.String())
			}
		})
	}
}

func TestCompletionEndpointListsCommandsAndGlobalFlags(t *testing.T) {
	application, err := app.New(config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCmd(application)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"__complete", ""})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "download") {
		t.Fatalf("completion endpoint does not describe the command surface: %q", output)
	}
	stdout.Reset()
	cmd.SetArgs([]string{"__complete", "--"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("flag completion Execute() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "--format") {
		t.Fatalf("completion endpoint does not describe global flags: %q", stdout.String())
	}
}

func TestCompletionRejectsUnsupportedShell(t *testing.T) {
	application, err := app.New(config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCmd(application)
	cmd.SetArgs([]string{"completion", "powershell"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unsupported shell") {
		t.Fatalf("Execute() error = %v, want unsupported shell", err)
	}
}
