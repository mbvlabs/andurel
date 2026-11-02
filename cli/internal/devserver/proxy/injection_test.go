package proxy

import (
	"strings"
	"testing"
)

func TestRewriteStylesheetHrefs(t *testing.T) {
	in := `<html><head><link rel="stylesheet" href="/assets/css/style.css"></head><body></body></html>`
	out := string(RewriteStylesheetHrefs([]byte(in)))

	if !strings.Contains(out, `href="/__andurel/assets/css/style.css"`) {
		t.Fatalf("expected stylesheet href to be rewritten, got: %s", out)
	}
}

func TestRewriteStylesheetHrefsRelativePath(t *testing.T) {
	in := `<link rel="stylesheet" href="assets/css/style.css?v=1">`
	out := string(RewriteStylesheetHrefs([]byte(in)))

	if !strings.Contains(out, `href="/__andurel/assets/css/style.css?v=1"`) {
		t.Fatalf("expected relative stylesheet href to be rewritten, got: %s", out)
	}
}

func TestRewriteStylesheetHrefsExternalUnchanged(t *testing.T) {
	in := `<link rel="stylesheet" href="https://cdn.example.com/style.css">`
	out := string(RewriteStylesheetHrefs([]byte(in)))

	if out != in {
		t.Fatalf("expected external stylesheet href to remain unchanged, got: %s", out)
	}
}

func TestRewriteStylesheetHrefsTimestampedPathCanonicalized(t *testing.T) {
	in := `<link rel="stylesheet" href="/assets/css/1770715671/style.css">`
	out := string(RewriteStylesheetHrefs([]byte(in)))

	if !strings.Contains(out, `href="/__andurel/assets/css/style.css"`) {
		t.Fatalf("expected timestamp segment to be removed, got: %s", out)
	}
}
