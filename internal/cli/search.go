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
	var contentTypes []string
	var interactive bool
	cmd := &cobra.Command{
		Use:     "search <title>",
		Short:   "Search manga by title using the default provider",
		Example: "  mangate search \"example title\" --limit 10\n  mangate --json search \"example title\"",
		Args:    requireSearchQuery,
		RunE: func(cmd *cobra.Command, args []string) error {
			title := strings.TrimSpace(strings.Join(args, " "))
			if title == "" {
				return fmt.Errorf("title cannot be empty")
			}
			if interactive && wantsJSON(cmd) {
				return fmt.Errorf("--interactive cannot be combined with --json")
			}

			results, err := a.UseCases().SearchManga(cmd.Context(), title)
			if err != nil {
				return fmt.Errorf("search %q with provider %q: %w", title, a.Cfg.Provider, err)
			}
			languageFilter := ""
			if cmd.Flags().Changed("language") {
				languageFilter = a.Cfg.Language
			}
			results = filterSearchResults(results, languageFilter, contentTypes)

			if len(results) == 0 {
				record := searchRecord{Provider: a.Cfg.Provider, Query: title, Results: []*source.Manga{}}
				if wantsJSON(cmd) {
					if err := writeJSONStatus(cmd, "search", "no_results", record); err != nil {
						return err
					}
				} else {
					writeHuman(cmd.OutOrStdout(), "no results found for %q\n", title)
				}
				return &ReportedError{Cause: fmt.Errorf("no results found for %q", title), Code: 1, Silent: true}
			}
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}
			if interactive {
				return runInteractiveSearchResults(cmd, a, title, results)
			}
			if wantsJSON(cmd) {
				return writeJSON(cmd, "search", searchRecord{Provider: a.Cfg.Provider, Query: title, Results: results})
			}

			for i, manga := range results {
				writeHuman(cmd.OutOrStdout(), "%d. %s\n", i+1, manga.Title)
				writeHuman(cmd.OutOrStdout(), "   Provider: %s\n", a.Cfg.Provider)
				if manga.Metadata.AlternativeTitle != "" {
					writeHuman(cmd.OutOrStdout(), "   Alternative title: %s\n", manga.Metadata.AlternativeTitle)
				}
				if manga.Metadata.ContentType != "" {
					writeHuman(cmd.OutOrStdout(), "   Content type: %s\n", manga.Metadata.ContentType)
				}
				if manga.Metadata.Status != "" {
					writeHuman(cmd.OutOrStdout(), "   Status: %s\n", manga.Metadata.Status)
				}
				if manga.Metadata.Language != "" {
					writeHuman(cmd.OutOrStdout(), "   Language: %s\n", manga.Metadata.Language)
				}
				if manga.Metadata.Year > 0 {
					writeHuman(cmd.OutOrStdout(), "   Year: %d\n", manga.Metadata.Year)
				}
				if manga.ID != "" {
					writeHuman(cmd.OutOrStdout(), "   Reference: %s\n", manga.ID)
				}
				if manga.URL != "" {
					writeHuman(cmd.OutOrStdout(), "   URL: %s\n", manga.URL)
				}

				writeHuman(cmd.OutOrStdout(), "\n")
			}

			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of results")
	cmd.Flags().StringSliceVar(&contentTypes, "content-type", nil, "Filter by content type (repeatable; any value matches, duplicates ignored)")
	cmd.Flags().BoolVar(&interactive, "interactive", false, "Open the matching results in the terminal interface")
	return cmd
}

func filterSearchResults(results []*source.Manga, language string, contentTypes []string) []*source.Manga {
	language = strings.TrimSpace(language)
	wantedContentTypes := make(map[string]struct{}, len(contentTypes))
	for _, contentType := range contentTypes {
		if contentType = strings.ToLower(strings.TrimSpace(contentType)); contentType != "" {
			wantedContentTypes[contentType] = struct{}{}
		}
	}

	filtered := make([]*source.Manga, 0, len(results))
	for _, manga := range results {
		if manga == nil {
			continue
		}
		if language != "" && !strings.EqualFold(strings.TrimSpace(manga.Metadata.Language), language) {
			continue
		}
		if len(wantedContentTypes) > 0 {
			if _, ok := wantedContentTypes[strings.ToLower(strings.TrimSpace(manga.Metadata.ContentType))]; !ok {
				continue
			}
		}
		filtered = append(filtered, manga)
	}
	return filtered
}

type searchRecord struct {
	Provider string          `json:"provider"`
	Query    string          `json:"query"`
	Results  []*source.Manga `json:"results"`
}
