package acquisition

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/evgen2571/mangate/internal/downloader"
)

type Image struct {
	Index  int    `json:"index"`
	Path   string `json:"path"`
	Format string `json:"format"`
}

// Manifest records selection separately from local files. Replay resolves the
// IDs anew; Verify checks only the existing files, without contacting a provider.
type Manifest struct {
	Version     int          `json:"version"`
	Plan        DownloadPlan `json:"plan"`
	Images      []Image      `json:"images"`
	ActualPages int          `json:"actualPages"`
	Status      string       `json:"status"`
	Validation  string       `json:"validation"`
	Error       string       `json:"error,omitempty"`
}

func (m Manifest) Validate() error {
	if m.Version != Version {
		return fmt.Errorf("manifest validation: unsupported version %d", m.Version)
	}
	if err := m.Plan.Validate(); err != nil {
		return err
	}
	if m.ActualPages != len(m.Images) || m.ActualPages > m.Plan.ExpectedPages {
		return fmt.Errorf("manifest validation: inconsistent actual page count")
	}
	previous := 0
	for _, img := range m.Images {
		stem := strings.TrimSuffix(img.Path, filepath.Ext(img.Path))
		number, err := strconv.Atoi(stem)
		if img.Index <= previous || img.Index > m.Plan.ExpectedPages || filepath.Base(img.Path) != img.Path || strings.ContainsAny(img.Path, "/\\\x00") || err != nil || number != img.Index {
			return fmt.Errorf("manifest validation: unsafe or unordered image path")
		}
		if img.Format != "png" && img.Format != "jpeg" && img.Format != "gif" && img.Format != "webp" && img.Format != "bmp" && img.Format != "avif" && !(img.Format == "unknown" && m.Validation == "incomplete") {
			return fmt.Errorf("manifest validation: unsupported detected image format")
		}
		previous = img.Index
	}
	switch m.Status {
	case "planned":
		if m.ActualPages != 0 || m.Validation != "eligible" || m.Error != "" {
			return fmt.Errorf("manifest validation: inconsistent planned status")
		}
	case "complete":
		if m.ActualPages != m.Plan.ExpectedPages || m.Validation != "complete" || m.Error != "" {
			return fmt.Errorf("manifest validation: inconsistent completion status")
		}
	case "partial", "failed", "cancelled":
		if m.Validation != "incomplete" || m.Error == "" || m.Status == "partial" && m.ActualPages == 0 || m.Status == "failed" && m.ActualPages != 0 {
			return fmt.Errorf("manifest validation: inconsistent failure status")
		}
	default:
		return fmt.Errorf("manifest validation: unsupported status %q", m.Status)
	}
	return nil
}

func NewManifest(plan DownloadPlan, pages []downloader.PageDownloadResult, status string, cause error) (Manifest, error) {
	m := Manifest{Version: Version, Plan: plan, Images: []Image{}, Status: status, Validation: "incomplete"}
	if status == "complete" {
		m.Validation = "complete"
	}
	if status == "planned" {
		m.Validation = "eligible"
	}
	// Transport errors may include URLs or credentials. Store a safe terminal reason.
	if cause != nil {
		reason := "selected chapter could not be acquired or validated"
		switch {
		case errors.Is(cause, context.Canceled):
			reason = "operation cancelled"
		case errors.Is(cause, context.DeadlineExceeded):
			reason = "operation timed out"
		case strings.Contains(cause.Error(), "plan validation:"):
			reason = "selected source metadata or page count changed"
		case strings.Contains(cause.Error(), "not found"):
			reason = "selected chapter unavailable"
		case strings.Contains(cause.Error(), "destination"):
			reason = "output destination rejected"
		}
		m.Error = "acquisition " + status + ": " + reason
	}
	for _, page := range pages {
		format, err := detectedFormat(page.Path)
		if err != nil {
			format = "unknown"
			if m.Status == "complete" {
				m.Status = "partial"
			}
			m.Validation = "incomplete"
			m.Error = "acquisition " + m.Status + ": downloaded image validation failed"
		}
		m.Images = append(m.Images, Image{Index: page.PageIndex, Path: filepath.Base(page.Path), Format: format})
	}
	m.ActualPages = len(m.Images)
	return m, m.Validate()
}

func detectedFormat(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	_, format, err := image.DecodeConfig(f)
	if err == nil {
		return format, nil
	}
	if _, seekErr := f.Seek(0, 0); seekErr != nil {
		return "", seekErr
	}
	header := make([]byte, 512)
	n, readErr := f.Read(header)
	if readErr != nil && readErr != io.EOF {
		return "", readErr
	}
	if n >= 16 && string(header[4:8]) == "ftyp" && (bytes.Contains(header[8:n], []byte("avif")) || bytes.Contains(header[8:n], []byte("avis"))) {
		return "avif", nil
	}
	switch http.DetectContentType(header[:n]) {
	case "image/webp":
		return "webp", nil
	case "image/bmp":
		return "bmp", nil
	}
	return "", err
}

func LoadManifest(path string) (*Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	defer f.Close()
	dec := json.NewDecoder(io.LimitReader(f, (4<<20)+1))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("malformed manifest: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("malformed manifest: trailing data")
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(m.Plan.Output) {
		m.Plan.Output = filepath.Join(filepath.Dir(path), filepath.FromSlash(m.Plan.Output))
	}
	m.Plan.Output, err = filepath.Abs(m.Plan.Output)
	return &m, err
}

// ValidateManifestDestination prevents manifests from replacing chapter files/state.
func ValidateManifestDestination(output, path string) error {
	if path == "" {
		return nil
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	root, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, absolute)
	if err != nil {
		return err
	}
	if relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("manifest validation: save manifest outside the chapter output directory")
	}
	return nil
}

func (m Manifest) Write(path string) error {
	if err := ValidateManifestDestination(m.Plan.Output, path); err != nil {
		return err
	}
	if err := m.Validate(); err != nil {
		return err
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	m.Plan.Output, err = filepath.Rel(filepath.Dir(absolute), m.Plan.Output)
	if err != nil {
		return err
	}
	m.Plan.Output = filepath.ToSlash(m.Plan.Output)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(absolute), ".mangate-manifest-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), absolute)
}

func (m Manifest) Verify() error {
	if err := m.Validate(); err != nil {
		return err
	}
	if m.Status != "complete" {
		return fmt.Errorf("manifest verification: acquisition is %s", m.Status)
	}
	chapter := *m.Plan.Chapter
	chapter.From = m.Plan.Manga
	if err := downloader.ValidateExactDestination(m.Plan.Output, &chapter, m.Plan.Provider); err != nil {
		return err
	}
	for _, img := range m.Images {
		path := filepath.Join(m.Plan.Output, img.Path)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("manifest verification: image is not a regular file")
		}
		format, err := detectedFormat(path)
		if err != nil {
			return err
		}
		if format != img.Format {
			return fmt.Errorf("manifest verification: image format changed")
		}
	}
	return nil
}
