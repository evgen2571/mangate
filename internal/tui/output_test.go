package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestOutputModelRejectsEmptyPath(t *testing.T) {
	m := newOutputModel("")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || updated.status != "output root cannot be empty" {
		t.Fatalf("output model = %#v, command = %v", updated, cmd)
	}
}

func TestOutputPathWarningRejectsExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := outputPathWarning(path); err == nil {
		t.Fatal("outputPathWarning() error = nil, want existing-file error")
	}
}

func TestOutputPathWarningReportsNonWritableParent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "readonly")
	if err := os.Chmod(filepath.Dir(path), 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Dir(path), 0o755) })
	warning, err := outputPathWarning(path)
	if err != nil || warning == "" {
		t.Fatalf("outputPathWarning() = %q, %v; want warning", warning, err)
	}
}

func TestOutputModelEmitsSelectedPath(t *testing.T) {
	m := newOutputModel("./library")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Update(enter) returned nil command")
	}
	msg := cmd()
	selected, ok := msg.(outputPathSelectedMsg)
	if !ok || selected.Path != "./library" {
		t.Fatalf("message = %#v, want selected output path", msg)
	}
}
