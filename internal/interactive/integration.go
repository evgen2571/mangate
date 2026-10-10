package interactive

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/source"
)

// Result is the file-only contract for an interactive download. Chapters keep
// their own image sequence; failed chapters never expose consumable images.
type Result struct {
	FormatVersion string          `json:"formatVersion"`
	Status        string          `json:"status"`
	Title         *source.Manga   `json:"title"`
	OutputDir     string          `json:"outputDir"`
	Chapters      []ChapterResult `json:"chapters"`
	Error         string          `json:"error,omitempty"`
}

type ChapterResult struct {
	ExpectedPages int             `json:"expectedPages"`
	Chapter       *source.Chapter `json:"chapter"`
	Status        string          `json:"status"`
	OutputDir     string          `json:"outputDir"`
	ImagePaths    []string        `json:"imagePaths"`
}

func NewIntegration(a *app.App, ctx context.Context, language string, maxChapters int) tea.Model {
	m := NewWithContextAndLanguage(a, ctx, language).(*model)
	m.integration, m.maxChapters = true, maxChapters
	m.format = archive.FormatDirectory
	return m
}

func (m *model) Result() Result {
	root, _ := filepath.Abs(m.app.Cfg.Download.Dir)
	result := Result{FormatVersion: "1", Status: "cancelled", Title: m.manga, OutputDir: root, Chapters: m.downloadResults}
	if result.Chapters == nil {
		result.Chapters = []ChapterResult{}
	}
	if m.screen == doneScreen {
		switch {
		case errors.Is(m.doneErr, context.Canceled):
			result.Status = "cancelled"
		case m.doneErr != nil || m.doneFailed > 0:
			result.Status = "partial"
		case m.doneCompleted > 0:
			result.Status = "success"
		default:
			result.Status = "error"
		}
	}
	if m.doneErr != nil {
		result.Error = m.doneErr.Error()
	}
	return result
}

// WriteResult replaces the destination only after the complete JSON is synced
// and closed. The temporary file lives beside it so rename stays atomic.
func WriteResult(path string, result Result) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".mangate-result-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err = json.NewEncoder(file).Encode(result); err == nil {
		err = file.Sync()
	}
	err = errors.Join(err, file.Close())
	if err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func imagePaths(directory string, expected int) ([]string, error) {
	return archive.ValidatedImagePaths(directory, expected)
}

// Stable metadata ordering avoids provider response and map iteration order.
func orderChapters(chapters []*source.Chapter) {
	sort.SliceStable(chapters, func(i, j int) bool {
		a, b := chapters[i], chapters[j]
		if a == nil {
			return false
		}
		if b == nil {
			return true
		}
		av, ae := strconv.ParseFloat(a.Index, 64)
		bv, be := strconv.ParseFloat(b.Index, 64)
		an, bn := ae == nil && !math.IsNaN(av), be == nil && !math.IsNaN(bv)
		if an != bn {
			return an
		}
		if an && bn && av != bv {
			return av < bv
		}
		if a.Index != b.Index {
			return a.Index < b.Index
		}
		return a.ID < b.ID
	})
}
