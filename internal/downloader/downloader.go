package downloader

import (
	"net/http"
	"log"
	"os"
	"io"
	
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

