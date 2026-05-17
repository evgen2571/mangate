package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/evgen2571/mangate/internal/project"
)

const (
	envConfigPath         = "MANGATE_CONFIG"
	defaultConfigFileName = "config.json"
)

func DefaultConfigPath() string {
	if path := strings.TrimSpace(os.Getenv(envConfigPath)); path != "" {
		return path
	}

	root, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", "."+project.Name, defaultConfigFileName)
	}

	return filepath.Join(root, project.Name, defaultConfigFileName)
}

func defaultDownloadDir() string {
	root, err := os.UserHomeDir()
	if err != nil {
		return "./downloads"
	}

	return filepath.Join(root, "downloads", project.Name)
}

func defaultCacheDir() string {
	root, err := os.UserCacheDir()
	if err != nil {
		return "./.cache"
	}

	return filepath.Join(root, project.Name)
}

func defaultTempDir() string {
	return filepath.Join(os.TempDir(), project.Name)
}
