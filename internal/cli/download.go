package cli

import (
	// "github.com/evgen2571/manga-downloader/internal/api"
	"github.com/spf13/cobra"
)

var downloadCmd = &cobra.Command{
	Use:   "download <query>",
	Short: "Download manga chapter by chapter id",

	RunE: func(cmd *cobra.Command, args []string) error {
		// chapterID := args[0]

		// client := api.NewClient()

		// TODO: Downloader request to download manga chapter

		return nil
	},
}

func init() {
	rootCmd.AddCommand(downloadCmd)

	downloadCmd.Flags().StringP("type", "t", "plain", "Set download type (default: plain)")
	downloadCmd.Flags().StringP("dir", "d", ".", "Set download path (default: '.')")
}
