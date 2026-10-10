package downloader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

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

// DownloadChapterTo uses the existing transfer, retry, conversion, and atomic
// finalization code while letting a caller choose a stable chapter directory.
// Ordinary downloads keep using their historical display-name destinations.
func (d *Downloader) DownloadChapterTo(ctx context.Context, chapter *source.Chapter, directory string, pageLoader PageLoader) ([]PageDownloadResult, error) {
	if chapter == nil || chapter.From == nil {
		return nil, fmt.Errorf("download chapter to: chapter and parent manga are required")
	}

	if strings.TrimSpace(directory) == "" {
		return nil, fmt.Errorf("download chapter to: directory cannot be empty")
	}

	stateData, stateErr := os.ReadFile(filepath.Join(directory, ".mangate.json"))
	if stateErr == nil {
		var state chapterState
		if err := json.Unmarshal(stateData, &state); err != nil {
			return nil, fmt.Errorf("read chapter state: %w", err)
		}
		if state.TitleID != chapter.From.ID || state.ChapterID != chapter.ID || state.Provider != d.cfg.Provider {
			return nil, fmt.Errorf("download chapter to: destination belongs to a different chapter or provider")
		}
	} else if !os.IsNotExist(stateErr) {
		return nil, fmt.Errorf("read chapter state: %w", stateErr)
	}

	if err := util.EnsureDir(directory, "chapter directory"); err != nil {
		return nil, err
	}

	for attempt := 0; ; attempt++ {
		results, err := d.downloadChapterToAttempt(ctx, chapter, directory, pageLoader)
		if err == nil {
			if err := writeChapterState(directory, chapter, d.cfg.Provider, true); err != nil {
				return results, err
			}

			return results, nil
		}

		err = errors.Join(err, writeChapterState(directory, chapter, d.cfg.Provider, false))
		if pageLoader == nil || attempt >= maxPageRefreshRetries || !isForbiddenPageError(err) {
			return results, err
		}

		chapter.Pages = nil
		chapter.PageCount = 0
	}
}

func (d *Downloader) downloadChapterToAttempt(ctx context.Context, chapter *source.Chapter, directory string, pageLoader PageLoader) ([]PageDownloadResult, error) {
	if len(chapter.Pages) == 0 && pageLoader != nil {
		pages, err := pageLoader(ctx, chapter)
		if err != nil {
			return nil, fmt.Errorf("load pages for chapter %q: %w", chapter.ID, err)
		}

		chapter.Pages, chapter.PageCount = pages, len(pages)
	}

	if len(chapter.Pages) == 0 {
		return nil, fmt.Errorf("download chapter %q: provider returned no pages", chapter.ID)
	}

	results := make([]PageDownloadResult, len(chapter.Pages))
	var g errgroup.Group
	g.SetLimit(d.pageDownloadLimit())
	for index, page := range chapter.Pages {
		index, page := index, page
		g.Go(func() (err error) {
			defer func() {
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
