package asr

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/alexandr/audio-text-manager-desktop/internal/download"
	"github.com/alexandr/audio-text-manager-desktop/internal/sidecar"
)

var modelURLs = map[string]string{
	"ggml-small-q5_1.bin":  "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small-q5_1.bin",
	"ggml-medium-q5_0.bin": "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium-q5_0.bin",
}

// EnsureModel returns ggml weights from the app bundle, then the user cache, else downloads.
func EnsureModel(ctx context.Context, modelsDir, preset string) (string, error) {
	name, err := ModelFile(preset)
	if err != nil {
		return "", err
	}
	if p := sidecar.FindModel("", "", name); p != "" {
		return p, nil
	}
	if modelsDir == "" {
		return "", fmt.Errorf("whisper model %s not found; bundle it (make fetch-runtime) or set whisper_models_dir", name)
	}
	dest := filepath.Join(modelsDir, name)
	url, ok := modelURLs[name]
	if !ok {
		return "", fmt.Errorf("no download URL for %s", name)
	}
	if err := download.Ensure(ctx, dest, url); err != nil {
		return "", err
	}
	return dest, nil
}
