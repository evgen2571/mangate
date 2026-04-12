package cli

import (
	"github.com/spf13/cobra"
	"github.com/evgen2571/manga-downloader/internal/tui"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch TUI for more convenient use and clear design",
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.Run()
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}