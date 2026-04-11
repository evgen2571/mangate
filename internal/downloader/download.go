package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/evgen2571/manga-downloader/internal/config"
	"github.com/evgen2571/manga-downloader/internal/sources"
)

func DownloadChapter(c *sources.Chapter, folderPath string) error {
	chapterDir := filepath.Join(
		folderPath,
		sanitizeFileName(c.From.Title),
		sanitizeFileName(c.Title),
	)

	err := os.MkdirAll(chapterDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create a folder '%s'", chapterDir)
	}

	for idx, page := range c.Pages {
		filePath := filepath.Join(
			chapterDir,
			fmt.Sprintf("%d%v", idx+1, config.DefaultDownloadType),
		)

		err := downloadPage(page, filePath)
		if err != nil {
			return err
		}
	}

	return nil
}

func DownloadManga(m *sources.Manga, folderPath string) error {
	mangaDir := filepath.Join(
		folderPath,
		sanitizeFileName(m.Title),
	)

	if err := os.MkdirAll(mangaDir, 0755); err != nil {
		return fmt.Errorf("failed to create folder %q: %w", mangaDir, err)
	}

	for _, chapter := range m.Chapters {
		if err := DownloadChapter(chapter, folderPath); err != nil {
			return err
		}
	}

	return nil
}

func downloadPage(p *sources.Page, filePath string) error {
	resp, err := http.Get(p.URL)
	if err != nil {
		return fmt.Errorf("failed to get response from the client")
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

func sanitizeFileName(name string) string {
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

