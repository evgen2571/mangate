package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSearchModelCompletesHistorySuggestionWithTab(t *testing.T) {
	m := newSearchModel([]string{"Puniru wa Kawaii Slime", "One Piece"})
	m.input.SetValue("pun")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})

	if got := updated.input.Value(); got != "Puniru wa Kawaii Slime" {
		t.Fatalf("input value = %q, want completed history query", got)
	}
}

func TestSearchModelCyclesHistorySuggestions(t *testing.T) {
	m := newSearchModel([]string{"Puniru", "Pokemon", "One Piece"})
	m.input.SetValue("p")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyTab})

	if got := updated.input.Value(); got != "Pokemon" {
		t.Fatalf("input value = %q, want second matching suggestion", got)
	}
}

func TestSearchModelRendersSuggestionAsVirtualTextUnderInput(t *testing.T) {
	m := newSearchModel([]string{"Puniru wa Kawaii Slime"})
	m.input.SetValue("Pun")

	view := m.View()
	if strings.Contains(view, "History:") {
		t.Fatalf("View() contains separate history label, want virtual suggestion only:\n%s", view)
	}
	if !strings.Contains(view, "iru wa Kawaii Slime") {
		t.Fatalf("View() missing suggestion suffix under input:\n%s", view)
	}
}

func TestSearchModelDoesNotRenderSuggestionForEmptyInput(t *testing.T) {
	m := newSearchModel([]string{"Puniru wa Kawaii Slime"})

	view := m.View()
	if strings.Contains(view, "Puniru wa Kawaii Slime") {
		t.Fatalf("View() shows history suggestion for empty input:\n%s", view)
	}
}

func TestSearchModelSubmitReturnsQuery(t *testing.T) {
	m := newSearchModel([]string{"Puniru"})
	m.input.SetValue(" puniru ")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Update() command = nil, want searchSubmittedMsg command")
	}
	msg := cmd()
	submitted, ok := msg.(searchSubmittedMsg)
	if !ok {
		t.Fatalf("command returned %T, want searchSubmittedMsg", msg)
	}
	if submitted.Query != "puniru" {
		t.Fatalf("submitted query = %q, want %q", submitted.Query, "puniru")
	}
}
