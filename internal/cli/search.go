package cli

import (
	"fmt"

	"github.com/evgen2571/manga-downloader/internal/providers"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <manga-title>",
	Short: "Search for manga by title",

	RunE: func(cmd *cobra.Command, args []string) error {
		title := args[0]
		provider := providers.Providers["mangadex"]

		mangas, _ := provider.GetManga(title)

		for idx, manga := range mangas {
			fmt.Printf("%v. Title: %v\n   ID: %v\n\n", idx+1, manga.GetTitle(), manga.GetID())
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)
}
