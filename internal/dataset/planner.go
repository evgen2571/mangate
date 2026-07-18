package dataset

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"

	"github.com/evgen2571/mangate/internal/providers"
	"github.com/evgen2571/mangate/internal/source"
)

type Plan struct {
	Candidates, Titles, Chapters int            `json:"candidates"`
	EstimatedPages               int64          `json:"estimatedPages"`
	SplitCounts                  map[string]int `json:"splitCounts"`
	Warnings                     []string       `json:"warnings,omitempty"`
}

// BuildPlan discovers a bounded candidate set, samples titles reproducibly,
// then persists the exact chapter releases selected for later resume.
func BuildPlan(ctx context.Context, store *Store, provider providers.Provider, cfg Config) (Plan, error) {
	browser, ok := provider.(providers.BrowseProvider)
	if !ok {
		return Plan{}, fmt.Errorf("provider %q does not support dataset browsing", provider.Name())
	}
	if err := store.SetRun(ctx, "planning", "", ""); err != nil {
		return Plan{}, err
	}
	candidates := []source.BrowseTitle{}
	offset := 0
	for len(candidates) < cfg.Discovery.CandidatePoolSize {
		limit := 100
		if remaining := cfg.Discovery.CandidatePoolSize - len(candidates); remaining < limit {
			limit = remaining
		}
		page, err := browser.BrowseManga(ctx, source.BrowseRequest{Limit: limit, Offset: offset, OriginalLanguages: cfg.Discovery.OriginalLanguages, ChapterLanguages: cfg.Discovery.ChapterLanguages, Statuses: cfg.Discovery.Statuses, ContentRatings: cfg.Discovery.ContentRatings, IncludedTags: cfg.Discovery.IncludedTags, ExcludedTags: cfg.Discovery.ExcludedTags, OrderBy: cfg.Discovery.OrderBy, OrderDirection: cfg.Discovery.OrderDirection})
		if err != nil {
			return Plan{}, err
		}
		for _, candidate := range page.Titles {
			if candidate.Manga != nil && strings.TrimSpace(candidate.Manga.ID) != "" {
				candidates = append(candidates, candidate)
				if len(candidates) == cfg.Discovery.CandidatePoolSize {
					break
				}
			}
		}
		if !page.HasMore || page.NextOffset <= offset {
			break
		}
		offset = page.NextOffset
	}
	selected := sampleTitles(candidates, cfg)
	titles := make([]Title, 0, len(selected))
	chapters := []Chapter{}
	splitCounts := map[string]int{}
	estimated := int64(0)
	for rank, candidate := range selected {
		manga := candidate.Manga
		stratum := titleStratum(manga)
		split := splitFor(cfg, manga.ID)
		splitCounts[split]++
		titles = append(titles, Title{ID: manga.ID, Name: manga.Title, URL: manga.URL, OriginalLanguage: manga.Metadata.Language, Status: manga.Metadata.Status, ContentRating: manga.Metadata.ContentType, Year: manga.Metadata.Year, DiscoveryOrder: indexOfCandidate(candidates, manga.ID), Stratum: stratum, SampleRank: rank, Split: split})
		available, err := provider.Chapters(ctx, manga)
		if err != nil {
			return Plan{}, fmt.Errorf("list chapters for title %q: %w", manga.ID, err)
		}
		chosen := sampleChapters(available, cfg)
		if len(chosen) == 0 {
			continue
		}
		for order, chapter := range chosen {
			chapters = append(chapters, Chapter{ID: chapter.ID, TitleID: manga.ID, Number: chapter.Index, Name: chapter.Title, Volume: chapter.Volume, Language: chapter.Language, ReleaseGroup: chapter.ReleaseGroup, PublishedAt: chapter.PublishedAt, URL: chapter.URL, ProviderOrder: order, ExpectedPages: chapter.PageCount})
			estimated += int64(chapter.PageCount)
		}
	}
	if err := store.ReplacePlan(ctx, titles, chapters); err != nil {
		return Plan{}, err
	}
	return Plan{Candidates: len(candidates), Titles: len(titles), Chapters: len(chapters), EstimatedPages: estimated, SplitCounts: splitCounts}, nil
}

func sampleTitles(candidates []source.BrowseTitle, cfg Config) []source.BrowseTitle {
	limit := cfg.Sampling.MaxTitles
	if limit <= 0 || limit > len(candidates) {
		limit = len(candidates)
	}
	copyCandidates := append([]source.BrowseTitle(nil), candidates...)
	switch cfg.Sampling.TitleStrategy {
	case "random":
		r := rand.New(rand.NewSource(cfg.Sampling.Seed))
		r.Shuffle(len(copyCandidates), func(i, j int) { copyCandidates[i], copyCandidates[j] = copyCandidates[j], copyCandidates[i] })
		return copyCandidates[:limit]
	case "stratified":
		groups := map[string][]source.BrowseTitle{}
		keys := []string{}
		for _, candidate := range copyCandidates {
			key := titleStratum(candidate.Manga)
			if _, ok := groups[key]; !ok {
				keys = append(keys, key)
			}
			groups[key] = append(groups[key], candidate)
		}
		sort.Strings(keys)
		result := make([]source.BrowseTitle, 0, limit)
		for len(result) < limit {
			progressed := false
			for _, key := range keys {
				if len(groups[key]) > 0 {
					result = append(result, groups[key][0])
					groups[key] = groups[key][1:]
					progressed = true
					if len(result) == limit {
						break
					}
				}
			}
			if !progressed {
				break
			}
		}
		return result
	default:
		return copyCandidates[:limit]
	}
}
func titleStratum(m *source.Manga) string {
	if m == nil {
		return "unknown|unknown"
	}
	year := "unknown"
	if m.Metadata.Year > 0 {
		year = strconv.Itoa((m.Metadata.Year/10)*10) + "s"
	}
	status := strings.TrimSpace(m.Metadata.Status)
	if status == "" {
		status = "unknown"
	}
	return year + "|" + status
}
func splitFor(cfg Config, id string) string {
	if !cfg.Splits.Enabled {
		return ""
	}
	r := rand.New(rand.NewSource(cfg.Sampling.Seed + int64(hashID(id)))).Float64()
	if r < cfg.Splits.Train {
		return "train"
	}
	if r < cfg.Splits.Train+cfg.Splits.Validation {
		return "validation"
	}
	return "test"
}
func hashID(id string) uint32 {
	var h uint32 = 2166136261
	for _, b := range []byte(id) {
		h ^= uint32(b)
		h *= 16777619
	}
	return h
}
func indexOfCandidate(candidates []source.BrowseTitle, id string) int {
	for i, c := range candidates {
		if c.Manga != nil && c.Manga.ID == id {
			return i
		}
	}
	return -1
}

func sampleChapters(chapters []*source.Chapter, cfg Config) []*source.Chapter {
	filtered := make([]*source.Chapter, 0, len(chapters))
	for _, chapter := range chapters {
		if chapter == nil || chapter.ID == "" {
			continue
		}
		if len(cfg.Discovery.ChapterLanguages) > 0 && !contains(cfg.Discovery.ChapterLanguages, chapter.Language) {
			continue
		}
		filtered = append(filtered, chapter)
	}
	if !cfg.Sampling.KeepDuplicateChapterReleases {
		filtered = uniqueReleases(filtered)
	}
	limit := cfg.Sampling.MaxChaptersPerTitle
	if limit <= 0 || limit > len(filtered) {
		limit = len(filtered)
	}
	if limit == 0 {
		return nil
	}
	switch cfg.Sampling.ChapterStrategy {
	case "all":
		return filtered
	case "first":
		return filtered[:1]
	case "latest":
		return filtered[len(filtered)-1:]
	case "random":
		r := rand.New(rand.NewSource(cfg.Sampling.Seed + int64(hashID(filtered[0].From.ID))))
		r.Shuffle(len(filtered), func(i, j int) { filtered[i], filtered[j] = filtered[j], filtered[i] })
		return filtered[:limit]
	case "uniform":
		if limit == 1 {
			return []*source.Chapter{filtered[0]}
		}
		result := make([]*source.Chapter, 0, limit)
		seen := map[int]bool{}
		for i := 0; i < limit; i++ {
			index := i * (len(filtered) - 1) / (limit - 1)
			if !seen[index] {
				result = append(result, filtered[index])
				seen[index] = true
			}
		}
		return result
	default:
		return filtered[:limit]
	}
}
func uniqueReleases(chapters []*source.Chapter) []*source.Chapter {
	groups := map[string][]*source.Chapter{}
	keys := []string{}
	for _, c := range chapters {
		key := strings.TrimSpace(c.Index)
		if key == "" {
			key = "special:" + c.ID
		}
		if _, ok := groups[key]; !ok {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], c)
	}
	sort.Strings(keys)
	out := []*source.Chapter{}
	for _, key := range keys {
		choices := groups[key]
		sort.SliceStable(choices, func(i, j int) bool {
			a, b := choices[i], choices[j]
			if a.PageCount > 0 != (b.PageCount > 0) {
				return a.PageCount > 0
			}
			if a.PublishedAt != b.PublishedAt {
				return a.PublishedAt < b.PublishedAt
			}
			return a.ID < b.ID
		})
		out = append(out, choices[0])
	}
	sort.SliceStable(out, func(i, j int) bool { return chapterPosition(out[i]) < chapterPosition(out[j]) })
	return out
}
func chapterPosition(c *source.Chapter) float64 {
	v, err := strconv.ParseFloat(c.Index, 64)
	if err != nil {
		return 1e18
	}
	return v
}
func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
