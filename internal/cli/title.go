package cli

import (
	"fmt"
	"strings"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/spf13/cobra"
)

type titleRecord struct {
	Provider string        `json:"provider"`
	Title    *source.Manga `json:"title"`
}

func NewTitleCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "title <title-id>",
		Short: "Show title metadata",
		Args:  requireOneArgument("a stable <title-id> from `mangate search`", "mangate title <title-id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.TrimSpace(args[0])
			if id == "" {
				return fmt.Errorf("title id cannot be empty")
			}
			provider, err := a.Provider()
			if err != nil {
				return err
			}
			title, err := provider.Title(cmd.Context(), id)
			if err != nil {
				return fmt.Errorf("get title %q from provider %q: %w", id, provider.Name(), err)
			}
			record := titleRecord{Provider: provider.Name(), Title: title}
			if wantsJSON(cmd) {
				return writeJSON(cmd, "title.get", record)
			}
			writeHuman(cmd.OutOrStdout(), "%s\nID: %s\nProvider: %s\n", title.Title, title.ID, provider.Name())
			if title.URL != "" {
				writeHuman(cmd.OutOrStdout(), "URL: %s\n", title.URL)
			}
			if title.Metadata.Status != "" {
				writeHuman(cmd.OutOrStdout(), "Status: %s\n", title.Metadata.Status)
			}
			if description := preferredDescription(title.Metadata.Description); description != "" {
				writeHuman(cmd.OutOrStdout(), "\n%s\n", description)
			}
			return nil
		},
	}
}

func preferredDescription(descriptions map[string]string) string {
	for _, language := range []string{"en", "ja", "ko", "zh"} {
		if value := strings.TrimSpace(descriptions[language]); value != "" {
			return value
		}
	}
	for _, value := range descriptions {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
