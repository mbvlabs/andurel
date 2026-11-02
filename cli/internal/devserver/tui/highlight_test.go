package tui

import (
	"strings"
	"testing"
)

func TestHighlightLineColorsLevelsAndHTTP(t *testing.T) {
	got := HighlightLine(`15:04:05 INF GET /users 200 1.2ms msg="ok"`)
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected ANSI codes, got %q", got)
	}
	if !strings.Contains(got, "INF") || !strings.Contains(got, "GET") || !strings.Contains(got, "200") {
		t.Fatalf("expected original tokens, got %q", got)
	}
	plain := HighlightLine("just a message")
	if strings.Contains(plain, "\x1b[") {
		t.Fatalf("plain text should stay unstyled, got %q", plain)
	}
}

func TestHighlightLinePreservesExistingANSI(t *testing.T) {
	in := "\x1b[32mINFO\x1b[0m already colored"
	if got := HighlightLine(in); got != in {
		t.Fatalf("got %q", got)
	}
}

func TestHighlightJSON(t *testing.T) {
	got := HighlightLine(`{"level":"ERROR","msg":"fail","n":1}`)
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected JSON highlighting, got %q", got)
	}
	if !strings.Contains(got, `"level"`) || !strings.Contains(got, `"ERROR"`) {
		t.Fatalf("expected JSON keys, got %q", got)
	}
}

func TestHighlightPrefixAndGoPos(t *testing.T) {
	got := HighlightLine(`[run] cmd/app/main.go:12:3: undefined: Foo`)
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected highlighting, got %q", got)
	}
	if !strings.Contains(got, "[run]") || !strings.Contains(got, "cmd/app/main.go:12:3") {
		t.Fatalf("expected source tokens, got %q", got)
	}
}
