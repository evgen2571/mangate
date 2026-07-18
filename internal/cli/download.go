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

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/evgen2571/mangate/internal/usecase"
	"github.com/evgen2571/mangate/internal/util"
	"github.com/spf13/cobra"
)

type downloadRecord struct {
	Provider    string            `json:"provider"`
	Title       *source.Manga     `json:"title"`
	Format      archive.Format    `json:"format"`
	OutputRoot  string            `json:"outputRoot"`
	Status      string            `json:"status"`
	StartedAt   time.Time         `json:"startedAt"`
	CompletedAt time.Time         `json:"completedAt"`
	Chapters    []chapterDownload `json:"chapters"`
	Error       string            `json:"error,omitempty"`
}

type chapterDownload struct {
	ID            string              `json:"id"`
	Number        string              `json:"number,omitempty"`
	Title         string              `json:"title,omitempty"`
	Status        string              `json:"status"`
	OutputPath    string              `json:"outputPath"`
	ArchivePath   string              `json:"archivePath,omitempty"`
	Validation    *archive.Validation `json:"validation,omitempty"`
	ExpectedPages int                 `json:"expectedPages,omitempty"`
}

func NewDownloadCmd(a *app.App) *cobra.Command {
	var chapterIDs []string
	var chapterNumbers []string
	var chapterRange string
	var first, latest, all bool
	var before, after, language string
	var dryRun bool
	var assumeYes bool

	cmd := &cobra.Command{
		Use:     "download <title-id>",
		Short:   "Download selected chapters that you are authorized to save",
		Example: "  mangate download <title-id> --chapter-id <chapter-id>\n  mangate download <title-id> --range 1-10\n  mangate --format cbz download <title-id> --latest\n  mangate --json download <title-id> --chapter 1",
		Args:    requireOneArgument("a stable <title-id> from `mangate search`", "mangate download <title-id> --latest"),
		RunE: func(cmd *cobra.Command, args []string) error {
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
			selection, err := selectChapters(chapters, chapterSelection{IDs: chapterIDs, Numbers: chapterNumbers, Range: chapterRange, First: first, Latest: latest, All: all, Before: before, After: after, Language: language})
			if err != nil {
				return err
			}

			started := time.Now().UTC()
			record := downloadRecord{Provider: provider.Name(), Title: title, Format: format, OutputRoot: a.Cfg.Download.Dir, Status: "in_progress", StartedAt: started, Chapters: chapterRecords(a.Cfg.Download.Dir, title, selection, format, "pending")}
			if dryRun {
				if format.IsArchive() && a.Cfg.Download.ExistingFileMode == string(archive.ExistingSkip) {
					if _, err := reusableArchiveSelection(&record, selection, title); err != nil {
						return err
					}
				}
				record.Status = "planned"
				record.CompletedAt = time.Now().UTC()
				for index := range record.Chapters {
					if record.Chapters[index].Status != "skipped" {
						record.Chapters[index].Status = "planned"
					}
				}
				if wantsJSON(cmd) {
					return writeJSON(cmd, "download.plan", record)
				}
				writeHuman(cmd.OutOrStdout(), "Title: %s\nProvider: %s\nChapters: %d selected\nFormat: %s\nOutput: %s\nDry run: no files will be changed\n", title.Title, provider.Name(), len(selection), format, a.Cfg.Download.Dir)
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
				writeDownloadPreflight(cmd.ErrOrStderr(), title, provider.Name(), selection, format, a.Cfg.Download.Dir, a.Cfg.Download.ExistingFileMode)
			}
			pendingSelection := selection
			if format.IsArchive() && a.Cfg.Download.ExistingFileMode == string(archive.ExistingSkip) {
				pendingSelection, err = reusableArchiveSelection(&record, selection, title)
				if err != nil {
					return err
				}
			}
			notify := func(progress usecase.DownloadProgress) {
				if wantsJSON(cmd) || isQuiet(cmd) {
					return
				}
				writeHuman(cmd.ErrOrStderr(), "Downloaded %d/%d pages, %d/%d chapters\r", progress.CompletedPages, progress.TotalPages, progress.CompletedChapters, progress.TotalChapters)
			}
			if len(pendingSelection) == 0 {
				record.CompletedAt = time.Now().UTC()
				record.Status = "complete"
				if wantsJSON(cmd) {
					return writeJSON(cmd, "download", record)
				}
				if !isQuiet(cmd) {
					writeDownloadSummary(cmd.OutOrStdout(), record)
				}
				return nil
			}
			err = a.UseCases().DownloadChapters(cmd.Context(), title, pendingSelection, notify)
			record.CompletedAt = time.Now().UTC()
			updateChapterRecordStates(&record)
			if err != nil {
				record.Error = err.Error()
				if format.IsArchive() {
					if archiveErr := finalizeArchives(cmd.Context(), record, title, selection, format, a.Cfg.Download.ExistingFileMode, true); archiveErr != nil {
						record.Error = errors.Join(err, archiveErr).Error()
					}
				}
				return reportDownloadResult(cmd, &record, fmt.Errorf("download title %q: %w", titleID, err))
			}
			if format.IsArchive() {
				if err := finalizeArchives(cmd.Context(), record, title, selection, format, a.Cfg.Download.ExistingFileMode, true); err != nil {
					record.Error = err.Error()
					return reportDownloadResult(cmd, &record, err)
				}
			}
			record.Status = "complete"
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
	flags := cmd.Flags()
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
	writeHuman(out, "Download plan\nTitle: %s\nProvider: %s\nChapters: %d selected\nFormat: %s\nOutput: %s\nExisting files: %s\nSource pages: %s\n", titleName, provider, len(chapters), format, outputRoot, existingFileMode, sourcePages)
}

func reportDownloadResult(cmd *cobra.Command, record *downloadRecord, cause error) error {
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
	if completed > 0 {
		record.Status = "partial"
	} else {
		record.Status = "incomplete"
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

func writeDownloadSummary(out io.Writer, record downloadRecord) {
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
	writeHuman(out, "Download summary\nCompleted: %d\nSkipped/reused: %d\nFailed or incomplete: %d\nArchive failures: %d\nExpected pages: %d\nReused pages: %d\n", completed, skipped, failed, archiveFailed, expectedPages, reusedPages)
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

func updateChapterRecordStates(record *downloadRecord) {
	for index := range record.Chapters {
		if record.Chapters[index].Status == "skipped" {
			continue
		}
		stateData, err := os.ReadFile(filepath.Join(record.Chapters[index].OutputPath, ".mangate.json"))
		if err != nil {
			record.Chapters[index].Status = "incomplete"
			continue
		}
		var state struct {
			Complete bool `json:"complete"`
		}
		if json.Unmarshal(stateData, &state) == nil && state.Complete {
			record.Chapters[index].Status = "complete"
		} else {
			record.Chapters[index].Status = "incomplete"
		}
	}
}

func reusableArchiveSelection(record *downloadRecord, selection []*source.Chapter, title *source.Manga) ([]*source.Chapter, error) {
	pending := make([]*source.Chapter, 0, len(selection))
	for index, chapter := range selection {
		chapterRecord := &record.Chapters[index]
		if _, err := os.Stat(chapterRecord.ArchivePath); err != nil {
			if os.IsNotExist(err) {
				pending = append(pending, chapter)
				continue
			}
			return nil, fmt.Errorf("inspect existing archive %q: %w", chapterRecord.ArchivePath, err)
		}
		inspection, err := archive.Inspect(chapterRecord.ArchivePath)
		if err == nil && inspection.Complete && inspection.TitleID == title.ID && inspection.ChapterID == chapter.ID {
			chapterRecord.Status = "skipped"
			chapterRecord.Validation = &inspection.Validation
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("existing archive %q is invalid; use --existing-files replace to replace it: %w", chapterRecord.ArchivePath, err)
		}
		return nil, fmt.Errorf("existing archive %q belongs to a different chapter; use --existing-files replace to replace it", chapterRecord.ArchivePath)
	}
	return pending, nil
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
	if len(selection.IDs) == 0 && len(selection.Numbers) == 0 && selection.Range == "" && !selection.First && !selection.Latest && !selection.All && selection.Before == "" && selection.After == "" {
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

func chapterRecords(root string, title *source.Manga, chapters []*source.Chapter, format archive.Format, status string) []chapterDownload {
	records := make([]chapterDownload, 0, len(chapters))
	titleDir := util.SanitizeString(title.Title)
	if id := util.SanitizeString(title.ID); id != "unknown" {
		titleDir += "-" + id
	}
	names := downloader.ChapterDirectoryNames(chapters)
	for index, chapter := range chapters {
		directory := filepath.Join(root, titleDir, names[index])
		record := chapterDownload{ID: chapter.ID, Number: chapter.Index, Title: chapter.Title, Status: status, OutputPath: directory, ExpectedPages: chapter.PageCount}
		if format.IsArchive() {
			record.ArchivePath = directory + format.Extension()
		}
		records = append(records, record)
	}
	return records
}

func finalizeArchives(ctx context.Context, record downloadRecord, title *source.Manga, chapters []*source.Chapter, format archive.Format, existingMode string, removeSource bool) error {
	var failures []error
	for index, chapter := range chapters {
		chapterRecord := &record.Chapters[index]
		if chapterRecord.Status != "complete" {
			continue
		}
		result, err := archive.CreateFromDirectoryContext(ctx, archive.Options{
			Format:           format,
			SourceDir:        chapterRecord.OutputPath,
			OutputPath:       chapterRecord.ArchivePath,
			ExistingFileMode: archive.ExistingFileMode(existingMode),
			RemoveSource:     removeSource,
			Metadata: archive.Metadata{
				Provider:      record.Provider,
				TitleID:       title.ID,
				Title:         title.Title,
				ChapterID:     chapter.ID,
				Volume:        chapter.Volume,
				ChapterNumber: chapter.Index,
				ChapterTitle:  chapter.Title,
				Language:      chapter.Language,
				ReleaseGroup:  chapter.ReleaseGroup,
				PublishedAt:   chapter.PublishedAt,
				ExpectedPages: chapter.PageCount,
			},
		})
		if err != nil {
			chapterRecord.Status = "archive_failed"
			failures = append(failures, fmt.Errorf("create %s for chapter %q: %w", format, chapter.ID, err))
			continue
		}
		chapterRecord.Validation = &result.Validation
		if result.Status == archive.StatusSkipped {
			chapterRecord.Status = "skipped"
		}
	}
	return errors.Join(failures...)
}

func hasCapability(info source.ProviderInfo, capability string) bool {
	for _, value := range info.Capabilities {
		if value == capability {
			return true
		}
	}
	return false
}
