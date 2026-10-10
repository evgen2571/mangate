package util

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
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
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '_'
		}

		return r
	}, name)
	name = strings.ReplaceAll(name, "_-", "_")
	name = strings.ReplaceAll(name, "-_", "_")
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}

	name = strings.Trim(name, "-_. ")
	if name == "" {
		return "unknown"
	}

	stem, _, _ := strings.Cut(strings.ToUpper(name), ".")
	reserved := stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL"
	for _, prefix := range []string{"COM", "LPT"} {
		if digit, ok := strings.CutPrefix(stem, prefix); ok && utf8.RuneCountInString(digit) == 1 && strings.ContainsAny(digit, "123456789¹²³") {
			reserved = true
		}
	}

	if reserved {
		name = "_" + name
	}
	// Leave room for archive extensions, cache hashes, and collision suffixes.
	if len(name) > 200 {
		sum := sha256.Sum256([]byte(name))
		end := 183
		for !utf8.RuneStart(name[end]) {
			end--
		}

		name = fmt.Sprintf("%s-%x", name[:end], sum[:8])
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
