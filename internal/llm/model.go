package llm

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/alexandr/audio-text-manager-desktop/internal/download"
	"github.com/alexandr/audio-text-manager-desktop/internal/sidecar"
)

const (
	DefaultModelFile = "qwen2.5-3b-instruct-q4_k_m.gguf"
	defaultModelURL  = "https://huggingface.co/Qwen/Qwen2.5-3B-Instruct-GGUF/resolve/main/qwen2.5-3b-instruct-q4_k_m.gguf"
)

// EnsureModel returns a GGUF path: bundled sidecar, explicit setting, or a download into userDir.
func EnsureModel(ctx context.Context, explicit, userDir string) (string, error) {
	if p := sidecar.FindModel(explicit, "ATM_LLAMA_MODEL", DefaultModelFile); p != "" {
		return p, nil
	}
	if userDir == "" {
		return "", fmt.Errorf("llama GGUF not found (%s). Bundle it into the app (make fetch-runtime) or set llama_model", DefaultModelFile)
	}
	dest := filepath.Join(userDir, DefaultModelFile)
	if err := download.Ensure(ctx, dest, defaultModelURL); err != nil {
		return "", fmt.Errorf("llama model: %w", err)
	}
	return dest, nil
}
