package acquisition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/source"
)

type Result struct {
	Manifest    *Manifest       `json:"manifest,omitempty"`
	Provider    string          `json:"provider"`
	Title       *source.Manga   `json:"title"`
	Format      archive.Format  `json:"format"`
	OutputRoot  string          `json:"outputRoot"`
	Status      string          `json:"status"`
	StartedAt   time.Time       `json:"startedAt"`
	CompletedAt time.Time       `json:"completedAt"`
	Chapters    []ChapterResult `json:"chapters"`
	Error       string          `json:"error,omitempty"`
}

type ChapterResult struct {
	ID            string              `json:"id"`
	Number        string              `json:"number,omitempty"`
	Title         string              `json:"title,omitempty"`
	Status        string              `json:"status"`
	OutputPath    string              `json:"outputPath"`
	ArchivePath   string              `json:"archivePath,omitempty"`
	Validation    *archive.Validation `json:"validation,omitempty"`
	ExpectedPages int                 `json:"expectedPages,omitempty"`
	Chapter       *source.Chapter     `json:"chapter"`
	ImagePaths    []string            `json:"imagePaths"`
}

func updateChapterRecordStates(record *Result) error {
	var failures []error
	for index := range record.Chapters {
		if record.Chapters[index].Status == "skipped" || record.Chapters[index].Status == "failed" {
			continue
		}

		stateData, err := os.ReadFile(filepath.Join(record.Chapters[index].OutputPath, ".mangate.json"))
		if err != nil {
			record.Chapters[index].Status = "incomplete"
			failures = append(failures, fmt.Errorf("read chapter state: %w", err))
			continue
		}

		var state struct {
			Complete      bool   `json:"complete"`
			ExpectedPages int    `json:"expectedPages"`
			TitleID       string `json:"titleId"`
			ChapterID     string `json:"chapterId"`
			Provider      string `json:"provider"`
		}
		if err := json.Unmarshal(stateData, &state); err != nil {
			record.Chapters[index].Status = "incomplete"
			failures = append(failures, fmt.Errorf("decode chapter state: %w", err))
			continue
		}
		if record.Title != nil && (state.TitleID != record.Title.ID || state.ChapterID != record.Chapters[index].ID || state.Provider != record.Provider) {
			record.Chapters[index].Status = "incomplete"
			record.Chapters[index].ImagePaths = []string{}
			failures = append(failures, fmt.Errorf("chapter destination belongs to a different chapter or provider"))
			continue
		}
		record.Chapters[index].ExpectedPages = state.ExpectedPages
		if record.Chapters[index].Chapter != nil {
			record.Chapters[index].Chapter.PageCount = state.ExpectedPages
		}
		if state.Complete {
			paths, err := archive.ValidatedImagePaths(record.Chapters[index].OutputPath, state.ExpectedPages)
			if err != nil {
				record.Chapters[index].Status = "incomplete"
				failures = append(failures, err)
				paths, pathErr := downloader.AvailablePagePaths(record.Chapters[index].OutputPath, state.ExpectedPages)
				record.Chapters[index].ImagePaths = paths
				failures = append(failures, pathErr)
				continue
			}
			record.Chapters[index].ImagePaths = paths
			record.Chapters[index].Status = "complete"
		} else {
			record.Chapters[index].Status = "incomplete"
			failures = append(failures, fmt.Errorf("chapter %q output is incomplete", record.Chapters[index].ID))
			paths, err := downloader.AvailablePagePaths(record.Chapters[index].OutputPath, state.ExpectedPages)
			if err != nil {
				failures = append(failures, err)
			} else {
				record.Chapters[index].ImagePaths = paths
			}
		}
	}
	return errors.Join(failures...)
}

func markReusableArchives(record *Result) error {
	for index := range record.Chapters {
		chapterRecord := &record.Chapters[index]
		if _, err := os.Stat(chapterRecord.ArchivePath); err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return fmt.Errorf("inspect existing archive %q: %w", chapterRecord.ArchivePath, err)
		}

		inspection, err := archive.Inspect(chapterRecord.ArchivePath)
		if err == nil && inspection.Complete && inspection.TitleID == record.Title.ID && inspection.ChapterID == chapterRecord.ID {
			chapterRecord.Status = "skipped"
			chapterRecord.Validation = &inspection.Validation
			continue
		}

		if err != nil {
			return fmt.Errorf("existing archive %q is invalid; use --existing-files replace to replace it: %w", chapterRecord.ArchivePath, err)
		}

		return fmt.Errorf("existing archive %q belongs to a different chapter; use --existing-files replace to replace it", chapterRecord.ArchivePath)
	}

	return nil
}

func chapterRecords(root string, title *source.Manga, chapters []*source.Chapter, format archive.Format, status string) []ChapterResult {
	records := make([]ChapterResult, 0, len(chapters))
	titleDir := downloader.TitleDirectoryName(title)

	names := downloader.ChapterDirectoryNames(chapters)
	for index, chapter := range chapters {
		directory := filepath.Join(root, titleDir, names[index])
		chapterCopy := *chapter
		chapterCopy.Pages = nil
		record := ChapterResult{Chapter: &chapterCopy, ImagePaths: []string{}, ID: chapter.ID, Number: chapter.Index, Title: chapter.Title, Status: status, OutputPath: directory, ExpectedPages: chapter.PageCount}
		if format.IsArchive() {
			record.ArchivePath = directory + format.Extension()
		}

		records = append(records, record)
	}

	return records
}

func finalizeArchives(ctx context.Context, record Result, title *source.Manga, chapters []*source.Chapter, format archive.Format, existingMode string, removeSource bool) error {
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
		if result.SourceRemoved {
			chapterRecord.ImagePaths = []string{}
		}
		if result.Status == archive.StatusSkipped {
			chapterRecord.Status = "skipped"
		}
	}

	return errors.Join(failures...)
}

func buildDownloadManga(manga *source.Manga, chapters []*source.Chapter) (*source.Manga, error) {
	if manga == nil {
		return nil, fmt.Errorf("download chapters: nil manga")
	}

	if len(chapters) == 0 {
		return nil, fmt.Errorf("download chapters: no chapters selected")
	}

	downloadManga := &source.Manga{
		ID:       manga.ID,
		URL:      manga.URL,
		Title:    manga.Title,
		Cover:    manga.Cover,
		Metadata: manga.Metadata,
		Chapters: make([]*source.Chapter, 0, len(chapters)),
	}

	for _, chapter := range chapters {
		if chapter == nil {
			return nil, fmt.Errorf("download chapters: selected chapter is nil")
		}

		chapterCopy := *chapter
		chapterCopy.From = downloadManga
		chapterCopy.Pages = nil
		downloadManga.Chapters = append(downloadManga.Chapters, &chapterCopy)
	}

	return downloadManga, nil
}

// Normalize applies one outcome policy while retaining the established chapter status contract.
func (r *Result) Normalize(ctx context.Context, cause error) {
	r.Status = "complete"
	if cause == nil {
		return
	}
	r.Error = cause.Error()
	r.Status = "failed"
	for _, chapter := range r.Chapters {
		if chapter.Status == "complete" || chapter.Status == "skipped" || chapter.Status != "failed" && len(chapter.ImagePaths) > 0 {
			r.Status = "partial"
		}
	}
	if errors.Is(cause, context.Canceled) || ctx != nil && ctx.Err() != nil {
		r.Status = "cancelled"
		for i := range r.Chapters {
			if r.Chapters[i].Status != "complete" && r.Chapters[i].Status != "skipped" {
				r.Chapters[i].Status = "cancelled"
			}
		}
	}
}
