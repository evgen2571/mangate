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
	chapterDir := filepath.Join(
		d.basePath(),
		util.SanitizeString(c.From.Title),
		util.SanitizeString(chapterDirName(c)),
	)

	err := os.MkdirAll(chapterDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create a folder '%s'", chapterDir)
	}

	// Set limit for concurrent page downloads
	var g errgroup.Group
	g.SetLimit(d.cfg.Concurrency.PageDownloads)

	for idx, page := range c.Pages {
		g.Go(func() error {
			filePath := filepath.Join(
				chapterDir,
				fmt.Sprintf("%04d.%v", idx+1, d.cfg.Download.ImageType),
			)

			// Start page downloading
			if err := d.downloadPage(page, filePath); err != nil {
				return err
			}
			return nil
		})
	}

	return g.Wait()
}

func (d *Downloader) DownloadManga(m *source.Manga) error {
	mangaDir := filepath.Join(
		d.basePath(),
		util.SanitizeString(m.Title),
	)

	if err := os.MkdirAll(mangaDir, 0755); err != nil {
		return fmt.Errorf("failed to create folder %q: %w", mangaDir, err)
	}

	// Set limit for concurrent chapter downloads
	var g errgroup.Group
	g.SetLimit(d.cfg.Concurrency.ChapterDownloads)

	for _, chapter := range m.Chapters {
		g.Go(func() error {
			if err := d.DownloadChapter(chapter); err != nil {
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
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return nil
}

func (d *Downloader) basePath() string {
	if d.cfg.Download.Type == "plain" {
		return d.cfg.Download.Dir
	}

	workDir, err := os.MkdirTemp(d.cfg.Dirs.Temp, "mangate-*")
	if err != nil {
		fmt.Errorf("failed to create temp direcotry")
	}

	return workDir
}
