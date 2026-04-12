package cli

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use: "manga-downloader",
	Short: "Manga downloader",
}

func Execute() error {
	return rootCmd.Execute()
}
