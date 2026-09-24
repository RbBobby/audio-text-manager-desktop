package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/alexandr/audio-text-manager-desktop/internal/asr"
	"github.com/alexandr/audio-text-manager-desktop/internal/config"
	"github.com/alexandr/audio-text-manager-desktop/internal/jobs"
	"github.com/alexandr/audio-text-manager-desktop/internal/llm"
	"github.com/alexandr/audio-text-manager-desktop/internal/sidecar"
	"github.com/alexandr/audio-text-manager-desktop/internal/summary"
)

type Worker struct {
	store *jobs.Store
	cfg   *config.Config
	llm   *llm.Server

	mu      sync.Mutex
	cancel  context.CancelFunc
	current string
	stop    chan struct{}
	stopped chan struct{}
}

func New(store *jobs.Store, cfg *config.Config, llmServer *llm.Server) *Worker {
	if llmServer == nil {
		llmServer = llm.NewServer()
	}
	return &Worker{store: store, cfg: cfg, llm: llmServer, stop: make(chan struct{}), stopped: make(chan struct{})}
}

func (w *Worker) Start() {
	go w.loop()
}

func (w *Worker) Stop() {
	close(w.stop)
	w.mu.Lock()
	if w.cancel != nil {
		w.cancel()
	}
	w.mu.Unlock()
	<-w.stopped
	if w.llm != nil {
		w.llm.Stop()
	}
}

func (w *Worker) Interrupt(jobID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.current == jobID && w.cancel != nil {
		w.cancel()
	}
}

func (w *Worker) loop() {
	defer close(w.stopped)
	t := time.NewTicker(400 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-w.stop:
			return
		case <-t.C:
			id, err := w.store.ClaimNext()
			if err != nil || id == "" {
				continue
			}
			w.runJob(id)
		}
	}
}

func (w *Worker) runJob(id string) {
	ctx, cancel := context.WithCancel(context.Background())
	w.mu.Lock()
	w.cancel = cancel
	w.current = id
	w.mu.Unlock()
	defer func() {
		cancel()
		w.mu.Lock()
		if w.current == id {
			w.current = ""
			w.cancel = nil
		}
		w.mu.Unlock()
	}()

	j, err := w.store.Get(id)
	if err != nil || j.Status != "processing" {
		return
	}

	transcript := j.Transcript
	fail := func(e error) {
		latest, _ := w.store.Get(id)
		if latest != nil && latest.Status == "canceled" {
			return
		}
		st := j.Stages
		if latest != nil {
			st = latest.Stages
		}
		saved := ""
		if latest != nil {
			saved = latest.Transcript
		}
		if strings.TrimSpace(saved) == "" {
			saved = transcript
		}
		msg := e.Error()
		status := "error"
		up := jobs.Update{Status: &status, Error: &msg}
		if strings.TrimSpace(saved) != "" {
			st["summarize"] = "error"
			st["asr"] = "done"
			up.Transcript = &saved
		} else {
			st["asr"] = "error"
		}
		up.Stages = st
		_ = w.store.Update(id, up)
	}

	cfg := w.cfg.Snapshot()
	size, err := summary.ParseSize(j.SummarySize)
	if err != nil {
		fail(err)
		return
	}
	whisperModel := ""
	asrMS := 0
	if !j.SummarizeOnly {
		st := map[string]string{"upload": "done", "asr": "processing", "summarize": "pending"}
		_ = w.store.Update(id, jobs.Update{Stages: st})
		modelPath, err := asr.EnsureModel(ctx, cfg.WhisperModelsDir, j.ASRPreset)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			fail(err)
			return
		}
		t0 := time.Now()
		res, err := asr.Transcribe(ctx, sidecar.Whisper(cfg.WhisperBin), modelPath, j.AudioPath, j.ASRLanguage)
		if errors.Is(err, context.Canceled) {
			return
		}
		if err != nil {
			fail(err)
			return
		}
		asrMS = int(time.Since(t0).Milliseconds())
		transcript = res.Text
		whisperModel = res.ModelFile
		st = map[string]string{"upload": "done", "asr": "done", "summarize": "processing"}
		_ = w.store.Update(id, jobs.Update{Stages: st, Transcript: &transcript})
	} else {
		if transcript == "" {
			fail(fmt.Errorf("empty transcript for summarize-only job"))
			return
		}
		st := map[string]string{"upload": "done", "asr": "done", "summarize": "processing"}
		_ = w.store.Update(id, jobs.Update{Stages: st})
		if j.Timings != nil {
			if v, ok := j.Timings["asr_ms"].(float64); ok {
				asrMS = int(v)
			}
		}
		if j.ModelInfo != nil {
			if v, ok := j.ModelInfo["whisper_model"].(string); ok {
				whisperModel = v
			}
		}
	}

	gguf, err := llm.EnsureModel(ctx, cfg.LlamaModel, cfg.WhisperModelsDir)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		fail(err)
		return
	}
	baseURL, err := w.llm.Ensure(ctx, sidecar.Llama(cfg.LlamaBin), gguf)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		fail(err)
		return
	}
	client := summary.New(baseURL, filepath.Base(gguf))
	t1 := time.Now()
	sumText, mode, err := summary.Summarize(ctx, client, transcript, size, j.CustomPrompt)
	if errors.Is(err, context.Canceled) {
		return
	}
	if err != nil {
		fail(err)
		return
	}
	latest, _ := w.store.Get(id)
	if latest != nil && latest.Status == "canceled" {
		return
	}
	sumMS := int(time.Since(t1).Milliseconds())
	done := "done"
	falseV := false
	st := map[string]string{"upload": "done", "asr": "done", "summarize": "done"}
	_ = w.store.Update(id, jobs.Update{
		Status:        &done,
		Stages:        st,
		Transcript:    &transcript,
		Summary:       &sumText,
		Error:         jobsPtr(""),
		SummarizeOnly: &falseV,
		Timings:       map[string]any{"asr_ms": asrMS, "summarize_ms": sumMS},
		ModelInfo: map[string]any{
			"whisper_model": whisperModel,
			"asr_language":  j.ASRLanguage,
			"llm_model":     filepath.Base(gguf),
			"summary_mode":  mode,
		},
	})
	log.Printf("job %s done asr_ms=%d summarize_ms=%d", id, asrMS, sumMS)
}

func jobsPtr(s string) *string { return &s }
