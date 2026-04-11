package cli

import (
	"fmt"

	"github.com/evgen2571/manga-downloader/internal/providers"
	"github.com/evgen2571/manga-downloader/internal/source"
	"github.com/spf13/cobra"
)

var chaptersCmd = &cobra.Command{
	Use:   "chapters <manga-id>",
	Short: "Search for manga chapters by manga id",

	RunE: func(cmd *cobra.Command, args []string) error {
		manga := &source.Manga{
			ID: args[0],
		}
		provider := providers.providers[config.Provider]

		chapters, _ := provider.GetChapters(manga)

		for idx, chapter := range chapters {
			fmt.Printf("%v.  ID: %v\n", idx+1, chapter.GetID())
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(chaptersCmd)
}
