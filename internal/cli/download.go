package cli

import (
	"os"
	"log"
	"fmt"
	"strconv"
	"path/filepath"
	
	"github.com/evgen2571/manga-downloader/internal/providers"
	"github.com/evgen2571/manga-downloader/internal/sources"
	"github.com/evgen2571/manga-downloader/internal/downloader"
	"github.com/spf13/cobra"
)

var downloadCmd = &cobra.Command{
	Use:   "download <manga-id> <chapter-number>",
	Short: "Download chosen manga chapter by manga id",

	RunE: func(cmd *cobra.Command, args []string) error {
			manga := &sources.Manga{
			ID: args[0],
		}
		
		chapterNumber, _ := strconv.Atoi(args[1])
		chapterNumber--
		
		provider := providers.Providers["mangadex"]
		
		chapters, _ := provider.GetChapters(manga)
		pages, _ := provider.GetPages(chapters[chapterNumber])
		
		chapterName := "chapter " + strconv.Itoa(chapters[chapterNumber].Index)
		
		err := os.MkdirAll(chapterName, 0755)
        if err != nil {
			log.Fatalf("Couldn't create a folder '%s'", chapterName)
        }
		
		for _, page := range pages {
			filePath := filepath.Join(chapterName, fmt.Sprintf("%d.jpg", page.Index))
			err := downloader.DownloadPage(page, filePath)
			if err != nil {
				log.Fatalf("Couldn't download page '%v'", page.Index)
			}
			fmt.Printf("Page %v downloaded successfully.\n", page.Index)
		}
		
		return nil
	},
}

func init() {
	rootCmd.AddCommand(downloadCmd)

	downloadCmd.Flags().StringP("type", "t", "plain", "Set download type (default: plain)")
	downloadCmd.Flags().StringP("dir", "d", ".", "Set download path (default: '.')")
}
