package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func requireOneArgument(name, example string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
			return nil
		}
		return fmt.Errorf("%s requires %s\nExample: %s", cmd.CommandPath(), name, example)
	}
}

func requireSearchQuery(cmd *cobra.Command, args []string) error {
	if strings.TrimSpace(strings.Join(args, " ")) != "" {
		return nil
	}
	return fmt.Errorf("%s requires a search query\nExample: mangate search \"example title\"", cmd.CommandPath())
}
