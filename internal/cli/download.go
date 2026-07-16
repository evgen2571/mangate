package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/archive"
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

	cmd := &cobra.Command{
		Use:   "download <title-id>",
		Short: "Download selected chapters that you are authorized to save",
		Args:  cobra.ExactArgs(1),
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
				record.Status = "planned"
				record.CompletedAt = time.Now().UTC()
				for index := range record.Chapters {
					record.Chapters[index].Status = "planned"
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
					writeHuman(cmd.OutOrStdout(), "  %s -> %s\n", chapter.ID, path)
				}
				return nil
			}
			notify := func(progress usecase.DownloadProgress) {
				if wantsJSON(cmd) || isQuiet(cmd) {
					return
				}
				writeHuman(cmd.ErrOrStderr(), "Downloaded %d/%d pages, %d/%d chapters\r", progress.CompletedPages, progress.TotalPages, progress.CompletedChapters, progress.TotalChapters)
			}
			err = a.UseCases().DownloadChapters(cmd.Context(), title, selection, notify)
			record.CompletedAt = time.Now().UTC()
			updateChapterRecordStates(&record)
			if err != nil {
				record.Error = err.Error()
				if format != archive.FormatDirectory {
					if archiveErr := finalizeArchives(record, title, selection, format, a.Cfg.Download.ExistingFileMode, !a.Cfg.Download.RetainSource); archiveErr != nil {
						record.Error = errors.Join(err, archiveErr).Error()
					}
				}
				return reportDownloadResult(cmd, &record, fmt.Errorf("download title %q: %w", titleID, err))
			}
			for index := range record.Chapters {
				record.Chapters[index].Status = "complete"
			}
			if format != archive.FormatDirectory {
				if err := finalizeArchives(record, title, selection, format, a.Cfg.Download.ExistingFileMode, !a.Cfg.Download.RetainSource); err != nil {
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
				writeHuman(cmd.OutOrStdout(), "Downloaded %d chapter(s) as %s to %s\n", len(selection), format, a.Cfg.Download.Dir)
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
	return cmd
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
	}
	return &ReportedError{Cause: cause, Code: code}
}

func updateChapterRecordStates(record *downloadRecord) {
	for index := range record.Chapters {
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
	names := chapterDirectoryNames(chapters)
	for index, chapter := range chapters {
		directory := filepath.Join(root, titleDir, names[index])
		record := chapterDownload{ID: chapter.ID, Number: chapter.Index, Title: chapter.Title, Status: status, OutputPath: directory, ExpectedPages: chapter.PageCount}
		if format != archive.FormatDirectory {
			record.ArchivePath = directory + format.Extension()
		}
		records = append(records, record)
	}
	return records
}

func chapterDirectoryNames(chapters []*source.Chapter) []string {
	names := make([]string, len(chapters))
	used := make(map[string]struct{}, len(chapters))
	for index, chapter := range chapters {
		name := "unknown-chapter"
		if chapter.Index != "" {
			name = "Chapter-" + chapter.Index
		}
		if chapter.Title != "" {
			name += "-" + chapter.Title
		}
		name = util.SanitizeString(name)
		if _, exists := used[name]; exists {
			name = util.SanitizeString(name + "-" + chapter.ID)
		}
		used[name] = struct{}{}
		names[index] = name
	}
	return names
}

func finalizeArchives(record downloadRecord, title *source.Manga, chapters []*source.Chapter, format archive.Format, existingMode string, removeSource bool) error {
	var failures []error
	for index, chapter := range chapters {
		chapterRecord := &record.Chapters[index]
		if chapterRecord.Status != "complete" {
			continue
		}
		result, err := archive.CreateFromDirectory(archive.Options{
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
				ChapterNumber: chapter.Index,
				ChapterTitle:  chapter.Title,
				Language:      chapter.Language,
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
