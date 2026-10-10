package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/evgen2571/mangate/internal/acquisition"
	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/providers"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/spf13/cobra"
)

type automationFlags struct {
	jsonl                bool
	eventsFile, manifest string
}

func (f *automationFlags) bind(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&f.jsonl, "jsonl", false, "Stream progress and a terminal JSON envelope to stdout")
	cmd.Flags().StringVar(&f.eventsFile, "events-file", "", "Write progress as JSON Lines to a new file")
	cmd.Flags().StringVar(&f.manifest, "manifest", "", "Save a portable acquisition manifest (including partial results)")
}
func (f automationFlags) progress(cmd *cobra.Command) (acquisition.Notify, func(), error) {
	if f.jsonl && wantsJSON(cmd) {
		return nil, nil, fmt.Errorf("--jsonl and --json cannot be combined")
	}
	if f.jsonl && isQuiet(cmd) {
		return nil, nil, fmt.Errorf("--jsonl and --quiet cannot be combined")
	}
	var writer io.Writer
	close := func() {}
	if f.eventsFile != "" {
		file, err := os.OpenFile(f.eventsFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return nil, nil, err
		}
		writer = file
		close = func() { file.Close() }
	}
	if f.jsonl {
		if writer != nil {
			writer = io.MultiWriter(writer, cmd.OutOrStdout())
		} else {
			writer = cmd.OutOrStdout()
		}
	}
	if writer == nil {
		return nil, close, nil
	}
	encoder := json.NewEncoder(writer)
	return func(e acquisition.ProgressEvent) {
		if err := encoder.Encode(e); err != nil {
			cmd.PrintErrln("write progress:", err)
		}
	}, close, nil
}
func terminal(notify acquisition.Notify, status string) {
	if notify != nil {
		notify(acquisition.ProgressEvent{Stage: "completion", Status: status})
	}
}

func NewRandomCmd(a *app.App) *cobra.Command {
	var p acquisition.SelectionPolicy
	var maxPages int
	var chapterMin, chapterMax float64
	var seed int64
	var planOnly bool
	var f automationFlags
	cmd := &cobra.Command{Use: "random", Short: "Discover and acquire one eligible chapter", Args: cobra.NoArgs}
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		p.Provider = a.Cfg.Provider
		if cmd.Flags().Changed("max-pages") {
			p.MaxPages = &maxPages
		}
		if cmd.Flags().Changed("chapter-min") {
			p.ChapterMin = &chapterMin
		}
		if cmd.Flags().Changed("chapter-max") {
			p.ChapterMax = &chapterMax
		}
		if cmd.Flags().Changed("seed") {
			p.Seed = &seed
		}
		notify, close, err := f.progress(cmd)
		if err != nil {
			return err
		}
		defer close()
		provider, err := a.Provider()
		if err != nil {
			terminal(notify, "failed")
			return automationError(cmd, f, err)
		}
		format, err := archive.ParseFormat(a.Cfg.Download.Format)
		if err != nil {
			terminal(notify, "failed")
			return automationError(cmd, f, err)
		}
		plan, err := acquisition.PlanRandom(cmd.Context(), provider, p, a.Cfg.Download.Dir, format, notify)
		if err != nil {
			status := "failed"
			if cmd.Context().Err() != nil {
				status = "cancelled"
			}
			terminal(notify, status)
			return automationError(cmd, f, err)
		}
		if err := acquisition.ValidateManifestDestination(plan.Output, f.manifest); err != nil {
			terminal(notify, "failed")
			return automationError(cmd, f, err)
		}
		if planOnly {
			if f.manifest != "" {
				m, err := acquisition.NewManifest(*plan, nil, "planned", nil)
				if err == nil {
					err = m.Write(f.manifest)
				}
				if err != nil {
					terminal(notify, "failed")
					return automationError(cmd, f, err)
				}
			}
			terminal(notify, "planned")
			if wantsJSON(cmd) || f.jsonl {
				return writeJSON(cmd, "random.plan", plan)
			}
			writeHuman(cmd.OutOrStdout(), "Selected %s / %s (%d pages) -> %s\n", plan.Manga.ID, plan.Chapter.ID, plan.ExpectedPages, plan.Output)
			return nil
		}
		return executePlan(cmd, a, provider, *plan, f, notify)
	}
	flags := cmd.Flags()
	flags.StringVar(&p.OriginalLanguage, "original-language", "", "Required original language; missing metadata is rejected")
	flags.StringVar(&p.ChapterLanguage, "chapter-language", "", "Required chapter translation language")
	flags.IntVar(&p.MinPages, "min-pages", 1, "Minimum chapter page count")
	flags.IntVar(&maxPages, "max-pages", 0, "Maximum chapter page count")
	flags.Float64Var(&chapterMin, "chapter-min", 0, "Inclusive minimum numeric chapter number")
	flags.Float64Var(&chapterMax, "chapter-max", 0, "Inclusive maximum numeric chapter number")
	flags.IntVar(&p.MaxAttempts, "max-attempts", 20, "Maximum candidate discovery attempts")
	flags.Int64Var(&seed, "seed", 0, "Use seeded selection from a bounded, recorded catalog snapshot")
	flags.BoolVar(&planOnly, "plan", false, "Resolve an eligible chapter without downloading images")
	f.bind(cmd)
	return cmd
}

func automationError(cmd *cobra.Command, f automationFlags, err error) error {
	if f.jsonl {
		if writeErr := WriteError(cmd.OutOrStdout(), cmd.Name(), err); writeErr != nil {
			return writeErr
		}
		return &ReportedError{Cause: err, Code: ExitCode(err.Error())}
	}
	return err
}

func replayManifest(cmd *cobra.Command, a *app.App, path string, f automationFlags, verify bool) error {
	notify, close, err := f.progress(cmd)
	if err != nil {
		return err
	}
	defer close()
	m, err := acquisition.LoadManifest(path)
	if err != nil {
		terminal(notify, "failed")
		return automationError(cmd, f, err)
	}
	if verify {
		err = m.Verify()
		if err != nil {
			terminal(notify, "failed")
			return automationError(cmd, f, err)
		}
		terminal(notify, "complete")
		if wantsJSON(cmd) || f.jsonl {
			return writeJSON(cmd, "manifest.verify", m)
		}
		writeHuman(cmd.OutOrStdout(), "Manifest files verified\n")
		return nil
	}
	if cmd.Flags().Changed("output") || cmd.Flags().Changed("download-dir") {
		m.Plan.Output = a.Cfg.Download.Dir
	}
	if cmd.Flags().Changed("format") && archive.Format(a.Cfg.Download.Format) != m.Plan.Format {
		terminal(notify, "failed")
		return automationError(cmd, f, fmt.Errorf("plan validation: format override disagrees with manifest"))
	}
	if cmd.Flags().Changed("provider") && a.Cfg.Provider != m.Plan.Provider {
		terminal(notify, "failed")
		return automationError(cmd, f, fmt.Errorf("plan validation: provider override disagrees with manifest"))
	}
	cfg := a.Cfg.Clone()
	cfg.Provider = m.Plan.Provider
	cfg.Download.Format = string(m.Plan.Format)
	if err := a.ApplyConfig(cfg); err != nil {
		terminal(notify, "failed")
		return automationError(cmd, f, err)
	}
	provider, err := a.Provider()
	if err != nil {
		terminal(notify, "failed")
		return automationError(cmd, f, err)
	}
	return executePlan(cmd, a, provider, m.Plan, f, notify)
}

func executePlan(cmd *cobra.Command, a *app.App, provider providers.Provider, plan acquisition.DownloadPlan, f automationFlags, notify acquisition.Notify) error {
	var pathErr error
	plan.Output, pathErr = filepath.Abs(plan.Output)
	if pathErr != nil {
		return pathErr
	}
	if err := acquisition.ValidateManifestDestination(plan.Output, f.manifest); err != nil {
		terminal(notify, "failed")
		return automationError(cmd, f, err)
	}
	chapter, err := plan.Resolve(cmd.Context(), provider, notify)
	started := time.Now().UTC()
	record := downloadRecord{Provider: plan.Provider, Title: plan.Manga, Format: plan.Format, OutputRoot: plan.Output, StartedAt: started, Status: "in_progress", Chapters: chapterRecords(plan.Output, plan.Manga, []*source.Chapter{plan.Chapter}, plan.Format, "pending")}
	record.Chapters[0].OutputPath = plan.Output
	var pages []downloader.PageDownloadResult
	if err == nil {
		err = downloader.ValidateExactDestination(plan.Output, chapter, plan.Provider)
	}
	if err == nil {
		if notify != nil {
			notify(acquisition.ProgressEvent{Stage: "downloading", Status: "started", MangaID: plan.Manga.ID, ChapterID: chapter.ID, ExpectedPages: plan.ExpectedPages})
		}
		// This loader is also used on URL refresh: no retry may bypass eligibility.
		loader := func(ctx context.Context, c *source.Chapter) ([]*source.Page, error) {
			loaded, err := provider.Pages(ctx, c)
			if err != nil {
				return nil, err
			}
			if len(loaded) != plan.ExpectedPages || !plan.Policy.MatchPages(len(loaded)) {
				return nil, fmt.Errorf("plan validation: page count changed")
			}
			return loaded, nil
		}
		pages, err = a.Downloader.DownloadChapterToWithProgress(cmd.Context(), chapter, plan.Output, loader, func(completed, total int) {
			if notify != nil {
				notify(acquisition.ProgressEvent{Stage: "downloading", Status: "progress", ChapterID: chapter.ID, CompletedPages: completed, ExpectedPages: total})
			}
		})
		if !errors.Is(err, &downloader.ChapterPreflightError{ChapterID: chapter.ID}) {
			err = errors.Join(err, updateChapterRecordStates(&record))
			if len(record.Chapters[0].ImagePaths) > 0 {
				pages = nil
				for _, path := range record.Chapters[0].ImagePaths {
					index, parseErr := strconv.Atoi(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
					if parseErr != nil {
						err = errors.Join(err, parseErr)
						continue
					}
					pages = append(pages, downloader.PageDownloadResult{PageIndex: index, Path: path})
				}
			}
		}
	}
	record.Chapters[0].ImagePaths = []string{}
	for _, page := range pages {
		record.Chapters[0].ImagePaths = append(record.Chapters[0].ImagePaths, page.Path)
	}
	record.CompletedAt = time.Now().UTC()
	status := "complete"
	if err != nil {
		status = "failed"
		if len(pages) > 0 {
			status = "partial"
		}
		if errors.Is(err, context.Canceled) || cmd.Context().Err() != nil {
			status = "cancelled"
		}
		record.Error = err.Error()
		record.Chapters[0].Status = status
	}
	if err == nil {
		record.Chapters[0].Status = "complete"
	}
	record.Status = status
	m, manifestErr := acquisition.NewManifest(plan, pages, status, err)
	if manifestErr == nil {
		record.Manifest = &m
		if m.Status != status {
			status = m.Status
			record.Status = status
			record.Chapters[0].Status = status
			err = errors.Join(err, fmt.Errorf("acquisition validation: downloaded image validation failed"))
			record.Error = err.Error()
		}
		if f.manifest != "" {
			manifestErr = m.Write(f.manifest)
		}
	}
	if manifestErr != nil {
		err = errors.Join(err, fmt.Errorf("write manifest: %w", manifestErr))
		record.Error = err.Error()
		record.Status = "partial"
		status = "partial"
	}

	terminal(notify, status)
	responseStatus := status
	if err == nil {
		responseStatus = "success"
	}
	if wantsJSON(cmd) || f.jsonl {
		if writeErr := writeJSONStatus(cmd, "download", responseStatus, record); writeErr != nil {
			return writeErr
		}
	} else {
		writeDownloadSummary(cmd.OutOrStdout(), record)
	}
	if err != nil {
		code := 5
		if status == "cancelled" {
			code = 7
		}
		return &ReportedError{Cause: err, Code: code}
	}
	return nil
}
