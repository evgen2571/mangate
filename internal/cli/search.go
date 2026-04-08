package cli

import (
	"fmt"

	"github.com/evgen2571/manga-downloader/internal/api"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <manga-title>",
	Short: "Search for manga by title",

	RunE: func(cmd *cobra.Command, args []string) error {
		title := args[0]

		client := api.NewClient()

		resp, err := client.GetManga(title)
		if err != nil {
			panic(err)
		}

		for idx, manga := range resp.Data {
			fmt.Printf("%v. Title: %v\n   ID: %v\n\n", idx+1, manga.MangaAttributes.Title, manga.ID)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)
}
