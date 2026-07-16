package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/source"
)

func NewChaptersCmd(a *app.App) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "chapters <manga-id>",
		Short: "List chapters for a manga using the default provider",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("manga id cannot be empty")
			}
			return requireOneArgument("a stable <title-id> from `mangate search`", "mangate chapters <title-id>")(cmd, args)
		},
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
				if wantsJSON(cmd) {
					return writeJSON(cmd, "chapters.list", chapterListRecord{Provider: a.Cfg.Provider, TitleID: mangaID, Order: "ascending provider chapter sequence", Chapters: []*source.Chapter{}})
				}
				writeHuman(out, "no chapters found for manga %s\n", mangaID)
				return nil
			}
			if limit > 0 && len(chapters) > limit {
				chapters = chapters[:limit]
			}
			if wantsJSON(cmd) {
				return writeJSON(cmd, "chapters.list", chapterListRecord{Provider: a.Cfg.Provider, TitleID: mangaID, Order: "ascending provider chapter sequence", Chapters: chapters})
			}

			writeHuman(out, "Chapters for %s\n\n", mangaID)
			for i, chapter := range chapters {
				if chapter == nil {
					return fmt.Errorf("chapter #%d is nil", i+1)
				}

				writeHuman(out, "%d. %s\n", i+1, chapter.DisplayTitle(i))
				if chapter.ID != "" {
					writeHuman(out, "   ID:    %s\n", chapter.ID)
				}
				if chapter.PageCount > 0 {
					writeHuman(out, "   Pages: %d\n", chapter.PageCount)
				}
				if chapter.URL != "" {
					writeHuman(out, "   URL:   %s\n", chapter.URL)
				}
				writeHuman(out, "\n")
			}

			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of chapters")
	return cmd
}

type chapterListRecord struct {
	Provider string            `json:"provider"`
	TitleID  string            `json:"titleId"`
	Order    string            `json:"order"`
	Chapters []*source.Chapter `json:"chapters"`
}
