package cli

import (
	"fmt"

	"github.com/evgen2571/manga-downloader/internal/providers"
	"github.com/evgen2571/manga-downloader/internal/source"
	"github.com/spf13/cobra"
)

var pagesCmd = &cobra.Command{
	Use:   "pages <chapter-id>",
	Short: "Search for pages url by chapter id",

	RunE: func(cmd *cobra.Command, args []string) error {
		chapter := &source.Chapter{
			ID: args[0],
		}
		provider := providers.Provider

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
