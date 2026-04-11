package downloader

import (
	"net/http"
	"os"
	"io"
	"strconv"
	"path/filepath"
	"fmt"
	
	"github.com/evgen2571/manga-downloader/internal/sources"
)


func DownloadPage(p *sources.Page, filePath string) error {
	resp, err := http.Get(p.URL)
	if err != nil {
		return fmt.Errorf("Failed to get response from the client")
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
	    return fmt.Errorf("Unexpected status code: %v", resp.StatusCode)
	}
	
	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("Failed to create file %q: %w", filePath, err)
	}
	defer out.Close()
	
	_, err = io.Copy(out, resp.Body)
	return nil
}

func DownloadChapter(m *sources.Manga, chapterNumber int, folderPath string) error {
	chapterNumber--
	
	chapterName := "chapter-" + strconv.Itoa(m.Chapters[chapterNumber].Index)
	chapterDir := filepath.Join(folderPath, chapterName)
	
	err := os.MkdirAll(chapterDir, 0755)
        if err != nil {
		return fmt.Errorf("Failed to create a folder '%s'", chapterName)
        }
    
	for _, page := range m.Chapters[chapterNumber].Pages {
		filePath := filepath.Join(chapterDir, fmt.Sprintf("%d.jpg", page.Index))
		err := DownloadPage(page, filePath)
		if err != nil {
			return err
		}
		fmt.Printf("Page %v: download successful.\n", page.Index)
	}
	
	return nil
}

func DownloadManga(m *sources.Manga) error {
	err := os.MkdirAll(m.ID, 0755)
    if err != nil {
		return fmt.Errorf("Failed to create a folder '%s'", m.ID)
    }
        
    for idx := range m.Chapters {
      err := DownloadChapter(m, idx+1, m.ID)
      if err != nil {
        return err
      }
    }
	
	return nil
}