package cli

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/providers"
	"github.com/evgen2571/mangate/internal/source"
)

func TestChaptersCommandPrintsProviderChapters(t *testing.T) {
	a := newTestApp(t, fakeProvider{
		chapters: []*source.Chapter{
			{ID: "chapter-a", Index: "1", Title: "Beginning", PageCount: 24, URL: "https://example.test/chapter-a"},
			{ID: "chapter-b", Index: "2", PageCount: 18},
		},
	})

	cmd := NewChaptersCmd(a)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"manga-123"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := out.String()
	wantParts := []string{
		"Chapters for manga-123",
		"1. Chapter 1 - Beginning",
		"ID:    chapter-a",
		"Pages: 24",
		"URL:   https://example.test/chapter-a",
		"2. Chapter 2",
		"ID:    chapter-b",
		"Pages: 18",
	}
	for _, want := range wantParts {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q\noutput:\n%s", want, got)
		}
	}
}

func TestChaptersCommandRejectsNilChapter(t *testing.T) {
	a := newTestApp(t, fakeProvider{
		chapters: []*source.Chapter{nil},
	})

	cmd := NewChaptersCmd(a)
	cmd.SetArgs([]string{"manga-123"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "chapter #1 is nil") {
		t.Fatalf("Execute() error = %q, want nil chapter error", err)
	}
}

func TestChaptersCommandRejectsEmptyMangaID(t *testing.T) {
	a := newTestApp(t, fakeProvider{})

	cmd := NewChaptersCmd(a)
	cmd.SetArgs([]string{"   "})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "manga id cannot be empty") {
		t.Fatalf("Execute() error = %q, want empty manga id error", err)
	}
}

func newTestApp(t *testing.T, provider fakeProvider) *app.App {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.Provider = "fake"

	a, err := app.New(cfg)
	if err != nil {
		t.Fatalf("app.New() error = %v", err)
	}

	registry := providers.NewRegistry()
	registry.Register("fake", func(cfg config.Config, client *http.Client) (providers.Provider, error) {
		return provider, nil
	})
	a.Registry = registry

	return a
}

type fakeProvider struct {
	chapters []*source.Chapter
}

func (p fakeProvider) Name() string { return "fake" }

func (p fakeProvider) Search(context.Context, string) ([]*source.Manga, error) {
	return nil, nil
}

func (p fakeProvider) Chapters(_ context.Context, manga *source.Manga) ([]*source.Chapter, error) {
	for _, chapter := range p.chapters {
		if chapter != nil {
			chapter.From = manga
		}
	}
	return p.chapters, nil
}

func (p fakeProvider) Pages(context.Context, *source.Chapter) ([]*source.Page, error) {
	return nil, nil
}

func (p fakeProvider) Cover(context.Context, *source.Manga) (string, error) {
	return "", nil
}
