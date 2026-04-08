package cli

import (
	"fmt"
	
	"github.com/evgen2571/manga-downloader/internal/api"
	"github.com/spf13/cobra"
)

var chaptersCmd = &cobra.Command{
	Use:   "chapters <manga-id>",
	Short: "Search for manga chapters by manga id",

	RunE: func(cmd *cobra.Command, args []string) error {
		mangaID := args[0]

		client := api.NewClient()

		resp, err := client.GetChapters(mangaID);
		if err != nil {
			panic(err)
		}

		for idx, chapter := range resp.Data {
			fmt.Printf("%v.  ID: %v\n", idx+1, chapter.ID)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(chaptersCmd)
}
