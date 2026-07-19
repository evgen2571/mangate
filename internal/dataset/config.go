// Package dataset implements resumable, provider-backed image collection.
package dataset

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/evgen2571/mangate/internal/archive"
)

const ConfigVersion = 1

type Config struct {
	Version    int        `json:"version"`
	DatasetID  string     `json:"datasetId"`
	Provider   string     `json:"provider"`
	Output     Output     `json:"output"`
	Discovery  Discovery  `json:"discovery"`
	Sampling   Sampling   `json:"sampling"`
	Limits     Limits     `json:"limits"`
	Validation Validation `json:"validation"`
	Splits     Splits     `json:"splits"`
	Runtime    Runtime    `json:"runtime"`
}

type Output struct {
	Directory     string                   `json:"directory"`
	Format        archive.Format           `json:"format"`
	ExistingFiles archive.ExistingFileMode `json:"existingFiles"`
}
type Discovery struct {
	OriginalLanguages []string `json:"originalLanguages"`
	ChapterLanguages  []string `json:"chapterLanguages"`
	Statuses          []string `json:"statuses"`
	ContentRatings    []string `json:"contentRatings"`
	IncludedTags      []string `json:"includedTags"`
	ExcludedTags      []string `json:"excludedTags"`
	OrderBy           string   `json:"orderBy"`
	OrderDirection    string   `json:"orderDirection"`
	CandidatePoolSize int      `json:"candidatePoolSize"`
}
type Sampling struct {
	Seed                         int64  `json:"seed"`
	TitleStrategy                string `json:"titleStrategy"`
	ChapterStrategy              string `json:"chapterStrategy"`
	MaxTitles                    int    `json:"maxTitles"`
	MaxChaptersPerTitle          int    `json:"maxChaptersPerTitle"`
	KeepDuplicateChapterReleases bool   `json:"keepDuplicateChapterReleases"`
}
type Limits struct {
	MaxPages    int64 `json:"maxPages"`
	MaxBytes    int64 `json:"maxBytes"`
	MaxFailures int   `json:"maxFailures"`
}
type Validation struct {
	MinimumWidth            int   `json:"minimumWidth"`
	MinimumHeight           int   `json:"minimumHeight"`
	MaximumWidth            int   `json:"maximumWidth"`
	MaximumHeight           int   `json:"maximumHeight"`
	MaximumDecodedPixels    int64 `json:"maximumDecodedPixels"`
	FullDecode              bool  `json:"fullDecode"`
	CalculateSHA256         bool  `json:"calculateSHA256"`
	CalculatePerceptualHash bool  `json:"calculatePerceptualHash"`
}
type Splits struct {
	Enabled    bool    `json:"enabled"`
	Train      float64 `json:"train"`
	Validation float64 `json:"validation"`
	Test       float64 `json:"test"`
}
type Runtime struct {
	TitleWorkers      int `json:"titleWorkers"`
	ChapterWorkers    int `json:"chapterWorkers"`
	PageWorkers       int `json:"pageWorkers"`
	ValidationWorkers int `json:"validationWorkers"`
	RetryLimit        int `json:"retryLimit"`
}

func DefaultConfig(root, provider string) Config {
	return Config{Version: ConfigVersion, DatasetID: filepath.Base(filepath.Clean(root)), Provider: provider, Output: Output{Directory: root, Format: archive.FormatDirectory, ExistingFiles: archive.ExistingSkip}, Discovery: Discovery{OriginalLanguages: []string{"ko"}, ChapterLanguages: []string{"en"}, Statuses: []string{"ongoing", "completed"}, ContentRatings: []string{"safe", "suggestive"}, OrderBy: "updatedAt", OrderDirection: "desc", CandidatePoolSize: 3000}, Sampling: Sampling{Seed: 2571, TitleStrategy: "stratified", ChapterStrategy: "uniform", MaxTitles: 1000, MaxChaptersPerTitle: 20}, Limits: Limits{MaxFailures: 1000}, Validation: Validation{MinimumWidth: 256, MinimumHeight: 256, MaximumWidth: 20000, MaximumHeight: 100000, MaximumDecodedPixels: 500000000, FullDecode: true, CalculateSHA256: true, CalculatePerceptualHash: true}, Splits: Splits{Enabled: true, Train: .8, Validation: .1, Test: .1}, Runtime: Runtime{TitleWorkers: 2, ChapterWorkers: 4, PageWorkers: 8, ValidationWorkers: 2, RetryLimit: 5}}
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read collection configuration: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode collection configuration: %w", err)
	}
	return cfg, nil
}

func (c *Config) Normalize() error {
	if c.Version == 0 {
		c.Version = ConfigVersion
	}
	if c.Version != ConfigVersion {
		return fmt.Errorf("unsupported collection configuration version %d", c.Version)
	}
	c.DatasetID, c.Provider, c.Output.Directory = strings.TrimSpace(c.DatasetID), strings.TrimSpace(c.Provider), strings.TrimSpace(c.Output.Directory)
	if c.DatasetID == "" {
		return fmt.Errorf("dataset id cannot be empty")
	}
	if c.Provider == "" {
		return fmt.Errorf("dataset provider cannot be empty")
	}
	if c.Output.Directory == "" {
		return fmt.Errorf("dataset output directory cannot be empty")
	}
	format, err := archive.ParseFormat(string(c.Output.Format))
	if err != nil {
		return err
	}
	if format.IsArchive() {
		return fmt.Errorf("dataset output format %q is not supported; datasets store ordered page files", format)
	}
	c.Output.Format = format
	if c.Output.ExistingFiles == "" {
		c.Output.ExistingFiles = archive.ExistingSkip
	}
	if c.Output.ExistingFiles != archive.ExistingSkip && c.Output.ExistingFiles != archive.ExistingReplace && c.Output.ExistingFiles != archive.ExistingFail {
		return fmt.Errorf("dataset existing files must be skip, replace, or fail")
	}
	if c.Sampling.MaxTitles < 0 || c.Sampling.MaxChaptersPerTitle < 0 || c.Limits.MaxPages < 0 || c.Limits.MaxBytes < 0 || c.Limits.MaxFailures < 0 {
		return fmt.Errorf("dataset limits cannot be negative")
	}
	if c.Sampling.MaxTitles == 0 && c.Limits.MaxPages == 0 && c.Limits.MaxBytes == 0 {
		return fmt.Errorf("dataset collection needs at least one positive stopping condition")
	}
	if c.Discovery.CandidatePoolSize <= 0 {
		return fmt.Errorf("candidate pool size must be > 0")
	}
	if c.Sampling.TitleStrategy == "" {
		c.Sampling.TitleStrategy = "stratified"
	}
	if c.Sampling.ChapterStrategy == "" {
		c.Sampling.ChapterStrategy = "uniform"
	}
	if !oneOf(c.Sampling.TitleStrategy, "sequential", "random", "stratified") {
		return fmt.Errorf("unsupported title strategy %q", c.Sampling.TitleStrategy)
	}
	if !oneOf(c.Sampling.ChapterStrategy, "all", "first", "latest", "random", "uniform") {
		return fmt.Errorf("unsupported chapter strategy %q", c.Sampling.ChapterStrategy)
	}
	if c.Runtime.PageWorkers <= 0 || c.Runtime.ChapterWorkers <= 0 || c.Runtime.TitleWorkers <= 0 || c.Runtime.ValidationWorkers <= 0 || c.Runtime.RetryLimit < 0 {
		return fmt.Errorf("dataset worker counts must be positive and retry limit cannot be negative")
	}
	if c.Validation.MinimumWidth < 1 || c.Validation.MinimumHeight < 1 || c.Validation.MaximumWidth < c.Validation.MinimumWidth || c.Validation.MaximumHeight < c.Validation.MinimumHeight || c.Validation.MaximumDecodedPixels < 1 {
		return fmt.Errorf("invalid dataset image validation limits")
	}
	if c.Splits.Enabled {
		total := c.Splits.Train + c.Splits.Validation + c.Splits.Test
		if c.Splits.Train < 0 || c.Splits.Validation < 0 || c.Splits.Test < 0 || total < .999999 || total > 1.000001 {
			return fmt.Errorf("dataset split ratios must sum to 1")
		}
	}
	c.Discovery.OriginalLanguages = normalizedStrings(c.Discovery.OriginalLanguages)
	c.Discovery.ChapterLanguages = normalizedStrings(c.Discovery.ChapterLanguages)
	c.Discovery.Statuses = normalizedStrings(c.Discovery.Statuses)
	c.Discovery.ContentRatings = normalizedStrings(c.Discovery.ContentRatings)
	c.Discovery.IncludedTags = normalizedStrings(c.Discovery.IncludedTags)
	c.Discovery.ExcludedTags = normalizedStrings(c.Discovery.ExcludedTags)
	if c.Discovery.OrderBy == "" {
		c.Discovery.OrderBy = "updatedAt"
	}
	if !oneOf(strings.ToLower(c.Discovery.OrderDirection), "asc", "desc") {
		return fmt.Errorf("dataset order direction must be asc or desc")
	}
	c.Discovery.OrderDirection = strings.ToLower(c.Discovery.OrderDirection)
	return nil
}

func (c Config) CanonicalJSON() ([]byte, error) {
	if err := c.Normalize(); err != nil {
		return nil, err
	}
	return json.Marshal(c)
}
func (c Config) Hash() (string, error) {
	data, err := c.CanonicalJSON()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
func oneOf(value string, values ...string) bool {
	for _, v := range values {
		if value == v {
			return true
		}
	}
	return false
}
func normalizedStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; !ok {
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}
