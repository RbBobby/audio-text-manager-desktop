package jobs_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/alexandr/audio-text-manager-desktop/internal/jobs"

	_ "modernc.org/sqlite"
)

func TestCreateGetRequeueSummarizeOnly(t *testing.T) {
	dir := t.TempDir()
	store, err := jobs.Open(filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	audio := filepath.Join(dir, "a.wav")
	if err := os.WriteFile(audio, []byte("RIFF"), 0o644); err != nil {
		t.Fatal(err)
	}
	id := jobs.NewID()
	j := &jobs.Job{
		ID:               id,
		Status:           "done",
		ASRPreset:        "fast",
		SummarySize:      "gist",
		OriginalFilename: "a.wav",
		AudioPath:        audio,
		Transcript:       "hello world",
		Stages:           map[string]string{"upload": "done", "asr": "done", "summarize": "done"},
	}
	if err := store.Create(j); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Transcript != "hello world" {
		t.Fatalf("transcript %q", got.Transcript)
	}

	if err := store.QueueSummarizeOnly(id, "executive", "focus on risks"); err != nil {
		t.Fatal(err)
	}
	got, err = store.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if !got.SummarizeOnly || got.Status != "queued" {
		t.Fatalf("summarize-only: %+v", got)
	}
	if got.Stages["asr"] != "done" {
		t.Fatalf("asr stage should stay done, got %s", got.Stages["asr"])
	}
	if got.Transcript == "" {
		t.Fatal("transcript must be kept")
	}

	if got.ASRLanguage != "ru" {
		t.Fatalf("default language %q", got.ASRLanguage)
	}

	if err := store.Requeue(id, "medium", "en", "gist", ""); err != nil {
		t.Fatal(err)
	}
	got, err = store.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.ASRLanguage != "en" {
		t.Fatalf("requeue language %q", got.ASRLanguage)
	}
	if got.SummarizeOnly {
		t.Fatal("requeue must clear summarize_only")
	}
	if got.Transcript != "" {
		t.Fatal("requeue must clear transcript")
	}
	if got.Stages["asr"] != "pending" {
		t.Fatalf("asr pending, got %s", got.Stages["asr"])
	}
}

func TestCancelActive(t *testing.T) {
	dir := t.TempDir()
	store, err := jobs.Open(filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	id := jobs.NewID()
	if err := store.Create(&jobs.Job{
		ID: id, Status: "queued", ASRPreset: "fast", SummarySize: "gist",
		OriginalFilename: "a.wav", AudioPath: filepath.Join(dir, "a.wav"),
		Stages: map[string]string{"upload": "done", "asr": "pending", "summarize": "pending"},
	}); err != nil {
		t.Fatal(err)
	}
	canceled, err := store.CancelActive()
	if err != nil {
		t.Fatal(err)
	}
	if len(canceled) != 1 || canceled[0] != id {
		t.Fatalf("canceled %v", canceled)
	}
	got, _ := store.Get(id)
	if got.Status != "canceled" {
		t.Fatalf("status %s", got.Status)
	}
}

func TestClaimNext(t *testing.T) {
	dir := t.TempDir()
	store, err := jobs.Open(filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	id := jobs.NewID()
	if err := store.Create(&jobs.Job{
		ID: id, Status: "queued", ASRPreset: "fast", SummarySize: "gist",
		OriginalFilename: "a.wav", AudioPath: filepath.Join(dir, "a.wav"),
		Stages: map[string]string{"upload": "done", "asr": "pending", "summarize": "pending"},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := store.ClaimNext()
	if err != nil {
		t.Fatal(err)
	}
	if got != id {
		t.Fatalf("claim %q", got)
	}
	row, err := store.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != "processing" {
		t.Fatalf("status %s", row.Status)
	}
	again, err := store.ClaimNext()
	if err != nil {
		t.Fatal(err)
	}
	if again != "" {
		t.Fatal("second claim should be empty")
	}
}

func TestDeleteSkipsProcessing(t *testing.T) {
	dir := t.TempDir()
	store, err := jobs.Open(filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	audio := filepath.Join(dir, "a.wav")
	if err := os.WriteFile(audio, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	active := jobs.NewID()
	done := jobs.NewID()
	if err := store.Create(&jobs.Job{
		ID: active, Status: "processing", ASRPreset: "fast", SummarySize: "gist",
		OriginalFilename: "a.wav", AudioPath: audio,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(&jobs.Job{
		ID: done, Status: "done", ASRPreset: "fast", SummarySize: "gist",
		OriginalFilename: "b.wav", AudioPath: audio,
	}); err != nil {
		t.Fatal(err)
	}
	deleted, skipped, err := store.Delete([]string{active, done})
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 1 || deleted[0] != done {
		t.Fatalf("deleted %v", deleted)
	}
	if len(skipped) != 1 || skipped[0] != active {
		t.Fatalf("skipped %v", skipped)
	}
}

func TestMigrateASRLanguage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
CREATE TABLE jobs (
  id TEXT PRIMARY KEY,
  status TEXT NOT NULL,
  asr_preset TEXT NOT NULL,
  summary_size TEXT NOT NULL,
  original_filename TEXT,
  audio_path TEXT NOT NULL,
  error_message TEXT,
  transcript TEXT,
  summary TEXT,
  timings_json TEXT,
  stages_json TEXT NOT NULL,
  model_info_json TEXT,
  custom_prompt TEXT,
  summarize_only INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
INSERT INTO jobs (id, status, asr_preset, summary_size, audio_path, stages_json)
VALUES ('old1', 'done', 'fast', 'gist', 'a.wav', '{"upload":"done","asr":"done","summarize":"done"}');
`)
	_ = db.Close()
	if err != nil {
		t.Fatal(err)
	}

	store, err := jobs.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	got, err := store.Get("old1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ASRLanguage != "ru" {
		t.Fatalf("migrated language %q", got.ASRLanguage)
	}
}
