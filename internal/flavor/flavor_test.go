package flavor

import "testing"

func TestSpecAndModels(t *testing.T) {
	m, ok := Spec(Medium)
	if !ok || m.Speakers || len(m.Presets) != 2 {
		t.Fatalf("medium %+v", m)
	}
	s, ok := Spec(Speakers)
	if !ok || !s.Speakers || s.Presets[1] != "large" {
		t.Fatalf("speakers %+v", s)
	}
	if got := ModelFiles(Speakers); len(got) != 2 || got[1] != "ggml-large-v3-q5_0.bin" {
		t.Fatalf("models %v", got)
	}
}

func TestDiarize(t *testing.T) {
	if Diarize("fast") {
		t.Fatal("fast should not diarize")
	}
}
