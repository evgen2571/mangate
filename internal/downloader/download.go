package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/evgen2571/mangate/internal/source"
	"github.com/evgen2571/mangate/internal/util"
	"golang.org/x/sync/errgroup"
)

func (d *Downloader) DownloadChapter(c *source.Chapter) error {
	return d.downloadChapter(c, nil)
}

func (d *Downloader) downloadChapter(c *source.Chapter, reporter *progressReporter) error {
	if c == nil {
		return fmt.Errorf("download chapter: nil chapter")
	}
	if c.From == nil {
		return fmt.Errorf("download chapter %q: missing parent manga", c.ID)
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

	if err := util.EnsureDir(chapterDir, "chapter directory"); err != nil {
		return err
	}

	var g errgroup.Group
	g.SetLimit(d.cfg.Concurrency.PageDownloads)

	if reporter != nil {
		reporter.chapterStarted(c)
	}

	for idx, page := range c.Pages {
		idx := idx
		page := page

		g.Go(func() error {
			filePath := filepath.Join(
				chapterDir,
				fmt.Sprintf("%04d.%v", idx+1, d.cfg.Download.ImageType),
			)

			if err := d.downloadPage(page, filePath); err != nil {
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

	if reporter != nil {
		reporter.chapterCompleted(c)
	}

	return nil
}

func (d *Downloader) DownloadManga(m *source.Manga) error {
	return d.downloadManga(m, nil)
}

func (d *Downloader) DownloadMangaWithProgress(m *source.Manga, notify func(DownloadProgress)) error {
	return d.downloadManga(m, newProgressReporter(m, notify))
}

func (d *Downloader) downloadManga(m *source.Manga, reporter *progressReporter) error {
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
			if err := d.downloadChapter(chapter, reporter); err != nil {
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
		return index + "-" + title
	case index != "":
		return "Chapter-" + index
	case title != "":
		return title
	default:
		return "unknown-chapter"
	}
}

func (d *Downloader) downloadPage(p *source.Page, filePath string) error {
	if p == nil {
		return fmt.Errorf("download page: nil page")
	}
	if strings.TrimSpace(p.URL) == "" {
		return fmt.Errorf("download page: empty page url")
	}

	resp, err := d.client.Get(p.URL)
	if err != nil {
		return fmt.Errorf("failed to GET %q: %w", p.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %v", resp.StatusCode)
	}

	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %q: %w", filePath, err)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = out.Close()
		_ = os.Remove(filePath)
		return fmt.Errorf("failed to write file %q: %w", filePath, err)
	}

	if err := out.Close(); err != nil {
		_ = os.Remove(filePath)
		return fmt.Errorf("failed to close file %q: %w", filePath, err)
	}

	return nil
}

func (d *Downloader) basePath() (string, error) {
	d.basePathOnce.Do(func() {
		if d.cfg.Download.Type == "plain" {
			d.basePathErr = util.EnsureDir(d.cfg.Download.Dir, "download directory")
			if d.basePathErr != nil {
				return
			}
			d.workPath = d.cfg.Download.Dir
			d.ownsWorkPath = false
			return
		}

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
