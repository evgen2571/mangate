package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/evgen2571/manga-downloader/internal/client"
	"github.com/evgen2571/manga-downloader/internal/config"
	"github.com/evgen2571/manga-downloader/internal/source"
	"golang.org/x/sync/errgroup"
)

func DownloadChapter(c *source.Chapter) error {
	chapterDir := filepath.Join(
		config.DownloadPath,
		sanitizeFileName(c.From.Title),
		sanitizeFileName(c.Title),
	)

	err := os.MkdirAll(chapterDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create a folder '%s'", chapterDir)
	}

	// Set limit for concurrent page downloads
	var g errgroup.Group
	g.SetLimit(config.MaxConcurrentPageDownloads)

	for idx, page := range c.Pages {
		g.Go(func() error {
			filePath := filepath.Join(
				chapterDir,
				fmt.Sprintf("%04d.%v", idx+1, config.DownloadType),
			)

			// Start page downloading
			if err := downloadPage(page, filePath); err != nil {
				return err
			}
			return nil
		})
	}

	return g.Wait()
}

func DownloadManga(m *source.Manga) error {
	mangaDir := filepath.Join(
		config.DownloadPath,
		sanitizeFileName(m.Title),
	)

	if err := os.MkdirAll(mangaDir, 0755); err != nil {
		return fmt.Errorf("failed to create folder %q: %w", mangaDir, err)
	}

	// Set limit for concurrent chapter downloads
	var g errgroup.Group
	g.SetLimit(config.MaxConcurrentChapterDownloads)

	for _, chapter := range m.Chapters {
		g.Go(func() error {
			if err := DownloadChapter(chapter); err != nil {
				return err
			}

			return nil
		})
	}

	return g.Wait()	
}

func downloadPage(p *source.Page, filePath string) error {
	resp, err := client.Client.Get(p.URL)
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

func sanitizeFileName(name string) string {                      // <-- eto pizdec bratan  D:
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, "*", "_")
	name = strings.ReplaceAll(name, "?", "_")
	name = strings.ReplaceAll(name, "\"", "_")
	name = strings.ReplaceAll(name, "<", "_")
	name = strings.ReplaceAll(name, ">", "_")
	name = strings.ReplaceAll(name, "|", "_")
	if name == "" {
		return "unknown"
	}
	return name
}
