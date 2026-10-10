package acquisition

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/evgen2571/mangate/internal/source"
)

// SelectionPolicy rejects missing metadata needed by any requested filter.
// Missing page counts are resolved from the page list before a plan is accepted.
type SelectionPolicy struct {
	Provider         string   `json:"provider"`
	OriginalLanguage string   `json:"originalLanguage,omitempty"`
	ChapterLanguage  string   `json:"chapterLanguage,omitempty"`
	MinPages         int      `json:"minPages"`
	MaxPages         *int     `json:"maxPages,omitempty"`
	ChapterMin       *float64 `json:"chapterMin,omitempty"`
	ChapterMax       *float64 `json:"chapterMax,omitempty"`
	MaxAttempts      int      `json:"maxAttempts"`
	Seed             *int64   `json:"seed,omitempty"`
}

func (p SelectionPolicy) Validate() error {
	if strings.TrimSpace(p.Provider) == "" || p.Provider != strings.TrimSpace(p.Provider) {
		return fmt.Errorf("selection policy: provider cannot be empty or padded")
	}
	if p.MinPages < 1 || p.MaxAttempts < 1 {
		return fmt.Errorf("selection policy: minPages and maxAttempts must be positive")
	}
	if p.MaxPages != nil && *p.MaxPages < p.MinPages {
		return fmt.Errorf("selection policy: maxPages must be >= minPages")
	}
	for _, language := range []string{p.OriginalLanguage, p.ChapterLanguage} {
		for _, c := range language {
			if !(c >= 'a' && c <= 'z' || c == '-') {
				return fmt.Errorf("selection policy: malformed language")
			}
		}
	}
	for _, v := range []*float64{p.ChapterMin, p.ChapterMax} {
		if v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0) || *v < 0) {
			return fmt.Errorf("selection policy: chapter bounds must be finite and nonnegative")
		}
	}
	if p.ChapterMin != nil && p.ChapterMax != nil && *p.ChapterMin > *p.ChapterMax {
		return fmt.Errorf("selection policy: chapterMin must be <= chapterMax")
	}
	return nil
}

func (p SelectionPolicy) MatchTitle(m *source.Manga) bool {
	return m != nil && m.ID != "" && (p.OriginalLanguage == "" || m.Metadata.Language == p.OriginalLanguage)
}

func (p SelectionPolicy) MatchChapter(c *source.Chapter) bool {
	if c == nil || c.ID == "" || p.ChapterLanguage != "" && c.Language != p.ChapterLanguage {
		return false
	}
	if p.ChapterMin != nil || p.ChapterMax != nil {
		n, err := strconv.ParseFloat(c.Index, 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || p.ChapterMin != nil && n < *p.ChapterMin || p.ChapterMax != nil && n > *p.ChapterMax {
			return false
		}
	}
	return c.PageCount == 0 || p.MatchPages(c.PageCount)
}

func (p SelectionPolicy) MatchPages(n int) bool {
	return n >= p.MinPages && (p.MaxPages == nil || n <= *p.MaxPages)
}
