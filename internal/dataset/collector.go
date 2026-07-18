package dataset

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/providers"
	"github.com/evgen2571/mangate/internal/source"
)

type Service struct {
	Store      *Store
	Provider   providers.Provider
	Downloader *downloader.Downloader
}
type CollectResult struct {
	DatasetRoot, DatasetID, Provider string         `json:"datasetRoot"`
	Format                           archive.Format `json:"format"`
	State, StoppingReason            string         `json:"state"`
	Counters                         Counters       `json:"counters"`
	ManifestPath, SummaryPath        string         `json:"manifestPath"`
	Resumed                          bool           `json:"resumed"`
}

func (s Service) Collect(ctx context.Context, cfg Config, resume bool) (CollectResult, error) {
	if err := s.Store.Initialize(ctx, cfg, resume); err != nil {
		return CollectResult{}, err
	}
	if resume {
		if _, err := Verify(ctx, s.Store, true); err != nil {
			return CollectResult{}, fmt.Errorf("reconcile resumed dataset: %w", err)
		}
	}
	titles, chapters, err := s.Store.Planned(ctx)
	if err != nil {
		return CollectResult{}, err
	}
	if len(titles) == 0 {
		if _, err := BuildPlan(ctx, s.Store, s.Provider, cfg); err != nil {
			return CollectResult{}, err
		}
		titles, chapters, err = s.Store.Planned(ctx)
		if err != nil {
			return CollectResult{}, err
		}
	}
	if err := s.Store.SetRun(ctx, "collecting", "", ""); err != nil {
		return CollectResult{}, err
	}
	titleByID := map[string]Title{}
	for _, title := range titles {
		titleByID[title.ID] = title
	}
	reason := ""
	for _, chapterRecord := range chapters {
		if err := ctx.Err(); err != nil {
			_ = s.Store.SetRun(context.Background(), "interrupted", "interrupted", err.Error())
			return s.result(ctx, cfg, resume, "interrupted", "interrupted")
		}
		info, _ := s.Store.Info(ctx)
		if cfg.Limits.MaxPages > 0 && int64(info.Counters.ValidPages)+int64(chapterRecord.ExpectedPages) > cfg.Limits.MaxPages {
			reason = "max_pages"
			break
		}
		if cfg.Limits.MaxBytes > 0 && info.Counters.StoredBytes >= cfg.Limits.MaxBytes {
			reason = "max_bytes"
			break
		}
		if cfg.Limits.MaxFailures > 0 && info.Counters.FailedChapters >= cfg.Limits.MaxFailures {
			reason = "max_failures"
			break
		}
		if chapterRecord.State == "completed" {
			continue
		}
		title := titleByID[chapterRecord.TitleID]
		manga := &source.Manga{ID: title.ID, Title: title.Name, URL: title.URL, Metadata: source.MangaMetadata{Language: title.OriginalLanguage, Status: title.Status, ContentType: title.ContentRating, Year: title.Year}}
		chapter := &source.Chapter{ID: chapterRecord.ID, Index: chapterRecord.Number, Title: chapterRecord.Name, Volume: chapterRecord.Volume, Language: chapterRecord.Language, ReleaseGroup: chapterRecord.ReleaseGroup, PublishedAt: chapterRecord.PublishedAt, URL: chapterRecord.URL, PageCount: chapterRecord.ExpectedPages, From: manga}
		if err := s.collectChapter(ctx, cfg, chapter, title.Split); err != nil {
			_ = s.Store.FailChapter(ctx, chapter.ID, err.Error())
			if cfg.Limits.MaxFailures > 0 {
				info, _ := s.Store.Info(ctx)
				if info.Counters.FailedChapters >= cfg.Limits.MaxFailures {
					reason = "max_failures"
					break
				}
			}
		}
	}
	state := "completed"
	if reason != "" {
		state = "partial"
	}
	if err := s.Store.SetRun(ctx, state, reason, ""); err != nil {
		return CollectResult{}, err
	}
	if err := Export(ctx, s.Store, ExportOptions{}); err != nil {
		return CollectResult{}, err
	}
	if err := Failures(ctx, s.Store); err != nil {
		return CollectResult{}, err
	}
	return s.result(ctx, cfg, resume, state, reason)
}
func (s Service) collectChapter(ctx context.Context, cfg Config, chapter *source.Chapter, split string) error {
	providerDir := filepath.Join(s.Store.Root(), "data", safeSegment(cfg.Provider), safeSegment(chapter.From.ID))
	chapterDir := filepath.Join(providerDir, safeSegment(chapter.ID))
	staging := chapterDir
	if cfg.Output.Format.IsArchive() {
		staging = filepath.Join(s.Store.Root(), ".staging", safeSegment(cfg.Provider), safeSegment(chapter.From.ID), safeSegment(chapter.ID))
	}
	results, err := s.Downloader.DownloadChapterTo(ctx, chapter, staging, s.Provider.Pages)
	if err != nil {
		return err
	}
	valid := true
	type validatedPage struct {
		result downloader.PageDownloadResult
		image  ValidatedImage
	}
	validated := make([]validatedPage, 0, len(results))
	for _, result := range results {
		record, code, err := ValidateFile(result.Path, cfg.Validation)
		if err != nil {
			valid = false
			_ = s.Store.RecordPage(ctx, Page{TitleID: chapter.From.ID, ChapterID: chapter.ID, Index: result.PageIndex, State: "rejected", RejectionCode: code, Split: split})
			continue
		}
		validated = append(validated, validatedPage{result: result, image: record})
	}
	if !valid {
		return s.Store.CompleteChapter(ctx, chapter.ID, chapterDir, false, "one or more pages failed validation")
	}
	relativeBase := ""
	archivePath := ""
	if cfg.Output.Format.IsArchive() {
		archivePath = filepath.Join(providerDir, safeSegment(chapter.ID)+cfg.Output.Format.Extension())
		metadata := archive.Metadata{Provider: cfg.Provider, TitleID: chapter.From.ID, Title: chapter.From.Title, ChapterID: chapter.ID, Volume: chapter.Volume, ChapterNumber: chapter.Index, ChapterTitle: chapter.Title, Language: chapter.Language, ReleaseGroup: chapter.ReleaseGroup, PublishedAt: chapter.PublishedAt, ExpectedPages: len(results), SchemaVersion: "1", Completion: "complete"}
		if _, err := archive.CreateFromDirectoryContext(ctx, archive.Options{Format: cfg.Output.Format, SourceDir: staging, OutputPath: archivePath, ExistingFileMode: cfg.Output.ExistingFiles, Metadata: metadata}); err != nil {
			return err
		}
		relativeBase, err = filepath.Rel(s.Store.Root(), archivePath)
		if err != nil {
			return err
		}
	}
	for _, item := range validated {
		result, record := item.result, item.image
		relative := ""
		entry := ""
		storage := "file"
		if cfg.Output.Format.IsArchive() {
			relative = relativeBase
			entry = filepath.Base(result.Path)
			storage = "archive"
		} else {
			relative, err = filepath.Rel(s.Store.Root(), result.Path)
			if err != nil {
				return err
			}
		}
		duplicate := ""
		if record.SHA256 != "" {
			duplicate, _ = s.findDuplicate(ctx, record.SHA256, chapter.ID, result.PageIndex)
		}
		if err := s.Store.RecordPage(ctx, Page{TitleID: chapter.From.ID, ChapterID: chapter.ID, Index: result.PageIndex, StorageType: storage, RelativePath: filepath.ToSlash(relative), ArchiveEntry: entry, MIMEType: record.MIMEType, Width: record.Width, Height: record.Height, Bytes: record.Bytes, SHA256: record.SHA256, PerceptualHash: record.PerceptualHash, ExactDuplicateOf: duplicate, State: "valid", Split: split}); err != nil {
			return err
		}
	}
	if cfg.Output.Format.IsArchive() {
		if err := os.RemoveAll(staging); err != nil {
			return fmt.Errorf("archive completed but remove staging: %w", err)
		}
	}
	output := chapterDir
	if archivePath != "" {
		output = archivePath
	}
	if err := s.Store.CompleteChapter(ctx, chapter.ID, output, valid, ""); err != nil {
		return err
	}
	if archivePath != "" {
		_, err := s.Store.db.ExecContext(ctx, "UPDATE chapters SET archive_path=? WHERE id=?", archivePath, chapter.ID)
		return err
	}
	return nil
}
func (s Service) findDuplicate(ctx context.Context, hash, chapterID string, index int) (string, error) {
	var identity string
	err := s.Store.db.QueryRowContext(ctx, "SELECT chapter_id || ':' || page_index FROM pages WHERE sha256=? AND state='valid' AND NOT (chapter_id=? AND page_index=?) ORDER BY chapter_id,page_index LIMIT 1", hash, chapterID, index).Scan(&identity)
	if err != nil {
		return "", nil
	}
	return identity, nil
}
func (s Service) result(ctx context.Context, cfg Config, resume bool, state, reason string) (CollectResult, error) {
	info, err := s.Store.Info(ctx)
	if err != nil {
		return CollectResult{}, err
	}
	return CollectResult{DatasetRoot: s.Store.Root(), DatasetID: cfg.DatasetID, Provider: cfg.Provider, Format: cfg.Output.Format, State: state, StoppingReason: reason, Counters: info.Counters, ManifestPath: filepath.Join(s.Store.Root(), "manifest.jsonl"), SummaryPath: filepath.Join(s.Store.Root(), "summary.json"), Resumed: resume}, nil
}
func safeSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, "\\", "_")
	return value
}
