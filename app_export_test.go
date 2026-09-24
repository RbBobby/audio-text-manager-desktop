package main

import (
	"strings"
	"testing"
)

func TestSanitizeExportName(t *testing.T) {
	got := sanitizeExportName("transcript_abc.txt", "txt")
	if got != "transcript_abc.txt" {
		t.Fatalf("got %q", got)
	}
	got = sanitizeExportName("../evil:name", "doc")
	if strings.Contains(got, "..") || strings.Contains(got, ":") {
		t.Fatalf("unsafe name %q", got)
	}
	if !strings.HasSuffix(got, ".doc") {
		t.Fatalf("ext %q", got)
	}
}

func TestWordDocBytes(t *testing.T) {
	raw := string(wordDocBytes("t <x>", "hello\n\n&"))
	if !strings.Contains(raw, "t &lt;x&gt;") {
		t.Fatalf("title escape: %s", raw)
	}
	if !strings.Contains(raw, "&amp;") {
		t.Fatalf("body escape: %s", raw)
	}
	if !strings.Contains(raw, "<br/>") {
		t.Fatalf("blank line: %s", raw)
	}
}
