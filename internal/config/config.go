package config

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/alexandr/audio-text-manager-desktop/internal/appdir"
)

type Config struct {
	mu sync.RWMutex

	LlamaBin            string `json:"llama_bin"`
	LlamaModel          string `json:"llama_model"`
	MaxUploadBytes      int64  `json:"max_upload_bytes"`
	MaxVideoUploadBytes int64  `json:"max_video_upload_bytes"`
	WhisperBin          string `json:"whisper_bin"`
	FFmpegBin           string `json:"ffmpeg_bin"`
	FFprobeBin          string `json:"ffprobe_bin"`
	WhisperModelsDir    string `json:"whisper_models_dir"`
	MaxAudioDurationSec int    `json:"max_audio_duration_sec"`
}

func Defaults(dataRoot string) *Config {
	return &Config{
		MaxUploadBytes:      500 * 1024 * 1024,
		MaxVideoUploadBytes: 4 * 1024 * 1024 * 1024,
		WhisperModelsDir:    appdir.Models(dataRoot),
	}
}

func Load(dataRoot string) *Config {
	cfg := Defaults(dataRoot)
	raw, err := os.ReadFile(appdir.ConfigPath(dataRoot))
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(raw, cfg)
	if cfg.MaxUploadBytes <= 0 {
		cfg.MaxUploadBytes = 500 * 1024 * 1024
	}
	if cfg.MaxVideoUploadBytes <= 0 {
		cfg.MaxVideoUploadBytes = 4 * 1024 * 1024 * 1024
	}
	if cfg.WhisperModelsDir == "" {
		cfg.WhisperModelsDir = appdir.Models(dataRoot)
	}
	return cfg
}

func (c *Config) Snapshot() Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return Config{
		LlamaBin:            c.LlamaBin,
		LlamaModel:          c.LlamaModel,
		MaxUploadBytes:      c.MaxUploadBytes,
		MaxVideoUploadBytes: c.MaxVideoUploadBytes,
		WhisperBin:          c.WhisperBin,
		FFmpegBin:           c.FFmpegBin,
		FFprobeBin:          c.FFprobeBin,
		WhisperModelsDir:    c.WhisperModelsDir,
		MaxAudioDurationSec: c.MaxAudioDurationSec,
	}
}

func (c *Config) Update(next Config) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if next.MaxUploadBytes > 0 {
		c.MaxUploadBytes = next.MaxUploadBytes
	}
	if next.MaxVideoUploadBytes > 0 {
		c.MaxVideoUploadBytes = next.MaxVideoUploadBytes
	}
	c.LlamaBin = next.LlamaBin
	c.LlamaModel = next.LlamaModel
	c.WhisperBin = next.WhisperBin
	c.FFmpegBin = next.FFmpegBin
	c.FFprobeBin = next.FFprobeBin
	if next.WhisperModelsDir != "" {
		c.WhisperModelsDir = next.WhisperModelsDir
	}
	c.MaxAudioDurationSec = next.MaxAudioDurationSec
}

func (c *Config) Save(dataRoot string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	raw, err := json.MarshalIndent(struct {
		LlamaBin            string `json:"llama_bin"`
		LlamaModel          string `json:"llama_model"`
		MaxUploadBytes      int64  `json:"max_upload_bytes"`
		MaxVideoUploadBytes int64  `json:"max_video_upload_bytes"`
		WhisperBin          string `json:"whisper_bin"`
		FFmpegBin           string `json:"ffmpeg_bin"`
		FFprobeBin          string `json:"ffprobe_bin"`
		WhisperModelsDir    string `json:"whisper_models_dir"`
		MaxAudioDurationSec int    `json:"max_audio_duration_sec"`
	}{
		LlamaBin:            c.LlamaBin,
		LlamaModel:          c.LlamaModel,
		MaxUploadBytes:      c.MaxUploadBytes,
		MaxVideoUploadBytes: c.MaxVideoUploadBytes,
		WhisperBin:          c.WhisperBin,
		FFmpegBin:           c.FFmpegBin,
		FFprobeBin:          c.FFprobeBin,
		WhisperModelsDir:    c.WhisperModelsDir,
		MaxAudioDurationSec: c.MaxAudioDurationSec,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(appdir.ConfigPath(dataRoot), raw, 0o644)
}
