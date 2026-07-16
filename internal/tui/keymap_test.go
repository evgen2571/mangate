package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
)

func TestChaptersShortHelpIsReadableAndShowsSelectionCommands(t *testing.T) {
	m := newChaptersModel(nil, nil)
	bindings := m.HelpKeys(newKeyMap()).ShortHelp()

	if got, max := len(bindings), 9; got > max {
		t.Fatalf("len(ShortHelp()) = %d, want at most %d bindings", got, max)
	}

	assertHelpContains(t, bindings, "a", "all")
	assertHelpContains(t, bindings, "d", "clear")
}

func TestResultsShortHelpKeepsOnlyPrimaryActions(t *testing.T) {
	m := newResultsModel("query", "mangadex", nil)
	bindings := m.HelpKeys(newKeyMap()).ShortHelp()

	if got, max := len(bindings), 8; got > max {
		t.Fatalf("len(ShortHelp()) = %d, want at most %d bindings", got, max)
	}
	assertHelpContains(t, bindings, "f", "full")
	assertHelpContains(t, bindings, "/", "filter")
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
