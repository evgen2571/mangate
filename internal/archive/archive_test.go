package archive

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateFromDirectoryCreatesOrderedCBZWithMetadata(t *testing.T) {
	source := t.TempDir()
	writePage(t, source, "0001.jpg", "first")
	writePage(t, source, "0010.png", "tenth")
	writeState(t, source, "title-id", "chapter-id", 2, true)

	output := filepath.Join(t.TempDir(), "chapter.cbz")
	result, err := CreateFromDirectory(Options{
		Format:     FormatCBZ,
		SourceDir:  source,
		OutputPath: output,
		Metadata:   Metadata{Provider: "example", TitleID: "title-id", Title: "Example", ChapterID: "chapter-id", ChapterNumber: "10"},
	})
	if err != nil {
		t.Fatalf("CreateFromDirectory() error = %v", err)
	}
	if result.Status != StatusComplete || result.OutputPath != output || result.IncludedPages != 2 {
		t.Fatalf("result = %#v", result)
	}

	reader, err := zip.OpenReader(output)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer reader.Close()
	got := make([]string, 0, len(reader.File))
	for _, file := range reader.File {
		got = append(got, file.Name)
	}
	want := []string{"0001.jpg", "0010.png", "ComicInfo.xml", ".mangate.json"}
	if len(got) != len(want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("entries = %v, want %v", got, want)
		}
	}

	inspection, err := Inspect(output)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if !inspection.Valid || !inspection.Complete || inspection.PageCount != 2 || inspection.Format != FormatCBZ {
		t.Fatalf("inspection = %#v", inspection)
	}
}

func TestCreateFromDirectoryRejectsIncompleteSourceAndLeavesNoFinalArchive(t *testing.T) {
	source := t.TempDir()
	writePage(t, source, "0001.jpg", "page")
	writeState(t, source, "title-id", "chapter-id", 2, false)
	output := filepath.Join(t.TempDir(), "chapter.zip")

	_, err := CreateFromDirectory(Options{Format: FormatZIP, SourceDir: source, OutputPath: output})
	if err == nil {
		t.Fatal("CreateFromDirectory() error = nil, want incomplete source error")
	}
	if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
		t.Fatalf("final archive exists after failure: %v", statErr)
	}
}

func TestCreateFromDirectoryExistingArchivePolicies(t *testing.T) {
	source := t.TempDir()
	writePage(t, source, "0001.webp", "page")
	writeState(t, source, "title-id", "chapter-id", 1, true)
	output := filepath.Join(t.TempDir(), "chapter.zip")
	options := Options{Format: FormatZIP, SourceDir: source, OutputPath: output, Metadata: Metadata{TitleID: "title-id", ChapterID: "chapter-id"}}
	if _, err := CreateFromDirectory(options); err != nil {
		t.Fatalf("initial CreateFromDirectory() error = %v", err)
	}
	result, err := CreateFromDirectory(options)
	if err != nil {
		t.Fatalf("skip CreateFromDirectory() error = %v", err)
	}
	if result.Status != StatusSkipped || !result.Validation.Complete {
		t.Fatalf("skip result = %#v", result)
	}
	_, err = CreateFromDirectory(Options{Format: FormatZIP, SourceDir: source, OutputPath: output, ExistingFileMode: ExistingFail})
	if err == nil {
		t.Fatal("fail policy error = nil")
	}
}

func TestInspectRejectsUnsafeAndDuplicateEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unsafe.zip")
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(out)
	for _, name := range []string{"../page.jpg", "0001.jpg", "0001.jpg"} {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte("page")); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	inspection, err := Inspect(path)
	if err == nil || inspection.Valid {
		t.Fatalf("Inspect() = %#v, %v; want invalid archive", inspection, err)
	}
}

func writePage(t *testing.T, directory, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeState(t *testing.T, directory, titleID, chapterID string, expectedPages int, complete bool) {
	t.Helper()
	state, err := json.Marshal(map[string]any{"formatVersion": "1", "titleId": titleID, "chapterId": chapterID, "expectedPages": expectedPages, "complete": complete})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, ".mangate.json"), state, 0o644); err != nil {
		t.Fatal(err)
	}
}
