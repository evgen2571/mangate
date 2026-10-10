package acquisition

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/providers"
	"github.com/evgen2571/mangate/internal/source"
)

const Version = 1

type ProgressEvent struct {
	Stage          string `json:"stage"`
	Status         string `json:"status"`
	Attempt        int    `json:"attempt,omitempty"`
	MangaID        string `json:"mangaId,omitempty"`
	ChapterID      string `json:"chapterId,omitempty"`
	CompletedPages int    `json:"completedPages,omitempty"`
	ExpectedPages  int    `json:"expectedPages,omitempty"`
	Reason         string `json:"reason,omitempty"`
}
type Notify func(ProgressEvent)

func emit(notify Notify, e ProgressEvent) {
	if notify != nil {
		notify(e)
	}
}

type DownloadPlan struct {
	Version       int             `json:"version"`
	Provider      string          `json:"provider"`
	Manga         *source.Manga   `json:"manga"`
	Chapter       *source.Chapter `json:"chapter"`
	Policy        SelectionPolicy `json:"policy"`
	Strategy      string          `json:"strategy"`
	CandidateIDs  []string        `json:"candidateIds,omitempty"`
	ExpectedPages int             `json:"expectedPages"`
	Output        string          `json:"output"`
	Format        archive.Format  `json:"format"`
	Validation    string          `json:"validation"`
}

func (p DownloadPlan) Validate() error {
	if p.Version != Version {
		return fmt.Errorf("plan validation: unsupported version %d", p.Version)
	}
	if err := p.Policy.Validate(); err != nil {
		return err
	}
	if p.Provider != p.Policy.Provider || !p.Policy.MatchTitle(p.Manga) || !p.Policy.MatchChapter(p.Chapter) || p.Chapter.PageCount != p.ExpectedPages || !p.Policy.MatchPages(p.ExpectedPages) {
		return fmt.Errorf("plan validation: inconsistent identity, metadata, or eligibility")
	}
	if strings.TrimSpace(p.Output) == "" {
		return fmt.Errorf("plan validation: output cannot be empty")
	}
	if p.Format != archive.FormatDirectory && p.Format != archive.FormatPNG && p.Format != archive.FormatJPEG {
		return fmt.Errorf("plan validation: unsupported output format %q", p.Format)
	}
	if p.Validation != "eligible" || p.Manga.URL != "" || p.Chapter.URL != "" || p.Manga.Cover.URL != "" || len(p.Chapter.Pages) != 0 || len(p.Manga.Chapters) != 0 {
		return fmt.Errorf("plan validation: invalid validation state or embedded pages")
	}
	switch p.Strategy {
	case "provider_random":
		if p.Policy.Seed != nil || len(p.CandidateIDs) != 0 {
			return fmt.Errorf("plan validation: provider_random cannot have a seed or catalog")
		}
	case "seeded_catalog":
		if p.Policy.Seed == nil || len(p.CandidateIDs) == 0 || len(p.CandidateIDs) > p.Policy.MaxAttempts {
			return fmt.Errorf("plan validation: seeded catalog requires seed and bounded candidates")
		}
		seen, found := map[string]bool{}, false
		for _, id := range p.CandidateIDs {
			if id == "" || seen[id] {
				return fmt.Errorf("plan validation: invalid candidate set")
			}
			seen[id] = true
			found = found || id == p.Manga.ID
		}
		if !found {
			return fmt.Errorf("plan validation: selected manga missing from candidates")
		}
	default:
		return fmt.Errorf("plan validation: unsupported strategy %q", p.Strategy)
	}
	return nil
}

// PlanRandom resolves page identities/counts but never transfers image files.
func PlanRandom(ctx context.Context, provider providers.Provider, policy SelectionPolicy, output string, format archive.Format, notify Notify) (*DownloadPlan, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(output) == "" {
		return nil, fmt.Errorf("plan validation: output cannot be empty")
	}
	if format != archive.FormatDirectory && format != archive.FormatPNG && format != archive.FormatJPEG {
		return nil, fmt.Errorf("random: unsupported output format %q", format)
	}
	if provider.Name() != policy.Provider || !provider.Info().DownloadPermitted {
		return nil, fmt.Errorf("provider %q does not permit acquisition", provider.Name())
	}
	output, err := filepath.Abs(output)
	if err != nil {
		return nil, err
	}
	var random providers.RandomProvider
	var candidates []*source.Manga
	var ids []string
	strategy := "provider_random"
	if policy.Seed == nil {
		var ok bool
		random, ok = provider.(providers.RandomProvider)
		if !ok {
			return nil, fmt.Errorf("provider %q: unsupported random discovery", provider.Name())
		}
	} else {
		browse, ok := provider.(providers.BrowseProvider)
		if !ok {
			return nil, fmt.Errorf("provider %q: unsupported seeded catalog discovery", provider.Name())
		}
		strategy = "seeded_catalog"
		seen := map[string]bool{}
		offset := 0
		// Bound catalog reads even if the provider keeps returning duplicates.
		for pageAttempt := 0; pageAttempt < policy.MaxAttempts && len(candidates) < policy.MaxAttempts; pageAttempt++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			emit(notify, ProgressEvent{Stage: "discovery", Status: "catalog", Attempt: pageAttempt + 1})
			req := source.BrowseRequest{Limit: min(100, policy.MaxAttempts-len(candidates)), Offset: offset, OrderBy: "createdAt", OrderDirection: "asc"}
			if policy.OriginalLanguage != "" {
				req.OriginalLanguages = []string{policy.OriginalLanguage}
			}
			if policy.ChapterLanguage != "" {
				req.ChapterLanguages = []string{policy.ChapterLanguage}
			}
			page, err := browse.BrowseManga(ctx, req)
			if err != nil {
				return nil, err
			}
			for _, item := range page.Titles {
				if item.Manga != nil && item.Manga.ID != "" && !seen[item.Manga.ID] {
					seen[item.Manga.ID] = true
					candidates = append(candidates, item.Manga)
					if len(candidates) == policy.MaxAttempts {
						break
					}
				}
			}
			if !page.HasMore {
				break
			}
			if page.NextOffset <= offset {
				return nil, fmt.Errorf("catalog validation: pagination did not advance")
			}
			offset = page.NextOffset
		}
		sort.Slice(candidates, func(i, j int) bool { return candidates[i].ID < candidates[j].ID })
		for _, m := range candidates {
			ids = append(ids, m.ID)
		}
		rng := rand.New(rand.NewSource(*policy.Seed))
		rng.Shuffle(len(candidates), func(i, j int) { candidates[i], candidates[j] = candidates[j], candidates[i] })
	}
	seen := map[string]bool{}
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		emit(notify, ProgressEvent{Stage: "discovery", Status: "searching", Attempt: attempt})
		var manga *source.Manga
		if random != nil {
			manga, err = random.RandomManga(ctx)
			if err != nil {
				return nil, err
			}
		} else {
			if attempt > len(candidates) {
				break
			}
			manga = candidates[attempt-1]
		}
		reason := "title metadata does not match policy"
		if manga != nil && seen[manga.ID] {
			reason = "duplicate candidate"
		} else if policy.MatchTitle(manga) {
			seen[manga.ID] = true
			emit(notify, ProgressEvent{Stage: "selection", Status: "selecting", Attempt: attempt, MangaID: manga.ID})
			chapters, err := provider.Chapters(ctx, manga)
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, source.ErrCandidateUnavailable) {
					return nil, err
				}
				emit(notify, ProgressEvent{Stage: "discovery", Status: "rejected", Attempt: attempt, MangaID: manga.ID, Reason: "candidate chapters unavailable or invalid"})
				continue
			}
			// Stable chapter order is independent of provider response order.
			sort.SliceStable(chapters, func(i, j int) bool {
				if chapters[i] == nil {
					return false
				}
				if chapters[j] == nil {
					return true
				}
				return chapters[i].ID < chapters[j].ID
			})
			chapterSeen := map[string]bool{}
			for _, chapter := range chapters {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				if !policy.MatchChapter(chapter) || chapterSeen[chapter.ID] {
					continue
				}
				chapterSeen[chapter.ID] = true
				emit(notify, ProgressEvent{Stage: "validation", Status: "checking", Attempt: attempt, MangaID: manga.ID, ChapterID: chapter.ID})
				pages, err := provider.Pages(ctx, chapter)
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				if err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, source.ErrCandidateUnavailable) {
						return nil, err
					}
					emit(notify, ProgressEvent{Stage: "validation", Status: "rejected", Attempt: attempt, MangaID: manga.ID, ChapterID: chapter.ID, Reason: "chapter pages unavailable or invalid"})
					continue
				}
				validPages := true
				for _, page := range pages {
					if page == nil {
						validPages = false
						break
					}
					req, pageErr := http.NewRequestWithContext(ctx, http.MethodGet, page.URL, nil)
					if pageErr != nil || (req.URL.Scheme != "http" && req.URL.Scheme != "https") || req.URL.Host == "" || req.URL.User != nil {
						validPages = false
						break
					}
				}
				if !validPages || chapter.PageCount > 0 && chapter.PageCount != len(pages) || !policy.MatchPages(len(pages)) {
					emit(notify, ProgressEvent{Stage: "validation", Status: "rejected", Attempt: attempt, MangaID: manga.ID, ChapterID: chapter.ID, Reason: "page count disagrees with metadata or policy"})
					continue
				}
				m, c := *manga, *chapter
				m.Chapters, m.URL, m.Cover = nil, "", source.Cover{}
				c.Pages, c.From, c.URL, c.PageCount = nil, nil, "", len(pages)
				plan := &DownloadPlan{Version: Version, Provider: provider.Name(), Manga: &m, Chapter: &c, Policy: policy, Strategy: strategy, CandidateIDs: ids, ExpectedPages: len(pages), Output: output, Format: format, Validation: "eligible"}
				if err := plan.Validate(); err != nil {
					return nil, err
				}
				emit(notify, ProgressEvent{Stage: "selection", Status: "selected", Attempt: attempt, MangaID: m.ID, ChapterID: c.ID, ExpectedPages: len(pages)})
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				return plan, nil
			}
			reason = "no eligible available chapters"
		}
		if manga != nil {
			seen[manga.ID] = true
		}
		emit(notify, ProgressEvent{Stage: "discovery", Status: "rejected", Attempt: attempt, Reason: reason})
	}
	return nil, fmt.Errorf("no results found: no eligible chapter within %d discovery attempts", policy.MaxAttempts)
}

// Resolve checks the original stable selection again without any discovery.
// Changed metadata or page counts are rejected before files can be written.
func (p DownloadPlan) Resolve(ctx context.Context, provider providers.Provider, notify Notify) (*source.Chapter, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if provider.Name() != p.Provider || !provider.Info().DownloadPermitted {
		return nil, fmt.Errorf("provider does not permit plan execution")
	}
	emit(notify, ProgressEvent{Stage: "validation", Status: "checking", MangaID: p.Manga.ID, ChapterID: p.Chapter.ID})
	manga, err := provider.Title(ctx, p.Manga.ID)
	if err != nil {
		return nil, err
	}
	if !p.Policy.MatchTitle(manga) || manga.ID != p.Manga.ID || manga.Metadata.Language != p.Manga.Metadata.Language {
		return nil, fmt.Errorf("plan validation: title metadata changed")
	}
	chapters, err := provider.Chapters(ctx, manga)
	if err != nil {
		return nil, err
	}
	for _, chapter := range chapters {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if chapter == nil || chapter.ID != p.Chapter.ID {
			continue
		}
		if !p.Policy.MatchChapter(chapter) || chapter.Language != p.Chapter.Language || chapter.Index != p.Chapter.Index || chapter.PageCount > 0 && chapter.PageCount != p.ExpectedPages {
			return nil, fmt.Errorf("plan validation: chapter metadata changed")
		}
		c := *chapter
		c.From = manga
		c.Pages = nil
		return &c, nil
	}
	return nil, fmt.Errorf("selected chapter %q not found or unavailable", p.Chapter.ID)
}
