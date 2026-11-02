package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/layout"
)

func TestSyncQueriesReportsQuerySchemaFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub uses a POSIX shell")
	}
	resetCLITestSeams(t)
	root := t.TempDir()
	writeTestFile(t, root, "domains/user.sql", "-- name: FindUser :one\nSELECT missing FROM users;\n")
	lock := layout.NewAndurelLock("test")
	lock.Tools["narsilc"] = validTestTool("narsilc", "v0.1.0")
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatal(err)
	}
	writeExecutable(t, root, "bin/narsilc", "#!/bin/sh\necho 'unknown column missing' >&2\nexit 1\n")
	ensureToolFunc = func(string, string, *layout.Tool, string, string) error { return nil }
	err := generateNarsilcIfNeeded(root)
	if err == nil || !strings.Contains(err.Error(), querySchemaMessage) || !strings.Contains(err.Error(), "unknown column missing") {
		t.Fatalf("query failure = %v", err)
	}
	if got := output.Fail(err).Hint; got != querySchemaHint {
		t.Fatalf("hint = %q", got)
	}
}

func TestSyncQueriesDryRunDisclosesSharedManifests(t *testing.T) {
	resetCLITestSeams(t)
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/test\n")
	findGoModRoot = func() (string, error) { return root, nil }
	for _, structured := range []bool{false, true} {
		var out bytes.Buffer
		cmd := NewRootCommand("test", "test-date")
		cmd.SetOut(&out)
		args := []string{"sync", "queries", "--dry-run"}
		if structured {
			args = append(args, "--json")
		}
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("sync queries: %v", err)
		}
		for _, want := range []string{"go.mod", "go.sum"} {
			if !strings.Contains(out.String(), want) {
				t.Fatalf("missing %s disclosure: %s", want, out.String())
			}
		}
	}
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || string(data) != "module example.com/test\n" {
		t.Fatalf("go.mod changed: %q, %v", data, err)
	}
}

func TestSyncQueriesManifestPreviewApplyAndFailure(t *testing.T) {
	resetCLITestSeams(t)
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "before\n")
	writeTestFile(t, root, "go.sum", "before\n")
	writeTestFile(t, root, "domains/widget.sql", "-- name: Widget :one\nSELECT 1;\n")
	findGoModRoot = func() (string, error) { return root, nil }
	original := generateQueriesFunc
	t.Cleanup(func() { generateQueriesFunc = original })
	fail := false
	generateQueriesFunc = func(dir string) error {
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("after\n"), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "go.sum"), []byte("after\n"), 0o644); err != nil {
			return err
		}
		if fail {
			return errors.New("generation failed")
		}
		return nil
	}
	run := func(args ...string) (string, error) {
		var out bytes.Buffer
		cmd := NewRootCommand("test", "test-date")
		cmd.SetOut(&out)
		cmd.SetArgs(append([]string{"sync", "queries"}, args...))
		err := cmd.Execute()
		return out.String(), err
	}
	check := func(want string) {
		t.Helper()
		for _, name := range []string{"go.mod", "go.sum"} {
			data, err := os.ReadFile(filepath.Join(root, name))
			if err != nil || string(data) != want {
				t.Fatalf("%s = %q, %v; want %q", name, data, err, want)
			}
		}
	}
	out, err := run("--dry-run", "--json")
	if err != nil || !strings.Contains(out, `"dry_run": true`) || !strings.Contains(out, `"go.mod"`) || !strings.Contains(out, `"go.sum"`) {
		t.Fatalf("preview: %v\n%s", err, out)
	}
	check("before\n")
	out, err = run("--dry-run")
	if err != nil || !strings.Contains(out, "update go.mod") || !strings.Contains(out, "update go.sum") {
		t.Fatalf("human preview: %v\n%s", err, out)
	}
	check("before\n")
	fail = true
	_, err = run("--dry-run", "--json")
	if err == nil {
		t.Fatal("failed preview reported success")
	}
	check("before\n")
	_, err = run("--json")
	if err == nil || !strings.Contains(err.Error(), "partially changed") {
		t.Fatalf("partial write not disclosed: %v", err)
	}
	check("after\n")
	fail = false
	writeTestFile(t, root, "go.mod", "before\n")
	writeTestFile(t, root, "go.sum", "before\n")
	out, err = run("--json")
	if err != nil || !strings.Contains(out, `"go.mod"`) || !strings.Contains(out, `"go.sum"`) {
		t.Fatalf("apply: %v\n%s", err, out)
	}
	check("after\n")
}
