package cli

import (
	"fmt"
	"strconv"
	
	"github.com/evgen2571/manga-downloader/internal/api"
	"github.com/spf13/cobra"
)

var pagesCmd = &cobra.Command{
	Use:   "pages <manga-id> <chapter-number>",
	Short: "Output chapter pages using manga ID",

	RunE: func(cmd *cobra.Command, args []string) error {
		mangaID := args[0]
        chapterNumber, err := strconv.Atoi(args[1])
        if err != nil {
			panic(err)
        }
        
		client := api.NewClient()

		resp, err := client.GetChapters(mangaID);
		if err != nil {
			panic(err)
		}

		for idx, page := range resp.Data[chapterNumber-1].ChapterInfo.Pages {
			fmt.Printf("%v.  URL: %s\n", idx+1, page)
        }
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pagesCmd)
}