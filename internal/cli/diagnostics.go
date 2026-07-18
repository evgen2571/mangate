package cli

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/spf13/cobra"
)

type diagnosticsRecord struct {
	Platform            string              `json:"platform"`
	InteractiveTerminal bool                `json:"interactiveTerminal"`
	Provider            source.ProviderInfo `json:"provider"`
	DownloadDirectory   pathDiagnostic      `json:"downloadDirectory"`
	CacheDirectory      pathDiagnostic      `json:"cacheDirectory"`
	SupportedFormats    []archive.Format    `json:"supportedFormats"`
}

type pathDiagnostic struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	IsDir  bool   `json:"isDirectory"`
	Error  string `json:"error,omitempty"`
}

func NewDiagnosticsCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:     "diagnostics",
		Aliases: []string{"doctor"},
		Short:   "Check local setup without contacting a provider",
		Long:    "Report the effective local setup, provider capabilities, configured paths, supported archive formats, and terminal suitability. This command makes no provider requests and creates no files.",
		Example: "  mangate diagnostics\n  mangate --json diagnostics",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			provider, err := a.Provider()
			if err != nil {
				return fmt.Errorf("diagnostics: configured provider: %w", err)
			}
			record := diagnosticsRecord{
				Platform:            runtime.GOOS + "/" + runtime.GOARCH,
				InteractiveTerminal: interactiveTerminal(),
				Provider:            provider.Info(),
				DownloadDirectory:   inspectPath(a.Cfg.Download.Dir),
				CacheDirectory:      inspectPath(a.Cfg.Dirs.Cache),
				SupportedFormats:    []archive.Format{archive.FormatDirectory, archive.FormatPNG, archive.FormatJPEG, archive.FormatCBZ, archive.FormatZIP},
			}
			if wantsJSON(cmd) {
				return writeJSON(cmd, "diagnostics", record)
			}
			writeHuman(cmd.OutOrStdout(), "Platform: %s\nInteractive terminal: %t\nProvider: %s (%s)\nProvider availability: %s\nDownload directory: %s\nCache directory: %s\nOutput formats: directory, png, jpeg, cbz, zip\n", record.Platform, record.InteractiveTerminal, record.Provider.Name, record.Provider.ID, record.Provider.Availability, formatPathDiagnostic(record.DownloadDirectory), formatPathDiagnostic(record.CacheDirectory))
			return nil
		},
	}
}

func inspectPath(path string) pathDiagnostic {
	record := pathDiagnostic{Path: path}
	info, err := os.Stat(path)
	if err == nil {
		record.Exists = true
		record.IsDir = info.IsDir()
		return record
	}
	if !os.IsNotExist(err) {
		record.Error = err.Error()
	}
	return record
}

func formatPathDiagnostic(record pathDiagnostic) string {
	path := strings.TrimSpace(record.Path)
	if record.Error != "" {
		return path + " (unavailable: " + record.Error + ")"
	}
	if !record.Exists {
		return path + " (will be created when needed)"
	}
	if !record.IsDir {
		return path + " (exists but is not a directory)"
	}
	return path + " (available)"
}
