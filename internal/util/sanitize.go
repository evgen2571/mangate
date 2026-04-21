package util

import "strings"

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
