package acquisition

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/providers"
	"github.com/evgen2571/mangate/internal/source"
)

// Service owns acquisition execution; discovery and presentation stay with callers.
type Service struct {
	Config     config.Config
	Provider   providers.Provider
	Downloader *downloader.Downloader
}

// Plan describes historical chapter paths or a single exact destination without writes.
func (s Service) Plan(manga *source.Manga, chapters []*source.Chapter, exact string) (Result, error) {
	model, err := buildDownloadManga(manga, chapters)
	if err != nil {
		return Result{}, err
	}
	root, err := filepath.Abs(s.Config.Download.Dir)
	if err != nil {
		return Result{}, err
	}
	format, err := archive.ParseFormat(s.Config.Download.Format)
	if err != nil {
		return Result{}, err
	}
	record := Result{Provider: s.Provider.Name(), Title: manga, Format: format, OutputRoot: root, Status: "planned", StartedAt: time.Now().UTC(), Chapters: chapterRecords(root, manga, model.Chapters, format, "planned")}
	if exact != "" {
		if len(chapters) != 1 || format.IsArchive() {
			return record, fmt.Errorf("select chapters: exact destination requires one chapter and directory, png, or jpeg format")
		}
		exact, err = filepath.Abs(exact)
		if err != nil {
			return record, err
		}
		if err := downloader.ValidateExactDestination(exact, model.Chapters[0], record.Provider); err != nil {
			return record, err
		}
		record.OutputRoot, record.Chapters[0].OutputPath = exact, exact
	}
	if format.IsArchive() && s.Config.Download.ExistingFileMode == "skip" {
		if err := markReusableArchives(&record); err != nil {
			return record, err
		}
	}
	record.CompletedAt = time.Now().UTC()
	return record, nil
}

func (s Service) Execute(ctx context.Context, manga *source.Manga, chapters []*source.Chapter, exact string, notify func(downloader.DownloadProgress)) (Result, error) {
	record, err := s.Plan(manga, chapters, exact)
	if err != nil {
		return record, err
	}
	model, err := buildDownloadManga(manga, chapters)
	if err != nil {
		return record, err
	}
	return s.execute(ctx, record, model, exact, s.Provider.Pages, notify)
}

func (s Service) execute(ctx context.Context, record Result, manga *source.Manga, exact string, loader downloader.PageLoader, notify func(downloader.DownloadProgress)) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	record.Status = "in_progress"
	for i := range record.Chapters {
		if record.Chapters[i].Status != "skipped" {
			record.Chapters[i].Status = "pending"
		}
	}
	timedLoader := func(ctx context.Context, c *source.Chapter) ([]*source.Page, error) {
		if s.Config.HTTP.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, s.Config.HTTP.Timeout)
			defer cancel()
		}
		return loader(ctx, c)
	}
	var err error
	if exact != "" {
		chapter := manga.Chapters[0]
		_, err = s.Downloader.DownloadChapterToWithProgress(ctx, chapter, record.Chapters[0].OutputPath, timedLoader, func(completed, total int) {
			if notify != nil {
				notify(downloader.DownloadProgress{CompletedPages: completed, TotalPages: total, TotalChapters: 1, Chapters: []downloader.ChapterDownloadProgress{{ChapterID: chapter.ID, Name: chapter.DisplayName(), CompletedPages: completed, TotalPages: total, Active: true}}})
			}
		})
	} else {
		// Keep the original selected sequence: filtering archives would change collision-safe names.
		err = s.Downloader.DownloadMangaWithProgressAndPageLoader(ctx, manga, timedLoader, notify)
	}
	for i := range record.Chapters {
		if errors.Is(err, &downloader.ChapterPreflightError{ChapterID: record.Chapters[i].ID}) {
			record.Chapters[i].Status = "failed"
		}
	}
	err = errors.Join(err, updateChapterRecordStates(&record))
	if record.Format.IsArchive() {
		if notify != nil {
			progress := downloader.DownloadProgress{TotalChapters: len(record.Chapters)}
			for _, chapter := range record.Chapters {
				progress.TotalPages += chapter.ExpectedPages
				if chapter.Status == "complete" || chapter.Status == "skipped" {
					progress.CompletedPages += chapter.ExpectedPages
					progress.CompletedChapters++
				} else {
					progress.CompletedPages += len(chapter.ImagePaths)
				}
			}
			progress.Chapters = []downloader.ChapterDownloadProgress{{Name: "Creating and validating archive...", Active: true}}
			notify(progress)
		}
		err = errors.Join(err, finalizeArchives(ctx, record, record.Title, manga.Chapters, record.Format, s.Config.Download.ExistingFileMode, true))
	}
	record.CompletedAt = time.Now().UTC()
	record.Normalize(ctx, err)
	return record, err
}

// ExecutePlan revalidates the selection and applies its eligibility checks on every URL refresh.
func (s Service) ExecutePlan(ctx context.Context, plan DownloadPlan, notify Notify) (Result, error) {
	if err := plan.Validate(); err != nil {
		return Result{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var pathErr error
	plan.Output, pathErr = filepath.Abs(plan.Output)
	if pathErr != nil {
		return Result{}, pathErr
	}
	record := Result{Provider: plan.Provider, Title: plan.Manga, Format: plan.Format, OutputRoot: plan.Output, StartedAt: time.Now().UTC(), Status: "in_progress", Chapters: chapterRecords(plan.Output, plan.Manga, []*source.Chapter{plan.Chapter}, plan.Format, "pending")}
	record.Chapters[0].OutputPath = plan.Output
	chapter, err := plan.Resolve(ctx, s.Provider, notify)
	if err == nil {
		err = downloader.ValidateExactDestination(plan.Output, chapter, plan.Provider)
	}
	if err == nil {
		emit(notify, ProgressEvent{Stage: "downloading", Status: "started", MangaID: plan.Manga.ID, ChapterID: chapter.ID, ExpectedPages: plan.ExpectedPages})
		loader := func(ctx context.Context, c *source.Chapter) ([]*source.Page, error) {
			loaded, err := s.Provider.Pages(ctx, c)
			if err != nil {
				return nil, err
			}
			if len(loaded) != plan.ExpectedPages || !plan.Policy.MatchPages(len(loaded)) {
				return nil, fmt.Errorf("plan validation: page count changed")
			}
			return loaded, nil
		}
		model := *chapter.From
		model.Chapters = []*source.Chapter{chapter}
		record, err = s.execute(ctx, record, &model, plan.Output, loader, func(p downloader.DownloadProgress) {
			emit(notify, ProgressEvent{Stage: "downloading", Status: "progress", ChapterID: chapter.ID, CompletedPages: p.CompletedPages, ExpectedPages: p.TotalPages})
		})
	} else {
		record.Chapters[0].Status = "failed"
		record.CompletedAt = time.Now().UTC()
		record.Normalize(ctx, err)
	}
	pages := []downloader.PageDownloadResult{}
	for _, path := range record.Chapters[0].ImagePaths {
		index, parseErr := strconv.Atoi(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
		if parseErr != nil {
			err = errors.Join(err, parseErr)
			continue
		}
		pages = append(pages, downloader.PageDownloadResult{PageIndex: index, Path: path})
	}
	m, manifestErr := NewManifest(plan, pages, record.Status, err)
	if manifestErr == nil {
		record.Manifest = &m
		if m.Status != record.Status {
			err = errors.Join(err, fmt.Errorf("acquisition validation: downloaded image validation failed"))
			record.Chapters[0].Status = m.Status
		}
	}
	err = errors.Join(err, manifestErr)
	record.Normalize(ctx, err)
	record.Chapters[0].Status = record.Status
	return record, err
}
