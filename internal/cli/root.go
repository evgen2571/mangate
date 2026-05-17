package cli

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/constant"
	"github.com/evgen2571/mangate/internal/tui"
	"github.com/evgen2571/mangate/internal/tuiapp"
)

func NewRootCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:           constant.ProjectName,
		Short:         "Download manga from providers",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return a.ApplyConfig(a.Cfg)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return cmd.Help()
			}

			p := tea.NewProgram(tui.New(tuiapp.New(a)))
			_, err := p.Run()
			return err
		},
	}

	bindPersistentConfigFlags(cmd, &a.Cfg)

	cmd.AddCommand(
		NewChaptersCmd(a),
		NewConfigCmd(a),
		NewSearchCmd(a),
	)

	return cmd
}
