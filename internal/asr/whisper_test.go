package asr_test

import (
	"testing"

	"github.com/alexandr/audio-text-manager-desktop/internal/asr"
)

func TestModelFile(t *testing.T) {
	name, err := asr.ModelFile("fast")
	if err != nil || name != "ggml-small-q5_1.bin" {
		t.Fatalf("%s %v", name, err)
	}
	name, err = asr.ModelFile("medium")
	if err != nil || name != "ggml-medium-q5_0.bin" {
		t.Fatalf("%s %v", name, err)
	}
	if _, err := asr.ModelFile("high"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := asr.ModelFile("turbo"); err == nil {
		t.Fatal("expected error")
	}
}

func TestNormalizeLanguage(t *testing.T) {
	got, err := asr.NormalizeLanguage("")
	if err != nil || got != "ru" {
		t.Fatalf("empty -> %q %v", got, err)
	}
	got, err = asr.NormalizeLanguage(" EN ")
	if err != nil || got != "en" {
		t.Fatalf("en -> %q %v", got, err)
	}
	got, err = asr.NormalizeLanguage("auto")
	if err != nil || got != "auto" {
		t.Fatalf("auto -> %q %v", got, err)
	}
	if _, err := asr.NormalizeLanguage("xx"); err == nil {
		t.Fatal("expected error")
	}
}
