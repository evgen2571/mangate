package cli

import (
	"fmt"
	"strings"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/spf13/cobra"
)

func NewSearchCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "search <title>",
		Short: "Search manga by title using the default provider",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := strings.TrimSpace(strings.Join(args, " "))
			if title == "" {
				return fmt.Errorf("title cannot be empty")
			}

			results, err := a.UseCases().SearchManga(cmd.Context(), title)
			if err != nil {
				return fmt.Errorf("search %q with provider %q: %w", title, a.Cfg.Provider, err)
			}

			out := cmd.OutOrStdout()
			if len(results) == 0 {
				writef(out, "no results found for %q\n", title)
				return nil
			}

			for i, manga := range results {
				writef(out, "%d. %s\n", i+1, manga.Title)

				if manga.URL != "" {
					writef(out, "   URL: %s\n", manga.URL)
				}

				if manga.ID != "" {
					writef(out, "   ID:  %s\n", manga.ID)
				}

				writeln(out)
			}

			return nil
		},
	}
}
