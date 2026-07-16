package cli

import (
	"fmt"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/spf13/cobra"
)

func NewConfigCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:     "config",
		Aliases: []string{"configuration"},
		Short:   "Show the effective configuration",
		Example: "  mangate config\n  mangate --format cbz --output ./library config --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			record := configRecord{Path: a.ConfigPath, Config: a.Cfg}
			if wantsJSON(cmd) {
				return writeJSON(cmd, "config.inspect", record)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Configuration: %s\nProvider: %s\nLanguage: %s\nOutput: %s\nFormat: %s\nExisting files: %s\nRetain source pages: %t\n", record.Path, a.Cfg.Provider, a.Cfg.Language, a.Cfg.Download.Dir, a.Cfg.Download.Format, a.Cfg.Download.ExistingFileMode, a.Cfg.Download.RetainSource)
			return nil
		},
	}
}

type configRecord struct {
	Path   string `json:"path,omitempty"`
	Config any    `json:"config"`
}
