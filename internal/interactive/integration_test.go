package interactive

import (
	"context"
	"encoding/json"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/evgen2571/mangate/internal/source"
	"os"
	"path/filepath"
	"testing"
)

func TestIntegrationCompletionAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		done   downloadDone
		status string
	}{
		{"success", downloadDone{completed: 1}, "success"},
		{"partial", downloadDone{completed: 1, failed: 1}, "partial"},
		{"cancelled", downloadDone{err: context.Canceled}, "cancelled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := testModel(t)
			m.integration = true
			_, cmd := m.Update(tc.done)
			if cmd == nil {
				t.Fatal("integration did not exit")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatal("expected quit")
			}
			if m.Result().Status != tc.status {
				t.Fatalf("result = %#v", m.Result())
			}
		})
	}
}

func TestIntegrationSelectionLimit(t *testing.T) {
	m := testModel(t)
	m.integration, m.maxChapters = true, 1
	m.chapters = []*source.Chapter{{ID: "a"}, {ID: "b"}}
	m.selected = map[int]bool{0: true, 1: true}
	m.openFormat()
	if m.screen != searchScreen || m.status == "" {
		t.Fatal("selection limit not enforced")
	}
	m.selected = map[int]bool{0: true}
	m.openFormat()
	if m.screen != outputScreen {
		t.Fatal("integration must select an image directory")
	}
}

func TestImagePathsValidateCompletionAndOrder(t *testing.T) {
	directory := t.TempDir()
	page := []byte{'R', 'I', 'F', 'F', 0, 0, 0, 0, 'W', 'E', 'B', 'P', 'V', 'P', '8', ' ', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	for _, name := range []string{"0002.webp", "0001.webp"} {
		if err := os.WriteFile(filepath.Join(directory, name), page, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(directory, ".mangate.json"), []byte(`{"complete":true,"expectedPages":2}`), 0600); err != nil {
		t.Fatal(err)
	}
	paths, err := imagePaths(directory, 2)
	if err != nil || len(paths) != 2 || filepath.Base(paths[0]) != "0001.webp" {
		t.Fatalf("paths = %v, %v", paths, err)
	}
	if err := os.Remove(paths[1]); err != nil {
		t.Fatal(err)
	}
	if _, err := imagePaths(directory, 2); err == nil {
		t.Fatal("missing page accepted")
	}
}

func TestIntegrationChapterOrdering(t *testing.T) {
	chapters := []*source.Chapter{{ID: "z", Index: "2"}, {ID: "a", Index: "10"}, {ID: "b", Index: "1a"}, {ID: "c", Index: "2"}}
	orderChapters(chapters)
	for index, want := range []string{"c", "z", "a", "b"} {
		if chapters[index].ID != want {
			t.Fatalf("position %d = %s, want %s", index, chapters[index].ID, want)
		}
	}
}

func TestResultFileIsAtomicAndCleansTemporaryFiles(t *testing.T) {
	directory := t.TempDir()
	destination := filepath.Join(directory, "result.json")
	want := Result{FormatVersion: "1", Status: "cancelled", Chapters: []ChapterResult{}}
	if err := WriteResult(destination, want); err != nil {
		t.Fatal(err)
	}
	var got Result
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &got); err != nil || got.Status != "cancelled" {
		t.Fatalf("result = %#v, %v", got, err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files remain: %v, %v", entries, err)
	}
	if err := WriteResult(directory, want); err == nil {
		t.Fatal("directory accepted as result file")
	}
	entries, err = os.ReadDir(directory)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files remain after failure: %v, %v", entries, err)
	}
}
