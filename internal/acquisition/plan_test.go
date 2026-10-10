package acquisition

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/source"
)

type fixtureProvider struct {
	randomCalls  int
	chapterCalls int
	titleCalls   int
	pageCalls    int
	manga        *source.Manga
	chapters     []*source.Chapter
	pages        []*source.Page
	unavailable  bool
}

func (p *fixtureProvider) Name() string { return "fixture" }
func (p *fixtureProvider) Info() source.ProviderInfo {
	return source.ProviderInfo{DownloadPermitted: true}
}
func (p *fixtureProvider) Search(context.Context, string) ([]*source.Manga, error) { return nil, nil }
func (p *fixtureProvider) Title(ctx context.Context, id string) (*source.Manga, error) {
	p.titleCalls++
	return p.manga, ctx.Err()
}
func (p *fixtureProvider) Chapters(ctx context.Context, m *source.Manga) ([]*source.Chapter, error) {
	p.chapterCalls++
	return p.chapters, ctx.Err()
}
func (p *fixtureProvider) Pages(ctx context.Context, c *source.Chapter) ([]*source.Page, error) {
	p.pageCalls++
	if p.unavailable {
		return nil, source.ErrCandidateUnavailable
	}
	return p.pages, ctx.Err()
}
func (p *fixtureProvider) Cover(context.Context, *source.Manga) (string, error) { return "", nil }

type randomFixture struct{ *fixtureProvider }

func (p randomFixture) RandomManga(ctx context.Context) (*source.Manga, error) {
	p.randomCalls++
	return p.manga, ctx.Err()
}

type catalogFixture struct{ *fixtureProvider }

func (p catalogFixture) BrowseManga(ctx context.Context, r source.BrowseRequest) (source.BrowsePage, error) {
	return source.BrowsePage{Titles: []source.BrowseTitle{{Manga: p.manga}, {Manga: p.manga}}, Total: 2}, ctx.Err()
}
func fixture() *fixtureProvider {
	return &fixtureProvider{manga: &source.Manga{ID: "m", Title: "Title", Metadata: source.MangaMetadata{Language: "ko"}}, chapters: []*source.Chapter{{ID: "c", Index: "1", Language: "en", PageCount: 2}}, pages: []*source.Page{{URL: "http://local/1.png"}, {URL: "http://local/2.png"}}}
}
func policy() SelectionPolicy {
	return SelectionPolicy{Provider: "fixture", OriginalLanguage: "ko", ChapterLanguage: "en", MinPages: 2, MaxAttempts: 3}
}

func TestOptionalCapabilities(t *testing.T) {
	for _, seed := range []*int64{nil, new(int64)} {
		p := policy()
		p.Seed = seed
		_, err := PlanRandom(context.Background(), fixture(), p, t.TempDir(), archive.FormatDirectory, nil)
		if err == nil || !strings.Contains(err.Error(), "unsupported") {
			t.Fatalf("error=%v", err)
		}
	}
}
func TestBoundedDuplicateCandidates(t *testing.T) {
	f := fixture()
	f.manga.Metadata.Language = "ja"
	var events []ProgressEvent
	_, err := PlanRandom(context.Background(), randomFixture{f}, policy(), t.TempDir(), archive.FormatDirectory, func(e ProgressEvent) { events = append(events, e) })
	if err == nil || f.randomCalls != 3 || f.chapterCalls != 0 || events[len(events)-1].Reason != "duplicate candidate" {
		t.Fatalf("calls=%d events=%v err=%v", f.randomCalls, events, err)
	}
}
func TestSelectionRejectsPagesAndUnavailableChapters(t *testing.T) {
	for _, mode := range []string{"mismatch", "unavailable", "too_few"} {
		t.Run(mode, func(t *testing.T) {
			f := fixture()
			if mode == "mismatch" {
				f.chapters[0].PageCount = 3
			}
			if mode == "unavailable" {
				f.unavailable = true
			}
			if mode == "too_few" {
				f.chapters[0].PageCount = 1
			}
			_, err := PlanRandom(context.Background(), randomFixture{f}, policy(), t.TempDir(), archive.FormatDirectory, nil)
			if err == nil || f.randomCalls != 3 || f.chapterCalls != 1 {
				t.Fatalf("calls=%d chapters=%d err=%v", f.randomCalls, f.chapterCalls, err)
			}
		})
	}
}
func TestSeededCatalogAndStableResolve(t *testing.T) {
	f := fixture()
	p := policy()
	seed := int64(9)
	p.Seed = &seed
	plan, err := PlanRandom(context.Background(), catalogFixture{f}, p, t.TempDir(), archive.FormatDirectory, nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Strategy != "seeded_catalog" || !reflect.DeepEqual(plan.CandidateIDs, []string{"m"}) || f.randomCalls != 0 {
		t.Fatalf("plan=%+v", plan)
	}
	if _, err := plan.Resolve(context.Background(), f, nil); err != nil {
		t.Fatal(err)
	}
	if f.titleCalls != 1 || f.randomCalls != 0 {
		t.Fatal("resolve rediscovered")
	}
	f.chapters = nil
	if _, err := plan.Resolve(context.Background(), f, nil); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("err=%v", err)
	}
}
func TestCancellationAtSelection(t *testing.T) {
	f := fixture()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := PlanRandom(ctx, randomFixture{f}, policy(), t.TempDir(), archive.FormatDirectory, func(e ProgressEvent) {
		if e.Stage == "selection" {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) || f.pageCalls != 0 {
		t.Fatalf("err=%v pages=%d", err, f.pageCalls)
	}
}
func TestManifestRoundTripAndInvalidInput(t *testing.T) {
	f := fixture()
	plan, err := PlanRandom(context.Background(), randomFixture{f}, policy(), t.TempDir(), archive.FormatPNG, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, err := NewManifest(*plan, nil, "planned", nil)
	if err != nil {
		t.Fatal(err)
	}
	path := t.TempDir() + "/manifest.json"
	if err := m.Write(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m, *loaded) {
		t.Fatalf("roundtrip=%+v expected=%+v", loaded, m)
	}
	m.Version = 99
	if m.Validate() == nil {
		t.Fatal("accepted version")
	}
	m.Version = ManifestVersion
	m.Images = []Image{{Index: 1, Path: "../0001.png", Format: "png"}}
	m.ActualPages = 1
	if m.Validate() == nil {
		t.Fatal("accepted traversal")
	}
}
func TestNumericConstraintsAndMissingMetadata(t *testing.T) {
	p := policy()
	lo, hi := 1.5, 2.5
	p.ChapterMin = &lo
	p.ChapterMax = &hi
	for _, number := range []string{"", "special", "1", "3", "NaN", "Inf"} {
		if p.MatchChapter(&source.Chapter{ID: "c", Language: "en", Index: number, PageCount: 2}) {
			t.Fatalf("accepted %q", number)
		}
	}
	if !p.MatchChapter(&source.Chapter{ID: "c", Language: "en", Index: "2", PageCount: 2}) {
		t.Fatal("rejected valid chapter")
	}
	if p.MatchTitle(&source.Manga{ID: "m"}) || p.MatchChapter(&source.Chapter{ID: "c", Index: "2"}) {
		t.Fatal("accepted missing metadata")
	}
	p.MaxAttempts = 0
	if p.Validate() == nil {
		t.Fatal("accepted invalid policy")
	}
}

type sequenceFixture struct {
	*fixtureProvider
	candidates []*source.Manga
}

func (p *sequenceFixture) RandomManga(ctx context.Context) (*source.Manga, error) {
	candidate := p.candidates[p.randomCalls%len(p.candidates)]
	p.randomCalls++
	return candidate, ctx.Err()
}
func TestRandomMovesPastRejectedAndDuplicateTitles(t *testing.T) {
	f := fixture()
	wrong := *f.manga
	wrong.ID = "wrong"
	wrong.Metadata.Language = "ja"
	p := &sequenceFixture{fixtureProvider: f, candidates: []*source.Manga{&wrong, &wrong, f.manga}}
	plan, err := PlanRandom(context.Background(), p, policy(), t.TempDir(), archive.FormatDirectory, nil)
	if err != nil || plan.Manga.ID != "m" || f.randomCalls != 3 || f.chapterCalls != 1 {
		t.Fatalf("plan=%+v calls=%d chapters=%d err=%v", plan, f.randomCalls, f.chapterCalls, err)
	}
}

type pagedCatalog struct {
	*fixtureProvider
	titles []*source.Manga
	calls  int
}

func (p *pagedCatalog) BrowseManga(ctx context.Context, r source.BrowseRequest) (source.BrowsePage, error) {
	p.calls++
	end := min(r.Offset+2, len(p.titles))
	page := source.BrowsePage{Offset: r.Offset, Total: len(p.titles), NextOffset: end, HasMore: end < len(p.titles)}
	for _, m := range p.titles[r.Offset:end] {
		page.Titles = append(page.Titles, source.BrowseTitle{Manga: m})
	}
	return page, ctx.Err()
}
func TestSeededSelectionRecordsBoundedMultiPageCatalog(t *testing.T) {
	f := fixture()
	var titles []*source.Manga
	for _, id := range []string{"c", "a", "b", "d"} {
		m := *f.manga
		m.ID = id
		titles = append(titles, &m)
	}
	p := &pagedCatalog{fixtureProvider: f, titles: titles}
	selection := policy()
	seed := int64(42)
	selection.Seed = &seed
	output := t.TempDir()
	first, err := PlanRandom(context.Background(), p, selection, output, archive.FormatDirectory, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := PlanRandom(context.Background(), p, selection, output, archive.FormatDirectory, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) || !reflect.DeepEqual(first.CandidateIDs, []string{"a", "b", "c"}) || p.calls != 4 || f.randomCalls != 0 {
		t.Fatalf("first=%+v second=%+v calls=%d", first, second, p.calls)
	}
}
func TestPlanningRejectsInvalidPageLocations(t *testing.T) {
	for _, page := range []*source.Page{nil, {URL: ""}, {URL: "file:///image.png"}, {URL: "https://secret:token@local/image.png"}} {
		f := fixture()
		f.pages[0] = page
		if _, err := PlanRandom(context.Background(), randomFixture{f}, policy(), t.TempDir(), archive.FormatDirectory, nil); err == nil {
			t.Fatal("accepted invalid page location")
		}
	}
}

func TestCancellationWhenSelectionIsFinalized(t *testing.T) {
	f := fixture()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	plan, err := PlanRandom(ctx, randomFixture{f}, policy(), t.TempDir(), archive.FormatDirectory, func(e ProgressEvent) {
		if e.Status == "selected" {
			cancel()
		}
	})
	if plan != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
}

type failingCandidateFixture struct {
	*sequenceFixture
	chapterErr error
	pageErr    error
	cancel     context.CancelFunc
	always     bool
}

func (p *failingCandidateFixture) Chapters(ctx context.Context, m *source.Manga) ([]*source.Chapter, error) {
	p.chapterCalls++
	if p.cancel != nil {
		p.cancel()
		return nil, ctx.Err()
	}
	if p.always || m.ID == "bad" {
		return nil, p.chapterErr
	}
	return p.chapters, ctx.Err()
}
func (p *failingCandidateFixture) Pages(ctx context.Context, c *source.Chapter) ([]*source.Page, error) {
	if p.pageErr != nil {
		p.pageCalls++
		return nil, p.pageErr
	}
	return p.fixtureProvider.Pages(ctx, c)
}
func TestCandidateChapterFailures(t *testing.T) {
	for _, mode := range []string{"unavailable", "permanent", "cancelled", "deadline", "exhausted", "page_auth"} {
		t.Run(mode, func(t *testing.T) {
			f := fixture()
			bad := *f.manga
			bad.ID = "bad"
			p := &failingCandidateFixture{sequenceFixture: &sequenceFixture{fixtureProvider: f, candidates: []*source.Manga{&bad, f.manga}}, chapterErr: fmt.Errorf("private URL secret: %w", source.ErrCandidateUnavailable)}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			fatal := errors.New("provider authentication failed")
			switch mode {
			case "permanent":
				p.chapterErr = fatal
			case "cancelled":
				p.cancel = cancel
			case "deadline":
				p.chapterErr = context.DeadlineExceeded
			case "exhausted":
				p.always = true
			case "page_auth":
				p.chapterErr = nil
				p.pageErr = fatal
				p.candidates = []*source.Manga{f.manga}
			}
			var events []ProgressEvent
			plan, err := PlanRandom(ctx, p, policy(), t.TempDir(), archive.FormatDirectory, func(e ProgressEvent) { events = append(events, e) })
			switch mode {
			case "unavailable":
				if err != nil || plan == nil || plan.Manga.ID != "m" || p.randomCalls != 2 {
					t.Fatalf("plan=%+v err=%v calls=%d", plan, err, p.randomCalls)
				}
			case "exhausted":
				if err == nil || !strings.Contains(err.Error(), "discovery attempts") || p.randomCalls != 3 || p.chapterCalls != 2 {
					t.Fatalf("err=%v calls=%d chapters=%d", err, p.randomCalls, p.chapterCalls)
				}
			default:
				expected := fatal
				if mode == "cancelled" {
					expected = context.Canceled
				}
				if mode == "deadline" {
					expected = context.DeadlineExceeded
				}
				if !errors.Is(err, expected) || p.randomCalls != 1 {
					t.Fatalf("err=%v calls=%d", err, p.randomCalls)
				}
			}
			for _, e := range events {
				if strings.Contains(e.Reason, "secret") {
					t.Fatal("unsanitized rejection")
				}
			}
		})
	}
}
