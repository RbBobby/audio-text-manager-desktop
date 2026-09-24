package appdir_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alexandr/audio-text-manager-desktop/internal/appdir"
)

func TestDirOS(t *testing.T) {
	dir, err := appdir.Dir()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dir, "AudioTextManager") {
		t.Fatalf("dir %s", dir)
	}
	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(dir, "Application Support") {
			t.Fatalf("darwin dir %s", dir)
		}
	case "windows":
		if !strings.Contains(strings.ToLower(dir), "appdata") && os.Getenv("APPDATA") == "" {
			t.Fatalf("windows dir %s", dir)
		}
	}
}

func TestEnsure(t *testing.T) {
	root, err := appdir.Ensure()
	if err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"uploads", "models"} {
		st, err := os.Stat(filepath.Join(root, sub))
		if err != nil || !st.IsDir() {
			t.Fatalf("%s: %v", sub, err)
		}
	}
	if appdir.SQLitePath(root) != filepath.Join(root, "app.db") {
		t.Fatal("sqlite path")
	}
}
