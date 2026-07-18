package dataset

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/source"
)

func TestConfigValidationAndStableHash(t *testing.T) {
	cfg := DefaultConfig(filepath.Join(t.TempDir(), "set"), "fake")
	cfg.Limits.MaxPages = 10
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	first, err := cfg.Hash()
	if err != nil {
		t.Fatal(err)
	}
	second, err := cfg.Hash()
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("unstable hash: %s != %s", first, second)
	}
	cfg.Output.Format = archive.Format("rar")
	if err := cfg.Normalize(); err == nil {
		t.Fatal("expected invalid format")
	}
}

func TestDatasetInfoUsesStableJSONFieldNames(t *testing.T) {
	data, err := json.Marshal(DatasetInfo{ConfigHash: "hash", State: "planned", StoppingReason: "limit", CreatedAt: "created", UpdatedAt: "updated", Counters: Counters{ValidPages: 2, StoredBytes: 3}})
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	if value["configurationHash"] != "hash" || value["stoppingReason"] != "limit" {
		t.Fatalf("dataset info JSON = %#v", value)
	}
	counters, ok := value["counters"].(map[string]any)
	if !ok || counters["validPages"] != float64(2) || counters["storedBytes"] != float64(3) {
		t.Fatalf("counter JSON = %#v", value["counters"])
	}
	if _, exists := value["ConfigHash"]; exists {
		t.Fatalf("legacy Go field leaked into JSON: %#v", value)
	}
}

func TestOpenMigratesVersionOneDatabase(t *testing.T) {
	root := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(root, "dataset.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE dataset_meta (id INTEGER PRIMARY KEY, schema_version INTEGER NOT NULL, config_json TEXT NOT NULL, config_hash TEXT NOT NULL, state TEXT NOT NULL, stopping_reason TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL, completed_at TEXT, final_error TEXT NOT NULL DEFAULT ''); CREATE TABLE chapters (id TEXT PRIMARY KEY); CREATE TABLE pages (chapter_id TEXT, page_index INTEGER, PRIMARY KEY(chapter_id,page_index));`)
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, column := range []string{"claim_owner", "claimed_at"} {
		var name string
		err := store.db.QueryRowContext(context.Background(), "SELECT name FROM pragma_table_info('chapters') WHERE name=?", column).Scan(&name)
		if err != nil {
			t.Fatalf("chapter column %s: %v", column, err)
		}
	}
	var name string
	for _, column := range []string{"near_duplicate_of", "source_mime_type", "extension"} {
		if err := store.db.QueryRowContext(context.Background(), "SELECT name FROM pragma_table_info('pages') WHERE name=?", column).Scan(&name); err != nil {
			t.Fatalf("page column %s: %v", column, err)
		}
	}
	for _, column := range []string{"alternative_title", "tags_json", "available_languages_json", "provider_created_at", "provider_updated_at"} {
		if err := store.db.QueryRowContext(context.Background(), "SELECT name FROM pragma_table_info('titles') WHERE name=?", column).Scan(&name); err != nil {
			t.Fatalf("title column %s: %v", column, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "dataset.sqlite")); err != nil {
		t.Fatal(err)
	}
}

func TestRepairReleasesInterruptedChapterClaim(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultConfig(root, "fake")
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Initialize(context.Background(), cfg, false); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplacePlan(context.Background(), []Title{{ID: "title", Name: "Title", SampleRank: 0}}, []Title{{ID: "title", Name: "Title", SampleRank: 0}}, []Chapter{{ID: "chapter", TitleID: "title", ProviderOrder: 0}}); err != nil {
		t.Fatal(err)
	}
	claimed, err := store.ClaimChapter(context.Background(), "chapter", "worker")
	if err != nil || !claimed {
		t.Fatalf("claim = %v, %v", claimed, err)
	}
	if _, err := Verify(context.Background(), store, true); err != nil {
		t.Fatal(err)
	}
	var state string
	var owner sqlString
	if err := store.db.QueryRow("SELECT state,claim_owner FROM chapters WHERE id='chapter'").Scan(&state, &owner); err != nil {
		t.Fatal(err)
	}
	if state != "partial" || owner.Valid {
		t.Fatalf("claim was not released: state=%q owner=%#v", state, owner)
	}
	var attemptCount int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM attempts WHERE entity_type='chapter' AND entity_id='chapter'").Scan(&attemptCount); err != nil {
		t.Fatal(err)
	}
	if attemptCount != 1 {
		t.Fatalf("chapter attempts = %d, want 1", attemptCount)
	}
}

func TestBuildPlanReusesPersistedCandidateAndChapterSelection(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultConfig(root, "fake")
	cfg.Sampling.MaxTitles = 1
	cfg.Sampling.MaxChaptersPerTitle = 1
	cfg.Discovery.CandidatePoolSize = 1
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Initialize(context.Background(), cfg, false); err != nil {
		t.Fatal(err)
	}
	first, err := BuildPlan(context.Background(), store, datasetProvider{}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildPlan(context.Background(), store, nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if first.Candidates != 1 || second.Candidates != first.Candidates || second.Titles != first.Titles || second.Chapters != first.Chapters || second.EstimatedPages != first.EstimatedPages || len(second.Warnings) != len(first.Warnings) {
		t.Fatalf("plan changed on resume: first=%#v second=%#v", first, second)
	}
}

type noChapterDatasetProvider struct{ datasetProvider }

func (noChapterDatasetProvider) Chapters(context.Context, *source.Manga) ([]*source.Chapter, error) {
	return nil, nil
}

type boundedTitlePlanningProvider struct {
	datasetProvider
	started chan<- struct{}
	release <-chan struct{}
	active  atomic.Int32
	maximum atomic.Int32
}

func (p *boundedTitlePlanningProvider) BrowseManga(context.Context, source.BrowseRequest) (source.BrowsePage, error) {
	return source.BrowsePage{Titles: []source.BrowseTitle{
		{Manga: &source.Manga{ID: "title-1", Title: "One", Metadata: source.MangaMetadata{Language: "ko", Status: "ongoing"}}},
		{Manga: &source.Manga{ID: "title-2", Title: "Two", Metadata: source.MangaMetadata{Language: "ko", Status: "ongoing"}}},
		{Manga: &source.Manga{ID: "title-3", Title: "Three", Metadata: source.MangaMetadata{Language: "ko", Status: "ongoing"}}},
	}}, nil
}

func (p *boundedTitlePlanningProvider) Chapters(_ context.Context, manga *source.Manga) ([]*source.Chapter, error) {
	current := p.active.Add(1)
	for {
		observed := p.maximum.Load()
		if current <= observed || p.maximum.CompareAndSwap(observed, current) {
			break
		}
	}
	p.started <- struct{}{}
	<-p.release
	p.active.Add(-1)
	return []*source.Chapter{{ID: "chapter-" + manga.ID, Index: "1", Language: "en", PageCount: 1}}, nil
}

func TestBuildPlanWarnsWhenSelectedTitleHasNoMatchingChapters(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultConfig(root, "fake")
	cfg.Sampling.MaxTitles, cfg.Discovery.CandidatePoolSize = 1, 1
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Initialize(context.Background(), cfg, false); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(context.Background(), store, noChapterDatasetProvider{}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Warnings) != 1 || plan.Warnings[0] != `title "title" has no chapters matching the collection filters` {
		t.Fatalf("plan warnings = %#v", plan.Warnings)
	}
	resumed, err := BuildPlan(context.Background(), store, nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(resumed.Warnings) != 1 || resumed.Warnings[0] != plan.Warnings[0] {
		t.Fatalf("resumed plan warnings = %#v", resumed.Warnings)
	}
}

func TestBuildPlanBoundsConcurrentTitleMetadataRequests(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultConfig(root, "fake")
	cfg.Sampling.MaxTitles, cfg.Sampling.MaxChaptersPerTitle, cfg.Discovery.CandidatePoolSize = 3, 1, 3
	cfg.Runtime.TitleWorkers = 2
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Initialize(context.Background(), cfg, false); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{}, 3)
	release := make(chan struct{})
	provider := &boundedTitlePlanningProvider{started: started, release: release}
	result := make(chan error, 1)
	go func() {
		_, err := BuildPlan(context.Background(), store, provider, cfg)
		result <- err
	}()
	for range 2 {
		<-started
	}
	close(release)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if provider.maximum.Load() != 2 {
		t.Fatalf("maximum metadata requests = %d", provider.maximum.Load())
	}
}

func TestUniformChapterSamplingSortsUnorderedProviderResults(t *testing.T) {
	cfg := DefaultConfig(t.TempDir(), "fake")
	cfg.Discovery.ChapterLanguages = []string{"en"}
	cfg.Sampling.ChapterStrategy = "uniform"
	cfg.Sampling.MaxChaptersPerTitle = 3
	chapters := []*source.Chapter{
		{ID: "chapter-10", Index: "10", Language: "en"},
		{ID: "chapter-1", Index: "1", Language: "en"},
		{ID: "chapter-5", Index: "5", Language: "en"},
		{ID: "chapter-7", Index: "7", Language: "en"},
		{ID: "chapter-3", Index: "3", Language: "en"},
	}
	selected := sampleChapters(chapters, cfg)
	if len(selected) != 3 || selected[0].ID != "chapter-1" || selected[1].ID != "chapter-5" || selected[2].ID != "chapter-10" {
		t.Fatalf("uniform selection = %#v", selected)
	}
}

func TestOpenExistingDoesNotCreateMissingDataset(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	if _, err := OpenExisting(root); err == nil {
		t.Fatal("expected missing dataset error")
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("OpenExisting created root: %v", err)
	}
}

func TestVerifyReportsAndRepairsAbandonedParts(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultConfig(root, "fake")
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Initialize(context.Background(), cfg, false); err != nil {
		t.Fatal(err)
	}
	part := filepath.Join(root, "data", "abandoned.jpg.part")
	if err := os.MkdirAll(filepath.Dir(part), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(part, []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Verify(context.Background(), store, false)
	if err != nil {
		t.Fatal(err)
	}
	if result["temporaryFiles"] != 1 || result["valid"] != false {
		t.Fatalf("verify result = %#v", result)
	}
	result, err = Verify(context.Background(), store, true)
	if err != nil {
		t.Fatal(err)
	}
	if result["temporaryFiles"] != 0 || result["valid"] != true {
		t.Fatalf("repair result = %#v", result)
	}
	if _, err := os.Stat(part); !os.IsNotExist(err) {
		t.Fatalf("temporary file remained: %v", err)
	}
}

func TestVerifyReportsAndRepairsAbandonedArchiveStaging(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultConfig(root, "fake")
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Initialize(context.Background(), cfg, false); err != nil {
		t.Fatal(err)
	}
	staging := filepath.Join(root, ".staging", "fake", "title", "chapter")
	page := filepath.Join(staging, "0001.jpg")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(page, []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Verify(context.Background(), store, false)
	if err != nil {
		t.Fatal(err)
	}
	if result["stagingDirectories"] != 1 || result["valid"] != false {
		t.Fatalf("verify result = %#v", result)
	}
	result, err = Verify(context.Background(), store, true)
	if err != nil {
		t.Fatal(err)
	}
	if result["stagingDirectories"] != 0 || result["valid"] != true {
		t.Fatalf("repair result = %#v", result)
	}
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("staging directory remained: %v", err)
	}
}

func TestVerifyReportsUnexpectedDataWithoutDeletingIt(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultConfig(root, "fake")
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Initialize(context.Background(), cfg, false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "data", "fake", "title", "extra.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("unexpected"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Verify(context.Background(), store, true)
	if err != nil {
		t.Fatal(err)
	}
	if result["unexpectedFiles"] != 1 || result["valid"] != false {
		t.Fatalf("verify result = %#v", result)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("repair removed unexpected content: %v", err)
	}
}

func TestVerifyReportsAndRepairsIncompleteCompletedChapter(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultConfig(root, "fake")
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Initialize(context.Background(), cfg, false); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplacePlan(context.Background(), []Title{{ID: "title", Name: "Title"}}, []Title{{ID: "title", Name: "Title", SampleRank: 0}}, []Chapter{{ID: "chapter", TitleID: "title", ExpectedPages: 1}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("UPDATE chapters SET state='completed'; UPDATE titles SET state='completed'"); err != nil {
		t.Fatal(err)
	}
	result, err := Verify(context.Background(), store, false)
	if err != nil {
		t.Fatal(err)
	}
	if result["stateInconsistencies"] != 1 || result["valid"] != false {
		t.Fatalf("verify result = %#v", result)
	}
	result, err = Verify(context.Background(), store, true)
	if err != nil {
		t.Fatal(err)
	}
	if result["stateInconsistencies"] != 0 || result["valid"] != true {
		t.Fatalf("repair result = %#v", result)
	}
	var chapterState, titleState string
	if err := store.db.QueryRow("SELECT state FROM chapters WHERE id='chapter'").Scan(&chapterState); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow("SELECT state FROM titles WHERE id='title'").Scan(&titleState); err != nil {
		t.Fatal(err)
	}
	if chapterState != "partial" || titleState != "partial" {
		t.Fatalf("repaired states: chapter=%q title=%q", chapterState, titleState)
	}
}

func TestVerifyRejectsPageThatViolatesConfiguredOutputFormat(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultConfig(root, "fake")
	cfg.Output.Format = archive.FormatPNG
	cfg.Validation.MinimumWidth, cfg.Validation.MinimumHeight = 1, 1
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Initialize(context.Background(), cfg, false); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplacePlan(context.Background(), []Title{{ID: "title", Name: "Title"}}, []Title{{ID: "title", Name: "Title", SampleRank: 0}}, []Chapter{{ID: "chapter", TitleID: "title", ExpectedPages: 1}}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "data", "fake", "title", "chapter", "0001.jpg")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, newPNG(t), 0o644); err != nil {
		t.Fatal(err)
	}
	image, _, err := ValidateFile(path, cfg.Validation)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordPage(context.Background(), Page{TitleID: "title", ChapterID: "chapter", Index: 1, StorageType: "file", RelativePath: "data/fake/title/chapter/0001.jpg", MIMEType: image.MIMEType, Width: image.Width, Height: image.Height, Bytes: image.Bytes, SHA256: image.SHA256, State: "valid"}); err != nil {
		t.Fatal(err)
	}
	result, err := Verify(context.Background(), store, false)
	if err != nil {
		t.Fatal(err)
	}
	if result["invalidPages"] != 1 || result["valid"] != false {
		t.Fatalf("verify result = %#v", result)
	}
}

func TestFailuresIncludesChapterFailures(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultConfig(root, "fake")
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Initialize(context.Background(), cfg, false); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplacePlan(context.Background(), []Title{{ID: "title", Name: "Title"}}, []Title{{ID: "title", Name: "Title", SampleRank: 0}}, []Chapter{{ID: "chapter", TitleID: "title", ExpectedPages: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := store.FailChapter(context.Background(), "chapter", "provider response failed"); err != nil {
		t.Fatal(err)
	}
	if err := Failures(context.Background(), store); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "reports", "failures.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if record["entityType"] != "chapter" || record["chapterId"] != "chapter" || record["message"] != "provider response failed" {
		t.Fatalf("failure record = %#v", record)
	}
	if _, ok := record["pageIndex"]; ok {
		t.Fatalf("chapter record has a page index: %#v", record)
	}
}

func TestRecordPageStoresValidationFailureMessage(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultConfig(root, "fake")
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Initialize(context.Background(), cfg, false); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplacePlan(context.Background(), []Title{{ID: "title", Name: "Title"}}, []Title{{ID: "title", Name: "Title", SampleRank: 0}}, []Chapter{{ID: "chapter", TitleID: "title", ExpectedPages: 1}}); err != nil {
		t.Fatal(err)
	}
	page := Page{TitleID: "title", ChapterID: "chapter", Index: 1, State: "rejected", RejectionCode: "width_too_small", ErrorMessage: "width 12 is below 256"}
	if err := store.RecordPage(context.Background(), page); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordPage(context.Background(), page); err != nil {
		t.Fatal(err)
	}
	var message string
	var attempts int
	if err := store.db.QueryRow("SELECT last_error,attempts FROM pages WHERE chapter_id='chapter' AND page_index=1").Scan(&message, &attempts); err != nil {
		t.Fatal(err)
	}
	if message != "width 12 is below 256" || attempts != 2 {
		t.Fatalf("page state: last error=%q attempts=%d", message, attempts)
	}
	var attemptRows, firstAttempt, lastAttempt int
	if err := store.db.QueryRow("SELECT COUNT(*),MIN(attempt),MAX(attempt) FROM attempts WHERE entity_type='page' AND entity_id='chapter:1'").Scan(&attemptRows, &firstAttempt, &lastAttempt); err != nil {
		t.Fatal(err)
	}
	if attemptRows != 2 || firstAttempt != 1 || lastAttempt != 2 {
		t.Fatalf("attempt history: rows=%d first=%d last=%d", attemptRows, firstAttempt, lastAttempt)
	}
}

func TestConfigRequiresStoppingCondition(t *testing.T) {
	cfg := DefaultConfig(t.TempDir(), "fake")
	cfg.Sampling.MaxTitles, cfg.Limits.MaxPages, cfg.Limits.MaxBytes = 0, 0, 0
	if err := cfg.Normalize(); err == nil {
		t.Fatal("expected stopping condition error")
	}
}
