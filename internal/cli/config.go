package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/evgen2571/mangate/internal/app"
)

func NewConfigCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Print config",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := a.Cfg

			fmt.Fprintf(cmd.OutOrStdout(), "ConfigPath: %s\n", a.ConfigPath)
			fmt.Fprintf(cmd.OutOrStdout(), "Provider: %s\n", cfg.Provider)
			fmt.Fprintf(cmd.OutOrStdout(), "Language: %s\n", cfg.Language)
			fmt.Fprintf(cmd.OutOrStdout(), "\n")

			fmt.Fprintf(cmd.OutOrStdout(), "MangaDex:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  SiteURL:    %s\n", cfg.Providers.MangaDex.SiteURL)
			fmt.Fprintf(cmd.OutOrStdout(), "  BaseURL:    %s\n", cfg.Providers.MangaDex.BaseURL)
			fmt.Fprintf(cmd.OutOrStdout(), "  UploadsURL: %s\n", cfg.Providers.MangaDex.UploadsURL)
			fmt.Fprintf(cmd.OutOrStdout(), "\n")

			fmt.Fprintf(cmd.OutOrStdout(), "HTTP:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  Timeout: %s\n", cfg.HTTP.Timeout)
			fmt.Fprintf(cmd.OutOrStdout(), "\n")

			fmt.Fprintf(cmd.OutOrStdout(), "Download:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  Dir:  %s\n", cfg.Download.Dir)
			fmt.Fprintf(cmd.OutOrStdout(), "  Type: %s\n", cfg.Download.Type)
			fmt.Fprintf(cmd.OutOrStdout(), "\n")

			fmt.Fprintf(cmd.OutOrStdout(), "Concurrency:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  PageFetches:      %d\n", cfg.Concurrency.PageFetches)
			fmt.Fprintf(cmd.OutOrStdout(), "  PageDownloads:    %d\n", cfg.Concurrency.PageDownloads)
			fmt.Fprintf(cmd.OutOrStdout(), "  ChapterDownloads: %d\n", cfg.Concurrency.ChapterDownloads)
			fmt.Fprintf(cmd.OutOrStdout(), "\n")

			fmt.Fprintf(cmd.OutOrStdout(), "Dirs:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  Cache: %s\n", cfg.Dirs.Cache)
			fmt.Fprintf(cmd.OutOrStdout(), "  Temp:  %s\n", cfg.Dirs.Temp)

			return nil
		},
	}
}
