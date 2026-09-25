package flavor

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/alexandr/audio-text-manager-desktop/internal/sidecar"
)

const (
	Medium   = "medium"
	Speakers = "speakers"
)

type Info struct {
	ID       string   `json:"id"`
	Speakers bool     `json:"speakers"`
	Presets  []string `json:"presets"`
}

type Option struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type Config struct {
	ID       string   `json:"id"`
	Speakers bool     `json:"speakers"`
	Default  string   `json:"default"`
	Presets  []Option `json:"presets"`
}

var labels = map[string]string{
	"fast":   "Очень быстро (small Q5)",
	"medium": "Средне (medium Q5)",
	"large":  "Точно (large-v3 Q5, спикеры)",
}

func Spec(id string) (Info, bool) {
	switch strings.TrimSpace(id) {
	case Medium:
		return Info{ID: Medium, Speakers: false, Presets: []string{"fast", "medium"}}, true
	case Speakers:
		return Info{ID: Speakers, Speakers: true, Presets: []string{"fast", "large"}}, true
	default:
		return Info{}, false
	}
}

func ModelFiles(id string) []string {
	switch id {
	case Speakers:
		return []string{"ggml-small-q5_1.bin", "ggml-large-v3-q5_0.bin"}
	default:
		return []string{"ggml-small-q5_1.bin", "ggml-medium-q5_0.bin"}
	}
}

func Current() Info {
	if p := sidecar.FlavorFile(); p != "" {
		raw, err := os.ReadFile(p)
		if err == nil {
			var doc Info
			if json.Unmarshal(raw, &doc) == nil && len(doc.Presets) > 0 {
				if spec, ok := Spec(doc.ID); ok {
					return spec
				}
				return doc
			}
		}
	}
	return infer()
}

func infer() Info {
	hasMedium := sidecar.FindModel("", "", "ggml-medium-q5_0.bin") != ""
	hasLarge := sidecar.FindModel("", "", "ggml-large-v3-q5_0.bin") != ""
	if hasLarge && !hasMedium {
		info, _ := Spec(Speakers)
		return info
	}
	if hasMedium && !hasLarge {
		info, _ := Spec(Medium)
		return info
	}
	if hasMedium && hasLarge {
		return Info{ID: "dev", Speakers: true, Presets: []string{"fast", "medium", "large"}}
	}
	info, _ := Spec(Medium)
	return info
}

func Allows(preset string) bool {
	preset = strings.TrimSpace(preset)
	for _, p := range Current().Presets {
		if p == preset {
			return true
		}
	}
	return false
}

func Diarize(preset string) bool {
	cur := Current()
	return cur.Speakers && preset == "large"
}

func UI() Config {
	cur := Current()
	def := cur.Presets[0]
	if cur.ID == Speakers {
		def = "large"
	} else if len(cur.Presets) > 1 {
		def = cur.Presets[len(cur.Presets)-1]
	}
	opts := make([]Option, 0, len(cur.Presets))
	for _, id := range cur.Presets {
		label := labels[id]
		if label == "" {
			label = id
		}
		opts = append(opts, Option{ID: id, Label: label})
	}
	return Config{ID: cur.ID, Speakers: cur.Speakers, Default: def, Presets: opts}
}
