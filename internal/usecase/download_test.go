package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/evgen2571/mangate/internal/source"
)

func TestBuildDownloadMangaLeavesPagesForLazyLoading(t *testing.T) {
	manga := &source.Manga{ID: "manga-id", Title: "My Manga"}
	chapter := &source.Chapter{
		ID:        "chapter-1",
		Index:     "1",
		Title:     "Intro",
		PageCount: 23,
		Pages:     []*source.Page{{URL: "https://example.com/page.png"}},
	}

	downloadManga, err := buildDownloadManga(manga, []*source.Chapter{chapter})
	if err != nil {
		t.Fatalf("buildDownloadManga() error = %v", err)
	}
	if len(downloadManga.Chapters) != 1 {
		t.Fatalf("len(downloadManga.Chapters) = %d, want 1", len(downloadManga.Chapters))
	}

	downloadChapter := downloadManga.Chapters[0]
	if len(downloadChapter.Pages) != 0 {
		t.Fatalf("len(downloadChapter.Pages) = %d, want 0 for lazy loading", len(downloadChapter.Pages))
	}
	if downloadChapter.PageCount != 23 {
		t.Fatalf("download chapter PageCount = %d, want 23", downloadChapter.PageCount)
	}
	if downloadChapter == chapter {
		t.Fatalf("download chapter reuses original pointer")
	}
	if downloadChapter.From != downloadManga {
		t.Fatalf("download chapter parent = %p, want download manga %p", downloadChapter.From, downloadManga)
	}
	if len(chapter.Pages) != 1 {
		t.Fatalf("original chapter pages were mutated")
	}
}

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

func TestServiceDownloadChaptersOrchestratesProviderPagesAndDownloader(t *testing.T) {
	manga := &source.Manga{ID: "manga-id", Title: "Manga"}
	chapter := &source.Chapter{ID: "chapter-1", Index: "1", Title: "One"}
	pages := []*source.Page{{URL: "https://example.com/1.jpg"}}
	progress := DownloadProgress{
		CompletedPages:    1,
		TotalPages:        1,
		CompletedChapters: 1,
		TotalChapters:     1,
		Chapters: []ChapterDownloadProgress{{
			Name:           "Chapter 1: One",
			CompletedPages: 1,
			TotalPages:     1,
			Completed:      true,
		}},
	}
	provider := &fakeProvider{pages: pages}
	downloader := &fakeMangaDownloader{progress: progress}
	service := New(Deps{
		ProviderResolver: fakeProviderResolver{provider: provider},
		Downloader:       downloader,
	})

	var gotProgress DownloadProgress
	err := service.DownloadChapters(context.Background(), manga, []*source.Chapter{chapter}, func(progress DownloadProgress) {
		gotProgress = progress
	})
	if err != nil {
		t.Fatalf("DownloadChapters() error = %v", err)
	}
	if downloader.manga == nil {
		t.Fatalf("downloader manga was nil")
	}
	if downloader.manga == manga {
		t.Fatalf("downloader got original manga pointer, want defensive download model")
	}
	if len(downloader.manga.Chapters) != 1 {
		t.Fatalf("len(downloader.manga.Chapters) = %d, want 1", len(downloader.manga.Chapters))
	}
	if downloader.manga.Chapters[0].From != downloader.manga {
		t.Fatalf("download chapter parent = %p, want download manga %p", downloader.manga.Chapters[0].From, downloader.manga)
	}
	if !reflect.DeepEqual(provider.pagesChapter, downloader.manga.Chapters[0]) {
		t.Fatalf("provider pages chapter = %#v, want downloader chapter %#v", provider.pagesChapter, downloader.manga.Chapters[0])
	}
	if !reflect.DeepEqual(downloader.loadedPages, pages) {
		t.Fatalf("page loader pages = %#v, want %#v", downloader.loadedPages, pages)
	}
	if !reflect.DeepEqual(gotProgress, progress) {
		t.Fatalf("progress = %#v, want %#v", gotProgress, progress)
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
	pages         []*source.Page
	pagesChapter  *source.Chapter
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

func (p *fakeProvider) Pages(_ context.Context, chapter *source.Chapter) ([]*source.Page, error) {
	p.pagesChapter = chapter
	return p.pages, nil
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

type fakeMangaDownloader struct {
	manga       *source.Manga
	loadedPages []*source.Page
	progress    DownloadProgress
}

func (d *fakeMangaDownloader) DownloadManga(ctx context.Context, manga *source.Manga, pageLoader PageLoader, notify func(DownloadProgress)) error {
	d.manga = manga
	if len(manga.Chapters) > 0 {
		pages, err := pageLoader(ctx, manga.Chapters[0])
		if err != nil {
			return err
		}
		d.loadedPages = pages
	}
	if notify != nil {
		notify(d.progress)
	}
	return nil
}
