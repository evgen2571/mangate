package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/archive"
	"github.com/spf13/cobra"
)

func NewArchiveCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Convert, inspect, and verify chapter archives",
		Long:  "Create CBZ or ZIP archives from local chapter directories, then inspect or verify them without extracting files.",
	}
	cmd.AddCommand(newArchiveConvertCmd(a), newArchiveInspectCmd("inspect"), newArchiveInspectCmd("verify"))
	return cmd
}

func newArchiveConvertCmd(a *app.App) *cobra.Command {
	var output string
	var removeSource bool
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "convert <chapter-directory>",
		Short:   "Create a CBZ or ZIP archive from local chapter pages",
		Example: "  mangate --format cbz archive convert ./library/Example/Chapter-1\n  mangate --format zip archive convert ./library/Example/Chapter-1 --remove-source",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := archive.ParseFormat(a.Cfg.Download.Format)
			if err != nil {
				return err
			}
			if format == archive.FormatDirectory {
				return fmt.Errorf("archive convert: choose --format cbz or --format zip")
			}
			sourceDir := strings.TrimSpace(args[0])
			if sourceDir == "" {
				return fmt.Errorf("archive convert: chapter directory cannot be empty")
			}
			if output == "" {
				output = filepath.Clean(sourceDir) + format.Extension()
			}
			if dryRun {
				plan, err := planArchiveConversion(sourceDir, output, format, removeSource)
				if err != nil {
					return err
				}
				if wantsJSON(cmd) {
					return writeJSON(cmd, "archive.convert.plan", plan)
				}
				writeHuman(cmd.OutOrStdout(), "Source: %s\nFormat: %s\nOutput: %s\nDestination exists: %t\nRemove source after validation: %t\nDry run: no files will be changed\n", plan.SourceDir, plan.Format, plan.OutputPath, plan.DestinationExists, plan.RemoveSource)
				return nil
			}
			result, err := archive.CreateFromDirectory(archive.Options{
				Format:           format,
				SourceDir:        sourceDir,
				OutputPath:       output,
				ExistingFileMode: archive.ExistingFileMode(a.Cfg.Download.ExistingFileMode),
				RemoveSource:     removeSource,
			})
			if wantsJSON(cmd) {
				if err != nil {
					return err
				}
				return writeJSON(cmd, "archive.convert", result)
			}
			if err != nil {
				return err
			}
			writeHuman(cmd.OutOrStdout(), "%s archive: %s\n", result.Status, result.OutputPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "Destination archive path")
	cmd.Flags().BoolVar(&removeSource, "remove-source", false, "Remove the source directory after archive validation")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the conversion plan without writing an archive")
	return cmd
}

type archiveConversionPlan struct {
	SourceDir         string         `json:"sourceDir"`
	OutputPath        string         `json:"outputPath"`
	Format            archive.Format `json:"format"`
	DestinationExists bool           `json:"destinationExists"`
	RemoveSource      bool           `json:"removeSource"`
}

func planArchiveConversion(sourceDir, outputPath string, format archive.Format, removeSource bool) (archiveConversionPlan, error) {
	info, err := os.Stat(sourceDir)
	if err != nil {
		return archiveConversionPlan{}, fmt.Errorf("archive convert: inspect source directory: %w", err)
	}
	if !info.IsDir() {
		return archiveConversionPlan{}, fmt.Errorf("archive convert: source %q is not a directory", sourceDir)
	}
	_, err = os.Stat(outputPath)
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return archiveConversionPlan{}, fmt.Errorf("archive convert: inspect destination: %w", err)
	}
	return archiveConversionPlan{SourceDir: sourceDir, OutputPath: outputPath, Format: format, DestinationExists: exists, RemoveSource: removeSource}, nil
}

func newArchiveInspectCmd(name string) *cobra.Command {
	return &cobra.Command{
		Use:   name + " <archive-path>",
		Short: map[string]string{"inspect": "Show archive contents and completion state", "verify": "Check archive structure and completion state"}[name],
		Args:  requireOneArgument("a local <chapter-directory>", "mangate --format cbz archive convert ./library/Example/Chapter-1"),
		RunE: func(cmd *cobra.Command, args []string) error {
			inspection, err := archive.Inspect(args[0])
			if wantsJSON(cmd) {
				if err != nil {
					return err
				}
				return writeJSON(cmd, "archive."+name, inspection)
			}
			if err != nil {
				return err
			}
			writeHuman(cmd.OutOrStdout(), "Archive: %s\nFormat: %s\nPages: %d\nEntries: %d\nComplete: %t\n", inspection.Path, inspection.Format, inspection.PageCount, inspection.EntryCount, inspection.Complete)
			if metadata := inspection.Metadata; metadata != nil {
				writeHuman(cmd.OutOrStdout(), "Provider: %s\nTitle: %s\nTitle ID: %s\nChapter: %s\nChapter ID: %s\n", metadata.Provider, metadata.Title, metadata.TitleID, metadata.ChapterNumber, metadata.ChapterID)
				if metadata.Volume != "" {
					writeHuman(cmd.OutOrStdout(), "Volume: %s\n", metadata.Volume)
				}
				if metadata.ChapterTitle != "" {
					writeHuman(cmd.OutOrStdout(), "Chapter title: %s\n", metadata.ChapterTitle)
				}
				if metadata.Language != "" {
					writeHuman(cmd.OutOrStdout(), "Language: %s\n", metadata.Language)
				}
			}
			return nil
		},
	}
}
