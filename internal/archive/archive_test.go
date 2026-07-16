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
	writePage(t, source, "0001.jpg", jpegPage())
	writePage(t, source, "0010.png", pngPage())
	writePage(t, source, "0100.gif", gifPage())
	writeState(t, source, "title-id", "chapter-id", 3, true)

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
	if result.Status != StatusComplete || result.OutputPath != output || result.IncludedPages != 3 {
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
	want := []string{"0001.jpg", "0010.png", "0100.gif", "ComicInfo.xml", ".mangate.json"}
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
	if !inspection.Valid || !inspection.Complete || inspection.PageCount != 3 || inspection.Format != FormatCBZ {
		t.Fatalf("inspection = %#v", inspection)
	}
}

func TestCreateFromDirectoryRejectsIncompleteSourceAndLeavesNoFinalArchive(t *testing.T) {
	source := t.TempDir()
	writePage(t, source, "0001.jpg", jpegPage())
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
	writePage(t, source, "0001.webp", webpPage())
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

func TestCreateFromDirectoryCarriesProviderFromLocalState(t *testing.T) {
	source := t.TempDir()
	writePage(t, source, "0001.jpg", jpegPage())
	state := []byte(`{"provider":"mangadex","titleId":"title-id","chapterId":"chapter-id","expectedPages":1,"complete":true}`)
	if err := os.WriteFile(filepath.Join(source, ".mangate.json"), state, 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "chapter.zip")
	if _, err := CreateFromDirectory(Options{Format: FormatZIP, SourceDir: source, OutputPath: output}); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.Name != ".mangate.json" {
			continue
		}
		in, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		var metadata Metadata
		if err := json.NewDecoder(in).Decode(&metadata); err != nil {
			t.Fatal(err)
		}
		if err := in.Close(); err != nil {
			t.Fatal(err)
		}
		if metadata.Provider != "mangadex" {
			t.Fatalf("provider = %q, want mangadex", metadata.Provider)
		}
		return
	}
	t.Fatal("archive metadata entry not found")
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

func TestInspectRejectsPageWithInvalidImageBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.zip")
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(out)
	page, err := writer.Create("0001.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := page.Write([]byte("not an image")); err != nil {
		t.Fatal(err)
	}
	metadata, err := writer.Create(".mangate.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := metadata.Write([]byte(`{"completion":"complete","expectedPages":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	inspection, err := Inspect(path)
	if err == nil || inspection.Valid {
		t.Fatalf("Inspect() = %#v, %v; want invalid page error", inspection, err)
	}
}

func TestCreateFromDirectoryRejectsNonImagePageBytes(t *testing.T) {
	source := t.TempDir()
	writePage(t, source, "0001.jpg", []byte("not an image"))
	output := filepath.Join(t.TempDir(), "chapter.zip")

	_, err := CreateFromDirectory(Options{Format: FormatZIP, SourceDir: source, OutputPath: output})
	if err == nil {
		t.Fatal("CreateFromDirectory() error = nil, want invalid image error")
	}
	if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
		t.Fatalf("final archive exists after invalid source: %v", statErr)
	}
}

func TestCreateFromDirectoryNormalizesNumericPageNamesForLexicographicOrder(t *testing.T) {
	source := t.TempDir()
	writePage(t, source, "1.jpg", jpegPage())
	writePage(t, source, "10.jpg", jpegPage())
	writePage(t, source, "2.jpg", jpegPage())
	output := filepath.Join(t.TempDir(), "chapter.zip")
	if _, err := CreateFromDirectory(Options{Format: FormatZIP, SourceDir: source, OutputPath: output}); err != nil {
		t.Fatalf("CreateFromDirectory() error = %v", err)
	}
	reader, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	got := []string{reader.File[0].Name, reader.File[1].Name, reader.File[2].Name}
	want := []string{"0001.jpg", "0002.jpg", "0010.jpg"}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("entries = %v, want %v", got, want)
		}
	}
}

func writePage(t *testing.T, directory, name string, body []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, name), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func jpegPage() []byte { return []byte{0xff, 0xd8, 0xff, 0xd9} }

func pngPage() []byte { return []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'} }

func gifPage() []byte { return []byte("GIF89a") }

func webpPage() []byte { return []byte("RIFF\x00\x00\x00\x00WEBP") }

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
