package integration

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestExecutableWorkflow(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "mangate")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, "./cmd/mangate")
	build.Dir = ".."
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	var page bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 3, 4))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&page, img); err != nil {
		t.Fatal(err)
	}
	title := map[string]any{"id": "title-id", "attributes": map[string]any{"title": map[string]string{"en": "Fixture Manga"}, "description": map[string]string{"en": "Offline integration fixture"}, "originalLanguage": "en"}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/manga/":
			data := []any{title}
			if r.URL.Query().Get("title") == "empty" {
				data = []any{}
			}
			json.NewEncoder(w).Encode(map[string]any{"result": "ok", "data": data, "limit": 100, "offset": 0, "total": len(data)})
		case "/manga/title-id":
			json.NewEncoder(w).Encode(map[string]any{"result": "ok", "data": title})
		case "/manga/title-id/feed":
			fmt.Fprint(w, `{"result":"ok","data":[{"id":"chapter-id","attributes":{"chapter":"1","title":"Opening / scene","pages":2,"translatedLanguage":"en"}}],"limit":500,"offset":0,"total":1}`)
		case "/at-home/server/chapter-id":
			fmt.Fprintf(w, `{"result":"ok","baseUrl":"http://%s","chapter":{"hash":"hash","data":["1.png","2.png"]}}`, r.Host)
		case "/data/hash/1.png", "/data/hash/2.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write(page.Bytes())
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	configuration := filepath.Join(t.TempDir(), "config.json")
	body := fmt.Sprintf(`{"providers":{"mangadex":{"baseUrl":%q,"siteUrl":%q,"uploadsUrl":%q}},"dirs":{"cache":%q}}`, server.URL, server.URL, server.URL, t.TempDir())
	if err := os.WriteFile(configuration, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	run := func(t *testing.T, wantCode int, args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Env = append(os.Environ(), "MANGATE_CONFIG="+configuration)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		code := 0
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				code = exit.ExitCode()
			} else {
				t.Fatalf("run: %v", err)
			}
		}
		if code != wantCode {
			t.Fatalf("%v exit %d, want %d\nstdout: %s\nstderr: %s", args, code, wantCode, stdout.String(), stderr.String())
		}
		return stdout.Bytes()
	}
	for _, tc := range []struct {
		name     string
		args     []string
		contains string
	}{
		{"help", []string{"--help"}, "download"},
		{"version", []string{"--version"}, "mangate"},
		{"providers", []string{"--json", "providers"}, "mangadex"},
		{"search", []string{"--json", "search", "Fixture"}, "title-id"},
		{"metadata", []string{"--json", "title", "title-id"}, "Offline integration fixture"},
		{"chapters", []string{"--json", "chapters", "title-id"}, "chapter-id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if output := run(t, 0, tc.args...); !bytes.Contains(output, []byte(tc.contains)) {
				t.Fatalf("unexpected output: %s", output)
			}
		})
	}
	t.Run("empty search", func(t *testing.T) {
		if output := run(t, 1, "--json", "search", "empty"); !bytes.Contains(output, []byte("no_results")) {
			t.Fatalf("unexpected empty response %s", output)
		}
	})
	for _, format := range []string{"directory", "png", "jpeg", "cbz", "zip"} {
		t.Run(format, func(t *testing.T) {
			root := t.TempDir()
			output := run(t, 0, "--json", "--output", root, "--format", format, "download", "title-id", "--chapter-id", "chapter-id", "--yes")
			var result struct {
				Data struct {
					Status   string
					Chapters []struct {
						Status      string
						OutputPath  string
						ArchivePath string
					}
				}
			}
			if err := json.Unmarshal(output, &result); err != nil {
				t.Fatal(err)
			}
			if result.Data.Status != "complete" || len(result.Data.Chapters) != 1 || result.Data.Chapters[0].Status != "complete" {
				t.Fatalf("download result: %s", output)
			}
			chapter := result.Data.Chapters[0]
			if format == "cbz" || format == "zip" {
				if _, err := os.Stat(chapter.OutputPath); !os.IsNotExist(err) {
					t.Fatalf("source directory remains: %v", err)
				}
				reader, err := zip.OpenReader(chapter.ArchivePath)
				if err != nil {
					t.Fatal(err)
				}
				defer reader.Close()
				count := 0
				for _, entry := range reader.File {
					if strings.HasSuffix(entry.Name, ".png") {
						count++
						r, err := entry.Open()
						if err != nil {
							t.Fatal(err)
						}
						data, err := io.ReadAll(r)
						r.Close()
						if err != nil || !bytes.Equal(data, page.Bytes()) {
							t.Fatal("archive changed page bytes")
						}
					}
					if strings.Contains(entry.Name, "\\") || strings.HasPrefix(entry.Name, "/") {
						t.Fatalf("invalid archive path %q", entry.Name)
					}
				}
				if count != 2 {
					t.Fatalf("archive contains %d pages", count)
				}
				run(t, 0, "--json", "archive", "verify", chapter.ArchivePath)
			} else {
				extension := ".png"
				wantEncoding := "png"
				if format == "jpeg" {
					extension = ".jpeg"
					wantEncoding = "jpeg"
				}
				for n := 1; n <= 2; n++ {
					data, err := os.ReadFile(filepath.Join(chapter.OutputPath, fmt.Sprintf("%04d%s", n, extension)))
					if err != nil {
						t.Fatal(err)
					}
					decoded, encoding, err := image.Decode(bytes.NewReader(data))
					if err != nil || encoding != wantEncoding || decoded.Bounds().Dx() != 3 || decoded.Bounds().Dy() != 4 {
						t.Fatalf("invalid produced image: %s %v", encoding, err)
					}
				}
			}
			if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if strings.HasSuffix(path, ".part") {
					t.Errorf("temporary output remains: %s", path)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
