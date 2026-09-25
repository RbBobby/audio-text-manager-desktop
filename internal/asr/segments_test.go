package asr_test

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandr/audio-text-manager-desktop/internal/asr"
)

func TestParseAndFormatSpeakers(t *testing.T) {
	raw := []byte(`{
	  "transcription": [
	    {"offsets":{"from":0,"to":1200},"text":" Привет"},
	    {"offsets":{"from":1300,"to":2400},"text":" Здравствуйте"},
	    {"offsets":{"from":2500,"to":3600},"text":" Как дела?"}
	  ]
	}`)
	segs := asr.ParseWhisperJSON(raw)
	if len(segs) != 3 {
		t.Fatalf("segs %d", len(segs))
	}
	segs[0].Speaker = 1
	segs[1].Speaker = 2
	segs[2].Speaker = 1
	got := asr.FormatSpeakers(segs)
	if !strings.Contains(got, "Спикер 1: Привет") || !strings.Contains(got, "Спикер 2: Здравствуйте") {
		t.Fatalf("got %q", got)
	}
	if strings.Count(got, "Спикер 1:") != 2 {
		t.Fatalf("speaker 1 turns: %q", got)
	}
}

func TestCollapseRepeats(t *testing.T) {
	segs := []asr.Segment{
		{StartMS: 0, EndMS: 1000, Text: "заголовок страницы то есть да есть какой-то"},
		{StartMS: 1000, EndMS: 1400, Text: "заголовок страницы то есть да есть какой-то"},
		{StartMS: 1400, EndMS: 2000, Text: "заголовок да это как строка"},
	}
	got := asr.CollapseRepeats(segs)
	if len(got) != 2 {
		t.Fatalf("len %d %+v", len(got), got)
	}
	if got[0].EndMS != 1400 {
		t.Fatalf("merged end %d", got[0].EndMS)
	}
}

func TestAssignSpeakersCapsAtTwo(t *testing.T) {
	dir := t.TempDir()
	wav := filepath.Join(dir, "many.wav")
	parts := make([]toneSpan, 12)
	for i := range parts {
		parts[i] = toneSpan{freq: 300 + float64(i*40), ms: 250}
	}
	if err := writeToneWAV(wav, parts); err != nil {
		t.Fatal(err)
	}
	segs := make([]asr.Segment, len(parts))
	for i := range segs {
		segs[i] = asr.Segment{StartMS: i * 250, EndMS: (i + 1) * 250, Text: "фрагмент"}
	}
	asr.AssignSpeakers(wav, segs)
	seen := map[int]struct{}{}
	for _, s := range segs {
		if s.Speaker < 1 {
			t.Fatalf("bad speaker %+v", segs)
		}
		seen[s.Speaker] = struct{}{}
	}
	if len(seen) > 2 {
		t.Fatalf("speakers %d: %+v", len(seen), segs)
	}
}

func TestAssignSpeakersDifferentTones(t *testing.T) {
	dir := t.TempDir()
	wav := filepath.Join(dir, "t.wav")
	if err := writeToneWAV(wav, []toneSpan{
		{freq: 220, ms: 400},
		{freq: 2800, ms: 400},
		{freq: 220, ms: 400},
		{freq: 2800, ms: 400},
	}); err != nil {
		t.Fatal(err)
	}
	segs := []asr.Segment{
		{StartMS: 0, EndMS: 400, Text: "a"},
		{StartMS: 400, EndMS: 800, Text: "b"},
		{StartMS: 800, EndMS: 1200, Text: "c"},
		{StartMS: 1200, EndMS: 1600, Text: "d"},
	}
	asr.AssignSpeakers(wav, segs)
	if segs[0].Speaker == segs[1].Speaker {
		t.Fatalf("expected different speakers, got %+v", segs)
	}
	if segs[0].Speaker != segs[2].Speaker || segs[1].Speaker != segs[3].Speaker {
		t.Fatalf("expected alternating pair, got %+v", segs)
	}
}

type toneSpan struct {
	freq float64
	ms   int
}

func writeToneWAV(path string, parts []toneSpan) error {
	rate := 16000
	var pcm []int16
	for _, p := range parts {
		n := rate * p.ms / 1000
		for i := 0; i < n; i++ {
			s := math.Sin(2 * math.Pi * p.freq * float64(i) / float64(rate))
			pcm = append(pcm, int16(s*20000))
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	dataBytes := len(pcm) * 2
	hdr := make([]byte, 44)
	copy(hdr[0:4], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(36+dataBytes))
	copy(hdr[8:12], "WAVE")
	copy(hdr[12:16], "fmt ")
	binary.LittleEndian.PutUint32(hdr[16:20], 16)
	binary.LittleEndian.PutUint16(hdr[20:22], 1)
	binary.LittleEndian.PutUint16(hdr[22:24], 1)
	binary.LittleEndian.PutUint32(hdr[24:28], uint32(rate))
	binary.LittleEndian.PutUint32(hdr[28:32], uint32(rate*2))
	binary.LittleEndian.PutUint16(hdr[32:34], 2)
	binary.LittleEndian.PutUint16(hdr[34:36], 16)
	copy(hdr[36:40], "data")
	binary.LittleEndian.PutUint32(hdr[40:44], uint32(dataBytes))
	if _, err := f.Write(hdr); err != nil {
		return err
	}
	return binary.Write(f, binary.LittleEndian, pcm)
}
