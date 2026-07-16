package tui

import (
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
