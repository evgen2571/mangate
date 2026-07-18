package cli

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/dataset"
	"github.com/spf13/cobra"
)

func NewDatasetCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{Use: "dataset", Short: "Plan, collect, inspect, verify, and export image datasets"}
	cmd.AddCommand(newDatasetPlanCmd(a), newDatasetCollectCmd(a), newDatasetStatusCmd(), newDatasetVerifyCmd(), newDatasetExportCmd())
	return cmd
}

type datasetFlags struct {
	configPath, datasetID                                                            string
	originalLanguages, chapterLanguages, statuses, ratings, includeTags, excludeTags []string
	candidatePool, maxTitles, maxChapters, maxPages, maxFailures                     int
	maxBytes                                                                         string
	titleStrategy, chapterStrategy                                                   string
	seed                                                                             int64
	keepReleases, resume, yes, dryRun                                                bool
}

func bindDatasetFlags(cmd *cobra.Command, f *datasetFlags, collect bool) {
	flags := cmd.Flags()
	flags.StringVar(&f.configPath, "collection-config", "", "Versioned dataset collection JSON configuration")
	flags.StringVar(&f.datasetID, "dataset-id", "", "Stable dataset identifier")
	flags.StringSliceVar(&f.originalLanguages, "original-language", nil, "Original title language, repeatable")
	flags.StringSliceVar(&f.chapterLanguages, "chapter-language", nil, "Chapter language, repeatable")
	flags.StringSliceVar(&f.statuses, "status", nil, "Publication status, repeatable")
	flags.StringSliceVar(&f.ratings, "content-rating", nil, "Content rating, repeatable")
	flags.StringSliceVar(&f.includeTags, "include-tag", nil, "Included tag, repeatable")
	flags.StringSliceVar(&f.excludeTags, "exclude-tag", nil, "Excluded tag, repeatable")
	flags.IntVar(&f.candidatePool, "candidate-pool-size", -1, "Maximum catalog candidates to consider")
	flags.StringVar(&f.titleStrategy, "title-strategy", "", "Title sampling: sequential, random, or stratified")
	flags.StringVar(&f.chapterStrategy, "chapter-strategy", "", "Chapter sampling: all, first, latest, random, or uniform")
	flags.Int64Var(&f.seed, "seed", math.MinInt64, "Deterministic sampling seed")
	flags.IntVar(&f.maxTitles, "max-titles", -1, "Maximum titles to select, 0 disables this limit")
	flags.IntVar(&f.maxChapters, "max-chapters-per-title", -1, "Maximum chapters per title, 0 selects all")
	flags.IntVar(&f.maxPages, "max-pages", -1, "Maximum final pages, 0 disables this limit")
	flags.StringVar(&f.maxBytes, "max-bytes", "", "Maximum final bytes, for example 500GiB")
	flags.IntVar(&f.maxFailures, "max-failures", -1, "Maximum failed chapters before stopping")
	flags.BoolVar(&f.keepReleases, "keep-duplicate-releases", false, "Keep multiple releases of the same logical chapter")
	flags.BoolVar(&f.resume, "resume", false, "Resume an existing dataset without changing its plan")
	if collect {
		flags.BoolVar(&f.yes, "yes", false, "Confirm collection writes")
		flags.BoolVar(&f.dryRun, "dry-run", false, "Create or show the plan without downloading pages")
	}
}

func newDatasetPlanCmd(a *app.App) *cobra.Command {
	var flags datasetFlags
	cmd := &cobra.Command{Use: "plan", Short: "Discover titles and persist a deterministic collection plan", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := effectiveDatasetConfig(cmd, a, flags)
		if err != nil {
			return err
		}
		service, err := a.DatasetService(cfg)
		if err != nil {
			return err
		}
		defer service.Store.Close()
		if err := service.Store.Initialize(cmd.Context(), cfg, flags.resume); err != nil {
			return err
		}
		plan, err := dataset.BuildPlan(cmd.Context(), service.Store, service.Provider, cfg)
		if err != nil {
			return err
		}
		result := map[string]any{"datasetRoot": cfg.Output.Directory, "datasetId": cfg.DatasetID, "provider": cfg.Provider, "format": cfg.Output.Format, "plan": plan, "confirmationRequired": true}
		if wantsJSON(cmd) {
			return writeJSON(cmd, "dataset.plan", result)
		}
		writeHuman(cmd.OutOrStdout(), "Dataset plan\nProvider: %s\nOutput: %s\nFormat: %s\nCandidates: %d\nTitles: %d\nChapters: %d\nEstimated pages: %d\n", cfg.Provider, cfg.Output.Directory, cfg.Output.Format, plan.Candidates, plan.Titles, plan.Chapters, plan.EstimatedPages)
		return nil
	}}
	bindDatasetFlags(cmd, &flags, false)
	return cmd
}
func newDatasetCollectCmd(a *app.App) *cobra.Command {
	var flags datasetFlags
	cmd := &cobra.Command{Use: "collect", Short: "Collect a resumable, validated image dataset", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := effectiveDatasetConfig(cmd, a, flags)
		if err != nil {
			return err
		}
		if flags.dryRun {
			flags.resume = true
			return newDatasetPlanResult(cmd, a, cfg, flags)
		}
		if !flags.yes {
			return fmt.Errorf("dataset collect is a broad operation; rerun with --yes after reviewing dataset plan")
		}
		service, err := a.DatasetService(cfg)
		if err != nil {
			return err
		}
		defer service.Store.Close()
		result, err := service.Collect(cmd.Context(), cfg, flags.resume)
		if err != nil {
			return err
		}
		if wantsJSON(cmd) {
			return writeJSON(cmd, "dataset.collect", result)
		}
		writeHuman(cmd.OutOrStdout(), "Dataset collection %s\nOutput: %s\nFormat: %s\nValid pages: %d\nStored bytes: %d\nManifest: %s\nSummary: %s\n", result.State, result.DatasetRoot, result.Format, result.Counters.ValidPages, result.Counters.StoredBytes, result.ManifestPath, result.SummaryPath)
		return nil
	}}
	bindDatasetFlags(cmd, &flags, true)
	return cmd
}
func newDatasetPlanResult(cmd *cobra.Command, a *app.App, cfg dataset.Config, flags datasetFlags) error {
	service, err := a.DatasetService(cfg)
	if err != nil {
		return err
	}
	defer service.Store.Close()
	if err := service.Store.Initialize(cmd.Context(), cfg, flags.resume); err != nil {
		return err
	}
	plan, err := dataset.BuildPlan(cmd.Context(), service.Store, service.Provider, cfg)
	if err != nil {
		return err
	}
	result := map[string]any{"datasetRoot": cfg.Output.Directory, "datasetId": cfg.DatasetID, "provider": cfg.Provider, "format": cfg.Output.Format, "plan": plan, "dryRun": true}
	if wantsJSON(cmd) {
		return writeJSON(cmd, "dataset.plan", result)
	}
	writeHuman(cmd.OutOrStdout(), "Dataset dry run\nTitles: %d\nChapters: %d\nEstimated pages: %d\n", plan.Titles, plan.Chapters, plan.EstimatedPages)
	return nil
}
func newDatasetStatusCmd() *cobra.Command {
	return &cobra.Command{Use: "status <dataset-root>", Short: "Show local dataset state without provider access", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		store, err := dataset.Open(args[0])
		if err != nil {
			return err
		}
		defer store.Close()
		info, err := store.Info(cmd.Context())
		if err != nil {
			return err
		}
		if wantsJSON(cmd) {
			return writeJSON(cmd, "dataset.status", info)
		}
		writeHuman(cmd.OutOrStdout(), "Dataset status\nID: %s\nProvider: %s\nFormat: %s\nState: %s\nValid pages: %d\nStored bytes: %d\n", info.Config.DatasetID, info.Config.Provider, info.Config.Output.Format, info.State, info.Counters.ValidPages, info.Counters.StoredBytes)
		return nil
	}}
}
func newDatasetVerifyCmd() *cobra.Command {
	var repair bool
	cmd := &cobra.Command{Use: "verify <dataset-root>", Short: "Verify local dataset output without provider access", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		store, err := dataset.Open(args[0])
		if err != nil {
			return err
		}
		defer store.Close()
		result, err := dataset.Verify(cmd.Context(), store, repair)
		if err != nil {
			return err
		}
		if wantsJSON(cmd) {
			return writeJSON(cmd, "dataset.verify", result)
		}
		writeHuman(cmd.OutOrStdout(), "Verified %v pages, invalid: %v\n", result["checkedPages"], result["invalidPages"])
		return nil
	}}
	cmd.Flags().BoolVar(&repair, "repair", false, "Repair local state and regenerate exports without provider downloads")
	return cmd
}
func newDatasetExportCmd() *cobra.Command {
	var split string
	var duplicates, rejected bool
	cmd := &cobra.Command{Use: "export <dataset-root>", Short: "Regenerate manifest, summary, and failure reports from local state", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		store, err := dataset.Open(args[0])
		if err != nil {
			return err
		}
		defer store.Close()
		if err := dataset.Export(cmd.Context(), store, dataset.ExportOptions{Split: split, IncludeDuplicates: duplicates, IncludeRejected: rejected}); err != nil {
			return err
		}
		_ = dataset.Failures(cmd.Context(), store)
		result := map[string]string{"manifestPath": filepath.Join(args[0], "manifest.jsonl"), "summaryPath": filepath.Join(args[0], "summary.json")}
		if wantsJSON(cmd) {
			return writeJSON(cmd, "dataset.export", result)
		}
		writeHuman(cmd.OutOrStdout(), "Exported %s and %s\n", result["manifestPath"], result["summaryPath"])
		return nil
	}}
	cmd.Flags().StringVar(&split, "split", "", "Only export one split")
	cmd.Flags().BoolVar(&duplicates, "include-duplicates", false, "Include exact duplicate pages")
	cmd.Flags().BoolVar(&rejected, "include-rejected", false, "Include rejected pages when supported")
	return cmd
}

func effectiveDatasetConfig(cmd *cobra.Command, a *app.App, f datasetFlags) (dataset.Config, error) {
	root := a.Cfg.Download.Dir
	cfg := dataset.DefaultConfig(root, a.Cfg.Provider)
	cfg.Output.Format = archive.Format(a.Cfg.Download.Format)
	cfg.Output.ExistingFiles = archive.ExistingFileMode(a.Cfg.Download.ExistingFileMode)
	cfg.Runtime.PageWorkers = a.Cfg.Concurrency.PageDownloads
	cfg.Runtime.ChapterWorkers = a.Cfg.Concurrency.ChapterDownloads
	if f.resume && f.configPath == "" {
		if existing, err := dataset.Open(root); err == nil {
			if saved, _, ok, loadErr := existing.LoadConfig(context.Background()); loadErr == nil && ok {
				cfg = saved
			}
			_ = existing.Close()
		}
	}
	if f.configPath != "" {
		loaded, err := dataset.LoadConfig(f.configPath)
		if err != nil {
			return cfg, err
		}
		cfg = loaded
	}
	changed := func(name string) bool { return cmd.Flags().Changed(name) || cmd.InheritedFlags().Changed(name) }
	if changed("output") || changed("download-dir") {
		cfg.Output.Directory = a.Cfg.Download.Dir
	}
	if changed("format") {
		cfg.Output.Format = archive.Format(a.Cfg.Download.Format)
	}
	if changed("provider") {
		cfg.Provider = a.Cfg.Provider
	}
	if changed("existing-files") {
		cfg.Output.ExistingFiles = archive.ExistingFileMode(a.Cfg.Download.ExistingFileMode)
	}
	if changed("page-downloads") {
		cfg.Runtime.PageWorkers = a.Cfg.Concurrency.PageDownloads
	}
	if changed("chapter-downloads") {
		cfg.Runtime.ChapterWorkers = a.Cfg.Concurrency.ChapterDownloads
	}
	if f.datasetID != "" {
		cfg.DatasetID = f.datasetID
	}
	if len(f.originalLanguages) > 0 {
		cfg.Discovery.OriginalLanguages = f.originalLanguages
	}
	if len(f.chapterLanguages) > 0 {
		cfg.Discovery.ChapterLanguages = f.chapterLanguages
	}
	if len(f.statuses) > 0 {
		cfg.Discovery.Statuses = f.statuses
	}
	if len(f.ratings) > 0 {
		cfg.Discovery.ContentRatings = f.ratings
	}
	if len(f.includeTags) > 0 {
		cfg.Discovery.IncludedTags = f.includeTags
	}
	if len(f.excludeTags) > 0 {
		cfg.Discovery.ExcludedTags = f.excludeTags
	}
	if f.candidatePool >= 0 {
		cfg.Discovery.CandidatePoolSize = f.candidatePool
	}
	if f.titleStrategy != "" {
		cfg.Sampling.TitleStrategy = f.titleStrategy
	}
	if f.chapterStrategy != "" {
		cfg.Sampling.ChapterStrategy = f.chapterStrategy
	}
	if f.seed != math.MinInt64 {
		cfg.Sampling.Seed = f.seed
	}
	if f.maxTitles >= 0 {
		cfg.Sampling.MaxTitles = f.maxTitles
	}
	if f.maxChapters >= 0 {
		cfg.Sampling.MaxChaptersPerTitle = f.maxChapters
	}
	if f.maxPages >= 0 {
		cfg.Limits.MaxPages = int64(f.maxPages)
	}
	if f.maxFailures >= 0 {
		cfg.Limits.MaxFailures = f.maxFailures
	}
	if f.maxBytes != "" {
		bytes, err := parseDatasetBytes(f.maxBytes)
		if err != nil {
			return cfg, err
		}
		cfg.Limits.MaxBytes = bytes
	}
	if f.keepReleases {
		cfg.Sampling.KeepDuplicateChapterReleases = true
	}
	return cfg, cfg.Normalize()
}
func parseDatasetBytes(value string) (int64, error) {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return 0, nil
	}
	multipliers := map[string]float64{"B": 1, "KIB": 1 << 10, "MIB": 1 << 20, "GIB": 1 << 30, "TIB": 1 << 40, "KB": 1e3, "MB": 1e6, "GB": 1e9, "TB": 1e12}
	for suffix, multiplier := range multipliers {
		if strings.HasSuffix(value, suffix) {
			number := strings.TrimSpace(strings.TrimSuffix(value, suffix))
			parsed, err := strconv.ParseFloat(number, 64)
			if err != nil || parsed < 0 || parsed*multiplier > math.MaxInt64 {
				return 0, fmt.Errorf("invalid byte limit %q", value)
			}
			return int64(parsed * multiplier), nil
		}
	}
	return 0, fmt.Errorf("invalid byte limit %q", value)
}
