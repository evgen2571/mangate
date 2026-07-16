package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/evgen2571/mangate/internal/usecase"
	"github.com/evgen2571/mangate/internal/util"
	"github.com/spf13/cobra"
)

type downloadRecord struct {
	Provider    string            `json:"provider"`
	Title       *source.Manga     `json:"title"`
	Status      string            `json:"status"`
	StartedAt   time.Time         `json:"startedAt"`
	CompletedAt time.Time         `json:"completedAt"`
	Chapters    []chapterDownload `json:"chapters"`
	Error       string            `json:"error,omitempty"`
}

type chapterDownload struct {
	ID            string `json:"id"`
	Number        string `json:"number,omitempty"`
	Title         string `json:"title,omitempty"`
	Status        string `json:"status"`
	OutputPath    string `json:"outputPath"`
	ExpectedPages int    `json:"expectedPages,omitempty"`
}

func NewDownloadCmd(a *app.App) *cobra.Command {
	var chapterIDs []string
	var chapterNumbers []string
	var chapterRange string
	var first, latest, all bool
	var before, after, language string

	cmd := &cobra.Command{
		Use:   "download <title-id>",
		Short: "Download selected chapters that you are authorized to save",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
			record := downloadRecord{Provider: provider.Name(), Title: title, Status: "in_progress", StartedAt: started, Chapters: chapterRecords(a.Cfg.Download.Dir, title, selection, "pending")}
			notify := func(progress usecase.DownloadProgress) {
				if wantsJSON(cmd) || isQuiet(cmd) {
					return
				}
				writeHuman(cmd.ErrOrStderr(), "Downloaded %d/%d pages, %d/%d chapters\r", progress.CompletedPages, progress.TotalPages, progress.CompletedChapters, progress.TotalChapters)
			}
			err = a.UseCases().DownloadChapters(cmd.Context(), title, selection, notify)
			record.CompletedAt = time.Now().UTC()
			if err != nil {
				record.Status = "incomplete"
				record.Error = err.Error()
				updateChapterRecordStates(&record)
				if wantsJSON(cmd) {
					return writeJSON(cmd, "download", record)
				}
				return fmt.Errorf("download title %q: %w", titleID, err)
			}
			for index := range record.Chapters {
				record.Chapters[index].Status = "complete"
			}
			record.Status = "complete"
			if wantsJSON(cmd) {
				return writeJSON(cmd, "download", record)
			}
			if !isQuiet(cmd) {
				writeHuman(cmd.ErrOrStderr(), "\n")
				writeHuman(cmd.OutOrStdout(), "Downloaded %d chapter(s) to %s\n", len(selection), a.Cfg.Download.Dir)
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
	return cmd
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
	if len(selection.IDs) == 0 && len(selection.Numbers) == 0 && selection.Range == "" && !selection.First && !selection.Latest && !selection.All && selection.Before == "" && selection.After == "" {
		return nil, fmt.Errorf("select chapters: choose --chapter-id, --chapter, --range, --first, --latest, or --all")
	}
	if (selection.First && selection.Latest) || (selection.First && selection.All) || (selection.Latest && selection.All) {
		return nil, fmt.Errorf("select chapters: --first, --latest, and --all cannot be combined")
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

func chapterRecords(root string, title *source.Manga, chapters []*source.Chapter, status string) []chapterDownload {
	records := make([]chapterDownload, 0, len(chapters))
	titleDir := util.SanitizeString(title.Title)
	if id := util.SanitizeString(title.ID); id != "unknown" {
		titleDir += "-" + id
	}
	for _, chapter := range chapters {
		name := "unknown-chapter"
		if chapter.Index != "" {
			name = "Chapter-" + chapter.Index
		}
		if chapter.Title != "" {
			name += "-" + chapter.Title
		}
		records = append(records, chapterDownload{ID: chapter.ID, Number: chapter.Index, Title: chapter.Title, Status: status, OutputPath: filepath.Join(root, titleDir, util.SanitizeString(name)), ExpectedPages: chapter.PageCount})
	}
	return records
}

func hasCapability(info source.ProviderInfo, capability string) bool {
	for _, value := range info.Capabilities {
		if value == capability {
			return true
		}
	}
	return false
}
