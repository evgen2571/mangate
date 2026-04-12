package cli

import (
	"github.com/evgen2571/manga-downloader/internal/tui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "manga-downloader",
	Short: "Manga downloader",

	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.Run()
	},
}

func Execute() error {
	return rootCmd.Execute()
}
