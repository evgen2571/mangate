package cli

import (
	"fmt"
	"strings"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/spf13/cobra"
)

type providerRecord struct {
	Info   source.ProviderInfo `json:"info"`
	Usable bool                `json:"usable"`
	Error  string              `json:"error,omitempty"`
}

func NewProvidersCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:     "providers",
		Short:   "List registered providers and their capabilities",
		Example: "  mangate providers\n  mangate --json providers",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			records := providerRecords(a)
			if wantsJSON(cmd) {
				return writeJSON(cmd, "providers.list", records)
			}
			for _, record := range records {
				writeHuman(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", record.Info.ID, record.Info.Name, record.Info.Availability, strings.Join(record.Info.Capabilities, ","))
				if record.Error != "" {
					writeHuman(cmd.OutOrStdout(), "  error: %s\n", record.Error)
				}
			}
			return nil
		},
	}
}

func NewProviderCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:     "provider <provider-id>",
		Short:   "Inspect one provider",
		Example: "  mangate provider mangadex\n  mangate --json provider mangadex",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			provider, err := a.Registry.New(name, a.Cfg, a.Client)
			if err != nil {
				return fmt.Errorf("inspect provider %q: %w", name, err)
			}
			record := providerRecord{Info: provider.Info(), Usable: true}
			if wantsJSON(cmd) {
				return writeJSON(cmd, "provider.inspect", record)
			}
			writeHuman(cmd.OutOrStdout(), "%s (%s)\n", record.Info.Name, record.Info.ID)
			writeHuman(cmd.OutOrStdout(), "Status: %s\nCapabilities: %s\nAuthentication: %s\nDownload permitted: %t\n", record.Info.Availability, strings.Join(record.Info.Capabilities, ", "), record.Info.Authentication, record.Info.DownloadPermitted)
			if record.Info.Description != "" {
				writeHuman(cmd.OutOrStdout(), "%s\n", record.Info.Description)
			}
			for _, restriction := range record.Info.Restrictions {
				writeHuman(cmd.OutOrStdout(), "Restriction: %s\n", restriction)
			}
			return nil
		},
	}
}

func providerRecords(a *app.App) []providerRecord {
	records := make([]providerRecord, 0, len(a.Registry.Names()))
	for _, name := range a.Registry.Names() {
		provider, err := a.Registry.New(name, a.Cfg, a.Client)
		if err != nil {
			records = append(records, providerRecord{Info: source.ProviderInfo{ID: name, Availability: "unavailable"}, Error: err.Error()})
			continue
		}
		records = append(records, providerRecord{Info: provider.Info(), Usable: true})
	}
	return records
}
