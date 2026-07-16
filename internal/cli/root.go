package cli

import (
	"fmt"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/constant"
	"github.com/evgen2571/mangate/internal/tui"
)

func NewRootCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:           constant.ProjectName,
		Short:         "Download authorized manga from supported providers",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return a.ApplyConfig(a.Cfg)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.Version = "0.1.0"
	cmd.SetVersionTemplate("{{.Name}} {{.Version}} (" + runtime.GOOS + ")\n")

	bindPersistentConfigFlags(cmd, &a.Cfg)
	cmd.PersistentFlags().Bool("json", false, "Write a structured JSON result to standard output")
	cmd.PersistentFlags().Bool("quiet", false, "Suppress nonessential human-readable output")
	cmd.PersistentFlags().Bool("verbose", false, "Write safe diagnostic context to standard error")

	cmd.AddCommand(
		NewChaptersCmd(a),
		NewConfigCmd(a),
		NewDownloadCmd(a),
		NewInteractiveCmd(a),
		NewProviderCmd(a),
		NewProvidersCmd(a),
		NewSearchCmd(a),
		NewTitleCmd(a),
	)

	return cmd
}

func NewInteractiveCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "interactive",
		Short: "Open the interactive terminal interface",
		RunE: func(cmd *cobra.Command, args []string) error {
			p := tea.NewProgram(tui.New(a))
			_, err := p.Run()
			return err
		},
	}
}

func commandError(cmd *cobra.Command, format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	if verbose, _ := cmd.Flags().GetBool("verbose"); verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "mangate: %v\n", err)
	}
	return err
}
