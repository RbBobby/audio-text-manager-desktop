package asr

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Segment struct {
	StartMS int
	EndMS   int
	Text    string
	Speaker int
}

type whisperJSON struct {
	Transcription []struct {
		Offsets struct {
			From int `json:"from"`
			To   int `json:"to"`
		} `json:"offsets"`
		Text string `json:"text"`
	} `json:"transcription"`
}

func compactText(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}

func isRepeatText(a, b string) bool {
	a, b = compactText(a), compactText(b)
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	ar, br := []rune(a), []rune(b)
	if len(ar) < 16 || len(br) < 16 {
		return false
	}
	return strings.Contains(a, b) || strings.Contains(b, a)
}

// CollapseRepeats drops Whisper loop fragments that repeat the previous phrase.
func CollapseRepeats(segs []Segment) []Segment {
	if len(segs) == 0 {
		return segs
	}
	out := make([]Segment, 0, len(segs))
	out = append(out, segs[0])
	for _, s := range segs[1:] {
		prev := &out[len(out)-1]
		if !isRepeatText(prev.Text, s.Text) {
			out = append(out, s)
			continue
		}
		if s.EndMS > prev.EndMS {
			prev.EndMS = s.EndMS
		}
		if len([]rune(s.Text)) > len([]rune(prev.Text)) {
			prev.Text = s.Text
		}
	}
	return out
}

func ParseWhisperJSON(raw []byte) []Segment {
	var doc whisperJSON
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil
	}
	var out []Segment
	for _, row := range doc.Transcription {
		text := strings.TrimSpace(row.Text)
		if text == "" {
			continue
		}
		end := row.Offsets.To
		if end < row.Offsets.From {
			end = row.Offsets.From
		}
		out = append(out, Segment{
			StartMS: row.Offsets.From,
			EndMS:   end,
			Text:    text,
			Speaker: 1,
		})
	}
	return out
}

func FormatSpeakers(segs []Segment) string {
	if len(segs) == 0 {
		return ""
	}
	maxSp := 1
	for _, s := range segs {
		if s.Speaker > maxSp {
			maxSp = s.Speaker
		}
	}
	var b strings.Builder
	cur := 0
	for _, s := range segs {
		sp := s.Speaker
		if sp < 1 {
			sp = 1
		}
		if maxSp <= 1 {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(s.Text)
			continue
		}
		if sp != cur {
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			fmt.Fprintf(&b, "Спикер %d: %s", sp, s.Text)
			cur = sp
			continue
		}
		b.WriteByte(' ')
		b.WriteString(s.Text)
	}
	return strings.TrimSpace(b.String())
}
