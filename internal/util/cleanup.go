package util

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func CleanupTemp(root string, olderThan time.Duration) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	cutoff := time.Now().Add(-olderThan)
	var cleanupErr error

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, "mangate-") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}

		fullPath := filepath.Join(root, name)
		if err := os.RemoveAll(fullPath); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}

	return cleanupErr
}
