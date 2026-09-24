package asr

import (
	"fmt"
	"strings"
)

// DefaultLanguage is used when the UI sends an empty value.
const DefaultLanguage = "ru"

// Languages are whisper.cpp -l codes shown in the frontend dropdown.
var Languages = map[string]struct{}{
	"auto": {},
	"ar":   {},
	"be":   {},
	"de":   {},
	"en":   {},
	"es":   {},
	"fr":   {},
	"it":   {},
	"ja":   {},
	"kk":   {},
	"ko":   {},
	"pl":   {},
	"pt":   {},
	"ru":   {},
	"tr":   {},
	"uk":   {},
	"zh":   {},
}

func NormalizeLanguage(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return DefaultLanguage, nil
	}
	if _, ok := Languages[s]; !ok {
		return "", fmt.Errorf("unknown asr language %q", s)
	}
	return s, nil
}
