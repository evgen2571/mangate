package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/constant"
	"github.com/evgen2571/mangate/internal/interactive"
	"github.com/evgen2571/mangate/internal/source"
)

func NewRootCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:           constant.ProjectName,
		Short:         "Download authorized manga from supported providers",
		Example:       "  mangate search \"example title\"\n  mangate download <title-id> --latest\n  mangate --format cbz download <title-id> --chapter 1\n  mangate tui",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := a.ApplyConfig(a.Cfg); err != nil {
				return err
			}

			if isQuiet(cmd) && !wantsJSON(cmd) {
				cmd.SetOut(io.Discard)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if interactiveTerminal() && !wantsJSON(cmd) && !isNonInteractive(cmd) {
				return runInteractive(cmd, a)
			}

			return cmd.Help()
		},
	}
	cmd.Version = constant.Version
	if build, ok := debug.ReadBuildInfo(); cmd.Version == "dev" && ok && build.Main.Version != "" && build.Main.Version != "(devel)" {
		cmd.Version = build.Main.Version
	}

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
		NewDiagnosticsCmd(a),
		NewDatasetCmd(a),
		NewDownloadCmd(a),
		NewRandomCmd(a),
		NewInteractiveCmd(a),
		NewProviderCmd(a),
		NewProvidersCmd(a),
		NewSearchCmd(a),
		NewTitleCmd(a),
	)

	return cmd
}

func NewInteractiveCmd(a *app.App) *cobra.Command {
	var exitOnComplete bool
	var resultFile string
	var maxChapters int
	cmd := &cobra.Command{
		Use: "tui", Aliases: []string{"interactive"}, Short: "Open the interactive terminal interface",
		Example: "  mangate tui\n  mangate tui --exit-on-complete --result-file result.json --max-chapters 1",
		RunE: func(cmd *cobra.Command, args []string) error {
			if exitOnComplete && resultFile == "" {
				return fmt.Errorf("--exit-on-complete requires --result-file")
			}
			if !exitOnComplete && (resultFile != "" || maxChapters != 0) {
				return fmt.Errorf("--result-file and --max-chapters require --exit-on-complete")
			}
			if maxChapters < 0 {
				return fmt.Errorf("--max-chapters cannot be negative")
			}
			if !exitOnComplete {
				return runInteractive(cmd, a)
			}
			if wantsJSON(cmd) {
				return fmt.Errorf("tui results use --result-file; --json cannot be combined with integration mode")
			}
			destination, err := filepath.Abs(resultFile)
			if err != nil {
				return err
			}
			if _, err := os.Lstat(destination); err == nil {
				return fmt.Errorf("result file already exists")
			} else if !os.IsNotExist(err) {
				return err
			}
			probe, err := os.CreateTemp(filepath.Dir(destination), ".mangate-probe-*")
			if err != nil {
				return err
			}
			if err := probe.Close(); err != nil {
				os.Remove(probe.Name())
				return err
			}
			os.Remove(probe.Name())
			ui := interactive.NewIntegration(a, cmd.Context(), tuiLanguage(cmd), maxChapters)
			err = runTUI(cmd, ui)
			result := ui.(interface{ Result() interactive.Result }).Result()
			if err != nil {
				result.Status, result.Error = "error", err.Error()
			}
			if errors.Is(err, tea.ErrInterrupted) || cmd.Context().Err() != nil {
				result.Status = "cancelled"
			}
			if writeErr := interactive.WriteResult(destination, result); writeErr != nil {
				return writeErr
			}
			switch result.Status {
			case "success":
				return nil
			case "cancelled":
				return &ReportedError{Cause: fmt.Errorf("interactive download cancelled"), Code: 7}
			case "partial":
				return &ReportedError{Cause: fmt.Errorf("interactive download incomplete"), Code: 5}
			default:
				return &ReportedError{Cause: fmt.Errorf("interactive download failed: %s", result.Error), Code: 10}
			}
		},
	}
	cmd.Flags().BoolVar(&exitOnComplete, "exit-on-complete", false, "Exit after downloading and write an integration result")
	cmd.Flags().StringVar(&resultFile, "result-file", "", "Write the atomic interactive result to a new JSON file")
	cmd.Flags().IntVar(&maxChapters, "max-chapters", 0, "Maximum selected chapters (0 means unlimited)")
	return cmd
}

func runInteractive(cmd *cobra.Command, a *app.App) error {
	return runTUI(cmd, interactive.NewWithContextAndLanguage(a, cmd.Context(), tuiLanguage(cmd)))
}

func runInteractiveSearchResults(cmd *cobra.Command, a *app.App, query string, results []*source.Manga) error {
	return runTUI(cmd, interactive.NewWithSearchResultsAndLanguage(a, cmd.Context(), query, results, tuiLanguage(cmd)))
}

func tuiLanguage(cmd *cobra.Command) string {
	if cmd.Flags().Changed("language") {
		value, err := cmd.Flags().GetString("language")
		if err == nil {
			return value
		}
	}

	return ""
}

func runTUI(cmd *cobra.Command, ui tea.Model) error {
	if isNonInteractive(cmd) {
		return fmt.Errorf("tui cannot run with --non-interactive")
	}

	if err := configureTUIColor(cmd); err != nil {
		return err
	}

	if !interactiveTerminal() {
		return fmt.Errorf("tui requires an interactive terminal; use direct commands such as search, chapters, or download")
	}
	// The alternate screen keeps TUI frames out of shell scrollback and Bubble
	// Tea restores the previous terminal state when the program exits.
	p := tea.NewProgram(ui, tea.WithAltScreen())
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
