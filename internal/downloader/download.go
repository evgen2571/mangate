package downloader

import (
	"context"
	"encoding/json"
	"errors"
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

const (
	maxPageDownloadRetries = 3
	maxPageRefreshRetries  = 1
)

type pageStatusError struct {
	url        string
	statusCode int
}

func (e pageStatusError) Error() string {
	return fmt.Sprintf("unexpected status code %d for %q", e.statusCode, e.url)
}

func (d *Downloader) DownloadChapter(c *source.Chapter) error {
	return d.downloadChapter(context.Background(), c, chapterDirBaseName(c), nil, nil)
}

func (d *Downloader) downloadChapter(ctx context.Context, c *source.Chapter, chapterName string, reporter *progressReporter, pageLoader PageLoader) error {
	if c == nil {
		return fmt.Errorf("download chapter: nil chapter")
	}
	if c.From == nil {
		return fmt.Errorf("download chapter %q: missing parent manga", c.ID)
	}
	if strings.TrimSpace(chapterName) == "" {
		chapterName = chapterDirBaseName(c)
	}
	if reporter != nil {
		reporter.chapterStarted(c)
	}

	for attempt := 0; ; attempt++ {
		err := d.downloadChapterAttempt(ctx, c, chapterName, reporter, pageLoader)
		if err == nil {
			return nil
		}
		if pageLoader == nil || attempt >= maxPageRefreshRetries || !isForbiddenPageError(err) {
			return err
		}
		c.Pages = nil
		c.PageCount = 0
	}
}

func (d *Downloader) downloadChapterAttempt(ctx context.Context, c *source.Chapter, chapterName string, reporter *progressReporter, pageLoader PageLoader) error {
	if len(c.Pages) == 0 && pageLoader != nil {
		pages, err := pageLoader(ctx, c)
		if err != nil {
			return fmt.Errorf("load pages for chapter %q: %w", c.ID, err)
		}
		c.Pages = pages
		c.PageCount = len(pages)
		if reporter != nil {
			reporter.pagesDiscovered(c)
		}
	}

	if len(c.Pages) == 0 {
		return fmt.Errorf("download chapter %q: provider returned no pages", c.ID)
	}

	chapterDir := filepath.Join(d.cfg.Download.Dir, titleDirName(c.From), util.SanitizeString(chapterName))
	if err := util.EnsureDir(chapterDir, "chapter directory"); err != nil {
		return err
	}

	var g errgroup.Group

	for idx, page := range c.Pages {
		idx := idx
		page := page

		g.Go(func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			filePathBase := filepath.Join(
				chapterDir,
				fmt.Sprintf("%04d", idx+1),
			)

			if existingPage(filePathBase) {
				switch d.cfg.Download.ExistingFileMode {
				case "skip":
					if reporter != nil {
						reporter.pageCompleted(c)
					}
					return nil
				case "fail":
					return fmt.Errorf("download page: destination already exists for page %d", idx+1)
				case "replace":
					if err := removeExistingPage(filePathBase); err != nil {
						return err
					}
				}
			}
			if _, err := d.downloadPage(ctx, page, filePathBase); err != nil {
				return err
			}
			if reporter != nil {
				reporter.pageCompleted(c)
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		_ = writeChapterState(chapterDir, c, false)
		return err
	}

	if err := writeChapterState(chapterDir, c, true); err != nil {
		return err
	}

	if reporter != nil {
		reporter.chapterCompleted(c)
	}

	return nil
}

func removeExistingPage(base string) error {
	paths, err := filepath.Glob(base + ".*")
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove existing page %q: %w", path, err)
		}
	}
	return nil
}

func titleDirName(manga *source.Manga) string {
	if manga == nil {
		return "unknown"
	}
	title := util.SanitizeString(manga.Title)
	if id := util.SanitizeString(manga.ID); id != "unknown" {
		return title + "-" + id
	}
	return title
}

func existingPage(base string) bool {
	paths, err := filepath.Glob(base + ".*")
	if err != nil {
		return false
	}
	for _, path := range paths {
		if strings.HasSuffix(path, ".part") {
			continue
		}
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() && info.Size() > 0 {
			return true
		}
	}
	return false
}

type chapterState struct {
	FormatVersion string `json:"formatVersion"`
	Provider      string `json:"provider,omitempty"`
	TitleID       string `json:"titleId,omitempty"`
	ChapterID     string `json:"chapterId,omitempty"`
	ExpectedPages int    `json:"expectedPages"`
	Complete      bool   `json:"complete"`
	UpdatedAt     string `json:"updatedAt"`
}

func writeChapterState(chapterDir string, chapter *source.Chapter, complete bool) error {
	state := chapterState{FormatVersion: "1", ChapterID: chapter.ID, ExpectedPages: len(chapter.Pages), Complete: complete, UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
	if chapter.From != nil {
		state.TitleID = chapter.From.ID
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode chapter state: %w", err)
	}
	temp := filepath.Join(chapterDir, ".mangate.json.part")
	final := filepath.Join(chapterDir, ".mangate.json")
	if err := os.WriteFile(temp, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write chapter state: %w", err)
	}
	if err := os.Rename(temp, final); err != nil {
		return fmt.Errorf("finalize chapter state: %w", err)
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

	mangaDir := filepath.Join(d.cfg.Download.Dir, titleDirName(m))

	if err := util.EnsureDir(mangaDir, "manga directory"); err != nil {
		return err
	}

	var g errgroup.Group
	g.SetLimit(d.cfg.Concurrency.ChapterDownloads)

	chapterNames := uniqueChapterDirNames(m.Chapters)
	for idx, chapter := range m.Chapters {
		chapter := chapter
		chapterName := chapterNames[idx]

		g.Go(func() error {
			if err := d.downloadChapter(ctx, chapter, chapterName, reporter, pageLoader); err != nil {
				return err
			}

			return nil
		})
	}

	return g.Wait()
}

func uniqueChapterDirNames(chapters []*source.Chapter) []string {
	names := make([]string, len(chapters))
	seenBase := make(map[string]int, len(chapters))
	used := make(map[string]struct{}, len(chapters))

	for idx, chapter := range chapters {
		baseName := chapterDirBaseName(chapter)
		seenBase[baseName]++

		name := baseName
		if seenBase[baseName] > 1 {
			name = disambiguatedChapterDirName(baseName, chapter, idx)
		}
		for suffix := 2; ; suffix++ {
			if _, exists := used[name]; !exists {
				break
			}
			name = fmt.Sprintf("%s-%d", baseName, suffix)
		}

		used[name] = struct{}{}
		names[idx] = name
	}

	return names
}

func chapterDirBaseName(chapter *source.Chapter) string {
	if chapter == nil {
		return "unknown-chapter"
	}

	index := strings.TrimSpace(chapter.Index)
	title := strings.TrimSpace(chapter.Title)
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

func disambiguatedChapterDirName(baseName string, chapter *source.Chapter, idx int) string {
	if chapter != nil && strings.TrimSpace(chapter.ID) != "" {
		return baseName + "-" + chapter.ID
	}
	return fmt.Sprintf("%s-%d", baseName, idx+1)
}

func isForbiddenPageError(err error) bool {
	var statusErr pageStatusError
	return errors.As(err, &statusErr) && statusErr.statusCode == http.StatusForbidden
}

func (d *Downloader) downloadPage(ctx context.Context, p *source.Page, filePathBase string) (string, error) {
	if p == nil {
		return "", fmt.Errorf("download page: nil page")
	}
	if strings.TrimSpace(p.URL) == "" {
		return "", fmt.Errorf("download page: empty page url")
	}

	d.acquirePageDownload()
	defer d.releasePageDownload()

	resp, err := d.getPageResponseWithRetry(ctx, p.URL)
	if err != nil {
		return "", fmt.Errorf("failed to GET %q: %w", p.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", pageStatusError{url: p.URL, statusCode: resp.StatusCode}
	}
	if contentType := resp.Header.Get("Content-Type"); contentType != "" && !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		return "", fmt.Errorf("download page: expected an image response, got %q", contentType)
	}

	filePath := filePathBase + detectPageExtension(resp.Header.Get("Content-Type"), p.URL)
	temporaryPath := filePath + ".part"
	out, err := os.Create(temporaryPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file %q: %w", filePath, err)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = out.Close()
		_ = os.Remove(temporaryPath)
		return "", fmt.Errorf("failed to write file %q: %w", filePath, err)
	}

	if err := out.Close(); err != nil {
		_ = os.Remove(temporaryPath)
		return "", fmt.Errorf("failed to close file %q: %w", filePath, err)
	}
	info, err := os.Stat(temporaryPath)
	if err != nil || info.Size() == 0 {
		_ = os.Remove(temporaryPath)
		return "", fmt.Errorf("download page: response for %q was empty", p.URL)
	}
	if err := os.Rename(temporaryPath, filePath); err != nil {
		_ = os.Remove(temporaryPath)
		return "", fmt.Errorf("finalize file %q: %w", filePath, err)
	}

	return filePath, nil
}

func (d *Downloader) getPageResponseWithRetry(ctx context.Context, rawURL string) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := d.client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusTooManyRequests || attempt >= maxPageDownloadRetries {
			return resp, nil
		}

		wait := pageRetryAfterDelay(resp.Header.Get("Retry-After"), attempt)
		resp.Body.Close()
		if err := sleepWithContext(ctx, wait); err != nil {
			return nil, err
		}
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

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
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
