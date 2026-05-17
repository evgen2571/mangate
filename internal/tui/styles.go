package tui

import (
	_ "embed"

	"github.com/charmbracelet/lipgloss"
)

const (
	charLimit  = 35
	inputWidth = 40

	logoColor        = lipgloss.Color("#FF5DB1")
	outerBorderColor = lipgloss.Color("#E6E6F0")
	inputBorderColor = lipgloss.Color("#CFCFE6")
	mutedColor       = lipgloss.Color("#7F8496")
	textColor        = lipgloss.Color("#FFFFFF")
)

//go:embed logo.txt
var asciiLogo string
