package util

import (
	"fmt"
	"os"
)

func EnsureDir(path, purpose string) error {
	if path == "" {
		if purpose == "" {
			purpose = "directory"
		}
		return fmt.Errorf("%s path cannot be empty", purpose)
	}

	if err := os.MkdirAll(path, 0o755); err != nil {
		if purpose == "" {
			purpose = "directory"
		}
		return fmt.Errorf("create %s %q: %w", purpose, path, err)
	}

	return nil
}
