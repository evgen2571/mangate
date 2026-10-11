package cli

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/evgen2571/mangate/internal/acquisition"
	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/spf13/cobra"
)

func NewDownloadCmd(a *app.App) *cobra.Command {
	var fromManifest string
	var verifyManifest bool
	var automation automationFlags
	var chapterIDs []string
	var chapterNumbers []string
	var chapterRange string
	var first, latest, all bool
	var before, after, language string
	var dryRun bool
	var assumeYes bool
	var chapterOutput string

	cmd := &cobra.Command{
		Use:     "download <title-id>",
		Short:   "Download selected chapters that you are authorized to save",
		Example: "  mangate download <title-id> --chapter-id <chapter-id>\n  mangate download <title-id> --range 1-10\n  mangate --format cbz download <title-id> --latest\n  mangate --json download <title-id> --chapter 1",
		Args: func(cmd *cobra.Command, args []string) error {
			if fromManifest != "" {
				if len(args) != 0 {
					return fmt.Errorf("select chapters: --from-manifest cannot be combined with a title")
				}
				return nil
			}
			return requireOneArgument("a stable <title-id> from `mangate search`", "mangate download <title-id> --latest")(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if fromManifest != "" {
				if len(chapterIDs) > 0 || len(chapterNumbers) > 0 || chapterRange != "" || first || latest || all || before != "" || after != "" || language != "" || dryRun || chapterOutput != "" {
					return fmt.Errorf("select chapters: manifest replay cannot be combined with chapter selectors; use --output to relocate")
				}
				return replayManifest(cmd, a, fromManifest, automation, verifyManifest)
			}
			if verifyManifest || automation.jsonl || automation.manifest != "" {
				return fmt.Errorf("unsupported: automation flags on download require --from-manifest")
			}
			format, err := archive.ParseFormat(a.Cfg.Download.Format)
			if err != nil {
				return err
			}

			titleID := strings.TrimSpace(args[0])
			if titleID == "" {
				return fmt.Errorf("title id cannot be empty")
			}

			provider, err := a.Provider()
			if err != nil {
				return err
			}

			if !hasCapability(provider.Info(), "download") || !provider.Info().DownloadPermitted {
				return fmt.Errorf("provider %q does not permit downloads through this integration", provider.Name())
			}

			title, err := provider.Title(cmd.Context(), titleID)
			if err != nil {
				return fmt.Errorf("get title %q from provider %q: %w", titleID, provider.Name(), err)
			}

			chapters, err := provider.Chapters(cmd.Context(), title)
			if err != nil {
				return fmt.Errorf("list chapters for title %q: %w", titleID, err)
			}

			selection, err := selectChapters(chapters, chapterSelection{
				IDs:      chapterIDs,
				Numbers:  chapterNumbers,
				Range:    chapterRange,
				First:    first,
				Latest:   latest,
				All:      all,
				Before:   before,
				After:    after,
				Language: language,
			})
			if err != nil {
				return err
			}

			if cmd.Flags().Changed("chapter-output") {
				if strings.TrimSpace(chapterOutput) == "" {
					return fmt.Errorf("chapter output cannot be empty")
				}
				if len(selection) != 1 || format.IsArchive() {
					return fmt.Errorf("select chapters: --chapter-output requires one chapter and directory, png, or jpeg format")
				}
				chapterOutput, err = filepath.Abs(chapterOutput)
				if err != nil {
					return fmt.Errorf("resolve chapter output: %w", err)
				}
			}
			service := a.Acquisition(provider)
			record, err := service.Plan(title, selection, chapterOutput)
			if err != nil {
				return err
			}
			if dryRun {
				if wantsJSON(cmd) {
					return writeJSON(cmd, "download.plan", record)
				}

				writeHuman(
					cmd.OutOrStdout(),
					"Title: %s\nProvider: %s\nChapters: %d selected\nFormat: %s\nOutput: %s\nDry run: no files will be changed\n",
					title.Title,
					provider.Name(),
					len(selection),
					format,
					record.OutputRoot,
				)
				for _, chapter := range record.Chapters {
					path := chapter.OutputPath
					if chapter.ArchivePath != "" {
						path = chapter.ArchivePath
					}

					writeHuman(cmd.OutOrStdout(), "  %s [%s] -> %s\n", chapter.ID, chapter.Status, path)
				}

				return nil
			}

			if requirement := downloadConfirmationRequirement(selection, format, a.Cfg.Download.ExistingFileMode); requirement != "" && !assumeYes {
				return fmt.Errorf("download: %s; review with --dry-run, then rerun with --yes to continue", requirement)
			}

			if !wantsJSON(cmd) && !isQuiet(cmd) {
				writeDownloadPreflight(cmd.ErrOrStderr(), title, provider.Name(), selection, format, record.OutputRoot, a.Cfg.Download.ExistingFileMode)
			}

			events, finish, err := automation.progress(cmd)
			if err != nil {
				return err
			}
			defer finish("")
			notify := func(progress downloader.DownloadProgress) {
				if events != nil {
					events(acquisition.ProgressEvent{Stage: "downloading", Status: "progress", MangaID: title.ID, CompletedPages: progress.CompletedPages, ExpectedPages: progress.TotalPages})
				}
				if wantsJSON(cmd) || isQuiet(cmd) {
					return
				}

				writeHuman(cmd.ErrOrStderr(), "Downloaded %d/%d pages, %d/%d chapters\r", progress.CompletedPages, progress.TotalPages, progress.CompletedChapters, progress.TotalChapters)
			}
			record, err = service.Execute(cmd.Context(), title, selection, chapterOutput, notify)
			if reportingErr := finish(record.Status); reportingErr != nil {
				err = errors.Join(err, reportingErr)
				record.Normalize(cmd.Context(), err)
			}
			if err != nil {
				return reportDownloadResult(cmd, &record, fmt.Errorf("download title %q: %w", titleID, err))
			}

			if wantsJSON(cmd) {
				return writeJSON(cmd, "download", record)
			}

			if !isQuiet(cmd) {
				writeHuman(cmd.ErrOrStderr(), "\n")
				writeDownloadSummary(cmd.OutOrStdout(), record)
			}

			return nil
		},
	}
	automation.bind(cmd)
	flags := cmd.Flags()
	flags.StringVar(&fromManifest, "from-manifest", "", "Replay the stable selection from a plan or acquisition manifest")
	flags.BoolVar(&verifyManifest, "verify-manifest", false, "Verify existing manifest files locally instead of replaying the selection")
	flags.StringVar(&chapterOutput, "chapter-output", "", "Exact directory for one chapter (directory, png, or jpeg)")
	flags.StringSliceVar(&chapterIDs, "chapter-id", nil, "Stable chapter ID to download, repeatable")
	flags.StringSliceVar(&chapterNumbers, "chapter", nil, "Chapter number to download, repeatable")
	flags.StringVar(&chapterRange, "range", "", "Inclusive chapter number range, for example 1-10")
	flags.BoolVar(&first, "first", false, "Download the first listed chapter")
	flags.BoolVar(&latest, "latest", false, "Download the latest listed chapter")
	flags.BoolVar(&all, "all", false, "Download all accessible chapters")
	flags.StringVar(&before, "before", "", "Only chapters before this chapter number")
	flags.StringVar(&after, "after", "", "Only chapters after this chapter number")
	flags.StringVar(&language, "chapter-language", "", "Only chapters in this provider language")
	flags.BoolVar(&dryRun, "dry-run", false, "Show selected chapters and output paths without downloading")
	flags.BoolVar(&assumeYes, "yes", false, "Confirm broad or destructive downloads without prompting")
	return cmd
}

const broadDownloadChapterThreshold = 25

func downloadConfirmationRequirement(chapters []*source.Chapter, format archive.Format, existingFileMode string) string {
	switch {
	case existingFileMode == string(archive.ExistingReplace):
		return "replacing existing output may discard data"
	case format.IsArchive():
		return "removing temporary source page directories after archive creation changes local files"
	case len(chapters) >= broadDownloadChapterThreshold:
		return fmt.Sprintf("downloading %d chapters is a broad operation", len(chapters))
	default:
		return ""
	}
}

func writeDownloadPreflight(out io.Writer, title *source.Manga, provider string, chapters []*source.Chapter, format archive.Format, outputRoot, existingFileMode string) {
	titleName := "Unknown title"
	if title != nil && title.Title != "" {
		titleName = title.Title
	}

	sourcePages := "kept"
	if format.IsArchive() {
		sourcePages = "removed after archive validation"
	}

	writeHuman(
		out,
		"Download plan\nTitle: %s\nProvider: %s\nChapters: %d selected\nFormat: %s\nOutput: %s\nExisting files: %s\nSource pages: %s\n",
		titleName,
		provider,
		len(chapters),
		format,
		outputRoot,
		existingFileMode,
		sourcePages,
	)
}

func reportDownloadResult(cmd *cobra.Command, record *acquisition.Result, cause error) error {
	completed := 0
	for _, chapter := range record.Chapters {
		if chapter.Status == "complete" || chapter.Status == "skipped" {
			completed++
		}
	}

	code := 5
	if completed == 0 && ErrorCategory(cause.Error()) == "archive" {
		code = 8
	}

	record.Normalize(cmd.Context(), cause)
	if record.Status == "cancelled" {
		code = 7
	}
	if wantsJSON(cmd) {
		if err := writeJSONStatus(cmd, "download", record.Status, record); err != nil {
			return err
		}
	} else if !isQuiet(cmd) {
		writeDownloadSummary(cmd.OutOrStdout(), *record)
	}

	return &ReportedError{Cause: cause, Code: code}
}

func writeDownloadSummary(out io.Writer, record acquisition.Result) {
	completed, skipped, failed, archiveFailed, expectedPages, reusedPages := 0, 0, 0, 0, 0, 0
	for _, chapter := range record.Chapters {
		expectedPages += chapter.ExpectedPages
		switch chapter.Status {
		case "complete":
			completed++
		case "skipped":
			skipped++
			reusedPages += chapter.ExpectedPages
		case "archive_failed":
			archiveFailed++
			failed++
		default:
			failed++
		}
	}

	writeHuman(
		out,
		"Download summary\nCompleted: %d\nSkipped/reused: %d\nFailed or incomplete: %d\nArchive failures: %d\nExpected pages: %d\nReused pages: %d\n",
		completed,
		skipped,
		failed,
		archiveFailed,
		expectedPages,
		reusedPages,
	)
	if len(record.Chapters) == 0 {
		return
	}

	writeHuman(out, "Outputs:\n")
	for _, chapter := range record.Chapters {
		path := chapter.OutputPath
		if chapter.ArchivePath != "" {
			path = chapter.ArchivePath
		}

		writeHuman(out, "  [%s] %s\n", chapter.Status, path)
	}
}

type chapterSelection struct {
	IDs, Numbers         []string
	Range, Before, After string
	First, Latest, All   bool
	Language             string
}

func selectChapters(chapters []*source.Chapter, selection chapterSelection) ([]*source.Chapter, error) {
	if err := selection.validate(); err != nil {
		return nil, err
	}

	if len(selection.IDs) == 0 &&
		len(selection.Numbers) == 0 &&
		selection.Range == "" &&
		!selection.First &&
		!selection.Latest &&
		!selection.All &&
		selection.Before == "" &&
		selection.After == "" {
		return nil, fmt.Errorf("select chapters: choose --chapter-id, --chapter, --range, --first, --latest, or --all")
	}

	filtered := make([]*source.Chapter, 0, len(chapters))
	for _, chapter := range chapters {
		if chapter == nil || (selection.Language != "" && chapter.Language != selection.Language) {
			continue
		}

		filtered = append(filtered, chapter)
	}

	if selection.First || selection.Latest {
		if len(filtered) == 0 {
			return nil, fmt.Errorf("select chapters: no accessible chapters matched")
		}

		if selection.First {
			return []*source.Chapter{filtered[0]}, nil
		}

		return []*source.Chapter{filtered[len(filtered)-1]}, nil
	}

	selected := make([]*source.Chapter, 0, len(filtered))
	seen := map[string]bool{}
	add := func(chapter *source.Chapter) {
		if !seen[chapter.ID] {
			seen[chapter.ID] = true
			selected = append(selected, chapter)
		}
	}
	if selection.All {
		for _, chapter := range filtered {
			add(chapter)
		}
	}

	for _, id := range selection.IDs {
		matched := false
		for _, chapter := range filtered {
			if chapter.ID == id {
				add(chapter)
				matched = true
			}
		}

		if !matched {
			return nil, fmt.Errorf("select chapters: chapter id %q was not found", id)
		}
	}

	for _, number := range selection.Numbers {
		matches := chaptersWithNumber(filtered, number)
		if len(matches) == 0 {
			return nil, fmt.Errorf("select chapters: chapter number %q was not found", number)
		}

		if len(matches) > 1 {
			return nil, fmt.Errorf("select chapters: chapter number %q is ambiguous; select one with --chapter-id", number)
		}

		add(matches[0])
	}

	if selection.Range != "" {
		start, end, ok := strings.Cut(selection.Range, "-")
		if !ok || strings.TrimSpace(start) == "" || strings.TrimSpace(end) == "" {
			return nil, fmt.Errorf("select chapters: malformed range %q; use START-END", selection.Range)
		}

		for _, chapter := range filtered {
			if chapterInRange(chapter.Index, strings.TrimSpace(start), strings.TrimSpace(end)) {
				add(chapter)
			}
		}
	}

	for _, chapter := range filtered {
		if selection.Before != "" && compareChapterLabels(chapter.Index, selection.Before) < 0 {
			add(chapter)
		}

		if selection.After != "" && compareChapterLabels(chapter.Index, selection.After) > 0 {
			add(chapter)
		}
	}

	if len(selected) == 0 {
		return nil, fmt.Errorf("select chapters: no accessible chapters matched")
	}

	return selected, nil
}

func (selection chapterSelection) validate() error {
	modeCount := 0
	for _, enabled := range []bool{selection.First, selection.Latest, selection.All} {
		if enabled {
			modeCount++
		}
	}

	if modeCount > 1 {
		return fmt.Errorf("select chapters: --first, --latest, and --all cannot be combined")
	}

	hasExplicit := len(selection.IDs) > 0 || len(selection.Numbers) > 0
	hasRange := selection.Range != "" || selection.Before != "" || selection.After != ""
	if modeCount > 0 && (hasExplicit || hasRange) {
		return fmt.Errorf("select chapters: --first, --latest, or --all cannot be combined with explicit chapter selectors or ranges")
	}

	if hasExplicit && hasRange {
		return fmt.Errorf("select chapters: explicit chapter selectors cannot be combined with --range, --before, or --after")
	}

	return nil
}

func chapterInRange(value, start, end string) bool {
	return compareChapterLabels(value, start) >= 0 && compareChapterLabels(value, end) <= 0
}

func compareChapterLabels(left, right string) int {
	leftNumber, leftErr := strconv.ParseFloat(left, 64)
	rightNumber, rightErr := strconv.ParseFloat(right, 64)
	if leftErr == nil && rightErr == nil {
		switch {
		case leftNumber < rightNumber:
			return -1
		case leftNumber > rightNumber:
			return 1
		default:
			return 0
		}
	}

	return strings.Compare(left, right)
}

func chaptersWithNumber(chapters []*source.Chapter, number string) []*source.Chapter {
	var matches []*source.Chapter
	for _, chapter := range chapters {
		if chapter.Index == strings.TrimSpace(number) {
			matches = append(matches, chapter)
		}
	}

	return matches
}

func hasCapability(info source.ProviderInfo, capability string) bool {
	for _, value := range info.Capabilities {
		if value == capability {
			return true
		}
	}

	return false
}
