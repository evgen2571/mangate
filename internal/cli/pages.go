package cli

import (
	"fmt"

	"github.com/evgen2571/manga-downloader/internal/providers"
	"github.com/evgen2571/manga-downloader/internal/sources"
	"github.com/spf13/cobra"
)

var pagesCmd = &cobra.Command{
	Use:   "pages <manga-title>",
	Short: "Search for manga by title",

	RunE: func(cmd *cobra.Command, args []string) error {
		chapter := &sources.Chapter{
			ID: args[0],
		}
		provider := providers.Providers["mangadex"]

		pages, _ := provider.GetPages(chapter)

		for idx, page := range pages {
			fmt.Printf("%v. URL: %v\n", idx+1, page.GetURL())
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(pagesCmd)
}
