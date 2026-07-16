// Package archive creates and verifies chapter archives without changing page bytes.
package archive

import (
	"archive/zip"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Format string

const (
	FormatDirectory Format = "directory"
	FormatCBZ       Format = "cbz"
	FormatZIP       Format = "zip"
)

func ParseFormat(value string) (Format, error) {
	format := Format(strings.ToLower(strings.TrimSpace(value)))
	switch format {
	case FormatDirectory, FormatCBZ, FormatZIP:
		return format, nil
	default:
		return "", fmt.Errorf("invalid output format %q; use directory, cbz, or zip", value)
	}
}

func (f Format) Extension() string {
	if f == FormatCBZ {
		return ".cbz"
	}
	if f == FormatZIP {
		return ".zip"
	}
	return ""
}

type ExistingFileMode string

const (
	ExistingSkip    ExistingFileMode = "skip"
	ExistingReplace ExistingFileMode = "replace"
	ExistingFail    ExistingFileMode = "fail"
)

type Status string

const (
	StatusComplete Status = "complete"
	StatusSkipped  Status = "skipped"
)

type Metadata struct {
	Provider      string `json:"provider,omitempty"`
	TitleID       string `json:"titleId,omitempty"`
	Title         string `json:"title,omitempty"`
	ChapterID     string `json:"chapterId,omitempty"`
	ChapterNumber string `json:"chapterNumber,omitempty"`
	ChapterTitle  string `json:"chapterTitle,omitempty"`
	Language      string `json:"language,omitempty"`
	ExpectedPages int    `json:"expectedPages,omitempty"`
	SchemaVersion string `json:"schemaVersion"`
	Completion    string `json:"completion"`
	CreatedAt     string `json:"createdAt"`
}

type Options struct {
	Format           Format
	SourceDir        string
	OutputPath       string
	ExistingFileMode ExistingFileMode
	Metadata         Metadata
	RemoveSource     bool
}

type Validation struct {
	Valid     bool   `json:"valid"`
	Complete  bool   `json:"complete"`
	Message   string `json:"message,omitempty"`
	PageCount int    `json:"pageCount"`
	Format    Format `json:"format,omitempty"`
	TitleID   string `json:"titleId,omitempty"`
	ChapterID string `json:"chapterId,omitempty"`
}

type Result struct {
	Format        Format     `json:"format"`
	OutputPath    string     `json:"outputPath"`
	SourceDir     string     `json:"sourceDir"`
	Status        Status     `json:"status"`
	IncludedPages int        `json:"includedPages"`
	Validation    Validation `json:"validation"`
	SourceRemoved bool       `json:"sourceRemoved"`
}

type Inspection struct {
	Validation
	Path          string   `json:"path"`
	EntryCount    int      `json:"entryCount"`
	Entries       []string `json:"entries,omitempty"`
	MetadataFound bool     `json:"metadataFound"`
}

type chapterState struct {
	TitleID       string `json:"titleId"`
	ChapterID     string `json:"chapterId"`
	ExpectedPages int    `json:"expectedPages"`
	Complete      bool   `json:"complete"`
}

func CreateFromDirectory(options Options) (Result, error) {
	if options.Format != FormatCBZ && options.Format != FormatZIP {
		return Result{}, fmt.Errorf("create archive: format must be cbz or zip")
	}
	if strings.TrimSpace(options.SourceDir) == "" || strings.TrimSpace(options.OutputPath) == "" {
		return Result{}, fmt.Errorf("create archive: source directory and output path are required")
	}
	if strings.ToLower(filepath.Ext(options.OutputPath)) != options.Format.Extension() {
		return Result{}, fmt.Errorf("create archive: %s output must use %s", options.Format, options.Format.Extension())
	}
	pages, state, err := sourcePages(options.SourceDir)
	if err != nil {
		return Result{}, err
	}
	if state != nil {
		if !state.Complete || state.ExpectedPages != len(pages) {
			return Result{}, fmt.Errorf("create archive: source chapter is incomplete")
		}
		if options.Metadata.TitleID == "" {
			options.Metadata.TitleID = state.TitleID
		}
		if options.Metadata.ChapterID == "" {
			options.Metadata.ChapterID = state.ChapterID
		}
		if options.Metadata.ExpectedPages == 0 {
			options.Metadata.ExpectedPages = state.ExpectedPages
		}
	}

	if options.ExistingFileMode == "" {
		options.ExistingFileMode = ExistingSkip
	}
	if info, statErr := os.Stat(options.OutputPath); statErr == nil && !info.IsDir() {
		inspection, inspectErr := Inspect(options.OutputPath)
		if options.ExistingFileMode == ExistingSkip && inspectErr == nil && inspection.Complete && identitiesMatch(inspection.Validation, options.Metadata) {
			return Result{Format: options.Format, OutputPath: options.OutputPath, SourceDir: options.SourceDir, Status: StatusSkipped, IncludedPages: inspection.PageCount, Validation: inspection.Validation}, nil
		}
		if options.ExistingFileMode != ExistingReplace {
			return Result{}, fmt.Errorf("create archive: destination %q already exists; use --existing-files replace to replace it", options.OutputPath)
		}
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return Result{}, fmt.Errorf("create archive: inspect destination %q: %w", options.OutputPath, statErr)
	}

	if err := os.MkdirAll(filepath.Dir(options.OutputPath), 0o755); err != nil {
		return Result{}, fmt.Errorf("create archive: create output directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(options.OutputPath), ".mangate-*"+options.Format.Extension())
	if err != nil {
		return Result{}, fmt.Errorf("create archive: create temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := writeArchive(temporary, pages, options.Format, options.Metadata); err != nil {
		_ = temporary.Close()
		return Result{}, err
	}
	if err := temporary.Close(); err != nil {
		return Result{}, fmt.Errorf("create archive: close temporary file: %w", err)
	}
	inspection, err := Inspect(temporaryPath)
	if err != nil || !inspection.Complete || inspection.PageCount != len(pages) {
		if err == nil {
			err = errors.New("archive did not validate as complete")
		}
		return Result{}, fmt.Errorf("create archive: validate temporary archive: %w", err)
	}
	if err := os.Rename(temporaryPath, options.OutputPath); err != nil {
		return Result{}, fmt.Errorf("create archive: finalize archive: %w", err)
	}
	result := Result{Format: options.Format, OutputPath: options.OutputPath, SourceDir: options.SourceDir, Status: StatusComplete, IncludedPages: len(pages), Validation: inspection.Validation}
	if options.RemoveSource {
		if err := os.RemoveAll(options.SourceDir); err != nil {
			return result, fmt.Errorf("create archive: archive completed but remove source directory: %w", err)
		}
		result.SourceRemoved = true
	}
	return result, nil
}

func sourcePages(directory string) ([]string, *chapterState, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, nil, fmt.Errorf("create archive: read source directory: %w", err)
	}
	pages := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".part") || !isPageName(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			return nil, nil, fmt.Errorf("create archive: unreadable page %q", entry.Name())
		}
		pages = append(pages, filepath.Join(directory, entry.Name()))
	}
	sort.Strings(pages)
	if len(pages) == 0 {
		return nil, nil, fmt.Errorf("create archive: source directory has no page files")
	}
	statePath := filepath.Join(directory, ".mangate.json")
	data, err := os.ReadFile(statePath)
	if os.IsNotExist(err) {
		return pages, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("create archive: read source state: %w", err)
	}
	var state chapterState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, nil, fmt.Errorf("create archive: parse source state: %w", err)
	}
	return pages, &state, nil
}

func writeArchive(destination io.Writer, pages []string, format Format, metadata Metadata) error {
	writer := zip.NewWriter(destination)
	defer writer.Close()
	for _, path := range pages {
		name := filepath.Base(path)
		if !isSafeEntry(name) {
			return fmt.Errorf("create archive: unsafe page filename %q", name)
		}
		if err := copyEntry(writer, name, path); err != nil {
			return err
		}
	}
	metadata.SchemaVersion = "1"
	metadata.Completion = "complete"
	metadata.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	if format == FormatCBZ {
		comicInfo, err := comicInfoXML(metadata)
		if err != nil {
			return fmt.Errorf("create archive: encode ComicInfo.xml: %w", err)
		}
		if err := writeEntry(writer, "ComicInfo.xml", comicInfo); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("create archive: encode metadata: %w", err)
	}
	if err := writeEntry(writer, ".mangate.json", append(data, '\n')); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("create archive: close zip writer: %w", err)
	}
	return nil
}

func copyEntry(writer *zip.Writer, name, path string) error {
	in, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("create archive: open page %q: %w", path, err)
	}
	defer in.Close()
	out, err := writer.Create(name)
	if err != nil {
		return fmt.Errorf("create archive: create entry %q: %w", name, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("create archive: write entry %q: %w", name, err)
	}
	return nil
}

func writeEntry(writer *zip.Writer, name string, data []byte) error {
	out, err := writer.Create(name)
	if err != nil {
		return fmt.Errorf("create archive: create entry %q: %w", name, err)
	}
	if _, err := out.Write(data); err != nil {
		return fmt.Errorf("create archive: write entry %q: %w", name, err)
	}
	return nil
}

type comicInfo struct {
	XMLName     xml.Name `xml:"ComicInfo"`
	Title       string   `xml:"Title,omitempty"`
	Series      string   `xml:"Series,omitempty"`
	Number      string   `xml:"Number,omitempty"`
	LanguageISO string   `xml:"LanguageISO,omitempty"`
	PageCount   int      `xml:"PageCount,omitempty"`
	Publisher   string   `xml:"Publisher,omitempty"`
}

func comicInfoXML(metadata Metadata) ([]byte, error) {
	return xml.MarshalIndent(comicInfo{Title: metadata.ChapterTitle, Series: metadata.Title, Number: metadata.ChapterNumber, LanguageISO: metadata.Language, PageCount: metadata.ExpectedPages, Publisher: metadata.Provider}, "", "  ")
}

func Inspect(path string) (Inspection, error) {
	inspection := Inspection{Path: path, Validation: Validation{Format: formatFromPath(path)}}
	if inspection.Format == "" {
		inspection.Message = "archive extension must be .cbz or .zip"
		return inspection, errors.New(inspection.Message)
	}
	reader, err := zip.OpenReader(path)
	if err != nil {
		inspection.Message = err.Error()
		return inspection, fmt.Errorf("inspect archive %q: %w", path, err)
	}
	defer reader.Close()
	seen := make(map[string]struct{}, len(reader.File))
	previousPage := ""
	var metadata Metadata
	for _, file := range reader.File {
		inspection.EntryCount++
		inspection.Entries = append(inspection.Entries, file.Name)
		if !isSafeEntry(file.Name) {
			inspection.Message = "archive has an unsafe entry path"
			return inspection, errors.New(inspection.Message)
		}
		if _, exists := seen[file.Name]; exists {
			inspection.Message = "archive has duplicate entry names"
			return inspection, errors.New(inspection.Message)
		}
		seen[file.Name] = struct{}{}
		if isPageName(file.Name) {
			if previousPage != "" && file.Name <= previousPage {
				inspection.Message = "archive page entries are not ordered"
				return inspection, errors.New(inspection.Message)
			}
			previousPage = file.Name
			inspection.PageCount++
		}
		if file.Name == ".mangate.json" {
			inspection.MetadataFound = true
			in, err := file.Open()
			if err != nil {
				return inspection, fmt.Errorf("inspect archive metadata: %w", err)
			}
			data, readErr := io.ReadAll(in)
			closeErr := in.Close()
			if readErr != nil || closeErr != nil || json.Unmarshal(data, &metadata) != nil {
				inspection.Message = "archive metadata is invalid"
				return inspection, errors.New(inspection.Message)
			}
		}
	}
	if inspection.PageCount == 0 {
		inspection.Message = "archive has no page entries"
		return inspection, errors.New(inspection.Message)
	}
	inspection.TitleID = metadata.TitleID
	inspection.ChapterID = metadata.ChapterID
	inspection.Valid = true
	inspection.Complete = metadata.Completion == "complete" && (metadata.ExpectedPages == 0 || metadata.ExpectedPages == inspection.PageCount)
	if !inspection.Complete {
		inspection.Message = "archive metadata does not confirm completion"
	}
	return inspection, nil
}

func formatFromPath(path string) Format {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".cbz":
		return FormatCBZ
	case ".zip":
		return FormatZIP
	default:
		return ""
	}
}

func identitiesMatch(validation Validation, metadata Metadata) bool {
	return (metadata.TitleID == "" || metadata.TitleID == validation.TitleID) && (metadata.ChapterID == "" || metadata.ChapterID == validation.ChapterID)
}

func isSafeEntry(name string) bool {
	if name == "" || filepath.IsAbs(name) || strings.Contains(name, "\\") || strings.Contains(name, ":") {
		return false
	}
	clean := filepath.Clean(name)
	return clean == name && clean != "." && !strings.HasPrefix(clean, "../") && !strings.Contains(clean, "/../")
}

func isPageName(name string) bool {
	extension := strings.ToLower(filepath.Ext(name))
	switch extension {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".avif", ".bmp", ".img":
		return !strings.Contains(name, "/") && !strings.Contains(name, "\\")
	default:
		return false
	}
}
