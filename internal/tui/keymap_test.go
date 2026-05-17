package tui

import (
	"errors"
	"os"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	"github.com/evgen2571/mangate/internal/tuiapp"
)

func TestKeysFileExistsAndKeymapFileIsGone(t *testing.T) {
	if _, err := os.Stat("keys.go"); err != nil {
		t.Fatalf("Stat(keys.go) error = %v, want file to exist", err)
	}

	_, err := os.Stat("keymap.go")
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(keymap.go) error = %v, want %v", err, os.ErrNotExist)
	}
}

func TestChaptersShortHelpIsReadableAndShowsSelectionCommands(t *testing.T) {
	m := newChaptersModel(tuiapp.MangaDetails{}, nil)
	bindings := m.HelpKeys(newKeyMap()).ShortHelp()

	if got, max := len(bindings), 9; got > max {
		t.Fatalf("len(ShortHelp()) = %d, want at most %d bindings", got, max)
	}

	assertHelpContains(t, bindings, "a", "all")
	assertHelpContains(t, bindings, "d", "clear")
}

func TestResultsShortHelpKeepsOnlyPrimaryActions(t *testing.T) {
	m := newResultsModel("query", nil)
	bindings := m.HelpKeys(newKeyMap()).ShortHelp()

	if got, max := len(bindings), 7; got > max {
		t.Fatalf("len(ShortHelp()) = %d, want at most %d bindings", got, max)
	}
	assertHelpContains(t, bindings, "f", "full")
	assertHelpContains(t, bindings, "?", "help")
}

func assertHelpContains(t *testing.T, bindings []key.Binding, keyText, desc string) {
	t.Helper()

	for _, binding := range bindings {
		help := binding.Help()
		if help.Key == keyText && help.Desc == desc {
			return
		}
	}

	t.Fatalf("ShortHelp() missing %q/%q in %#v", keyText, desc, bindings)
}
