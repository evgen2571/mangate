package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/evgen2571/mangate/internal/acquisition"
	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/providers"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/spf13/cobra"
)

type failingProgressWriter struct {
	bytes.Buffer
	writes   int
	failAt   int
	flushErr error
}

func (w *failingProgressWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes >= w.failAt {
		return 0, io.ErrClosedPipe
	}
	return w.Buffer.Write(p)
}
func (w *failingProgressWriter) Flush() error { return w.flushErr }
func TestProgressReportsWriteAndFlushFailures(t *testing.T) {
	for _, mode := range []string{"first", "later", "flush"} {
		t.Run(mode, func(t *testing.T) {
			writer := &failingProgressWriter{failAt: 1}
			if mode == "later" {
				writer.failAt = 2
			}
			if mode == "flush" {
				writer.failAt = 99
				writer.flushErr = io.ErrClosedPipe
			}
			cmd := &cobra.Command{}
			cmd.SetOut(writer)
			notify, finish, err := (automationFlags{jsonl: true}).progress(cmd)
			if err != nil {
				t.Fatal(err)
			}
			notify(acquisition.ProgressEvent{Stage: "discovery", Status: "searching"})
			notify(acquisition.ProgressEvent{Stage: "selection", Status: "selected"})
			if err := finish("planned"); !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("finish error=%v", err)
			}
			for _, line := range bytes.Split(bytes.TrimSpace(writer.Bytes()), []byte{'\n'}) {
				if len(line) == 0 {
					continue
				}
				var e acquisition.ProgressEvent
				if err := json.Unmarshal(line, &e); err != nil {
					t.Fatal(err)
				}
				if e.Stage == "completion" && e.Status == "planned" {
					t.Fatal("reported success after output failure")
				}
			}
		})
	}
}
func TestProgressUnavailableDestination(t *testing.T) {
	_, _, err := (automationFlags{eventsFile: filepath.Join(t.TempDir(), "missing", "events.jsonl")}).progress(&cobra.Command{})
	if err == nil {
		t.Fatal("accepted unavailable event destination")
	}
}

func TestProgressCannotShareManifestDestination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "both.json")
	_, finish, err := (automationFlags{eventsFile: path, manifest: path}).progress(&cobra.Command{})
	if finish != nil {
		finish("")
	}
	if err == nil {
		t.Fatal("accepted shared event/manifest destination")
	}
}

type randomCLIProvider struct{ fakeProvider }

func (p randomCLIProvider) Info() source.ProviderInfo {
	return source.ProviderInfo{DownloadPermitted: true}
}
func (p randomCLIProvider) RandomManga(context.Context) (*source.Manga, error) {
	return &source.Manga{ID: "m"}, nil
}
func (p randomCLIProvider) Pages(context.Context, *source.Chapter) ([]*source.Page, error) {
	return []*source.Page{{URL: "http://local/page.png"}}, nil
}
func TestRandomPlanDoesNotAppendResultToBrokenProgressStream(t *testing.T) {
	registry := providers.NewRegistry()
	registry.Register("fake", func(config.Config, *http.Client) (providers.Provider, error) {
		return randomCLIProvider{fakeProvider{chapters: []*source.Chapter{{ID: "c", PageCount: 1}}}}, nil
	})
	cfg := config.DefaultConfig()
	cfg.Provider = "fake"
	a, err := app.New(cfg, app.WithRegistry(registry))
	if err != nil {
		t.Fatal(err)
	}
	for _, failAt := range []int{1, 3, 5} {
		cmd := NewRootCmd(a)
		writer := &failingProgressWriter{failAt: failAt}
		cmd.SetOut(writer)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"random", "--plan", "--jsonl", "--output", t.TempDir()})
		err := cmd.Execute()
		if !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("failAt=%d err=%v", failAt, err)
		}
		if writer.writes != failAt {
			t.Fatalf("appended to broken stream: writes=%d want=%d", writer.writes, failAt)
		}
	}
}

type shortProgressWriter struct{ calls int }

func (w *shortProgressWriter) Write(p []byte) (int, error) { w.calls++; return len(p) - 1, nil }
func TestProgressRejectsShortWrites(t *testing.T) {
	writer := &shortProgressWriter{}
	cmd := &cobra.Command{}
	cmd.SetOut(writer)
	notify, finish, err := (automationFlags{jsonl: true}).progress(cmd)
	if err != nil {
		t.Fatal(err)
	}
	notify(acquisition.ProgressEvent{Stage: "discovery", Status: "searching"})
	if err := finish("planned"); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("err=%v", err)
	}
	if writer.calls != 1 {
		t.Fatal("appended to incomplete JSON line")
	}
}
