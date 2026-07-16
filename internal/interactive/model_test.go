package interactive

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	if !strings.Contains(view, "●") {
		t.Fatal("selected chapter was not rendered")
	}
}

func TestResultNavigationMovesExactlyOneItem(t *testing.T) {
	m := testModel(t)
	m.results = []*source.Manga{{Title: "One"}, {Title: "Two"}, {Title: "Three"}}
	m.newResultsList(m.results)
	m.screen = resultsScreen
	for _, key := range []tea.KeyMsg{{Type: tea.KeyDown}, {Type: tea.KeyRunes, Runes: []rune("j")}} {
		before := m.resultsList.Index()
		m.Update(key)
		if got := m.resultsList.Index(); got != before+1 {
			t.Fatalf("%q moved from %d to %d, want one item", key.String(), before, got)
		}
	}
	for _, key := range []tea.KeyMsg{{Type: tea.KeyUp}, {Type: tea.KeyRunes, Runes: []rune("k")}} {
		before := m.resultsList.Index()
		m.Update(key)
		if got := m.resultsList.Index(); got != before-1 {
			t.Fatalf("%q moved from %d to %d, want one item", key.String(), before, got)
		}
	}
}

func TestChapterSpaceTogglesExactlyOnceWithoutMoving(t *testing.T) {
	m := testModel(t)
	m.screen, m.manga = chaptersScreen, &source.Manga{Title: "Test"}
	m.chapters = []*source.Chapter{{Index: "1", Language: "en"}, {Index: "2", Language: "en"}}
	m.chapterCursor = 1
	key := tea.KeyMsg{Type: tea.KeySpace}
	m.Update(key)
	if !m.selected[1] || m.chapterCursor != 1 {
		t.Fatalf("first space selected=%v cursor=%d, want true and 1", m.selected[1], m.chapterCursor)
	}
	m.Update(key)
	if m.selected[1] || m.chapterCursor != 1 {
		t.Fatalf("second space selected=%v cursor=%d, want false and 1", m.selected[1], m.chapterCursor)
	}
}

func TestFocusedInputReceivesGlobalShortcutCharacters(t *testing.T) {
	m := testModel(t)
	resize(t, m, 80, 24)
	for _, key := range []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune("q")}, {Type: tea.KeyRunes, Runes: []rune("?")}} {
		next, _ := m.Update(key)
		if next == nil || m.showHelp {
			t.Fatalf("focused input key %q was handled globally", key.String())
		}
	}
	if got := m.input.Value(); got != "q?" {
		t.Fatalf("input value = %q, want q?", got)
	}
}

func TestFramedInputHasStableDimensions(t *testing.T) {
	m := testModel(t)
	resize(t, m, 80, 24)
	s := newStyles()
	for _, value := range []string{"", "d", "ten letters", strings.Repeat("long ", 30), "Привет 漫画 한국어"} {
		m.input.SetValue(value)
		field := m.inputView(s)
		if got, want := lipgloss.Width(field), m.contentWidth(); got != want {
			t.Fatalf("input width for %q = %d, want %d", value, got, want)
		}
		lines := strings.Split(field, "\n")
		if len(lines) != 3 || lipgloss.Width(lines[0]) != m.contentWidth() || lipgloss.Width(lines[2]) != m.contentWidth() {
			t.Fatalf("broken input border for %q: %q", value, field)
		}
	}
}

func TestOutputInputSharesStableFrameAfterResize(t *testing.T) {
	m := testModel(t)
	m.screen, m.manga = outputScreen, &source.Manga{Title: "Example"}
	resize(t, m, 80, 24)
	m.resetInput("Output: ", "/downloads")
	before := m.inputView(newStyles())
	m.input.SetValue("/a path with unicode 漫画 and enough text to scroll beyond the visible field")
	resize(t, m, 100, 30)
	after := m.inputView(newStyles())
	if got, want := lipgloss.Width(before), 74; got != want {
		t.Fatalf("initial output width = %d, want %d", got, want)
	}
	if got, want := lipgloss.Width(after), 92; got != want {
		t.Fatalf("resized output width = %d, want %d", got, want)
	}
	if got := m.input.Value(); !strings.Contains(got, "漫画") {
		t.Fatalf("resize lost output value: %q", got)
	}
}

func TestWorkflowIncludesOutputWithOneActiveStep(t *testing.T) {
	s := newStyles()
	for _, current := range []screen{chaptersScreen, formatScreen, outputScreen, reviewScreen} {
		crumb := workflow(current, s)
		for _, label := range []string{"Search", "Title", "Chapters", "Format", "Output", "Review"} {
			if !strings.Contains(crumb, label) {
				t.Fatalf("%v breadcrumb missing %q: %q", current, label, crumb)
			}
		}
	}
}

func TestReviewCardUsesConnectedFixedWidthBorder(t *testing.T) {
	m := testModel(t)
	resize(t, m, 80, 24)
	m.manga = &source.Manga{Title: strings.Repeat("A wide title 漫画 ", 12)}
	m.app.Cfg.Download.Dir = "/very/long/output/path/with/a/unicode/漫画/directory"
	card := m.reviewView()
	lines := strings.Split(card, "\n")
	if len(lines) < 3 || !strings.HasPrefix(lines[0], "┌") || !strings.HasPrefix(lines[len(lines)-1], "└") {
		t.Fatalf("review card has incomplete border: %q", card)
	}
	for _, line := range lines {
		if got, want := lipgloss.Width(line), m.contentWidth(); got != want {
			t.Fatalf("review line width = %d, want %d: %q", got, want, line)
		}
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
