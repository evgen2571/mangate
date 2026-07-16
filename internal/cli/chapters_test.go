package cli

import (
	"bytes"
	"context"
	"errors"
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

func TestSearchCommandShowsDistinguishingMetadata(t *testing.T) {
	a := newTestApp(t, fakeProvider{searchResults: []*source.Manga{{
		ID: "stable-id", Title: "Primary Title", URL: "https://example.test/title/stable-id",
		Metadata: source.MangaMetadata{
			AlternativeTitle: "Alternative Title", ContentType: "safe", Status: "ongoing", Language: "ja", Year: 2024,
		},
	}}})

	cmd := NewSearchCmd(a)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"primary"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	for _, want := range []string{"1. Primary Title", "Provider: fake", "Alternative title: Alternative Title", "Content type: safe", "Status: ongoing", "Language: ja", "Year: 2024", "Reference: stable-id"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q\noutput:\n%s", want, out.String())
		}
	}
}

func TestSearchCommandFiltersLanguageAndContentType(t *testing.T) {
	a := newTestApp(t, fakeProvider{searchResults: []*source.Manga{
		{ID: "wanted", Title: "Wanted", Metadata: source.MangaMetadata{Language: "ja", ContentType: "safe"}},
		{ID: "wrong-language", Title: "Wrong Language", Metadata: source.MangaMetadata{Language: "en", ContentType: "safe"}},
		{ID: "wrong-type", Title: "Wrong Type", Metadata: source.MangaMetadata{Language: "ja", ContentType: "erotica"}},
	}})

	cmd := NewRootCmd(a)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--language", "ja", "search", "wanted", "--content-type", "safe"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(out.String(), "Wanted") || strings.Contains(out.String(), "Wrong Language") || strings.Contains(out.String(), "Wrong Type") {
		t.Fatalf("unexpected filtered output:\n%s", out.String())
	}
}

func TestSearchContentTypeHelpExplainsRepeatedValueSemantics(t *testing.T) {
	cmd := NewSearchCmd(newTestApp(t, fakeProvider{}))
	flag := cmd.Flags().Lookup("content-type")
	if flag == nil || !strings.Contains(flag.Usage, "any value matches") || !strings.Contains(flag.Usage, "duplicates ignored") {
		t.Fatalf("content-type help = %#v, want repeated-value semantics", flag)
	}
}

func TestSearchCommandRejectsInteractiveJSON(t *testing.T) {
	cmd := NewRootCmd(newTestApp(t, fakeProvider{}))
	cmd.SetArgs([]string{"--json", "search", "wanted", "--interactive"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--interactive cannot be combined with --json") {
		t.Fatalf("Execute() error = %v, want interactive JSON conflict", err)
	}
}

func TestQuietSuppressesHumanSearchOutput(t *testing.T) {
	a := newTestApp(t, fakeProvider{searchResults: []*source.Manga{{ID: "manga", Title: "Manga"}}})
	cmd := NewRootCmd(a)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--quiet", "search", "manga"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := out.String(); got != "" {
		t.Fatalf("quiet output = %q, want empty", got)
	}
}

func TestQuietDoesNotSuppressJSONSearchOutput(t *testing.T) {
	a := newTestApp(t, fakeProvider{searchResults: []*source.Manga{{ID: "manga", Title: "Manga"}}})
	cmd := NewRootCmd(a)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--quiet", "--json", "search", "manga"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), `"operation":"search"`) {
		t.Fatalf("JSON output = %q, want search envelope", out.String())
	}
}

func TestSearchCommandReportsNoResults(t *testing.T) {
	cmd := NewSearchCmd(newTestApp(t, fakeProvider{}))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"missing"})
	err := cmd.Execute()
	var reported *ReportedError
	if !errors.As(err, &reported) || reported.Code != 1 || !reported.Silent {
		t.Fatalf("Execute() error = %#v, want silent no-results exit", err)
	}
	if !strings.Contains(out.String(), "no results found for") {
		t.Fatalf("output = %q, want no-results message", out.String())
	}
}

func newTestApp(t *testing.T, provider fakeProvider) *app.App {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.Provider = "fake"

	registry := providers.NewRegistry()
	registry.Register("fake", func(cfg config.Config, client *http.Client) (providers.Provider, error) {
		return provider, nil
	})

	a, err := app.New(cfg, app.WithRegistry(registry))
	if err != nil {
		t.Fatalf("app.New() error = %v", err)
	}

	return a
}

type fakeProvider struct {
	chapters      []*source.Chapter
	searchResults []*source.Manga
}

func (p fakeProvider) Name() string { return "fake" }

func (p fakeProvider) Info() source.ProviderInfo {
	return source.ProviderInfo{ID: "fake", Name: "Fake", Availability: "available"}
}

func (p fakeProvider) Title(_ context.Context, id string) (*source.Manga, error) {
	return &source.Manga{ID: id, Title: id}, nil
}

func (p fakeProvider) Search(context.Context, string) ([]*source.Manga, error) {
	return p.searchResults, nil
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
