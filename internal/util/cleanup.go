package util

import (
	"os"
	"path/filepath"
	"strings"
)

func CleanupTemp(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "mangate-") {
			continue
		}

		fullPath := filepath.Join(root, name)

		_ = os.RemoveAll(fullPath)
	}

	return nil
}
