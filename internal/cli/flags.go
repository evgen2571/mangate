package cli

import (
	"github.com/spf13/cobra"

	"github.com/evgen2571/mangate/internal/config"
)

func bindPersistentConfigFlags(cmd *cobra.Command, cfg *config.Config) {
	f := cmd.PersistentFlags()

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
		"Output format: directory, png, jpeg, cbz, or zip",
	)

	f.Bool(
		"non-interactive",
		false,
		"Refuse prompts and the terminal interface",
	)

	f.Bool("color", false, "Force color output when supported")
	f.Bool("no-color", false, "Disable terminal color output")

	f.StringVar(
		&cfg.Language,
		"language",
		cfg.Language,
		"Manga language",
	)

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

	f.IntVar(
		&cfg.Search.HistoryMax,
		"search-history-max",
		cfg.Search.HistoryMax,
		"Maximum number of search queries to remember (0 disables history)",
	)

	f.StringVar(
		&cfg.Dirs.Cache,
		"cache-dir",
		cfg.Dirs.Cache,
		"Cache directory",
	)

}
