package cli

import (
	"errors"
	"log"
	"strconv"

	"github.com/evgen2571/manga-downloader/internal/downloader"
	"github.com/evgen2571/manga-downloader/internal/providers"
	"github.com/evgen2571/manga-downloader/internal/source"
	"github.com/spf13/cobra"
)

var downloadCmd = &cobra.Command{
	Use:   "download <manga-id> [chapter-number]",
	Short: "Download manga by id. Add <chapter-number> to download a specific chapter",
	Args:  cobra.RangeArgs(1, 2),

	RunE: func(cmd *cobra.Command, args []string) error {
		provider := providers.Provider

		manga := &source.Manga{
			ID: args[0],
		}

		chapters, _ := provider.GetChapters(manga)
		manga.Chapters = chapters

		switch len(args) {
		case 1:
			for idx := range manga.Chapters {
				pages, _ := provider.GetPages(manga.Chapters[idx])
				manga.Chapters[idx].Pages = pages
			}

			err := downloader.DownloadManga(manga)
			if err != nil {
				log.Fatalf("Failed to download manga")
			}
			return nil

		case 2:
			chapterNumber, _ := strconv.Atoi(args[1])
			chapterNumber--

			if chapterNumber >= len(manga.Chapters) || chapterNumber <= -1 {
				return errors.New("Failed to find such chapter.")
			}

			pages, _ := provider.GetPages(chapters[chapterNumber])
			manga.Chapters[chapterNumber].Pages = pages

			err := downloader.DownloadChapter(chapters[chapterNumber])
			if err != nil {
				log.Fatalf("Failed to download a chapter.")
			}
			return nil
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(downloadCmd)

	downloadCmd.Flags().StringP("type", "t", "plain", "Set download type (default: plain)")
	downloadCmd.Flags().StringP("dir", "d", ".", "Set download path (default: '.')")
}
