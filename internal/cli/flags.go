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
		"Existing output behavior: skip, replace, or fail",
	)

	f.StringVar(
		&cfg.Download.Format,
		"format",
		cfg.Download.Format,
		"Output format: directory, cbz, or zip",
	)

	f.BoolVar(
		&cfg.Download.RetainSource,
		"retain-source",
		cfg.Download.RetainSource,
		"Keep page directories after successful archive creation",
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
		&cfg.Download.Dir,
		"output",
		cfg.Download.Dir,
		"Output root for downloaded titles and archives",
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

}
