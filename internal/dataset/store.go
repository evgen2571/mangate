package dataset

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const SchemaVersion = 1

type Store struct {
	root string
	mu   sync.RWMutex
	data datasetState
}

type datasetState struct {
	SchemaVersion  int                `json:"schemaVersion"`
	Config         Config             `json:"config"`
	ConfigHash     string             `json:"configurationHash"`
	State          string             `json:"state"`
	StoppingReason string             `json:"stoppingReason,omitempty"`
	CreatedAt      string             `json:"createdAt"`
	UpdatedAt      string             `json:"updatedAt"`
	Titles         map[string]Title   `json:"titles"`
	Chapters       map[string]Chapter `json:"chapters"`
	Pages          map[string]Page    `json:"pages"`
	Attempts       []Attempt          `json:"attempts,omitempty"`
}

type Attempt struct {
	EntityType string `json:"entityType"`
	EntityID   string `json:"entityId"`
	Operation  string `json:"operation"`
	Attempt    int    `json:"attempt"`
	Retryable  bool   `json:"retryable"`
	Message    string `json:"message,omitempty"`
	CreatedAt  string `json:"createdAt"`
}

type Title struct {
	ID, Name, URL, AlternativeTitle, OriginalLanguage, Status, ContentRating, Stratum, Split, State, ProviderCreatedAt, ProviderUpdatedAt string
	Tags, AvailableLanguages                                                                                                              []string
	Year, DiscoveryOrder, SampleRank                                                                                                      int
}
type Chapter struct {
	ID, TitleID, Number, Name, Volume, Language, ReleaseGroup, PublishedAt, URL, State, OutputPath, ArchivePath, ClaimOwner, LastError string
	ExpectedPages, ProviderOrder, Attempts                                                                                             int
}
type Page struct {
	TitleID, ChapterID, RelativePath, ArchiveEntry, StorageType, SourceMIMEType, MIMEType, Extension, SHA256, PerceptualHash, State, RejectionCode, ErrorMessage, ExactDuplicateOf, NearDuplicateOf, Split, DownloadedAt, ValidatedAt string
	Index, Width, Height, Attempts                                                                                                                                                                                                    int
	Bytes                                                                                                                                                                                                                             int64
}
type Counters struct {
	DiscoveredTitles  int   `json:"discoveredTitles"`
	PlannedTitles     int   `json:"plannedTitles"`
	CompletedTitles   int   `json:"completedTitles"`
	FailedTitles      int   `json:"failedTitles"`
	PlannedChapters   int   `json:"plannedChapters"`
	CompletedChapters int   `json:"completedChapters"`
	FailedChapters    int   `json:"failedChapters"`
	PlannedPages      int   `json:"plannedPages"`
	ValidPages        int   `json:"validPages"`
	DuplicatePages    int   `json:"duplicatePages"`
	RejectedPages     int   `json:"rejectedPages"`
	FailedPages       int   `json:"failedPages"`
	Archives          int   `json:"archives"`
	StoredBytes       int64 `json:"storedBytes"`
}
type DatasetInfo struct {
	Config         Config         `json:"config"`
	ConfigHash     string         `json:"configurationHash"`
	State          string         `json:"state"`
	StoppingReason string         `json:"stoppingReason"`
	CreatedAt      string         `json:"createdAt"`
	UpdatedAt      string         `json:"updatedAt"`
	Counters       Counters       `json:"counters"`
	SplitCounts    map[string]int `json:"splitCounts"`
}

func statePath(root string) string { return filepath.Join(root, "dataset-state.json") }
func Open(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create dataset root: %w", err)
	}
	s := &Store{root: root}
	if data, err := os.ReadFile(statePath(root)); err == nil {
		if err := json.Unmarshal(data, &s.data); err != nil {
			return nil, fmt.Errorf("read dataset state: %w", err)
		}
		if s.data.SchemaVersion != SchemaVersion {
			return nil, fmt.Errorf("unsupported dataset state version %d", s.data.SchemaVersion)
		}
		s.ensureMaps()
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read dataset state: %w", err)
	}
	return s, nil
}
func OpenExisting(root string) (*Store, error) {
	if _, err := os.Stat(statePath(root)); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("dataset not found at %q", root)
		}
		return nil, fmt.Errorf("inspect dataset state: %w", err)
	}
	return Open(root)
}
func (s *Store) Close() error { return nil }
func (s *Store) Root() string { return s.root }
func (s *Store) ensureMaps() {
	if s.data.Titles == nil {
		s.data.Titles = map[string]Title{}
	}
	if s.data.Chapters == nil {
		s.data.Chapters = map[string]Chapter{}
	}
	if s.data.Pages == nil {
		s.data.Pages = map[string]Page{}
	}
}
func (s *Store) saveLocked() error {
	s.data.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	s.ensureMaps()
	return writeJSONFile(statePath(s.root), s.data)
}
func (s *Store) LoadConfig(context.Context) (Config, string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Config, s.data.ConfigHash, s.data.ConfigHash != "", nil
}
func (s *Store) Initialize(_ context.Context, cfg Config, resume bool) error {
	if err := cfg.Normalize(); err != nil {
		return err
	}
	hash, err := cfg.Hash()
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.ConfigHash != "" {
		if s.data.ConfigHash != hash {
			return fmt.Errorf("dataset configuration mismatch; resume with the saved configuration or create a new dataset")
		}
		if !resume {
			return fmt.Errorf("dataset already exists; use --resume to continue it")
		}
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	s.data = datasetState{SchemaVersion: SchemaVersion, Config: cfg, ConfigHash: hash, State: "new", CreatedAt: now, UpdatedAt: now, Titles: map[string]Title{}, Chapters: map[string]Chapter{}, Pages: map[string]Page{}}
	if err := s.saveLocked(); err != nil {
		return err
	}
	return writeJSONFile(filepath.Join(s.root, "dataset-config.json"), cfg)
}
func (s *Store) SetRun(_ context.Context, state, reason, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.State, s.data.StoppingReason = state, reason
	return s.saveLocked()
}
func (s *Store) ReplacePlan(_ context.Context, discovered, titles []Title, chapters []Chapter) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.data.Titles) > 0 {
		return fmt.Errorf("dataset plan already exists")
	}
	titleDirectories := map[string]string{}
	for _, title := range titles {
		directory := normalizedTitle(title.Name)
		if other, exists := titleDirectories[directory]; exists && other != title.ID {
			return fmt.Errorf("titles %q and %q both normalize to data/%s", other, title.ID, directory)
		}
		titleDirectories[directory] = title.ID
	}
	chapterDirectories := map[string]string{}
	for _, chapter := range chapters {
		key := chapter.TitleID + "/chapter-" + normalizedChapterNumber(chapter.Number)
		if other, exists := chapterDirectories[key]; exists && other != chapter.ID {
			return fmt.Errorf("chapters %q and %q both normalize to data/%s", other, chapter.ID, key)
		}
		chapterDirectories[key] = chapter.ID
	}
	for _, title := range discovered {
		title.State = "discovered"
		s.data.Titles[title.ID] = title
	}
	for _, title := range titles {
		title.State = "planned"
		s.data.Titles[title.ID] = title
	}
	for _, chapter := range chapters {
		chapter.State = "planned"
		s.data.Chapters[chapter.ID] = chapter
	}
	s.data.State = "planned"
	return s.saveLocked()
}
func (s *Store) PlanSummary(_ context.Context) (Plan, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data.ConfigHash == "" || s.data.State == "new" || s.data.State == "planning" {
		return Plan{}, false, nil
	}
	plan := Plan{SplitCounts: map[string]int{}}
	for _, title := range s.data.Titles {
		plan.Candidates++
		if title.State != "discovered" {
			plan.Titles++
			plan.SplitCounts[title.Split]++
		}
	}
	for _, chapter := range s.data.Chapters {
		plan.Chapters++
		plan.EstimatedPages += int64(chapter.ExpectedPages)
	}
	if plan.Candidates == 0 {
		plan.Warnings = append(plan.Warnings, "no titles matched the discovery filters")
	}
	for _, title := range s.data.Titles {
		if title.State == "discovered" {
			continue
		}
		has := false
		for _, chapter := range s.data.Chapters {
			has = has || chapter.TitleID == title.ID
		}
		if !has {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("title %q has no chapters matching the collection filters", title.ID))
		}
	}
	sort.Strings(plan.Warnings)
	return plan, true, nil
}
func (s *Store) Planned(_ context.Context) ([]Title, []Chapter, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	titles := make([]Title, 0, len(s.data.Titles))
	for _, title := range s.data.Titles {
		titles = append(titles, title)
	}
	sort.Slice(titles, func(i, j int) bool {
		if titles[i].SampleRank != titles[j].SampleRank {
			return titles[i].SampleRank < titles[j].SampleRank
		}
		return titles[i].ID < titles[j].ID
	})
	chapters := make([]Chapter, 0, len(s.data.Chapters))
	for _, chapter := range s.data.Chapters {
		chapters = append(chapters, chapter)
	}
	sort.Slice(chapters, func(i, j int) bool {
		if chapters[i].TitleID != chapters[j].TitleID {
			return chapters[i].TitleID < chapters[j].TitleID
		}
		if chapters[i].ProviderOrder != chapters[j].ProviderOrder {
			return chapters[i].ProviderOrder < chapters[j].ProviderOrder
		}
		return chapters[i].ID < chapters[j].ID
	})
	return titles, chapters, nil
}
func pageKey(chapterID string, index int) string { return fmt.Sprintf("%s:%d", chapterID, index) }
func (s *Store) RecordPage(_ context.Context, page Page) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := pageKey(page.ChapterID, page.Index)
	existing := s.data.Pages[key]
	page.Attempts = existing.Attempts + 1
	page.DownloadedAt = existing.DownloadedAt
	if page.DownloadedAt == "" {
		page.DownloadedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	page.ValidatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	s.data.Pages[key] = page
	s.data.Attempts = append(s.data.Attempts, Attempt{"page", key, "validate", page.Attempts, page.State != "valid", page.ErrorMessage, page.ValidatedAt})
	return s.saveLocked()
}
func (s *Store) CompleteChapter(_ context.Context, id, output string, valid bool, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	chapter, ok := s.data.Chapters[id]
	if !ok {
		return fmt.Errorf("chapter %q not found", id)
	}
	if valid {
		chapter.State = "completed"
	} else {
		chapter.State = "partial"
	}
	chapter.OutputPath, chapter.LastError, chapter.ClaimOwner = output, message, ""
	s.data.Chapters[id] = chapter
	s.refreshTitleLocked(chapter.TitleID)
	return s.saveLocked()
}
func (s *Store) FailChapter(_ context.Context, id, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	chapter, ok := s.data.Chapters[id]
	if !ok {
		return fmt.Errorf("chapter %q not found", id)
	}
	chapter.State, chapter.LastError, chapter.ClaimOwner, chapter.Attempts = "failed", message, "", chapter.Attempts+1
	s.data.Chapters[id] = chapter
	s.data.Attempts = append(s.data.Attempts, Attempt{"chapter", id, "download", chapter.Attempts, false, message, time.Now().UTC().Format(time.RFC3339Nano)})
	return s.saveLocked()
}
func (s *Store) ClaimChapter(_ context.Context, id, owner string) (bool, error) {
	if owner == "" {
		return false, fmt.Errorf("chapter claim owner cannot be empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	chapter, ok := s.data.Chapters[id]
	if !ok {
		return false, fmt.Errorf("chapter %q not found", id)
	}
	if chapter.ClaimOwner != "" || (chapter.State != "planned" && chapter.State != "partial" && chapter.State != "failed") {
		return false, nil
	}
	chapter.State, chapter.ClaimOwner, chapter.Attempts = "downloading", owner, chapter.Attempts+1
	s.data.Chapters[id] = chapter
	s.data.Attempts = append(s.data.Attempts, Attempt{"chapter", id, "download", chapter.Attempts, true, "", time.Now().UTC().Format(time.RFC3339Nano)})
	return true, s.saveLocked()
}
func (s *Store) RecordAttempt(_ context.Context, entityType, entityID, operation string, attempt int, retryable bool, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Attempts = append(s.data.Attempts, Attempt{entityType, entityID, operation, attempt, retryable, message, time.Now().UTC().Format(time.RFC3339Nano)})
	return s.saveLocked()
}
func (s *Store) SetArchivePath(_ context.Context, id, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	chapter, ok := s.data.Chapters[id]
	if !ok {
		return fmt.Errorf("chapter %q not found", id)
	}
	chapter.ArchivePath = path
	s.data.Chapters[id] = chapter
	return s.saveLocked()
}
func (s *Store) refreshTitleLocked(id string) {
	title, ok := s.data.Titles[id]
	if !ok {
		return
	}
	total, completed := 0, 0
	for _, chapter := range s.data.Chapters {
		if chapter.TitleID == id {
			total++
			if chapter.State == "completed" {
				completed++
			}
		}
	}
	if total > 0 {
		if completed == total {
			title.State = "completed"
		} else {
			title.State = "partial"
		}
	}
	s.data.Titles[id] = title
}
func (s *Store) Info(_ context.Context) (DatasetInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data.ConfigHash == "" {
		return DatasetInfo{}, fmt.Errorf("dataset is not initialized")
	}
	out := DatasetInfo{Config: s.data.Config, ConfigHash: s.data.ConfigHash, State: s.data.State, StoppingReason: s.data.StoppingReason, CreatedAt: s.data.CreatedAt, UpdatedAt: s.data.UpdatedAt, SplitCounts: map[string]int{}}
	for _, title := range s.data.Titles {
		out.Counters.DiscoveredTitles++
		if title.State != "discovered" {
			out.Counters.PlannedTitles++
			out.SplitCounts[title.Split]++
		}
		if title.State == "completed" {
			out.Counters.CompletedTitles++
		}
		if title.State == "failed" {
			out.Counters.FailedTitles++
		}
	}
	for _, chapter := range s.data.Chapters {
		out.Counters.PlannedChapters++
		out.Counters.PlannedPages += chapter.ExpectedPages
		if chapter.State == "completed" {
			out.Counters.CompletedChapters++
		}
		if chapter.State == "failed" {
			out.Counters.FailedChapters++
		}
		if chapter.ArchivePath != "" && chapter.State == "completed" {
			out.Counters.Archives++
		}
	}
	for _, page := range s.data.Pages {
		switch page.State {
		case "valid":
			out.Counters.ValidPages++
			out.Counters.StoredBytes += page.Bytes
			if page.ExactDuplicateOf != "" {
				out.Counters.DuplicatePages++
			}
		case "rejected":
			out.Counters.RejectedPages++
		case "failed":
			out.Counters.FailedPages++
		}
	}
	return out, nil
}
