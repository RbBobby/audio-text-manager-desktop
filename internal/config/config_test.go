package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alexandr/audio-text-manager-desktop/internal/config"
)

func TestLoadSave(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Load(dir)
	if cfg.WhisperModelsDir == "" {
		t.Fatal("default models dir")
	}
	cfg.Update(config.Config{LlamaModel: "/tmp/qwen.gguf", WhisperModelsDir: cfg.WhisperModelsDir})
	if err := cfg.Save(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err != nil {
		t.Fatal(err)
	}
	again := config.Load(dir)
	if again.LlamaModel != "/tmp/qwen.gguf" {
		t.Fatalf("model %s", again.LlamaModel)
	}
}
