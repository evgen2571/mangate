package tui

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/evgen2571/mangate/internal/constant"
)

func renderCoverText(path string, width, height int) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("cover path is empty")
	}

	width = max(1, width)
	height = max(1, height)

	chafaPath, err := exec.LookPath("chafa")
	if err != nil {
		return "", fmt.Errorf("chafa not found in PATH")
	}

	// Chafa flags
	args := []string{
		"--format", "symbols",
		"--symbols", "block+border+space+half+quad+sextant",
		"--fill", "all",
		"--colors", "full",
		"--color-extractor", "median",
		"--color-space", "din99d",
		"--font-ratio", "1/2",
		"--work", "9",
		"--animate", "off",
		"--relative", "off",
		"--optimize", "0",
		"--polite", "off",
		"--size", fmt.Sprintf("%dx%d", width, height),
		path,
	}

	cmd := exec.Command(chafaPath, args...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("chafa render failed: %s", msg)
	}

	out := strings.TrimRight(stdout.String(), "\n")
	if strings.TrimSpace(out) == "" {
		return "", fmt.Errorf("chafa returned empty output")
	}

	return out, nil
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
