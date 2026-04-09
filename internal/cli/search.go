package cli

import (
	"fmt"

	mangadex "github.com/evgen2571/manga-downloader/internal/providers/MangaDex"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <manga-title>",
	Short: "Search for manga by title",

	RunE: func(cmd *cobra.Command, args []string) error {
		title := args[0]

		mangas := mangadex.MangaDexProvider.GetManga(title)

		for idx, manga := range mangas {
			fmt.Printf("%v. Title: %v\n   ID: %v\n\n", idx+1, manga.MangaAttributes.Title, manga.ID)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)
}
