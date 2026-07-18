package downloader

import (
	"context"
	"fmt"
	"mime"
	"os"
	"path/filepath"
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
	if err := util.EnsureDir(directory, "chapter directory"); err != nil {
		return nil, err
	}
	for attempt := 0; ; attempt++ {
		results, err := d.downloadChapterToAttempt(ctx, chapter, directory, pageLoader)
		if err == nil {
			if err := writeChapterState(directory, chapter, d.cfg.Provider, true); err != nil {
				return nil, err
			}
			return results, nil
		}
		_ = writeChapterState(directory, chapter, d.cfg.Provider, false)
		if pageLoader == nil || attempt >= maxPageRefreshRetries || !isForbiddenPageError(err) {
			return nil, err
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
	var mu sync.Mutex
	var g errgroup.Group
	for index, page := range chapter.Pages {
		index, page := index, page
		g.Go(func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			base := filepath.Join(directory, fmt.Sprintf("%04d", index+1))
			result := PageDownloadResult{TitleID: chapter.From.ID, ChapterID: chapter.ID, PageIndex: index + 1}
			if existingPage(base) {
				switch d.cfg.Download.ExistingFileMode {
				case "fail":
					return fmt.Errorf("download page: destination already exists for page %d", index+1)
				case "replace":
					if err := removeExistingPage(base); err != nil {
						return err
					}
				default:
					path := existingPagePath(base)
					info, err := os.Stat(path)
					if err != nil {
						return err
					}
					result.Path, result.Bytes, result.Reused = path, info.Size(), true
					result.OutputContentType = mime.TypeByExtension(filepath.Ext(path))
					mu.Lock()
					results[index] = result
					mu.Unlock()
					return nil
				}
			}
			path, err := d.downloadPage(ctx, page, base)
			if err != nil {
				return err
			}
			info, err := os.Stat(path)
			if err != nil {
				return fmt.Errorf("inspect downloaded page: %w", err)
			}
			result.Path, result.Bytes = path, info.Size()
			result.OutputContentType = mime.TypeByExtension(filepath.Ext(path))
			result.Converted = d.cfg.Download.Format == "png" || d.cfg.Download.Format == "jpeg"
			mu.Lock()
			results[index] = result
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}

func existingPagePath(base string) string {
	paths, err := filepath.Glob(base + ".*")
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
