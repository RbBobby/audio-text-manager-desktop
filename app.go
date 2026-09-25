package main

import (
	"context"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/alexandr/audio-text-manager-desktop/internal/appdir"
	"github.com/alexandr/audio-text-manager-desktop/internal/asr"
	"github.com/alexandr/audio-text-manager-desktop/internal/config"
	"github.com/alexandr/audio-text-manager-desktop/internal/flavor"
	"github.com/alexandr/audio-text-manager-desktop/internal/jobs"
	"github.com/alexandr/audio-text-manager-desktop/internal/llm"
	"github.com/alexandr/audio-text-manager-desktop/internal/media"
	"github.com/alexandr/audio-text-manager-desktop/internal/pipeline"
	"github.com/alexandr/audio-text-manager-desktop/internal/sidecar"
	"github.com/alexandr/audio-text-manager-desktop/internal/summary"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx      context.Context
	dataRoot string
	cfg      *config.Config
	store    *jobs.Store
	worker   *pipeline.Worker
	llm      *llm.Server
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	root, err := appdir.Ensure()
	if err != nil {
		runtime.LogErrorf(ctx, "data dir: %v", err)
		return
	}
	a.dataRoot = root
	a.cfg = config.Load(root)
	store, err := jobs.Open(appdir.SQLitePath(root))
	if err != nil {
		runtime.LogErrorf(ctx, "sqlite: %v", err)
		return
	}
	a.store = store
	a.llm = llm.NewServer()
	a.worker = pipeline.New(store, a.cfg, a.llm)
	a.worker.Start()
}

func (a *App) shutdown(ctx context.Context) {
	if a.worker != nil {
		a.worker.Stop()
	}
	if a.store != nil {
		_ = a.store.Close()
	}
}

type JobStatus struct {
	ID           string            `json:"id"`
	Status       string            `json:"status"`
	ASRPreset    string            `json:"asr_preset"`
	ASRLanguage  string            `json:"asr_language"`
	SummarySize  string            `json:"summary_size"`
	CustomPrompt string            `json:"custom_prompt"`
	Transcript   string            `json:"transcript"`
	Stages       map[string]string `json:"stages"`
	Error        string            `json:"error"`
	CreatedAt    string            `json:"created_at"`
	UpdatedAt    string            `json:"updated_at"`
}

type JobListItem struct {
	ID               string            `json:"id"`
	Status           string            `json:"status"`
	ASRPreset        string            `json:"asr_preset"`
	ASRLanguage      string            `json:"asr_language"`
	SummarySize      string            `json:"summary_size"`
	OriginalFilename string            `json:"original_filename"`
	Stages           map[string]string `json:"stages"`
	CreatedAt        string            `json:"created_at"`
	UpdatedAt        string            `json:"updated_at"`
}

type JobList struct {
	Jobs   []JobListItem `json:"jobs"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

type CreateJobResult struct {
	JobID string `json:"job_id"`
}

type TranscriptResult struct {
	JobID      string `json:"job_id"`
	Transcript string `json:"transcript"`
}

type JobResult struct {
	JobID      string         `json:"job_id"`
	Transcript string         `json:"transcript"`
	Summary    string         `json:"summary"`
	Timings    map[string]any `json:"timings"`
	ModelInfo  map[string]any `json:"model_info"`
}

type CancelResult struct {
	Canceled bool   `json:"canceled"`
	Status   string `json:"status"`
}

type CancelActiveResult struct {
	Canceled []string `json:"canceled"`
}

type BulkDeleteResult struct {
	Deleted []string `json:"deleted"`
	Skipped []string `json:"skipped"`
}

type SettingsDTO struct {
	LlamaBin            string `json:"llama_bin"`
	LlamaModel          string `json:"llama_model"`
	MaxUploadBytes      int64  `json:"max_upload_bytes"`
	MaxVideoUploadBytes int64  `json:"max_video_upload_bytes"`
	WhisperBin          string `json:"whisper_bin"`
	FFmpegBin           string `json:"ffmpeg_bin"`
	FFprobeBin          string `json:"ffprobe_bin"`
	WhisperModelsDir    string `json:"whisper_models_dir"`
	MaxAudioDurationSec int    `json:"max_audio_duration_sec"`
	DataDir             string `json:"data_dir"`
	ResolvedFFmpeg      string `json:"resolved_ffmpeg"`
	ResolvedFFprobe     string `json:"resolved_ffprobe"`
	ResolvedWhisper     string `json:"resolved_whisper"`
	ResolvedLlama       string `json:"resolved_llama"`
	ResolvedLLMModel    string `json:"resolved_llm_model"`
}

type PingResult struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

func toStatus(j *jobs.Job) JobStatus {
	return JobStatus{
		ID: j.ID, Status: j.Status, ASRPreset: j.ASRPreset, ASRLanguage: j.ASRLanguage,
		SummarySize: j.SummarySize, CustomPrompt: j.CustomPrompt, Transcript: j.Transcript,
		Stages: j.Stages, Error: j.Error,
		CreatedAt: j.CreatedAt, UpdatedAt: j.UpdatedAt,
	}
}

func (a *App) ready() error {
	if a.store == nil || a.cfg == nil {
		return fmt.Errorf("application is not ready")
	}
	return nil
}

func (a *App) SelectAudioFile() (string, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Выберите аудио или видео",
		Filters: []runtime.FileFilter{{
			DisplayName: "Audio / Video (wav, mp3, m4a, flac, ogg, mp4)",
			Pattern:     "*.wav;*.mp3;*.m4a;*.flac;*.ogg;*.mp4",
		}},
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

type SaveExportResult struct {
	Saved bool   `json:"saved"`
	Path  string `json:"path"`
}

func (a *App) SaveExport(suggestedName, format, text string) (*SaveExportResult, error) {
	if a.ctx == nil {
		return nil, fmt.Errorf("application is not ready")
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format != "txt" && format != "doc" {
		return nil, fmt.Errorf("unknown export format %q", format)
	}
	suggestedName = sanitizeExportName(suggestedName, format)
	filter := runtime.FileFilter{DisplayName: "Текст (*.txt)", Pattern: "*.txt"}
	title := "Сохранить текст"
	if format == "doc" {
		filter = runtime.FileFilter{DisplayName: "Word (*.doc)", Pattern: "*.doc"}
		title = "Сохранить документ Word"
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:                title,
		DefaultFilename:      suggestedName,
		Filters:              []runtime.FileFilter{filter},
		CanCreateDirectories: true,
	})
	if err != nil {
		return nil, err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return &SaveExportResult{Saved: false}, nil
	}
	if filepath.Ext(path) == "" {
		path += "." + format
	}
	var data []byte
	if format == "txt" {
		data = []byte(text)
	} else {
		data = wordDocBytes(suggestedName, text)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return nil, err
	}
	return &SaveExportResult{Saved: true, Path: path}, nil
}

func sanitizeExportName(name, format string) string {
	name = filepath.Base(strings.TrimSpace(name))
	var b strings.Builder
	for _, r := range name {
		switch {
		case r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|':
			b.WriteByte('_')
		case unicode.IsControl(r):
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	name = strings.TrimSpace(b.String())
	if name == "" || name == "." {
		name = "export"
	}
	if !strings.HasSuffix(strings.ToLower(name), "."+format) {
		name = strings.TrimSuffix(name, filepath.Ext(name)) + "." + format
	}
	if len(name) > 120 {
		name = name[:120]
	}
	return name
}

func wordDocBytes(title, text string) []byte {
	var b strings.Builder
	b.WriteString("\ufeff<!DOCTYPE html><html><head><meta charset=\"utf-8\"><title>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</title></head><body>")
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		b.WriteString("<p>")
		if line == "" {
			b.WriteString("<br/>")
		} else {
			b.WriteString(html.EscapeString(line))
		}
		b.WriteString("</p>")
	}
	b.WriteString("</body></html>")
	return []byte(b.String())
}

func (a *App) CreateJob(path, asrPreset, summarySize, customPrompt, language string) (*CreateJobResult, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("file is not selected")
	}
	if !media.IsAllowed(path) {
		return nil, fmt.Errorf("unsupported format %s", media.Ext(path))
	}
	if _, err := asr.ModelFile(asrPreset); err != nil {
		return nil, err
	}
	if !flavor.Allows(asrPreset) {
		return nil, fmt.Errorf("asr preset %q is not in this build", asrPreset)
	}
	lang, err := asr.NormalizeLanguage(language)
	if err != nil {
		return nil, err
	}
	size, err := summary.ParseSize(summarySize)
	if err != nil {
		return nil, err
	}
	cfg := a.cfg.Snapshot()
	id := jobs.NewID()
	wav, orig, err := media.PrepareUpload(
		sidecar.FFmpeg(cfg.FFmpegBin),
		sidecar.FFprobe(cfg.FFprobeBin),
		path,
		appdir.Uploads(a.dataRoot),
		id,
		cfg.MaxUploadBytes,
		cfg.MaxVideoUploadBytes,
		cfg.MaxAudioDurationSec,
	)
	if err != nil {
		return nil, err
	}
	j := &jobs.Job{
		ID:               id,
		Status:           "queued",
		ASRPreset:        asrPreset,
		ASRLanguage:      lang,
		SummarySize:      size,
		OriginalFilename: orig,
		AudioPath:        wav,
		CustomPrompt:     strings.TrimSpace(customPrompt),
		Stages:           map[string]string{"upload": "done", "asr": "pending", "summarize": "pending"},
	}
	if err := a.store.Create(j); err != nil {
		_ = os.Remove(wav)
		return nil, err
	}
	return &CreateJobResult{JobID: id}, nil
}

func (a *App) ListJobs(limit, offset int) (*JobList, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	rows, err := a.store.List(limit, offset)
	if err != nil {
		return nil, err
	}
	out := &JobList{Jobs: []JobListItem{}, Limit: limit, Offset: offset}
	if out.Limit <= 0 {
		out.Limit = 50
	}
	for _, j := range rows {
		out.Jobs = append(out.Jobs, JobListItem{
			ID: j.ID, Status: j.Status, ASRPreset: j.ASRPreset, ASRLanguage: j.ASRLanguage, SummarySize: j.SummarySize,
			OriginalFilename: j.OriginalFilename, Stages: j.Stages,
			CreatedAt: j.CreatedAt, UpdatedAt: j.UpdatedAt,
		})
	}
	return out, nil
}

func (a *App) GetJob(id string) (*JobStatus, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	j, err := a.store.Get(id)
	if err != nil {
		return nil, err
	}
	st := toStatus(j)
	return &st, nil
}

func (a *App) GetTranscript(id string) (*TranscriptResult, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	j, err := a.store.Get(id)
	if err != nil {
		return nil, err
	}
	tr := strings.TrimSpace(j.Transcript)
	if tr == "" {
		if j.Status == "error" || j.Status == "canceled" {
			if j.Error != "" {
				return nil, fmt.Errorf("%s", j.Error)
			}
			return nil, fmt.Errorf("no transcript")
		}
		return nil, fmt.Errorf("transcript is not ready")
	}
	return &TranscriptResult{JobID: id, Transcript: j.Transcript}, nil
}

func (a *App) GetResult(id string) (*JobResult, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	j, err := a.store.Get(id)
	if err != nil {
		return nil, err
	}
	if j.Status != "done" {
		return nil, fmt.Errorf("job is not finished")
	}
	return &JobResult{
		JobID: id, Transcript: j.Transcript, Summary: j.Summary,
		Timings: j.Timings, ModelInfo: j.ModelInfo,
	}, nil
}

func (a *App) CancelJob(id string) (*CancelResult, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	status, ok, err := a.store.Cancel(id)
	if err != nil {
		return nil, err
	}
	if ok && a.worker != nil {
		a.worker.Interrupt(id)
	}
	return &CancelResult{Canceled: ok, Status: status}, nil
}

func (a *App) CancelActive() (*CancelActiveResult, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	ids, err := a.store.CancelActive()
	if err != nil {
		return nil, err
	}
	if a.worker != nil {
		for _, id := range ids {
			a.worker.Interrupt(id)
		}
	}
	if ids == nil {
		ids = []string{}
	}
	return &CancelActiveResult{Canceled: ids}, nil
}

func (a *App) BulkDelete(ids []string) (*BulkDeleteResult, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	deleted, skipped, err := a.store.Delete(ids)
	if err != nil {
		return nil, err
	}
	if deleted == nil {
		deleted = []string{}
	}
	if skipped == nil {
		skipped = []string{}
	}
	return &BulkDeleteResult{Deleted: deleted, Skipped: skipped}, nil
}

func (a *App) Requeue(id, asrPreset, summarySize, customPrompt, language string) (*CreateJobResult, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if _, err := asr.ModelFile(asrPreset); err != nil {
		return nil, err
	}
	if !flavor.Allows(asrPreset) {
		return nil, fmt.Errorf("asr preset %q is not in this build", asrPreset)
	}
	lang, err := asr.NormalizeLanguage(language)
	if err != nil {
		return nil, err
	}
	size, err := summary.ParseSize(summarySize)
	if err != nil {
		return nil, err
	}
	if err := a.store.Requeue(id, asrPreset, lang, size, strings.TrimSpace(customPrompt)); err != nil {
		return nil, err
	}
	return &CreateJobResult{JobID: id}, nil
}

func (a *App) SummarizeOnly(id, summarySize, customPrompt string) (*CreateJobResult, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	size, err := summary.ParseSize(summarySize)
	if err != nil {
		return nil, err
	}
	if err := a.store.QueueSummarizeOnly(id, size, strings.TrimSpace(customPrompt)); err != nil {
		return nil, err
	}
	return &CreateJobResult{JobID: id}, nil
}

func (a *App) GetASRConfig() flavor.Config {
	return flavor.UI()
}

func (a *App) GetSettings() (*SettingsDTO, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	cfg := a.cfg.Snapshot()
	return &SettingsDTO{
		LlamaBin:            cfg.LlamaBin,
		LlamaModel:          cfg.LlamaModel,
		MaxUploadBytes:      cfg.MaxUploadBytes,
		MaxVideoUploadBytes: cfg.MaxVideoUploadBytes,
		WhisperBin:          cfg.WhisperBin,
		FFmpegBin:           cfg.FFmpegBin,
		FFprobeBin:          cfg.FFprobeBin,
		WhisperModelsDir:    cfg.WhisperModelsDir,
		MaxAudioDurationSec: cfg.MaxAudioDurationSec,
		DataDir:             a.dataRoot,
		ResolvedFFmpeg:      sidecar.FFmpeg(cfg.FFmpegBin),
		ResolvedFFprobe:     sidecar.FFprobe(cfg.FFprobeBin),
		ResolvedWhisper:     sidecar.Whisper(cfg.WhisperBin),
		ResolvedLlama:       sidecar.Llama(cfg.LlamaBin),
		ResolvedLLMModel:    sidecar.FindModel(cfg.LlamaModel, "ATM_LLAMA_MODEL", llm.DefaultModelFile),
	}, nil
}

func (a *App) SaveSettings(next SettingsDTO) (*SettingsDTO, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	a.cfg.Update(config.Config{
		LlamaBin:            strings.TrimSpace(next.LlamaBin),
		LlamaModel:          strings.TrimSpace(next.LlamaModel),
		MaxUploadBytes:      next.MaxUploadBytes,
		MaxVideoUploadBytes: next.MaxVideoUploadBytes,
		WhisperBin:          strings.TrimSpace(next.WhisperBin),
		FFmpegBin:           strings.TrimSpace(next.FFmpegBin),
		FFprobeBin:          strings.TrimSpace(next.FFprobeBin),
		WhisperModelsDir:    strings.TrimSpace(next.WhisperModelsDir),
		MaxAudioDurationSec: next.MaxAudioDurationSec,
	})
	if err := a.cfg.Save(a.dataRoot); err != nil {
		return nil, err
	}
	return a.GetSettings()
}

func (a *App) PingRuntime() (*PingResult, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	cfg := a.cfg.Snapshot()
	if sidecar.FFmpeg(cfg.FFmpegBin) == "" {
		return &PingResult{OK: false, Error: "ffmpeg не найден. Соберите бандл: make dist"}, nil
	}
	if sidecar.FFprobe(cfg.FFprobeBin) == "" {
		return &PingResult{OK: false, Error: "ffprobe не найден. Соберите бандл: make dist"}, nil
	}
	if sidecar.Whisper(cfg.WhisperBin) == "" {
		return &PingResult{OK: false, Error: "whisper-cli не найден. Соберите бандл: make dist"}, nil
	}
	llamaBin := sidecar.Llama(cfg.LlamaBin)
	if llamaBin == "" {
		return &PingResult{OK: false, Error: "llama-server не найден. Соберите бандл: make dist"}, nil
	}
	gguf, err := llm.EnsureModel(a.ctx, cfg.LlamaModel, cfg.WhisperModelsDir)
	if err != nil {
		return &PingResult{OK: false, Error: err.Error()}, nil
	}
	if a.llm == nil {
		a.llm = llm.NewServer()
	}
	baseURL, err := a.llm.Ensure(a.ctx, llamaBin, gguf)
	if err != nil {
		return &PingResult{OK: false, Error: err.Error()}, nil
	}
	client := summary.New(baseURL, "")
	if err := client.Ping(a.ctx); err != nil {
		return &PingResult{OK: false, Error: err.Error()}, nil
	}
	return &PingResult{OK: true}, nil
}

func (a *App) PingOllama() (*PingResult, error) {
	return a.PingRuntime()
}

func (a *App) DataDir() string {
	return a.dataRoot
}
