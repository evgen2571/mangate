package cli

import (
	"fmt"
	"strings"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/spf13/cobra"
)

func NewSearchCmd(a *app.App) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "search <title>",
		Short: "Search manga by title using the default provider",
		Args:  requireSearchQuery,
		RunE: func(cmd *cobra.Command, args []string) error {
			title := strings.TrimSpace(strings.Join(args, " "))
			if title == "" {
				return fmt.Errorf("title cannot be empty")
			}

			results, err := a.UseCases().SearchManga(cmd.Context(), title)
			if err != nil {
				return fmt.Errorf("search %q with provider %q: %w", title, a.Cfg.Provider, err)
			}

			if len(results) == 0 {
				if wantsJSON(cmd) {
					return writeJSON(cmd, "search", searchRecord{Provider: a.Cfg.Provider, Query: title, Results: []*source.Manga{}})
				}
				writeHuman(cmd.OutOrStdout(), "no results found for %q\n", title)
				return nil
			}
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}
			if wantsJSON(cmd) {
				return writeJSON(cmd, "search", searchRecord{Provider: a.Cfg.Provider, Query: title, Results: results})
			}

			for i, manga := range results {
				writeHuman(cmd.OutOrStdout(), "%d. %s\n", i+1, manga.Title)

				if manga.URL != "" {
					writeHuman(cmd.OutOrStdout(), "   URL: %s\n", manga.URL)
				}

				if manga.ID != "" {
					writeHuman(cmd.OutOrStdout(), "   ID:  %s\n", manga.ID)
				}

				writeHuman(cmd.OutOrStdout(), "\n")
			}

			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of results")
	return cmd
}

type searchRecord struct {
	Provider string          `json:"provider"`
	Query    string          `json:"query"`
	Results  []*source.Manga `json:"results"`
}
