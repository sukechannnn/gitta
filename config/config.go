package config

import (
	"os"
	"path/filepath"
)

// AppConfig はアプリケーション全体の設定を保持します
type AppConfig struct {
	PatchFilePath string
}

// LoadConfig はアプリケーションの設定を読み込みます
func LoadConfig() (*AppConfig, error) {
	tempDir := os.TempDir()
	// アプリケーション固有のファイル名を生成して衝突を避ける
	patchFileName := "gitta_selected.patch"
	patchFilePath := filepath.Join(tempDir, patchFileName)

	cfg := &AppConfig{
		PatchFilePath: patchFilePath,
	}

	return cfg, nil
}
