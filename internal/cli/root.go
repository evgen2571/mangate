package cli

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/constant"
	"github.com/evgen2571/mangate/internal/tui"
)

func NewRootCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:           constant.ProjectName,
		Short:         "Download authorized manga from supported providers",
		Example:       "  mangate search \"example title\"\n  mangate download <title-id> --latest\n  mangate --format cbz download <title-id> --chapter 1\n  mangate tui",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return a.ApplyConfig(a.Cfg)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if interactiveTerminal() && !wantsJSON(cmd) && !isNonInteractive(cmd) {
				return runInteractive(cmd, a)
			}
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
		NewArchiveCmd(a),
		NewChaptersCmd(a),
		NewCompletionCmd(cmd),
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
		Use:     "tui",
		Aliases: []string{"interactive"},
		Short:   "Open the interactive terminal interface",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInteractive(cmd, a)
		},
	}
}

func runInteractive(cmd *cobra.Command, a *app.App) error {
	if isNonInteractive(cmd) {
		return fmt.Errorf("tui cannot run with --non-interactive")
	}
	if err := configureTUIColor(cmd); err != nil {
		return err
	}
	if !interactiveTerminal() {
		return fmt.Errorf("tui requires an interactive terminal; use direct commands such as search, chapters, or download")
	}
	p := tea.NewProgram(tui.New(a))
	_, err := p.Run()
	return err
}

func configureTUIColor(cmd *cobra.Command) error {
	color, err := cmd.Flags().GetBool("color")
	if err != nil {
		return err
	}
	noColor, err := cmd.Flags().GetBool("no-color")
	if err != nil {
		return err
	}
	if color && noColor {
		return fmt.Errorf("--color and --no-color cannot be combined")
	}
	if noColor {
		lipgloss.SetColorProfile(termenv.Ascii)
	}
	if color {
		lipgloss.SetColorProfile(termenv.TrueColor)
	}
	return nil
}

func isNonInteractive(cmd *cobra.Command) bool {
	value, err := cmd.Flags().GetBool("non-interactive")
	return err == nil && value
}

func interactiveTerminal() bool {
	if strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	return isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stdout.Fd())
}

func commandError(cmd *cobra.Command, format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	if verbose, _ := cmd.Flags().GetBool("verbose"); verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "mangate: %v\n", err)
	}
	return err
}
