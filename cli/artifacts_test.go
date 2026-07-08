package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/spf13/cobra"
)

func TestBuildMutationReportSummarizesChangesRoutesAndDiff(t *testing.T) {
	before := fileSnapshot{
		"app/models/post.go": {
			Hash:    hashForTest("old\n"),
			Content: []byte("old\n"),
			Mode:    0o644,
		},
		"router/routes/old.go": {
			Hash:    hashForTest("old route\n"),
			Content: []byte("old route\n"),
			Mode:    0o644,
		},
		"assets/logo.png": {
			Hash:    hashForTest("\x00old"),
			Content: []byte("\x00old"),
			Mode:    0o644,
		},
	}
	after := fileSnapshot{
		"app/models/post.go": {
			Hash:    hashForTest("new\n"),
			Content: []byte("new\n"),
			Mode:    0o644,
		},
		"router/routes/posts.go": {
			Hash:    hashForTest("posts route\n"),
			Content: []byte("posts route\n"),
			Mode:    0o644,
		},
		"assets/logo.png": {
			Hash:    hashForTest("\x00new"),
			Content: []byte("\x00new"),
			Mode:    0o644,
		},
	}

	report := buildMutationReport(mutationOptions{
		Action:      "generate model",
		Resource:    "Post",
		DryRun:      true,
		Diff:        true,
		CommandsRun: []string{"andurel generate model Post"},
		Warnings:    []string{"preview"},
	}, before, after)

	assertStrings(t, report.FilesCreated, []string{"router/routes/posts.go"})
	assertStrings(t, report.FilesUpdated, []string{"app/models/post.go", "assets/logo.png"})
	assertStrings(t, report.FilesDeleted, []string{"router/routes/old.go"})
	assertStrings(t, report.RoutesAdded, []string{"router/routes/posts.go"})
	if report.CommandsRun[0] != "andurel generate model Post" || report.Warnings[0] != "preview" {
		t.Fatalf("metadata not copied into report: %#v", report)
	}
	if !strings.Contains(report.Diff, "diff --git a/app/models/post.go b/app/models/post.go") {
		t.Fatalf("expected text diff for updated file:\n%s", report.Diff)
	}
	if strings.Contains(report.Diff, "assets/logo.png") {
		t.Fatalf("binary file should be omitted from diff:\n%s", report.Diff)
	}
	if got, want := mutationSummary(report), "Would change 4 files for generate model"; got != want {
		t.Fatalf("mutationSummary = %q, want %q", got, want)
	}
}

func TestSnapshotFilesForReportSkipsGeneratedAndDependencyDirs(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/app\n")
	writeTestFile(t, root, "app/models/post.go", "package models\n")
	writeTestFile(t, root, ".git/config", "[core]\n")
	writeTestFile(t, root, "node_modules/pkg/index.js", "module.exports = {}\n")
	writeTestFile(t, root, "bin/tool", "#!/bin/sh\n")
	writeTestFile(t, root, ".andurel-cache/state.json", "{}\n")

	snapshot, err := snapshotFilesForReport(root)
	if err != nil {
		t.Fatalf("snapshotFilesForReport: %v", err)
	}
	if _, ok := snapshot["app/models/post.go"]; !ok {
		t.Fatalf("expected app file in snapshot: %#v", snapshot)
	}
	for _, skipped := range []string{".git/config", "node_modules/pkg/index.js", "bin/tool", ".andurel-cache/state.json"} {
		if _, ok := snapshot[skipped]; ok {
			t.Fatalf("expected %s to be skipped: %#v", skipped, snapshot)
		}
	}
}

func TestCopyDirCopiesNestedFilesAndSkipsIgnoredDirs(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "copy")
	writeTestFile(t, src, "app/views/posts/index.templ", "posts\n")
	writeTestFile(t, src, "node_modules/pkg/index.js", "ignored\n")

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(dst, "app", "views", "posts", "index.templ"))
	if err != nil {
		t.Fatalf("read copied file: %v", err)
	}
	if string(content) != "posts\n" {
		t.Fatalf("copied content = %q", string(content))
	}
	if _, err := os.Stat(filepath.Join(dst, "node_modules")); !os.IsNotExist(err) {
		t.Fatalf("expected ignored node_modules to be absent, stat err: %v", err)
	}
}

func TestRunMutationDryRunReportsChangesWithoutMutatingOriginal(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/app\n")
	writeTestFile(t, root, "app/models/post.go", "old\n")
	writeTestFile(t, root, "router/routes/old.go", "old route\n")

	var out bytes.Buffer
	cmd := &cobra.Command{Use: "andurel"}
	output.RegisterPersistentFlags(cmd)
	cmd.SetOut(&out)
	if err := cmd.PersistentFlags().Set("json", "true"); err != nil {
		t.Fatalf("set --json: %v", err)
	}

	err := runMutation(cmd, mutationOptions{
		Action:   "generate model",
		Resource: "Post",
		RootDir:  root,
		DryRun:   true,
		Diff:     true,
		Run: func(rootDir string) error {
			writeTestFile(t, rootDir, "app/models/post.go", "new\n")
			writeTestFile(t, rootDir, "router/routes/posts.go", "posts route\n")
			if err := os.Remove(filepath.Join(rootDir, "router", "routes", "old.go")); err != nil {
				return err
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("runMutation dry run: %v", err)
	}

	var envelope output.Envelope
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("decode dry-run envelope: %v\n%s", err, out.String())
	}
	data, ok := envelope.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %#v", envelope.Data)
	}
	if data["dry_run"] != true {
		t.Fatalf("expected dry_run in report, got %#v", data)
	}
	if !strings.Contains(envelope.Summary, "Would change 3 files") {
		t.Fatalf("unexpected summary: %q", envelope.Summary)
	}
	if !strings.Contains(out.String(), "dry run only; no files were changed") {
		t.Fatalf("expected dry-run warning in output:\n%s", out.String())
	}

	original, err := os.ReadFile(filepath.Join(root, "app", "models", "post.go"))
	if err != nil {
		t.Fatalf("read original model: %v", err)
	}
	if string(original) != "old\n" {
		t.Fatalf("dry run mutated original model: %q", original)
	}
	if _, err := os.Stat(filepath.Join(root, "router", "routes", "posts.go")); !os.IsNotExist(err) {
		t.Fatalf("dry run created route in original tree, stat err: %v", err)
	}
}

func TestRunMutationErrorsForMissingRunnerAndRoot(t *testing.T) {
	cmd := &cobra.Command{Use: "andurel"}
	output.RegisterPersistentFlags(cmd)

	if err := runMutation(cmd, mutationOptions{RootDir: t.TempDir()}); err == nil {
		t.Fatalf("expected missing runner error")
	}
	if err := runMutation(cmd, mutationOptions{Run: func(string) error { return nil }}); err == nil {
		t.Fatalf("expected missing root error")
	}
}

func hashForTest(content string) [32]byte {
	return sha256.Sum256([]byte(content))
}

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func assertStrings(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	}
}
