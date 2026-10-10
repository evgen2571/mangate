package acquisition

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/downloader"
)

func completeManifest(t *testing.T) Manifest {
	t.Helper()
	plan, err := PlanRandom(context.Background(), randomFixture{fixture()}, policy(), t.TempDir(), archive.FormatDirectory, nil)
	if err != nil {
		t.Fatal(err)
	}
	var pages []downloader.PageDownloadResult
	for i := 1; i <= 2; i++ {
		path := filepath.Join(plan.Output, fmt.Sprintf("%04d.png", i))
		var data bytes.Buffer
		if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		pages = append(pages, downloader.PageDownloadResult{PageIndex: i, Path: path})
	}
	writeState(t, plan.Output, map[string]any{"formatVersion": "1", "provider": "fixture", "titleId": "m", "chapterId": "c", "expectedPages": 2, "complete": true})
	m, err := NewManifest(*plan, pages, "complete", nil)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func writeState(t *testing.T, output string, state map[string]any) {
	t.Helper()
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, ".mangate.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}
func TestVerifyRejectsChangedState(t *testing.T) {
	for _, field := range []string{"complete", "expectedPages", "provider", "titleId", "chapterId", "formatVersion"} {
		t.Run(field, func(t *testing.T) {
			m := completeManifest(t)
			state := map[string]any{"formatVersion": "1", "provider": "fixture", "titleId": "m", "chapterId": "c", "expectedPages": 2, "complete": true}
			switch field {
			case "complete":
				state[field] = false
			case "expectedPages":
				state[field] = 3
			default:
				state[field] = "other"
			}
			writeState(t, m.Plan.Output, state)
			if m.Verify() == nil {
				t.Fatal("accepted changed chapter state")
			}
		})
	}
}
func TestVerifyDetectsModifiedAndMissingImages(t *testing.T) {
	for _, mode := range []string{"same_format", "appended", "missing"} {
		t.Run(mode, func(t *testing.T) {
			m := completeManifest(t)
			path := filepath.Join(m.Plan.Output, m.Images[0].Path)
			if mode == "missing" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if mode == "same_format" {
					img := image.NewRGBA(image.Rect(0, 0, 2, 2))
					img.Set(0, 0, color.White)
					var buf bytes.Buffer
					if err := png.Encode(&buf, img); err != nil {
						t.Fatal(err)
					}
					data = buf.Bytes()
				} else {
					data = append(data, 0)
				}
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if m.Verify() == nil {
				t.Fatal("accepted altered image")
			}
		})
	}
}
func TestCompleteManifestRoundTrip(t *testing.T) {
	m := completeManifest(t)
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := m.Write(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m, *loaded) {
		t.Fatal("manifest changed during roundtrip")
	}
	if err := loaded.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestManifestDestinationCannotAliasChapter(t *testing.T) {
	m := completeManifest(t)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(m.Plan.Output, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := m.Write(filepath.Join(alias, "0001.png")); err == nil {
		t.Fatal("manifest overwrote image through a directory alias")
	}
}

func TestManifestIntegrityContract(t *testing.T) {
	for _, mode := range []string{"missing_checksum", "bad_size", "legacy", "missing_record", "count"} {
		t.Run(mode, func(t *testing.T) {
			m := completeManifest(t)
			switch mode {
			case "missing_checksum":
				m.Images[0].SHA256 = ""
			case "bad_size":
				m.Images[0].Bytes = 0
			case "legacy":
				m.Version = 1
				for i := range m.Images {
					m.Images[i].SHA256 = ""
					m.Images[i].Bytes = 0
				}
				if err := m.Validate(); err != nil {
					t.Fatal(err)
				}
			case "missing_record":
				m.Images = m.Images[:1]
				m.ActualPages = 1
			case "count":
				m.ActualPages = 3
			}
			if err := m.Verify(); err == nil {
				t.Fatal("accepted incompatible or incomplete manifest")
			}
		})
	}
}

func TestManifestRelativeOutputUsesActualFileLocation(t *testing.T) {
	m := completeManifest(t)
	realDirectory := filepath.Join(t.TempDir(), "nested", "manifests")
	if err := os.MkdirAll(realDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(realDirectory, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := m.Write(filepath.Join(alias, "manifest.json")); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadManifest(filepath.Join(realDirectory, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Plan.Output != m.Plan.Output {
		t.Fatalf("resolved output=%q want=%q", loaded.Plan.Output, m.Plan.Output)
	}
	if err := loaded.Verify(); err != nil {
		t.Fatal(err)
	}
}
func TestManifestRejectsOversizedFile(t *testing.T) {
	m := completeManifest(t)
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := m.Write(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, bytes.Repeat([]byte(" "), 4<<20)...)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifest(path); err == nil {
		t.Fatal("accepted oversized manifest")
	}
}
