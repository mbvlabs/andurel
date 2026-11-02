package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/layout"
)

func TestParseRunTools(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    []string
		wantErr string
	}{
		{name: "empty omitted", raw: "", want: nil},
		{name: "mailpit", raw: "mailpit", want: []string{"mailpit"}},
		{name: "trimmed", raw: " mailpit ", want: []string{"mailpit"}},
		{name: "whitespace only", raw: "   ", wantErr: "empty list"},
		{name: "unknown", raw: "redis", wantErr: `unknown tool "redis"`},
		{name: "duplicate", raw: "mailpit,mailpit", wantErr: `duplicate tool "mailpit"`},
		{name: "trailing comma", raw: "mailpit,", wantErr: "empty name"},
		{name: "leading comma", raw: ",mailpit", wantErr: "empty name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseRunTools(tt.raw)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("parseRunTools(%q) error = %v, want containing %q", tt.raw, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRunTools(%q): %v", tt.raw, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseRunTools(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestDevServerOptions(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)

	lock := layout.NewAndurelLock("test")
	lock.ScaffoldConfig = &layout.ScaffoldConfig{
		ProjectName: "app",
	}
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	opts, err := devServerOptions(root, "")
	if err != nil {
		t.Fatalf("no tools: %v", err)
	}
	if opts.Dir != root || opts.PackageManager != "pnpm" || len(opts.Tools) != 0 {
		t.Fatalf("default options = %#v", opts)
	}

	if _, err := devServerOptions(root, "mailpit"); err == nil ||
		!strings.Contains(err.Error(), "bin/mailpit not found") {
		t.Fatalf("expected missing mailpit binary, got %v", err)
	}

	writeExecutable(t, root, "bin/mailpit", "#!/bin/sh\n")
	opts, err = devServerOptions(root, "mailpit")
	if err != nil {
		t.Fatalf("mailpit only: %v", err)
	}
	if !reflect.DeepEqual(opts.Tools, []string{"mailpit"}) {
		t.Fatalf("mailpit tools = %#v", opts.Tools)
	}

	lock.ScaffoldConfig.Inertia = "vue"
	lock.ScaffoldConfig.JavaScriptPackageManager = "bun"
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("rewrite inertia lock: %v", err)
	}
	opts, err = devServerOptions(root, "mailpit")
	if err != nil {
		t.Fatalf("inertia+tools: %v", err)
	}
	if opts.PackageManager != "bun" {
		t.Fatalf("PackageManager = %q, want bun", opts.PackageManager)
	}
}

func TestDevServerOptionsMissingToml(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	writeExecutable(t, root, "bin/mailpit", "#!/bin/sh\n")

	opts, err := devServerOptions(root, "mailpit")
	if err != nil {
		t.Fatalf("missing toml should still allow tools: %v", err)
	}
	if !reflect.DeepEqual(opts.Tools, []string{"mailpit"}) || opts.PackageManager != "pnpm" {
		t.Fatalf("options = %#v", opts)
	}
}
