package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/source"
)

func NewChaptersCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "chapters <manga-id>",
		Short: "List chapters for a manga using the default provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mangaID := strings.TrimSpace(args[0])
			if mangaID == "" {
				return fmt.Errorf("manga id cannot be empty")
			}

			manga := &source.Manga{ID: mangaID, Title: mangaID}
			chapters, err := a.UseCases().Chapters(cmd.Context(), manga)
			if err != nil {
				return fmt.Errorf("load chapters for manga %q with provider %q: %w", mangaID, a.Cfg.Provider, err)
			}

			out := cmd.OutOrStdout()
			if len(chapters) == 0 {
				fmt.Fprintf(out, "no chapters found for manga %s\n", mangaID)
				return nil
			}

			fmt.Fprintf(out, "Chapters for %s\n\n", mangaID)
			for i, chapter := range chapters {
				if chapter == nil {
					return fmt.Errorf("chapter #%d is nil", i+1)
				}

				fmt.Fprintf(out, "%d. %s\n", i+1, chapter.DisplayTitle(i))
				if chapter.ID != "" {
					fmt.Fprintf(out, "   ID:    %s\n", chapter.ID)
				}
				if chapter.PageCount > 0 {
					fmt.Fprintf(out, "   Pages: %d\n", chapter.PageCount)
				}
				if chapter.URL != "" {
					fmt.Fprintf(out, "   URL:   %s\n", chapter.URL)
				}
				fmt.Fprintln(out)
			}

			return nil
		},
	}
}
