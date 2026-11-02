package upgrade

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

func TestGitAnalyzerRecordsAttemptedCommands(t *testing.T) {
	root := t.TempDir()
	runUpgradeGit(t, root, "init")
	runUpgradeGit(t, root, "config", "user.email", "test@example.com")
	runUpgradeGit(t, root, "config", "user.name", "Test User")
	mustWriteTestFile(t, root, "tracked.txt", []byte("initial\n"))
	runUpgradeGit(t, root, "add", ".")
	runUpgradeGit(t, root, "commit", "-m", "initial")

	recorder := &commandRecorder{}
	analyzer := NewGitAnalyzer(root)
	analyzer.commands = recorder
	if _, err := analyzer.IsClean(); err != nil {
		t.Fatalf("IsClean: %v", err)
	}
	if got, want := recorder.commandsRun(), []string{"git status --porcelain"}; !slices.Equal(got, want) {
		t.Fatalf("recorded commands = %#v, want %#v", got, want)
	}

	failed := &commandRecorder{}
	outside := NewGitAnalyzer(t.TempDir())
	outside.commands = failed
	if _, err := outside.IsClean(); err == nil {
		t.Fatal("expected git status to fail outside a repository")
	}
	if got, want := failed.commandsRun(), []string{"git status --porcelain (failed)"}; !slices.Equal(got, want) {
		t.Fatalf("failed commands = %#v, want %#v", got, want)
	}
}

func TestUpgradeReportRecordsReadOnlyGitChecks(t *testing.T) {
	root := newUpgradeFixtureProject(t)

	upgrader, err := NewUpgrader(
		root,
		UpgradeOptions{DryRun: true, TargetVersion: fixtureTargetVersion},
	)
	if err != nil {
		t.Fatal(err)
	}
	report, err := upgrader.Execute()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(report.CommandsRun, "git status --porcelain") {
		t.Fatalf("dry-run commands = %#v, want the read-only git status check", report.CommandsRun)
	}
	for _, command := range report.CommandsRun {
		if strings.Contains(command, "go mod tidy") ||
			strings.Contains(command, "andurel tool sync") {
			t.Fatalf("dry run reported a proposed follow-up as executed: %#v", report.CommandsRun)
		}
	}

	current, err := NewUpgrader(root, UpgradeOptions{TargetVersion: fixtureSourceVersion})
	if err != nil {
		t.Fatal(err)
	}
	report, err = current.Execute()
	if err != nil {
		t.Fatal(err)
	}
	if !report.AlreadyCurrent {
		t.Fatalf("same-version report = %#v, want already current", report)
	}
	if !slices.Contains(report.CommandsRun, "git status --porcelain") {
		t.Fatalf("current commands = %#v, want the read-only git status check", report.CommandsRun)
	}
}

func TestUpgradeReportProvenanceIsExecutionLocal(t *testing.T) {
	root := newUpgradeFixtureProject(t)
	upgrader, err := NewUpgrader(
		root,
		UpgradeOptions{DryRun: true, TargetVersion: fixtureTargetVersion},
	)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"git status --porcelain", "git status --porcelain"}
	for run := 1; run <= 2; run++ {
		report, err := upgrader.Execute()
		if err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		if !slices.Equal(report.CommandsRun, want) {
			t.Fatalf("run %d commands = %#v, want isolated %#v", run, report.CommandsRun, want)
		}
	}
}

func TestUpgradeReportProvenanceConcurrentDryRuns(t *testing.T) {
	root := newUpgradeFixtureProject(t)
	upgrader, err := NewUpgrader(
		root,
		UpgradeOptions{DryRun: true, TargetVersion: fixtureTargetVersion},
	)
	if err != nil {
		t.Fatal(err)
	}

	const workers = 8
	want := []string{"git status --porcelain", "git status --porcelain"}
	results := make([][]string, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := range workers {
		wg.Go(func() {
			report, runErr := upgrader.Execute()
			errs[i] = runErr
			if report != nil {
				results[i] = report.CommandsRun
			}
		})
	}
	wg.Wait()

	for i := range workers {
		if errs[i] != nil {
			t.Fatalf("worker %d: %v", i, errs[i])
		}
		if !slices.Equal(results[i], want) {
			t.Fatalf("worker %d commands = %#v, want isolated %#v", i, results[i], want)
		}
	}
}

func TestUpgradeReportRecordsFailedGitCheck(t *testing.T) {
	root := newUpgradeFixtureProject(t)
	if err := os.RemoveAll(filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}

	upgrader, err := NewUpgrader(
		root,
		UpgradeOptions{TargetVersion: fixtureTargetVersion},
	)
	if err != nil {
		t.Fatal(err)
	}
	report, err := upgrader.Execute()
	if err == nil {
		t.Fatal("expected git validation to fail outside a repository")
	}
	if report == nil {
		t.Fatal("expected a report with the failed command")
	}
	if !slices.Contains(report.CommandsRun, "git status --porcelain (failed)") {
		t.Fatalf("failed commands = %#v, want the failed git status check", report.CommandsRun)
	}
}
