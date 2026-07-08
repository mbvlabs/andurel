package upgrade

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitAnalyzerCleanModifiedAndInitialFiles(t *testing.T) {
	root := t.TempDir()
	runUpgradeGit(t, root, "init")
	runUpgradeGit(t, root, "config", "user.email", "test@example.com")
	runUpgradeGit(t, root, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("initial\n"), 0o644); err != nil {
		t.Fatalf("write tracked: %v", err)
	}
	runUpgradeGit(t, root, "add", ".")
	runUpgradeGit(t, root, "commit", "-m", "initial")

	analyzer := NewGitAnalyzer(root)
	clean, err := analyzer.IsClean()
	if err != nil {
		t.Fatalf("IsClean clean: %v", err)
	}
	if !clean {
		t.Fatalf("expected repo to be clean")
	}

	initial, err := analyzer.GetFileFromInitialCommit("tracked.txt")
	if err != nil {
		t.Fatalf("GetFileFromInitialCommit tracked: %v", err)
	}
	if string(initial) != "initial\n" {
		t.Fatalf("initial file = %q", string(initial))
	}
	missing, err := analyzer.GetFileFromInitialCommit("missing.txt")
	if err != nil {
		t.Fatalf("GetFileFromInitialCommit missing: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected missing initial file to return nil, got %q", string(missing))
	}

	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("modify tracked: %v", err)
	}
	clean, err = analyzer.IsClean()
	if err != nil {
		t.Fatalf("IsClean dirty: %v", err)
	}
	if clean {
		t.Fatalf("expected repo to be dirty")
	}
	modified, err := analyzer.GetModifiedFiles()
	if err != nil {
		t.Fatalf("GetModifiedFiles: %v", err)
	}
	if !modified["tracked.txt"] {
		t.Fatalf("modified files = %#v", modified)
	}
}

func TestGitAnalyzerErrorsOutsideRepository(t *testing.T) {
	analyzer := NewGitAnalyzer(t.TempDir())
	if clean, err := analyzer.IsClean(); err == nil || clean || !strings.Contains(err.Error(), "git") {
		t.Fatalf("expected git status error, clean=%v err=%v", clean, err)
	}
	if _, err := analyzer.GetModifiedFiles(); err == nil || !strings.Contains(err.Error(), "failed to get first commit") {
		t.Fatalf("expected first commit error, got %v", err)
	}
}

func runUpgradeGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(out))
	}
}
