package jobs

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("job not found")

type Job struct {
	ID               string            `json:"id"`
	Status           string            `json:"status"`
	ASRPreset        string            `json:"asr_preset"`
	ASRLanguage      string            `json:"asr_language"`
	SummarySize      string            `json:"summary_size"`
	OriginalFilename string            `json:"original_filename"`
	AudioPath        string            `json:"audio_path"`
	Error            string            `json:"error"`
	Transcript       string            `json:"transcript"`
	Summary          string            `json:"summary"`
	Stages           map[string]string `json:"stages"`
	CustomPrompt     string            `json:"custom_prompt"`
	SummarizeOnly    bool              `json:"summarize_only"`
	CreatedAt        string            `json:"created_at"`
	UpdatedAt        string            `json:"updated_at"`
	Timings          map[string]any    `json:"timings"`
	ModelInfo        map[string]any    `json:"model_info"`
}

type Store struct {
	db *sql.DB
}

func Open(sqlitePath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(sqlitePath), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", sqlitePath+"?_pragma=busy_timeout(60000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) init() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS jobs (
  id TEXT PRIMARY KEY,
  status TEXT NOT NULL,
  asr_preset TEXT NOT NULL,
  asr_language TEXT NOT NULL DEFAULT 'ru',
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
CREATE INDEX IF NOT EXISTS idx_jobs_status_created ON jobs(status, created_at);
`)
	if err != nil {
		return err
	}
	return s.migrateASRLanguage()
}

func (s *Store) migrateASRLanguage() error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('jobs') WHERE name='asr_language'`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err := s.db.Exec(`ALTER TABLE jobs ADD COLUMN asr_language TEXT NOT NULL DEFAULT 'ru'`)
	return err
}

func defaultStages() map[string]string {
	return map[string]string{"upload": "done", "asr": "pending", "summarize": "pending"}
}

func encodeJSON(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func decodeMap(s string) map[string]string {
	out := map[string]string{}
	if strings.TrimSpace(s) == "" {
		return defaultStages()
	}
	_ = json.Unmarshal([]byte(s), &out)
	if len(out) == 0 {
		return defaultStages()
	}
	return out
}

func decodeAnyMap(s string) map[string]any {
	out := map[string]any{}
	if strings.TrimSpace(s) == "" {
		return out
	}
	_ = json.Unmarshal([]byte(s), &out)
	return out
}

func scanJob(row interface{ Scan(dest ...any) error }) (*Job, error) {
	var j Job
	var errMsg, transcript, summary, timings, stages, modelInfo, prompt sql.NullString
	var filename sql.NullString
	var summarizeOnly int
	if err := row.Scan(
		&j.ID, &j.Status, &j.ASRPreset, &j.ASRLanguage, &j.SummarySize, &filename, &j.AudioPath,
		&errMsg, &transcript, &summary, &timings, &stages, &modelInfo, &prompt,
		&summarizeOnly, &j.CreatedAt, &j.UpdatedAt,
	); err != nil {
		return nil, err
	}
	j.OriginalFilename = filename.String
	j.Error = errMsg.String
	j.Transcript = transcript.String
	j.Summary = summary.String
	j.CustomPrompt = prompt.String
	j.SummarizeOnly = summarizeOnly != 0
	if strings.TrimSpace(j.ASRLanguage) == "" {
		j.ASRLanguage = "ru"
	}
	j.Stages = decodeMap(stages.String)
	j.Timings = decodeAnyMap(timings.String)
	j.ModelInfo = decodeAnyMap(modelInfo.String)
	return &j, nil
}

const jobCols = `id, status, asr_preset, asr_language, summary_size, original_filename, audio_path,
error_message, transcript, summary, timings_json, stages_json, model_info_json,
custom_prompt, summarize_only, created_at, updated_at`

func (s *Store) Create(j *Job) error {
	if j.Stages == nil {
		j.Stages = defaultStages()
	}
	if j.Status == "" {
		j.Status = "queued"
	}
	if strings.TrimSpace(j.ASRLanguage) == "" {
		j.ASRLanguage = "ru"
	}
	_, err := s.db.Exec(`
INSERT INTO jobs (id, status, asr_preset, asr_language, summary_size, original_filename, audio_path,
  error_message, transcript, summary, timings_json, stages_json, model_info_json,
  custom_prompt, summarize_only)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.ID, j.Status, j.ASRPreset, j.ASRLanguage, j.SummarySize, j.OriginalFilename, j.AudioPath,
		nullStr(j.Error), nullStr(j.Transcript), nullStr(j.Summary),
		nullStr(encodeJSON(j.Timings)), encodeJSON(j.Stages), nullStr(encodeJSON(j.ModelInfo)),
		nullStr(j.CustomPrompt), boolInt(j.SummarizeOnly),
	)
	return err
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *Store) Get(id string) (*Job, error) {
	row := s.db.QueryRow(`SELECT `+jobCols+` FROM jobs WHERE id = ?`, id)
	j, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return j, err
}

func (s *Store) List(limit, offset int) ([]*Job, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.Query(`SELECT `+jobCols+` FROM jobs ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (s *Store) ClaimNext() (string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()
	var id string
	err = tx.QueryRow(`SELECT id FROM jobs WHERE status = 'queued' ORDER BY created_at ASC LIMIT 1`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	res, err := tx.Exec(`UPDATE jobs SET status = 'processing', updated_at = datetime('now') WHERE id = ? AND status = 'queued'`, id)
	if err != nil {
		return "", err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return "", nil
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
}

type Update struct {
	Status          *string
	Stages          map[string]string
	Transcript      *string
	Summary         *string
	Error           *string
	Timings         map[string]any
	ModelInfo       map[string]any
	SummarizeOnly   *bool
	ASRPreset       *string
	ASRLanguage     *string
	SummarySize     *string
	CustomPrompt    *string
	ClearTranscript bool
	ClearSummary    bool
}

func (s *Store) Update(id string, u Update) error {
	j, err := s.Get(id)
	if err != nil {
		return err
	}
	status := j.Status
	if u.Status != nil {
		status = *u.Status
	}
	stages := j.Stages
	if u.Stages != nil {
		stages = u.Stages
	}
	transcript := j.Transcript
	if u.ClearTranscript {
		transcript = ""
	} else if u.Transcript != nil {
		transcript = *u.Transcript
	}
	summary := j.Summary
	if u.ClearSummary {
		summary = ""
	} else if u.Summary != nil {
		summary = *u.Summary
	}
	errMsg := j.Error
	if u.Error != nil {
		errMsg = *u.Error
	}
	timings := j.Timings
	if u.Timings != nil {
		timings = u.Timings
	}
	modelInfo := j.ModelInfo
	if u.ModelInfo != nil {
		modelInfo = u.ModelInfo
	}
	sumOnly := j.SummarizeOnly
	if u.SummarizeOnly != nil {
		sumOnly = *u.SummarizeOnly
	}
	asr := j.ASRPreset
	if u.ASRPreset != nil {
		asr = *u.ASRPreset
	}
	lang := j.ASRLanguage
	if u.ASRLanguage != nil {
		lang = *u.ASRLanguage
	}
	if strings.TrimSpace(lang) == "" {
		lang = "ru"
	}
	size := j.SummarySize
	if u.SummarySize != nil {
		size = *u.SummarySize
	}
	prompt := j.CustomPrompt
	if u.CustomPrompt != nil {
		prompt = *u.CustomPrompt
	}
	_, err = s.db.Exec(`
UPDATE jobs SET status=?, asr_preset=?, asr_language=?, summary_size=?, error_message=?, transcript=?, summary=?,
  timings_json=?, stages_json=?, model_info_json=?, custom_prompt=?, summarize_only=?,
  updated_at=datetime('now') WHERE id=?`,
		status, asr, lang, size, nullStr(errMsg), nullStr(transcript), nullStr(summary),
		nullStr(encodeJSON(timings)), encodeJSON(stages), nullStr(encodeJSON(modelInfo)),
		nullStr(prompt), boolInt(sumOnly), id,
	)
	return err
}

func (s *Store) Cancel(id string) (string, bool, error) {
	j, err := s.Get(id)
	if err != nil {
		return "", false, err
	}
	if j.Status != "queued" && j.Status != "processing" {
		return j.Status, false, nil
	}
	st := j.Stages
	if st["asr"] == "processing" {
		st["asr"] = "canceled"
	}
	if st["summarize"] == "processing" {
		st["summarize"] = "canceled"
	}
	canceled := "canceled"
	if err := s.Update(id, Update{Status: &canceled, Stages: st, Error: ptr("")}); err != nil {
		return j.Status, false, err
	}
	return "canceled", true, nil
}

func (s *Store) CancelActive() ([]string, error) {
	rows, err := s.db.Query(`SELECT id FROM jobs WHERE status IN ('queued','processing')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	var canceled []string
	for _, id := range ids {
		if _, ok, err := s.Cancel(id); err == nil && ok {
			canceled = append(canceled, id)
		}
	}
	return canceled, nil
}

func (s *Store) Delete(ids []string) (deleted []string, skipped []string, err error) {
	for _, id := range ids {
		j, e := s.Get(id)
		if e != nil {
			skipped = append(skipped, id)
			continue
		}
		if j.Status == "processing" {
			skipped = append(skipped, id)
			continue
		}
		if j.AudioPath != "" {
			_ = os.Remove(j.AudioPath)
		}
		if _, e := s.db.Exec(`DELETE FROM jobs WHERE id = ?`, id); e != nil {
			skipped = append(skipped, id)
			continue
		}
		deleted = append(deleted, id)
	}
	return deleted, skipped, nil
}

func (s *Store) Requeue(id, asr, lang, size, prompt string) error {
	j, err := s.Get(id)
	if err != nil {
		return err
	}
	if j.Status == "processing" {
		return fmt.Errorf("job is processing")
	}
	if _, err := os.Stat(j.AudioPath); err != nil {
		return fmt.Errorf("original audio file is missing")
	}
	queued := "queued"
	sumOnly := false
	st := map[string]string{"upload": "done", "asr": "pending", "summarize": "pending"}
	return s.Update(id, Update{
		Status:          &queued,
		ASRPreset:       &asr,
		ASRLanguage:     &lang,
		SummarySize:     &size,
		CustomPrompt:    &prompt,
		SummarizeOnly:   &sumOnly,
		Stages:          st,
		ClearTranscript: true,
		ClearSummary:    true,
		Error:           ptr(""),
		Timings:         map[string]any{},
		ModelInfo:       map[string]any{},
	})
}

func (s *Store) QueueSummarizeOnly(id, size, prompt string) error {
	j, err := s.Get(id)
	if err != nil {
		return err
	}
	if j.Status == "processing" {
		return fmt.Errorf("job is processing")
	}
	if strings.TrimSpace(j.Transcript) == "" {
		return fmt.Errorf("no transcript to summarize")
	}
	queued := "queued"
	sumOnly := true
	st := map[string]string{"upload": "done", "asr": "done", "summarize": "pending"}
	return s.Update(id, Update{
		Status:        &queued,
		SummarySize:   &size,
		CustomPrompt:  &prompt,
		SummarizeOnly: &sumOnly,
		Stages:        st,
		ClearSummary:  true,
		Error:         ptr(""),
	})
}

func ptr[T any](v T) *T { return &v }

func NewID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
