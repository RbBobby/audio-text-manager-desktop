package summary_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexandr/audio-text-manager-desktop/internal/summary"
)

func TestParseSize(t *testing.T) {
	cases := map[string]string{
		"gist": "gist", "executive": "executive", "meeting": "meeting",
		"short": "gist", "medium": "executive", "long": "meeting",
	}
	for in, want := range cases {
		got, err := summary.ParseSize(in)
		if err != nil || got != want {
			t.Fatalf("%s -> %s %v", in, got, err)
		}
	}
	if _, err := summary.ParseSize("tiny"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSystemInstruction(t *testing.T) {
	if !strings.Contains(summary.SystemInstruction("gist"), "120–180") {
		t.Fatal("gist prompt")
	}
	if !strings.Contains(summary.SystemInstruction("executive"), "руководителя") {
		t.Fatal("executive prompt")
	}
	if !strings.Contains(summary.SystemInstruction("meeting"), "доклад") {
		t.Fatal("meeting prompt")
	}
}

func TestSummarizeClipsLongTranscript(t *testing.T) {
	var gotUser string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, m := range body.Messages {
			if m.Role == "user" {
				gotUser = m.Content
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "ok"}},
			},
		})
	}))
	defer srv.Close()
	client := summary.New(srv.URL, "")
	long := strings.Repeat("слово ", 20000)
	_, _, err := summary.Summarize(context.Background(), client, long, "gist", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotUser, "[truncated]") {
		t.Fatal("expected truncated transcript in prompt")
	}
	if strings.Count(gotUser, "слово") > 15000 {
		t.Fatalf("prompt still too long: %d words", strings.Count(gotUser, "слово"))
	}
}

func TestChatOpenAI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "кратко"}},
			},
		})
	}))
	defer srv.Close()
	client := summary.New(srv.URL, "")
	out, err := client.Chat(context.Background(), "sys", "user")
	if err != nil || out != "кратко" {
		t.Fatalf("%q %v", out, err)
	}
}
