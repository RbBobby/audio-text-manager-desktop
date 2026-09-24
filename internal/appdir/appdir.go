package appdir

import (
	"os"
	"path/filepath"
	"runtime"
)

const appName = "AudioTextManager"

// Dir returns the per-user data directory (SQLite, uploads, config, whisper cache).
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", appName), nil
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, appName), nil
		}
		return filepath.Join(home, "AppData", "Roaming", appName), nil
	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, appName), nil
		}
		return filepath.Join(home, ".local", "share", appName), nil
	}
}

func Ensure() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	for _, sub := range []string{"", "uploads", "models"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return "", err
		}
	}
	return dir, nil
}

func Uploads(root string) string {
	return filepath.Join(root, "uploads")
}

func Models(root string) string {
	return filepath.Join(root, "models")
}

func SQLitePath(root string) string {
	return filepath.Join(root, "app.db")
}

func ConfigPath(root string) string {
	return filepath.Join(root, "config.json")
}
