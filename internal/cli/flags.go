package cli

import (
	"github.com/spf13/cobra"

	"github.com/evgen2571/mangate/internal/config"
)

func bindPersistentConfigFlags(cmd *cobra.Command, cfg *config.Config) {
	f := cmd.PersistentFlags()

	// Top-level config
	f.StringVar(
		&cfg.Provider,
		"provider",
		cfg.Provider,
		"Provider name",
	)

	f.StringVar(
		&cfg.Download.ExistingFileMode,
		"existing-files",
		cfg.Download.ExistingFileMode,
		"Existing page behavior: skip, replace, or fail",
	)

	f.StringVar(
		&cfg.Language,
		"language",
		cfg.Language,
		"Manga language",
	)

	// Download
	f.StringVar(
		&cfg.Download.Dir,
		"download-dir",
		cfg.Download.Dir,
		"Directory where manga will be downloaded",
	)

	f.StringVar(
		&cfg.Download.Type,
		"download-type",
		cfg.Download.Type,
		"Download type (plain image directory)",
	)

	// Concurrency
	f.IntVar(
		&cfg.Concurrency.PageDownloads,
		"page-downloads",
		cfg.Concurrency.PageDownloads,
		"Number of concurrent page downloads",
	)

	f.IntVar(
		&cfg.Concurrency.ChapterDownloads,
		"chapter-downloads",
		cfg.Concurrency.ChapterDownloads,
		"Number of concurrent chapter downloads",
	)

	// Search
	f.IntVar(
		&cfg.Search.HistoryMax,
		"search-history-max",
		cfg.Search.HistoryMax,
		"Maximum number of search queries to remember (0 disables history)",
	)

	// Directories
	f.StringVar(
		&cfg.Dirs.Cache,
		"cache-dir",
		cfg.Dirs.Cache,
		"Cache directory",
	)

	f.StringVar(
		&cfg.Dirs.Temp,
		"temp-dir",
		cfg.Dirs.Temp,
		"Temporary directory",
	)
}
