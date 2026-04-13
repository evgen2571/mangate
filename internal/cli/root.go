package cli

import (
	"github.com/evgen2571/manga-downloader/internal/constant"
	"github.com/evgen2571/manga-downloader/internal/tui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use: constant.ProjectName,

	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.Run()
	},
}

// Persistent flags
func init() {
	rootCmd.PersistentFlags().StringVar(&cfg.Download.Dir, "download-dir", cfg.Download.Dir, "set download directory; default: './downloads'")
	rootCmd.PersistentFlags().StringVar(&cfg.Download.Type, "download-type", cfg.Download.Type, "set download type; default: 'plain'; options: 'zip, cbz, plain'")
}
