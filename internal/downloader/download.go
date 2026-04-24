package downloader

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	urlpkg "net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/evgen2571/mangate/internal/source"
	"github.com/evgen2571/mangate/internal/util"
	"golang.org/x/sync/errgroup"
)

type PageLoader func(context.Context, *source.Chapter) ([]*source.Page, error)

const maxPageDownloadRetries = 3

func (d *Downloader) DownloadChapter(c *source.Chapter) error {
	return d.downloadChapter(context.Background(), c, nil, nil)
}

func (d *Downloader) downloadChapter(ctx context.Context, c *source.Chapter, reporter *progressReporter, pageLoader PageLoader) error {
	if c == nil {
		return fmt.Errorf("download chapter: nil chapter")
	}
	if c.From == nil {
		return fmt.Errorf("download chapter %q: missing parent manga", c.ID)
	}
	if len(c.Pages) == 0 && pageLoader != nil {
		pages, err := pageLoader(ctx, c)
		if err != nil {
			return fmt.Errorf("load pages for chapter %q: %w", c.ID, err)
		}
		c.Pages = pages
		if reporter != nil {
			reporter.pagesDiscovered(c)
		}
	}

	basePath, err := d.basePath()
	if err != nil {
		return err
	}

	chapterDir := filepath.Join(
		basePath,
		util.SanitizeString(c.From.Title),
		util.SanitizeString(chapterDirName(c)),
	)

	if err := os.RemoveAll(chapterDir); err != nil {
		return fmt.Errorf("remove existing chapter workspace %q: %w", chapterDir, err)
	}
	if err := util.EnsureDir(chapterDir, "chapter directory"); err != nil {
		return err
	}

	var g errgroup.Group

	if reporter != nil {
		reporter.chapterStarted(c)
	}

	for idx, page := range c.Pages {
		idx := idx
		page := page

		g.Go(func() error {
			filePathBase := filepath.Join(
				chapterDir,
				fmt.Sprintf("%04d", idx+1),
			)

			if _, err := d.downloadPage(page, filePathBase); err != nil {
				return err
			}
			if reporter != nil {
				reporter.pageCompleted(c)
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	if err := d.converter.ConvertChapter(
		chapterDir,
		util.SanitizeString(c.From.Title),
		util.SanitizeString(chapterDirName(c)),
	); err != nil {
		return err
	}

	if reporter != nil {
		reporter.chapterCompleted(c)
	}

	return nil
}

func (d *Downloader) DownloadManga(m *source.Manga) error {
	return d.downloadManga(context.Background(), m, nil, nil)
}

func (d *Downloader) DownloadMangaWithProgress(m *source.Manga, notify func(DownloadProgress)) error {
	return d.downloadManga(context.Background(), m, newProgressReporter(m, notify), nil)
}

func (d *Downloader) DownloadMangaWithProgressAndPageLoader(ctx context.Context, m *source.Manga, pageLoader PageLoader, notify func(DownloadProgress)) error {
	return d.downloadManga(ctx, m, newProgressReporter(m, notify), pageLoader)
}

func (d *Downloader) downloadManga(ctx context.Context, m *source.Manga, reporter *progressReporter, pageLoader PageLoader) error {
	if m == nil {
		return fmt.Errorf("download manga: nil manga")
	}

	basePath, err := d.basePath()
	if err != nil {
		return err
	}

	mangaDir := filepath.Join(
		basePath,
		util.SanitizeString(m.Title),
	)

	if err := util.EnsureDir(mangaDir, "manga directory"); err != nil {
		return err
	}

	var g errgroup.Group
	g.SetLimit(d.cfg.Concurrency.ChapterDownloads)

	for _, chapter := range m.Chapters {
		chapter := chapter

		g.Go(func() error {
			if err := d.downloadChapter(ctx, chapter, reporter, pageLoader); err != nil {
				return err
			}

			return nil
		})
	}

	return g.Wait()
}

func chapterDirName(c *source.Chapter) string {
	if c == nil {
		return "unknown-chapter"
	}

	index := strings.TrimSpace(c.Index)
	title := strings.TrimSpace(c.Title)

	switch {
	case index != "" && title != "":
		return "Chapter-" + index + "-" + title
	case index != "":
		return "Chapter-" + index
	case title != "":
		return "Title-" + title
	default:
		return "unknown-chapter"
	}
}

func (d *Downloader) downloadPage(p *source.Page, filePathBase string) (string, error) {
	if p == nil {
		return "", fmt.Errorf("download page: nil page")
	}
	if strings.TrimSpace(p.URL) == "" {
		return "", fmt.Errorf("download page: empty page url")
	}

	d.acquirePageDownload()
	defer d.releasePageDownload()

	resp, err := d.getPageResponseWithRetry(p.URL)
	if err != nil {
		return "", fmt.Errorf("failed to GET %q: %w", p.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %v", resp.StatusCode)
	}

	filePath := filePathBase + detectPageExtension(resp.Header.Get("Content-Type"), p.URL)
	out, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file %q: %w", filePath, err)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = out.Close()
		_ = os.Remove(filePath)
		return "", fmt.Errorf("failed to write file %q: %w", filePath, err)
	}

	if err := out.Close(); err != nil {
		_ = os.Remove(filePath)
		return "", fmt.Errorf("failed to close file %q: %w", filePath, err)
	}

	return filePath, nil
}

func (d *Downloader) getPageResponseWithRetry(rawURL string) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		resp, err := d.client.Get(rawURL)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusTooManyRequests || attempt >= maxPageDownloadRetries {
			return resp, nil
		}

		wait := pageRetryAfterDelay(resp.Header.Get("Retry-After"), attempt)
		resp.Body.Close()
		time.Sleep(wait)
	}
}

func pageRetryAfterDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}

	if retryAt, err := http.ParseTime(value); err == nil {
		delay := time.Until(retryAt)
		if delay > 0 {
			return delay
		}
	}

	delay := 10 * time.Millisecond << attempt
	if delay > 250*time.Millisecond {
		return 250 * time.Millisecond
	}
	return delay
}

func (d *Downloader) acquirePageDownload() {
	if d.pageDownloads == nil {
		return
	}
	d.pageDownloads <- struct{}{}
}

func (d *Downloader) releasePageDownload() {
	if d.pageDownloads == nil {
		return
	}
	<-d.pageDownloads
}

func detectPageExtension(contentType, rawURL string) string {
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil && strings.HasPrefix(mediaType, "image/") {
		if exts, err := mime.ExtensionsByType(mediaType); err == nil {
			for _, ext := range exts {
				ext = strings.ToLower(ext)
				switch ext {
				case ".jpeg", ".jpe":
					return ".jpg"
				default:
					if ext != "" {
						return ext
					}
				}
			}
		}
	}

	return tempPageExtension(rawURL)
}

func tempPageExtension(rawURL string) string {
	parsed, err := urlpkg.Parse(rawURL)
	if err == nil {
		ext := strings.ToLower(filepath.Ext(parsed.Path))
		if ext != "" {
			return ext
		}
	}

	return ".img"
}

func (d *Downloader) basePath() (string, error) {
	d.basePathOnce.Do(func() {
		d.basePathErr = util.EnsureDir(d.cfg.Dirs.Temp, "temporary root directory")
		if d.basePathErr != nil {
			return
		}

		workDir, err := os.MkdirTemp(d.cfg.Dirs.Temp, "mangate-*")
		if err != nil {
			d.basePathErr = fmt.Errorf("create temporary work directory in %q: %w", d.cfg.Dirs.Temp, err)
			return
		}

		d.workPath = workDir
		d.ownsWorkPath = true
	})

	if d.basePathErr != nil {
		return "", d.basePathErr
	}

	return d.workPath, nil
}
