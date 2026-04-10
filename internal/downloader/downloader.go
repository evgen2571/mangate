package downloader

import (
	"net/http"
	"log"
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
		log.Fatal("Failed to download.")
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
	    log.Fatal("Invalid status code: ", resp.StatusCode)
	}
	
	out, err := os.Create(filePath)
	if err != nil {
		log.Fatal("Failed to create a file.")
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
		log.Fatalf("Failed to create a folder '%s'", chapterName)
        }
        
	for _, page := range m.Chapters[chapterNumber].Pages {
		filePath := filepath.Join(chapterDir, fmt.Sprintf("%d.jpg", page.Index))
		err := DownloadPage(page, filePath)
		if err != nil {
			log.Fatalf("Failed to download page '%v'", page.Index)
		}
		fmt.Printf("Page %v downloaded successfully.\n", page.Index)
	}
	
	return nil
}

func DownloadManga(m *sources.Manga) error {
	err := os.MkdirAll(m.ID, 0755)   // add: GetTitleById function to create folder with manga name instead of id?
        if err != nil {
		log.Fatalf("Failed to create a folder '%s'", m.ID)
        }
        
    for idx := range m.Chapters {
      err := DownloadChapter(m, idx+1, m.ID)
      if err!=nil {
      log.Fatalf("Failed to download chapter %v", idx+1)
      }
    }
	
	return nil
}