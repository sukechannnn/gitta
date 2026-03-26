package config

import (
	"os"
	"path/filepath"
)

// AppConfig holds the overall application configuration
type AppConfig struct {
	PatchFilePath string
}

// LoadConfig loads the application configuration
func LoadConfig() (*AppConfig, error) {
	tempDir := os.TempDir()
	// Generate an application-specific filename to avoid conflicts
	patchFileName := "giff_selected.patch"
	patchFilePath := filepath.Join(tempDir, patchFileName)

	cfg := &AppConfig{
		PatchFilePath: patchFilePath,
	}

	return cfg, nil
}
