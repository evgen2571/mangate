package tui

import (
	"fmt"
	"strings"

	termimg "github.com/blacktop/go-termimg"
	"github.com/charmbracelet/lipgloss"
	"github.com/evgen2571/mangate/internal/constant"
)

func renderCover(path string, width, height int) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("cover path is empty")
	}

	width = max(1, width)
	height = max(1, height)

	widget, err := termimg.NewImageWidgetFromFile(path)
	if err != nil {
		return "", fmt.Errorf("open cover %q: %w", path, err)
	}
	if widget == nil {
		return "", fmt.Errorf("open cover %q: image widget is nil", path)
	}

	proto, err := graphicsProtocol()
	if err != nil {
		return "", err
	}

	widget.SetProtocol(proto)
	widget.SetSizeWithCorrection(width, height)

	out, err := widget.Render()
	if err != nil {
		return "", fmt.Errorf("render cover %q: %w", path, err)
	}

	return out, nil
}

func graphicsProtocol() (termimg.Protocol, error) {
	proto := termimg.DetectProtocol()

	switch proto {
	case termimg.Kitty, termimg.Sixel, termimg.ITerm2:
		return proto, nil
	default:
		return termimg.Auto, fmt.Errorf(
			"terminal does not support inline images (need Kitty, Sixel, or iTerm2 protocol)",
		)
	}
}

func renderCoverPlaceholder(width, height int, text string) string {
	width = max(1, width)
	height = max(1, height)

	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(constant.MutedColor).
		Render(text)
}

func clearTerminalImages(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}

	img, err := termimg.Open(path)
	if err != nil || img == nil {
		return
	}

	_ = img.Protocol(termimg.DetectProtocol()).
		Clear(termimg.ClearOptions{All: true})
}
