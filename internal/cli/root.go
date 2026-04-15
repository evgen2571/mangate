package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/constant"
)

func NewRootCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:           constant.ProjectName,
		Short:         "Download manga from providers",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := validateConfig(a); err != nil {
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return cmd.Help()
			}

			return a.Run()
		},
	}

	bindPersistentConfigFlags(cmd, &a.Cfg)

	cmd.AddCommand(
		NewConfigCmd(a),
		NewSearchCmd(a),
	)

	return cmd
}

func validateConfig(a *app.App) error {
	cfg := a.Cfg

	if cfg.Provider == "" {
		return fmt.Errorf("provider cannot be empty")
	}
	if cfg.Language == "" {
		return fmt.Errorf("language cannot be empty")
	}
	if cfg.HTTP.Timeout <= 0 {
		return fmt.Errorf("http timeout must be > 0")
	}
	if cfg.Download.Dir == "" {
		return fmt.Errorf("download dir cannot be empty")
	}
	if cfg.Concurrency.PageFetches <= 0 {
		return fmt.Errorf("page-fetches must be > 0")
	}
	if cfg.Concurrency.PageDownloads <= 0 {
		return fmt.Errorf("page-downloads must be > 0")
	}
	if cfg.Concurrency.ChapterDownloads <= 0 {
		return fmt.Errorf("chapter-downloads must be > 0")
	}
	if cfg.Dirs.Cache == "" {
		return fmt.Errorf("cache dir cannot be empty")
	}
	if cfg.Dirs.Temp == "" {
		return fmt.Errorf("temp dir cannot be empty")
	}

	return nil
}
