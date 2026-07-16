package util

import (
	"strings"
	"unicode"
)

func SanitizeString(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "..", "_")
	name = strings.ReplaceAll(name, "~", "")
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, "*", "_")
	name = strings.ReplaceAll(name, "?", "_")
	name = strings.ReplaceAll(name, "\"", "_")
	name = strings.ReplaceAll(name, "<", "")
	name = strings.ReplaceAll(name, ">", "")
	name = strings.ReplaceAll(name, "|", "_")

	name = strings.Join(strings.Fields(name), "-")
	name = strings.ReplaceAll(name, "_-", "_")
	name = strings.ReplaceAll(name, "-_", "_")
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}
	name = strings.Trim(name, "-_")
	if name == "" {
		return "unknown"
	}
	return name
}

// SanitizeTerminalText makes untrusted display text safe for a terminal. It
// replaces all control characters, including escape, so metadata cannot alter
// terminal state or forge additional output lines.
func SanitizeTerminalText(text string) string {
	return strings.Map(func(character rune) rune {
		if unicode.IsControl(character) {
			return '�'
		}
		return character
	}, text)
}
