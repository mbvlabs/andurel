package devserver

import (
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestOptionsNormalize(t *testing.T) {
	if err := (&Options{}).normalize(); err == nil {
		t.Fatal("expected an error without a project directory")
	}

	opts := Options{Dir: t.TempDir()}
	if err := opts.normalize(); err != nil {
		t.Fatal(err)
	}
	if opts.PackageManager != DefaultPackageManager {
		t.Fatalf("PackageManager = %q, want %q", opts.PackageManager, DefaultPackageManager)
	}
	if opts.Stdin == nil || opts.Stdout == nil || opts.Stderr == nil {
		t.Fatal("expected standard streams to default")
	}

	relative := Options{Dir: "."}
	if err := relative.normalize(); err != nil || !filepath.IsAbs(relative.Dir) {
		t.Fatalf("normalize relative directory = %q, %v; want absolute path", relative.Dir, err)
	}

	bad := Options{Dir: t.TempDir(), Tools: []string{"dblab"}}
	if err := bad.normalize(); err == nil || !strings.Contains(err.Error(), "dblab") {
		t.Fatalf("normalize() = %v, want unknown tool error", err)
	}
}

func TestRunRequiresQueueEntrypoint(t *testing.T) {
	err := Run(t.Context(), Options{Dir: t.TempDir(), Inline: true, Stderr: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "cmd/queue/main.go") {
		t.Fatalf("Run() = %v, want missing queue entrypoint error", err)
	}
}
