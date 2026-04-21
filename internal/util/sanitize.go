package util

import "strings"

func SanitizeString(name string) string {
	name = strings.ReplaceAll(name, "..", "_")
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, "*", "_")
	name = strings.ReplaceAll(name, "?", "_")
	name = strings.ReplaceAll(name, "\"", "_")
	name = strings.ReplaceAll(name, "<", "")
	name = strings.ReplaceAll(name, ">", "")
	name = strings.ReplaceAll(name, "|", "_")
	name = strings.ReplaceAll(name, "~", "")
	if name == "" {
		return "unknown"
	}
	return name
}
