package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/evgen2571/mangate/internal/acquisition"
	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/providers"
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
func (f automationFlags) progress(cmd *cobra.Command) (acquisition.Notify, func(string) error, error) {
	if f.jsonl && wantsJSON(cmd) {
		return nil, nil, fmt.Errorf("--jsonl and --json cannot be combined")
	}
	if f.jsonl && isQuiet(cmd) {
		return nil, nil, fmt.Errorf("--jsonl and --quiet cannot be combined")
	}
	var file *os.File
	if f.eventsFile != "" {
		var err error
		file, err = os.OpenFile(f.eventsFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return nil, nil, fmt.Errorf("open progress file: %w", err)
		}
	}
	if file != nil && f.manifest != "" {
		eventInfo, statErr := file.Stat()
		manifestInfo, manifestErr := os.Stat(f.manifest)
		if statErr != nil || manifestErr == nil && os.SameFile(eventInfo, manifestInfo) {
			file.Close()
			os.Remove(f.eventsFile)
			if statErr != nil {
				return nil, nil, statErr
			}
			return nil, nil, fmt.Errorf("--events-file and --manifest must use different destinations")
		}
	}
	var output io.Writer
	if f.jsonl {
		output = cmd.OutOrStdout()
	}
	var fileErr, outputErr error
	encode := func(writer io.Writer, event acquisition.ProgressEvent, target *error) {
		if writer == nil || *target != nil {
			return
		}
		data, err := json.Marshal(event)
		if err == nil {
			data = append(data, '\n')
			var n int
			n, err = writer.Write(data)
			if err == nil && n != len(data) {
				err = io.ErrShortWrite
			}
		}
		if err != nil {
			*target = fmt.Errorf("write progress: %w", err)
		}
	}
	notify := func(e acquisition.ProgressEvent) {
		if file != nil {
			encode(file, e, &fileErr)
		}
		encode(output, e, &outputErr)
	}
	reportingError := func() error {
		err := errors.Join(fileErr, outputErr)
		if outputErr != nil {
			return &ReportedError{Cause: err, Code: 6}
		}
		return err
	}
	finished := false
	finish := func(status string) error {
		if finished {
			return reportingError()
		}
		finished = true
		// Flush before publishing completion; a reporting failure cannot claim success.
		if flusher, ok := output.(interface{ Flush() error }); ok {
			if err := flusher.Flush(); err != nil {
				outputErr = errors.Join(outputErr, fmt.Errorf("flush progress: %w", err))
			}
		}
		if file != nil {
			if err := file.Sync(); err != nil {
				fileErr = errors.Join(fileErr, fmt.Errorf("flush progress file: %w", err))
			}
			if status != "" && outputErr == nil {
				encode(file, acquisition.ProgressEvent{Stage: "completion", Status: status}, &fileErr)
			}
			if err := file.Sync(); err != nil {
				fileErr = errors.Join(fileErr, fmt.Errorf("flush progress file: %w", err))
			}
			if err := file.Close(); err != nil {
				fileErr = errors.Join(fileErr, fmt.Errorf("close progress file: %w", err))
			}
		}
		if status != "" {
			if reportingError() != nil {
				if status == "complete" {
					status = "partial"
				} else if status == "planned" {
					status = "failed"
				}
			}
			// A failed stream may contain a partial JSON line. Never append to it.
			encode(output, acquisition.ProgressEvent{Stage: "completion", Status: status}, &outputErr)
			if flusher, ok := output.(interface{ Flush() error }); ok {
				if err := flusher.Flush(); err != nil {
					outputErr = errors.Join(outputErr, fmt.Errorf("flush progress: %w", err))
				}
			}
		}
		return reportingError()
	}
	if file == nil && output == nil {
		return nil, finish, nil
	}
	return notify, finish, nil
}

func automationFailure(cmd *cobra.Command, f automationFlags, finish func(string) error, status string, err error) error {
	return automationError(cmd, f, errors.Join(err, finish(status)))
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
		notify, finish, err := f.progress(cmd)
		if err != nil {
			return automationError(cmd, f, err)
		}
		defer finish("")
		provider, err := a.Provider()
		if err != nil {
			return automationFailure(cmd, f, finish, "failed", err)
		}
		format, err := archive.ParseFormat(a.Cfg.Download.Format)
		if err != nil {
			return automationFailure(cmd, f, finish, "failed", err)
		}
		plan, err := acquisition.PlanRandom(cmd.Context(), provider, p, a.Cfg.Download.Dir, format, notify)
		if err != nil {
			status := "failed"
			if cmd.Context().Err() != nil {
				status = "cancelled"
			}
			return automationFailure(cmd, f, finish, status, err)
		}
		if err := acquisition.ValidateManifestDestination(plan.Output, f.manifest); err != nil {
			return automationFailure(cmd, f, finish, "failed", err)
		}
		if planOnly {
			if f.manifest != "" {
				m, err := acquisition.NewManifest(*plan, nil, "planned", nil)
				if err == nil {
					err = m.Write(f.manifest)
				}
				if err != nil {
					return automationFailure(cmd, f, finish, "failed", err)
				}
			}
			if err := finish("planned"); err != nil {
				return automationError(cmd, f, err)
			}
			if wantsJSON(cmd) || f.jsonl {
				return writeJSON(cmd, "random.plan", plan)
			}
			writeHuman(cmd.OutOrStdout(), "Selected %s / %s (%d pages) -> %s\n", plan.Manga.ID, plan.Chapter.ID, plan.ExpectedPages, plan.Output)
			return nil
		}
		return executePlan(cmd, a, provider, *plan, f, notify, finish)
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
	var reported *ReportedError
	if errors.As(err, &reported) {
		return err
	}
	if f.jsonl {
		if writeErr := WriteError(cmd.OutOrStdout(), cmd.Name(), err); writeErr != nil {
			return writeErr
		}
		return &ReportedError{Cause: err, Code: ExitCode(err.Error())}
	}
	return err
}

func replayManifest(cmd *cobra.Command, a *app.App, path string, f automationFlags, verify bool) error {
	notify, finish, err := f.progress(cmd)
	if err != nil {
		return automationError(cmd, f, err)
	}
	defer finish("")
	m, err := acquisition.LoadManifest(path)
	if err != nil {
		return automationFailure(cmd, f, finish, "failed", err)
	}
	if verify {
		err = m.Verify()
		if err != nil {
			return automationFailure(cmd, f, finish, "failed", err)
		}
		if err := finish("complete"); err != nil {
			return automationError(cmd, f, err)
		}
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
		return automationFailure(cmd, f, finish, "failed", fmt.Errorf("plan validation: format override disagrees with manifest"))
	}
	if cmd.Flags().Changed("provider") && a.Cfg.Provider != m.Plan.Provider {
		return automationFailure(cmd, f, finish, "failed", fmt.Errorf("plan validation: provider override disagrees with manifest"))
	}
	cfg := a.Cfg.Clone()
	cfg.Provider = m.Plan.Provider
	cfg.Download.Format = string(m.Plan.Format)
	if err := a.ApplyConfig(cfg); err != nil {
		return automationFailure(cmd, f, finish, "failed", err)
	}
	provider, err := a.Provider()
	if err != nil {
		return automationFailure(cmd, f, finish, "failed", err)
	}
	return executePlan(cmd, a, provider, m.Plan, f, notify, finish)
}

func executePlan(cmd *cobra.Command, a *app.App, provider providers.Provider, plan acquisition.DownloadPlan, f automationFlags, notify acquisition.Notify, finish func(string) error) error {
	var pathErr error
	plan.Output, pathErr = filepath.Abs(plan.Output)
	if pathErr != nil {
		return pathErr
	}
	if err := acquisition.ValidateManifestDestination(plan.Output, f.manifest); err != nil {
		return automationFailure(cmd, f, finish, "failed", err)
	}
	record, err := a.Acquisition(provider).ExecutePlan(cmd.Context(), plan, notify)
	status := record.Status
	if record.Manifest != nil && f.manifest != "" {
		if manifestErr := record.Manifest.Write(f.manifest); manifestErr != nil {
			err = errors.Join(err, fmt.Errorf("write manifest: %w", manifestErr))
			record.Normalize(cmd.Context(), err)
			status = record.Status
		}
	}

	if reportErr := finish(status); reportErr != nil {
		err = errors.Join(err, reportErr)
		record.Normalize(cmd.Context(), err)
		status = record.Status
		var reported *ReportedError
		if errors.As(reportErr, &reported) {
			return &ReportedError{Cause: err, Code: 6}
		}
	}
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
