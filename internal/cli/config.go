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

			out := cmd.OutOrStdout()
			writef(out, "ConfigPath: %s\n", a.ConfigPath)
			writef(out, "Provider: %s\n", cfg.Provider)
			writef(out, "Language: %s\n", cfg.Language)
			writef(out, "\n")

			writef(out, "MangaDex:\n")
			writef(out, "  SiteURL:    %s\n", cfg.Providers.MangaDex.SiteURL)
			writef(out, "  BaseURL:    %s\n", cfg.Providers.MangaDex.BaseURL)
			writef(out, "  UploadsURL: %s\n", cfg.Providers.MangaDex.UploadsURL)
			writef(out, "\n")

			writef(out, "HTTP:\n")
			writef(out, "  Timeout: %s\n", cfg.HTTP.Timeout)
			writef(out, "\n")

			writef(out, "Download:\n")
			writef(out, "  Dir:  %s\n", cfg.Download.Dir)
			writef(out, "  Type: %s\n", cfg.Download.Type)
			writef(out, "\n")

			writef(out, "Concurrency:\n")
			writef(out, "  PageDownloads:    %d\n", cfg.Concurrency.PageDownloads)
			writef(out, "  ChapterDownloads: %d\n", cfg.Concurrency.ChapterDownloads)
			writef(out, "\n")

			writef(out, "Search:\n")
			writef(out, "  HistoryMax: %d\n", cfg.Search.HistoryMax)
			writef(out, "\n")

			writef(out, "Dirs:\n")
			writef(out, "  Cache: %s\n", cfg.Dirs.Cache)
			writef(out, "  Temp:  %s\n", cfg.Dirs.Temp)

			return nil
		},
	}
}
