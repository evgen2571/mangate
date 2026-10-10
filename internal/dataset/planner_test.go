package dataset

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/evgen2571/mangate/internal/source"
)

type planningProvider struct {
	datasetProvider
	candidates  []source.BrowseTitle
	browseCalls int
}

func (p *planningProvider) BrowseManga(context.Context, source.BrowseRequest) (source.BrowsePage, error) {
	p.browseCalls++
	return source.BrowsePage{Titles: p.candidates}, nil
}

func TestBuildPlanPreservesMetadataAndReusesSavedPlan(t *testing.T) {
	ctx := context.Background()
	cfg := DefaultConfig(t.TempDir(), "fake")
	cfg.Discovery.CandidatePoolSize = 2
	cfg.Sampling.MaxTitles = 1
	cfg.Sampling.TitleStrategy = "sequential"
	candidate := source.BrowseTitle{
		Manga: &source.Manga{
			ID: "selected", Title: "Selected title", URL: "https://example.test/title",
			Metadata: source.MangaMetadata{
				AlternativeTitle: "Alternative", Language: "ko", Status: "completed",
				ContentType: "safe", Year: 2020,
			},
		},
		Tags: []string{"Drama"}, AvailableLanguages: []string{"en"},
		CreatedAt: "created", UpdatedAt: "updated",
	}
	provider := &planningProvider{candidates: []source.BrowseTitle{
		{}, {Manga: &source.Manga{ID: " "}}, candidate, candidate,
		{Manga: &source.Manga{ID: "discovered", Title: "Other title"}},
	}}
	store, err := Open(cfg.Output.Directory)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Initialize(ctx, cfg, false); err != nil {
		t.Fatal(err)
	}

	plan, err := BuildPlan(ctx, store, provider, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Candidates != 2 || plan.Titles != 1 || plan.Chapters != 1 || plan.EstimatedPages != 1 {
		t.Fatalf("plan = %#v", plan)
	}
	selected := store.data.Titles["selected"]
	if selected.Name != candidate.Manga.Title || selected.URL != candidate.Manga.URL ||
		selected.AlternativeTitle != "Alternative" || selected.OriginalLanguage != "ko" ||
		selected.Status != "completed" || selected.ContentRating != "safe" || selected.Year != 2020 ||
		selected.DiscoveryOrder != 0 || selected.SampleRank != 0 || selected.Stratum != "2020s|completed" ||
		selected.State != "planned" || selected.ProviderCreatedAt != "created" || selected.ProviderUpdatedAt != "updated" ||
		!reflect.DeepEqual(selected.Tags, []string{"Drama"}) || !reflect.DeepEqual(selected.AvailableLanguages, []string{"en"}) {
		t.Fatalf("persisted title = %#v", selected)
	}
	if store.data.Titles["discovered"].State != "discovered" {
		t.Fatal("unselected candidate was not retained")
	}

	reopened, err := OpenExisting(cfg.Output.Directory)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	provider.candidates = nil
	reused, err := BuildPlan(ctx, reopened, provider, cfg)
	want, wantErr := json.Marshal(plan)
	got, gotErr := json.Marshal(reused)
	if err != nil || wantErr != nil || gotErr != nil || string(got) != string(want) || provider.browseCalls != 1 {
		t.Fatalf("saved plan = %#v, err = %v, browse calls = %d", reused, err, provider.browseCalls)
	}
}
