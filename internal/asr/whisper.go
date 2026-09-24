package asr

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alexandr/audio-text-manager-desktop/internal/sidecar"
)

var Presets = map[string]string{
	"fast":   "ggml-small-q5_1.bin",
	"medium": "ggml-medium-q5_0.bin",
}

func ModelFile(preset string) (string, error) {
	name, ok := Presets[preset]
	if !ok {
		return "", fmt.Errorf("unknown asr preset %q (fast|medium)", preset)
	}
	return name, nil
}

func resolveBin(explicit string) (string, error) {
	candidates := []string{explicit, os.Getenv("ATM_WHISPER_BIN"), "whisper-cli", "whisper", "main"}
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if p, err := exec.LookPath(c); err == nil {
			return p, nil
		}
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("whisper.cpp CLI not found (whisper-cli). Bundle it with make fetch-runtime, or set whisper_bin")
}

type Result struct {
	Text      string
	ModelFile string
}

// Transcribe runs whisper.cpp CLI against an already-resolved ggml model path.
func Transcribe(ctx context.Context, bin, modelPath, wavPath, language string) (*Result, error) {
	cli, err := resolveBin(bin)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("whisper model missing: %s", modelPath)
	}
	lang, err := NormalizeLanguage(language)
	if err != nil {
		return nil, err
	}
	outBase := wavPath + ".asr"
	cmd := exec.CommandContext(ctx, cli,
		"-m", modelPath,
		"-f", wavPath,
		"-l", lang,
		"-nt",
		"-otxt",
		"-of", outBase,
	)
	sidecar.Prep(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("whisper.cpp failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	txt := outBase + ".txt"
	raw, err := os.ReadFile(txt)
	if err != nil {
		return nil, fmt.Errorf("whisper.cpp produced no transcript file: %w", err)
	}
	_ = os.Remove(txt)
	return &Result{Text: strings.TrimSpace(string(raw)), ModelFile: filepath.Base(modelPath)}, nil
}
