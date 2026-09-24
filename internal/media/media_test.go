package media_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandr/audio-text-manager-desktop/internal/media"
)

func TestIsAllowed(t *testing.T) {
	if !media.IsAllowed("talk.WAV") {
		t.Fatal("wav")
	}
	if !media.IsAllowed("clip.mp4") {
		t.Fatal("mp4")
	}
	if media.IsAllowed("notes.txt") {
		t.Fatal("txt must be rejected")
	}
}

func TestCopyLimit(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.wav")
	dst := filepath.Join(dir, "out", "a.wav")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := media.CopyLimit(src, dst, 4); err == nil {
		t.Fatal("expected size error")
	}
	if err := media.CopyLimit(src, dst, 100); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("got %q", got)
	}
}

func TestPrepareUploadRejectsUnknown(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	_ = os.WriteFile(src, []byte("x"), 0o644)
	_, _, err := media.PrepareUpload("", "", src, dir, "id", 100, 100, 0)
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("err=%v", err)
	}
}
