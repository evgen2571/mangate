package downloader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/evgen2571/mangate/internal/source"
	"github.com/evgen2571/mangate/internal/util"
	"golang.org/x/sync/errgroup"
)

// PageDownloadResult describes the finalized output of one page. It never
// includes the temporary page URL, which often expires and is not dataset
// identity.
type PageDownloadResult struct {
	TitleID           string `json:"titleId"`
	ChapterID         string `json:"chapterId"`
	PageIndex         int    `json:"pageIndex"`
	Path              string `json:"path"`
	Extension         string `json:"extension"`
	SourceContentType string `json:"sourceContentType,omitempty"`
	OutputContentType string `json:"outputContentType,omitempty"`
	Bytes             int64  `json:"bytes"`
	Reused            bool   `json:"reused"`
	Converted         bool   `json:"converted"`
}

// ChapterPreflightError identifies an operation rejected before transferring pages.
type ChapterPreflightError struct {
	ChapterID string
	Cause     error
}

func (e *ChapterPreflightError) Error() string { return e.Cause.Error() }
func (e *ChapterPreflightError) Unwrap() error { return e.Cause }
func (e *ChapterPreflightError) Is(target error) bool {
	other, ok := target.(*ChapterPreflightError)
	return ok && e.ChapterID == other.ChapterID
}

// DownloadChapterTo uses the existing transfer, retry, conversion, and atomic
// finalization code while letting a caller choose a stable chapter directory.
// Ordinary downloads keep using their historical display-name destinations.
func (d *Downloader) DownloadChapterTo(ctx context.Context, chapter *source.Chapter, directory string, pageLoader PageLoader) ([]PageDownloadResult, error) {
	return d.DownloadChapterToWithProgress(ctx, chapter, directory, pageLoader, nil)
}

// DownloadChapterToWithProgress reports finalized pages, serialized across workers.
func (d *Downloader) DownloadChapterToWithProgress(ctx context.Context, chapter *source.Chapter, directory string, pageLoader PageLoader, notify func(int, int)) ([]PageDownloadResult, error) {
	if chapter == nil || chapter.From == nil {
		return nil, fmt.Errorf("download chapter to: chapter and parent manga are required")
	}

	if strings.TrimSpace(directory) == "" {
		return nil, fmt.Errorf("download chapter to: directory cannot be empty")
	}

	completedPages := 0
	var progress func(int, int)
	if notify != nil {
		progress = func(completed, total int) {
			if completed > completedPages {
				completedPages = completed
				notify(completed, total)
			}
		}
	}
	var priorResults []PageDownloadResult
	for attempt := 0; ; attempt++ {
		if err := d.prepareChapterPages(ctx, chapter, directory, pageLoader); err != nil {
			if attempt > 0 {
				return priorResults, err
			}
			return nil, &ChapterPreflightError{ChapterID: chapter.ID, Cause: err}
		}
		if err := util.EnsureDir(directory, "chapter directory"); err != nil {
			return nil, err
		}
		results, err := d.downloadChapterToAttempt(ctx, chapter, directory, progress)
		if err == nil {
			if err := writeChapterState(directory, chapter, d.cfg.Provider, true); err != nil {
				return results, err
			}

			return results, nil
		}

		priorResults = results
		err = errors.Join(err, writeChapterState(directory, chapter, d.cfg.Provider, false))
		if pageLoader == nil || attempt >= maxPageRefreshRetries || !isForbiddenPageError(err) {
			return results, err
		}

		if d.cfg.Download.ExistingFileMode == "fail" {
			retry := *d
			retry.cfg.Download.ExistingFileMode = "skip"
			d = &retry
		}

		chapter.Pages = nil
		chapter.PageCount = 0
	}
}

func (d *Downloader) downloadChapterToAttempt(ctx context.Context, chapter *source.Chapter, directory string, notify func(int, int)) ([]PageDownloadResult, error) {
	var progressMu sync.Mutex
	completed := 0
	results := make([]PageDownloadResult, len(chapter.Pages))
	var g errgroup.Group
	g.SetLimit(d.pageDownloadLimit())
	for index, page := range chapter.Pages {
		index, page := index, page
		g.Go(func() (err error) {
			defer func() {
				if err == nil && notify != nil {
					progressMu.Lock()
					completed++
					notify(completed, len(chapter.Pages))
					progressMu.Unlock()
				}
				if err != nil {
					err = fmt.Errorf("manga %q chapter %q page %d to %q: %w", chapter.From.ID, chapter.ID, index+1, directory, err)
				}
			}()
			if err := ctx.Err(); err != nil {
				return err
			}

			base := filepath.Join(directory, fmt.Sprintf("%04d", index+1))
			result := PageDownloadResult{TitleID: chapter.From.ID, ChapterID: chapter.ID, PageIndex: index + 1}
			if existingPage(base) {
				if existingPath := existingPagePath(base); !existingOutputMatchesFormat(existingPath, d.cfg.Download.Format) && d.cfg.Download.ExistingFileMode == "skip" {
					return fmt.Errorf("download page: existing output %q does not match requested %s format; use --existing-files replace", filepath.Ext(existingPath), d.cfg.Download.Format)
				}

				switch d.cfg.Download.ExistingFileMode {
				case "fail":
					return fmt.Errorf("download page: destination already exists for page %d", index+1)
				case "replace":
				default:
					path := existingPagePath(base)
					info, err := os.Stat(path)
					if err != nil {
						return err
					}
					result.Path, result.Bytes, result.Reused = path, info.Size(), true
					result.Extension = filepath.Ext(path)
					result.OutputContentType = mime.TypeByExtension(filepath.Ext(path))
					results[index] = result
					return nil
				}
			}

			path, sourceContentType, err := d.downloadPage(ctx, page, base)
			if err != nil {
				return err
			}

			if d.cfg.Download.ExistingFileMode == "replace" {
				if err := removeExistingPage(base, path); err != nil {
					return err
				}
			}

			info, err := os.Stat(path)
			if err != nil {
				return fmt.Errorf("inspect downloaded page: %w", err)
			}

			result.Path, result.Bytes, result.SourceContentType = path, info.Size(), sourceContentType
			result.Extension = filepath.Ext(path)
			result.OutputContentType = mime.TypeByExtension(filepath.Ext(path))
			result.Converted = d.cfg.Download.Format == "png" || d.cfg.Download.Format == "jpeg"
			results[index] = result
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		available := results[:0]
		for _, result := range results {
			if result.Path != "" {
				available = append(available, result)
			}
		}
		return available, err
	}

	return results, nil
}

func existingPagePath(base string) string {
	paths, err := pagePaths(base)
	if err != nil {
		return ""
	}

	for _, path := range paths {
		if !strings.HasSuffix(path, ".part") {
			if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() && info.Size() > 0 {
				return path
			}
		}
	}

	return ""
}

func pagePaths(base string) ([]string, error) {
	dir, prefix := filepath.Dir(base), filepath.Base(base)+"."
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read page directory %q: %w", dir, err)
	}

	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
			paths = append(paths, filepath.Join(dir, entry.Name()))
		}
	}

	return paths, nil
}

// ValidateExactDestination refuses to reuse unrelated files or chapter identity.
// Matching incomplete chapters can be resumed without relocating their pages.
func ValidateExactDestination(directory string, chapter *source.Chapter, provider string) error {
	info, err := os.Lstat(directory)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect chapter destination: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("chapter destination must be a directory, not a file or symlink")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("read chapter destination: %w", err)
	}
	if len(entries) == 0 {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(directory, ".mangate.json"))
	if err != nil {
		return fmt.Errorf("chapter destination is not an identified Mangate chapter: %w", err)
	}
	var state chapterState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("read chapter destination state: %w", err)
	}
	if chapter == nil || chapter.From == nil || state.TitleID != chapter.From.ID || state.ChapterID != chapter.ID || state.Provider != provider {
		return fmt.Errorf("chapter destination belongs to a different chapter or provider")
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("chapter destination contains a symlink: %s", entry.Name())
		}
	}
	return nil
}

// AvailablePagePaths retains finalized files in page order after partial failure.
// Temporary files and symlinks never become acquisition results.
func AvailablePagePaths(directory string, expected int) ([]string, error) {
	paths := []string{}
	for index := 1; index <= expected; index++ {
		path := existingPagePath(filepath.Join(directory, fmt.Sprintf("%04d", index)))
		if path == "" {
			continue
		}
		info, err := os.Lstat(path)
		if err != nil {
			return nil, fmt.Errorf("inspect acquired page: %w", err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("acquired page is not a regular file: %s", path)
		}
		path, err = filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("resolve acquired page: %w", err)
		}
		paths = append(paths, path)
	}
	return paths, nil
}

// Preflight every page before workers can change any destination or metadata.
func (d *Downloader) prepareChapterPages(ctx context.Context, chapter *source.Chapter, directory string, pageLoader PageLoader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if info, err := os.Lstat(directory); err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
		return fmt.Errorf("chapter destination must be a directory, not a file or symlink")
	}
	entries, err := os.ReadDir(directory)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect chapter destination: %w", err)
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("chapter destination contains a symlink: %s", entry.Name())
		}
		stem, _, _ := strings.Cut(entry.Name(), ".")
		if number, err := strconv.Atoi(stem); err == nil && number > 0 && !entry.Type().IsRegular() {
			return fmt.Errorf("chapter page destination is not a regular file: %s", entry.Name())
		}
		if entry.Name() == ".mangate.json.part" && !entry.Type().IsRegular() {
			return fmt.Errorf("chapter state destination is not a regular file")
		}
	}
	stateData, stateErr := os.ReadFile(filepath.Join(directory, ".mangate.json"))
	if stateErr == nil {
		var state chapterState
		if err := json.Unmarshal(stateData, &state); err != nil {
			return fmt.Errorf("read chapter state: %w", err)
		}
		if state.TitleID != chapter.From.ID || state.ChapterID != chapter.ID || state.Provider != d.cfg.Provider {
			return fmt.Errorf("download chapter to: destination belongs to a different chapter or provider")
		}
	} else if !os.IsNotExist(stateErr) {
		return fmt.Errorf("read chapter state: %w", stateErr)
	}

	if d.cfg.Download.ExistingFileMode == "fail" && len(entries) > 0 {
		return fmt.Errorf("download chapter %q: destination already exists: %s", chapter.ID, directory)
	}
	if len(chapter.Pages) == 0 && pageLoader != nil {
		pages, err := pageLoader(ctx, chapter)
		if err != nil {
			return fmt.Errorf("load pages for chapter %q: %w", chapter.ID, err)
		}
		chapter.Pages, chapter.PageCount = pages, len(pages)
	}
	if len(chapter.Pages) == 0 {
		return fmt.Errorf("download chapter %q: provider returned no pages", chapter.ID)
	}
	for index, page := range chapter.Pages {
		if page == nil || strings.TrimSpace(page.URL) == "" {
			return fmt.Errorf("download chapter %q page %d: missing page URL", chapter.ID, index+1)
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, page.URL, nil)
		if err != nil {
			return fmt.Errorf("validate chapter %q page %d: %w", chapter.ID, index+1, err)
		}
		if (request.URL.Scheme != "http" && request.URL.Scheme != "https") || request.URL.Host == "" {
			return fmt.Errorf("validate chapter %q page %d: expected an HTTP URL", chapter.ID, index+1)
		}
		base := filepath.Join(directory, fmt.Sprintf("%04d", index+1))
		if path := existingPagePath(base); path != "" && d.cfg.Download.ExistingFileMode == "skip" && !existingOutputMatchesFormat(path, d.cfg.Download.Format) {
			return fmt.Errorf("download page: existing output %q does not match requested %s format; use --existing-files replace", filepath.Ext(path), d.cfg.Download.Format)
		}
	}
	return ctx.Err()
}
