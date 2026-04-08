package cli

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use: "manga-downloader",
}

func Execute() error {
	return rootCmd.Execute()
}
