package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/evgen2571/mangate/internal/source"
)

func TestServiceUsesProviderPortForSearchChaptersAndCover(t *testing.T) {
	manga := &source.Manga{ID: "manga-id", Title: "Manga"}
	chapters := []*source.Chapter{{ID: "chapter-1", Title: "One"}}
	provider := &fakeProvider{
		searchResults: []*source.Manga{manga},
		chapters:      chapters,
	}
	cache := &fakeCoverCache{path: "/tmp/cover.img"}
	service := New(Deps{
		ProviderResolver: fakeProviderResolver{provider: provider},
		Cache:            cache,
	})

	gotManga, err := service.SearchManga(context.Background(), "query")
	if err != nil {
		t.Fatalf("SearchManga() error = %v", err)
	}
	if !reflect.DeepEqual(gotManga, provider.searchResults) {
		t.Fatalf("SearchManga() = %#v, want %#v", gotManga, provider.searchResults)
	}
	if provider.searchQuery != "query" {
		t.Fatalf("provider search query = %q, want query", provider.searchQuery)
	}

	gotChapters, err := service.Chapters(context.Background(), manga)
	if err != nil {
		t.Fatalf("Chapters() error = %v", err)
	}
	if !reflect.DeepEqual(gotChapters, chapters) {
		t.Fatalf("Chapters() = %#v, want %#v", gotChapters, chapters)
	}
	if provider.chaptersManga != manga {
		t.Fatalf("provider chapters manga = %p, want %p", provider.chaptersManga, manga)
	}

	gotPath, err := service.CoverPath(context.Background(), manga)
	if err != nil {
		t.Fatalf("CoverPath() error = %v", err)
	}
	if gotPath != cache.path {
		t.Fatalf("CoverPath() = %q, want %q", gotPath, cache.path)
	}
	if cache.provider != provider {
		t.Fatalf("cache provider = %p, want %p", cache.provider, provider)
	}
	if cache.manga != manga {
		t.Fatalf("cache manga = %p, want %p", cache.manga, manga)
	}
}

func TestServiceChaptersByIDBuildsIDOnlyManga(t *testing.T) {
	chapters := []*source.Chapter{{ID: "chapter-1", Title: "One"}}
	provider := &fakeProvider{chapters: chapters}
	service := New(Deps{ProviderResolver: fakeProviderResolver{provider: provider}})

	gotChapters, err := service.ChaptersByID(context.Background(), "  manga-123  ")
	if err != nil {
		t.Fatalf("ChaptersByID() error = %v", err)
	}
	if !reflect.DeepEqual(gotChapters, chapters) {
		t.Fatalf("ChaptersByID() = %#v, want %#v", gotChapters, chapters)
	}
	if provider.chaptersManga == nil {
		t.Fatalf("provider chapters manga was nil")
	}
	if provider.chaptersManga.ID != "manga-123" {
		t.Fatalf("provider manga ID = %q, want manga-123", provider.chaptersManga.ID)
	}
	if provider.chaptersManga.Title != "manga-123" {
		t.Fatalf("provider manga Title = %q, want manga-123 fallback title", provider.chaptersManga.Title)
	}
}

func TestServiceChaptersByIDRejectsBlankID(t *testing.T) {
	service := New(Deps{ProviderResolver: fakeProviderResolver{provider: &fakeProvider{}}})

	_, err := service.ChaptersByID(context.Background(), " \t\n ")
	if err == nil {
		t.Fatal("ChaptersByID() error = nil, want error")
	}
	if err.Error() != "load chapters: manga id cannot be empty" {
		t.Fatalf("ChaptersByID() error = %q, want empty manga id error", err)
	}
}

func TestCoverCachePortAcceptsCoverOnlyProvider(t *testing.T) {
	provider := &fakeCoverProvider{}
	cache := &fakeCoverCache{path: "/tmp/cover.img"}
	manga := &source.Manga{ID: "manga-id", Title: "Manga"}

	gotPath, err := cache.Get(context.Background(), provider, manga)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if gotPath != cache.path {
		t.Fatalf("Get() = %q, want %q", gotPath, cache.path)
	}
	if cache.provider != provider {
		t.Fatalf("cache provider = %p, want %p", cache.provider, provider)
	}
}

func TestServicePropagatesProviderResolverError(t *testing.T) {
	resolverErr := errors.New("resolver failed")
	service := New(Deps{ProviderResolver: fakeProviderResolver{err: resolverErr}})

	_, err := service.SearchManga(context.Background(), "query")
	if !errors.Is(err, resolverErr) {
		t.Fatalf("SearchManga() error = %v, want %v", err, resolverErr)
	}
}

type fakeProviderResolver struct {
	provider Provider
	err      error
}

func (r fakeProviderResolver) Provider() (Provider, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.provider, nil
}

type fakeProvider struct {
	searchResults []*source.Manga
	searchQuery   string
	chapters      []*source.Chapter
	chaptersManga *source.Manga
}

func (p *fakeProvider) Name() string {
	return "fake"
}

func (p *fakeProvider) Search(_ context.Context, query string) ([]*source.Manga, error) {
	p.searchQuery = query
	return p.searchResults, nil
}

func (p *fakeProvider) Chapters(_ context.Context, manga *source.Manga) ([]*source.Chapter, error) {
	p.chaptersManga = manga
	return p.chapters, nil
}

func (p *fakeProvider) Cover(context.Context, *source.Manga) (string, error) {
	return "https://example.com/cover.jpg", nil
}

type fakeCoverProvider struct{}

func (p *fakeCoverProvider) Name() string {
	return "fake"
}

func (p *fakeCoverProvider) Cover(context.Context, *source.Manga) (string, error) {
	return "https://example.com/cover.jpg", nil
}

type fakeCoverCache struct {
	path     string
	provider CoverProvider
	manga    *source.Manga
}

func (c *fakeCoverCache) Get(_ context.Context, provider CoverProvider, manga *source.Manga) (string, error) {
	c.provider = provider
	c.manga = manga
	return c.path, nil
}
