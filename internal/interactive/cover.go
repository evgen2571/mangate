package interactive

import (
	"context"
	"fmt"
	"image"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evgen2571/mangate/internal/util"
)

type coverState struct {
	id      string
	request int
	loading bool
	image   image.Image
	err     error
	cancel  context.CancelFunc
}

type coverDone struct {
	request int
	image   image.Image
	err     error
}

func (m *model) loadSelectedCover() tea.Cmd {
	item, ok := m.resultsList.SelectedItem().(resultItem)
	if !ok || item.manga == nil {
		m.clearCover()
		return nil
	}

	if m.cover.id == item.manga.ID {
		return nil
	}

	m.clearCover()
	m.cover.id = item.manga.ID
	m.cover.loading = true
	ctx, cancel := context.WithCancel(m.ctx)
	m.cover.cancel = cancel
	request := m.cover.request
	provider, err := m.app.Provider()
	if err != nil {
		return func() tea.Msg { return coverDone{request: request, err: err} }
	}

	cache, manga := m.app.Cache, item.manga
	return func() tea.Msg {
		path, err := cache.Get(ctx, provider, manga)
		if err != nil {
			return coverDone{request: request, err: err}
		}

		file, err := os.Open(path)
		if err != nil {
			return coverDone{request: request, err: err}
		}

		defer file.Close()
		decoded, _, err := image.Decode(file)
		return coverDone{request: request, image: decoded, err: err}
	}
}

func (m *model) clearCover() {
	if m.cover.cancel != nil {
		m.cover.cancel()
	}

	m.cover = coverState{request: m.cover.request + 1}
}

func (m *model) resultListWidth() int {
	if m.contentWidth() >= 70 {
		return m.contentWidth() - 28
	}

	return m.contentWidth()
}

func (m *model) resultDetails() string {
	item, ok := m.resultsList.SelectedItem().(resultItem)
	if !ok || item.manga == nil {
		return ""
	}

	s := newStyles()
	width := 26
	parts := []string{s.heading.Render(util.SanitizeTerminalText(item.manga.Title))}
	switch {
	case m.cover.loading:
		parts = append(parts, "Loading cover...")
	case m.cover.err != nil:
		parts = append(parts, "Cover unavailable", util.SanitizeTerminalText(m.cover.err.Error()))
	case m.cover.image != nil:
		parts = append(parts, renderCover(m.cover.image, 18, max(1, min(8, m.mainHeight()-5))))
	default:
		parts = append(parts, "Cover not loaded")
	}

	meta := item.manga.Metadata
	parts = append(parts, util.SanitizeTerminalText(strings.Join(nonEmpty([]string{meta.Status, meta.ContentType, meta.Language}), " · ")))
	languages := make([]string, 0, len(meta.Description))
	for language := range meta.Description {
		languages = append(languages, language)
	}

	sort.Strings(languages)
	languages = append([]string{m.app.Cfg.Language, "en"}, languages...)
	for _, language := range languages {
		if description := strings.TrimSpace(meta.Description[language]); description != "" {
			parts = append(parts, util.SanitizeTerminalText(description))
			break
		}
	}
	// Bound the panel so long API descriptions cannot push navigation off-screen.
	text := lipgloss.NewStyle().Width(width).Render(strings.Join(parts, "\n"))
	lines := strings.Split(text, "\n")
	limit := max(3, m.mainHeight())
	if len(lines) > limit {
		lines = append(lines[:limit-1], "…")
	}

	return strings.Join(lines, "\n")
}

func renderCover(picture image.Image, width, maxRows int) string {
	bounds := picture.Bounds()
	rows := max(1, min(maxRows, width*bounds.Dy()/max(1, 2*bounds.Dx())))
	width = max(1, min(width, rows*2*bounds.Dx()/max(1, bounds.Dy())))
	colorAt := func(x, y int) lipgloss.Color {
		r, g, b, _ := picture.At(bounds.Min.X+x*bounds.Dx()/width, bounds.Min.Y+y*bounds.Dy()/(rows*2)).RGBA()
		return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8))
	}
	var out strings.Builder
	for row := 0; row < rows; row++ {
		if row > 0 {
			out.WriteByte('\n')
		}

		for column := 0; column < width; column++ {
			out.WriteString(lipgloss.NewStyle().Foreground(colorAt(column, row*2)).Background(colorAt(column, row*2+1)).Render("▀"))
		}
	}

	return out.String()
}
