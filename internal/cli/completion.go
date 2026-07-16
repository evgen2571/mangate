package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewCompletionCmd creates local shell-completion scripts. Generation only
// reads the command tree; it never contacts a provider or inspects a library.
func NewCompletionCmd(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:       "completion <bash|zsh|fish>",
		Short:     "Generate shell completion scripts",
		Long:      "Generate a completion script for a supported shell. Redirect the output or source it in your shell startup configuration.",
		Example:   "  source <(mangate completion bash)\n  source <(mangate completion zsh)\n  mangate completion fish | source",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return root.GenBashCompletionV2(cmd.OutOrStdout(), true)
			case "zsh":
				return root.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(cmd.OutOrStdout(), true)
			default:
				return fmt.Errorf("unsupported shell %q; use bash, zsh, or fish", args[0])
			}
		},
	}
	return cmd
}
