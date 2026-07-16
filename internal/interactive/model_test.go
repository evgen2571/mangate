package interactive

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/source"
)

func testModel(t *testing.T) *model {
	t.Helper()
	a, err := app.New(config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	return NewWithContext(a, context.Background()).(*model)
}

func resize(t *testing.T, m *model, width, height int) {
	t.Helper()
	next, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	if next != m {
		t.Fatal("update replaced model")
	}
}

func TestViewsRenderSharedFrame(t *testing.T) {
	m := testModel(t)
	resize(t, m, 80, 24)
	m.manga = &source.Manga{Title: "A very long title that should stay inside the terminal frame"}
	m.chapters = []*source.Chapter{{Index: "1", Title: "Start", Language: "en", PageCount: 10}}
	cases := []screen{searchScreen, resultsScreen, chaptersScreen, formatScreen, outputScreen, reviewScreen, workingScreen, doneScreen, configScreen}
	for _, screen := range cases {
		m.screen = screen
		if screen == doneScreen {
			m.completion = "complete: 1  reused: 0  failed: 0\n/path/to/output"
			m.doneViewport.SetContent(m.completion)
		}
		view := m.View()
		if !strings.Contains(view, "MANGATE") {
			t.Fatalf("screen %v missing header: %q", screen, view)
		}
		if !strings.Contains(view, "?") && screen != workingScreen {
			t.Fatalf("screen %v missing help: %q", screen, view)
		}
	}
}

func TestSmallTerminalPreservesState(t *testing.T) {
	m := testModel(t)
	m.screen, m.query, m.chapterFilter = chaptersScreen, "dorohedoro", "english"
	m.selected[3] = true
	resize(t, m, 40, 10)
	view := m.View()
	if !strings.Contains(view, "Terminal is too small") {
		t.Fatalf("missing resize message: %q", view)
	}
	if m.screen != chaptersScreen || m.query != "dorohedoro" || !m.selected[3] || m.chapterFilter != "english" {
		t.Fatal("resize changed workflow state")
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if next.(*model).screen != chaptersScreen {
		t.Fatal("small terminal accepted normal workflow input")
	}
}

func TestResizePreservesResultSelectionAndHelp(t *testing.T) {
	m := testModel(t)
	m.results = []*source.Manga{{Title: "One"}, {Title: "Two"}, {Title: "Three"}}
	m.newResultsList(m.results)
	m.screen = resultsScreen
	m.resultsList.Select(2)
	resize(t, m, 120, 40)
	if m.resultsList.Index() != 2 {
		t.Fatalf("selected result = %d, want 2", m.resultsList.Index())
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if !next.(*model).showHelp {
		t.Fatal("help did not open")
	}
	if !strings.Contains(m.View(), "move up") {
		t.Fatal("expanded help missing screen bindings")
	}
}

func TestChaptersRemainScrollableAndSelected(t *testing.T) {
	m := testModel(t)
	resize(t, m, 80, 18)
	m.screen, m.manga = chaptersScreen, &source.Manga{Title: "Long chapters"}
	for i := 0; i < 80; i++ {
		m.chapters = append(m.chapters, &source.Chapter{Index: string(rune('A' + i%26)), Title: strings.Repeat("long name ", 12), Language: "en", PageCount: 22})
	}
	m.chapterCursor, m.selected[55] = 55, true
	view := m.View()
	if strings.Count(view, "pages") >= len(m.chapters) {
		t.Fatal("chapter list was not constrained to the frame")
	}
	if !strings.Contains(view, "[x]") {
		t.Fatal("selected chapter was not rendered")
	}
}

func TestCompletionStatesAreDistinct(t *testing.T) {
	m := testModel(t)
	resize(t, m, 80, 24)
	m.screen, m.completion = doneScreen, "complete: 0  reused: 0  failed: 0"
	m.doneErr = context.Canceled
	if !strings.Contains(m.View(), "Download cancelled") {
		t.Fatal("cancelled download looked successful")
	}
	m.doneErr, m.doneCompleted, m.doneFailed = errors.New("provider unavailable"), 0, 0
	if !strings.Contains(m.View(), "Download failed") {
		t.Fatal("failed download looked successful")
	}
	m.doneErr, m.doneCompleted, m.doneFailed = errors.New("one chapter failed"), 2, 1
	if !strings.Contains(m.View(), "Download finished with failures") {
		t.Fatal("partial failure did not have its own state")
	}
}
