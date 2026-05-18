package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/evgen2571/mangate/internal/app"
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

			chapters, err := a.UseCases().ChaptersByID(cmd.Context(), mangaID)
			if err != nil {
				return fmt.Errorf("load chapters for manga %q with provider %q: %w", mangaID, a.Cfg.Provider, err)
			}

			out := cmd.OutOrStdout()
			if len(chapters) == 0 {
				writef(out, "no chapters found for manga %s\n", mangaID)
				return nil
			}

			writef(out, "Chapters for %s\n\n", mangaID)
			for i, chapter := range chapters {
				if chapter == nil {
					return fmt.Errorf("chapter #%d is nil", i+1)
				}

				writef(out, "%d. %s\n", i+1, chapter.DisplayTitle(i))
				if chapter.ID != "" {
					writef(out, "   ID:    %s\n", chapter.ID)
				}
				if chapter.PageCount > 0 {
					writef(out, "   Pages: %d\n", chapter.PageCount)
				}
				if chapter.URL != "" {
					writef(out, "   URL:   %s\n", chapter.URL)
				}
				writeln(out)
			}

			return nil
		},
	}
}
