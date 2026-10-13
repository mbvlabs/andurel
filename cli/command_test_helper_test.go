package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/internal/cache"
	"github.com/mbvlabs/andurel/v2/layout"
	"github.com/spf13/cobra"
)

type cliTestResult struct {
	cmd    *cobra.Command
	stdout string
	stderr string
	err    error
}

func validTestDownload(binaryName string) *layout.ToolDownload {
	return &layout.ToolDownload{
		URLTemplate: "https://example.invalid/{{version}}/" + binaryName,
		Archive:     "binary",
		BinaryName:  binaryName,
		SHA256: map[string]string{
			"linux/amd64":  strings.Repeat("1", 64),
			"linux/arm64":  strings.Repeat("2", 64),
			"darwin/amd64": strings.Repeat("3", 64),
			"darwin/arm64": strings.Repeat("4", 64),
		},
	}
}

func validTestTool(name, version string) *layout.Tool {
	return &layout.Tool{
		Version:      version,
		Download:     validTestDownload(name),
		VersionCheck: &layout.VersionCheck{Args: []string{"--version"}},
	}
}

func testChecksumArguments() []string {
	return []string{
		"linux/amd64=" + strings.Repeat("1", 64),
		"linux/arm64=" + strings.Repeat("2", 64),
		"darwin/amd64=" + strings.Repeat("3", 64),
		"darwin/arm64=" + strings.Repeat("4", 64),
	}
}

func runCLITest(t *testing.T, args ...string) cliTestResult {
	t.Helper()
	resetCLITestSeams(t)
	return executeCLITest(t, args...)
}

func executeCLITest(t *testing.T, args ...string) cliTestResult {
	return executeConfiguredCLITest(t, "", args...)
}

func executeConfiguredCLITest(t *testing.T, inertiaAdapter string, args ...string) cliTestResult {
	t.Helper()

	rootDir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(rootDir, "go.mod"),
		[]byte("module example.com/app\n"),
		0o644,
	); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	lock := layout.NewAndurelLock("test")
	lock.ScaffoldConfig = &layout.ScaffoldConfig{
		ProjectName: "app",
	}
	if inertiaAdapter != "" {
		lock.ScaffoldConfig.Inertia = inertiaAdapter
		lock.ScaffoldConfig.JavaScriptPackageManager = "pnpm"
	}
	if err := lock.WriteLockFile(rootDir); err != nil {
		t.Fatalf("write andurel.lock: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir temp project: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	findGoModRoot = func() (string, error) {
		return rootDir, nil
	}

	stdoutCapture := captureProcessOutput(t, &os.Stdout)
	stderrCapture := captureProcessOutput(t, &os.Stderr)

	var cobraStdout, cobraStderr bytes.Buffer
	cmd := NewRootCommand("test", "test-date")
	cmd.SetOut(&cobraStdout)
	cmd.SetErr(&cobraStderr)
	cmd.SetArgs(args)

	err = cmd.Execute()
	stdout := cobraStdout.String() + stdoutCapture()
	stderr := cobraStderr.String() + stderrCapture()
	return cliTestResult{
		cmd:    cmd,
		stdout: stdout,
		stderr: stderr,
		err:    err,
	}
}

func captureProcessOutput(t *testing.T, stream **os.File) func() string {
	t.Helper()

	original := *stream
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	*stream = writer

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, reader)
		done <- buf.String()
	}()

	return func() string {
		_ = writer.Close()
		*stream = original
		out := <-done
		_ = reader.Close()
		return out
	}
}

func resetCLITestSeams(t *testing.T) {
	t.Helper()

	cache.ClearFileSystemCache()
	defaultFindGoModRoot := findGoModRoot
	defaultRunTempl := runTemplFunc
	defaultRunFmt := runFmtFunc
	defaultRunGoFmt := runGoFmtFunc
	defaultRunGolines := runGolinesFunc
	defaultRunTemplFmt := runTemplFmtFunc
	defaultMakeDiagnosticTempDir := makeDiagnosticTempDir
	defaultRemoveDiagnosticTempDir := removeDiagnosticTempDir
	defaultSyncSingleTool := syncSingleToolFunc
	defaultEnsureTool := ensureToolFunc
	defaultDownloadFromLockTool := downloadFromLockToolFunc
	defaultInstallToolVersionAndLock := installToolVersionAndLockFunc
	defaultResolveToolChecksums := resolveToolChecksumsFunc
	defaultNewUpgrader := newUpgraderFunc
	defaultLookupLatestAndurelVersion := lookupLatestAndurelVersionFunc
	defaultLookupLatestModuleVersion := lookupLatestModuleVersionFunc
	defaultApplyAndurelPackageUpdates := applyAndurelPackageUpdatesFunc
	defaultOpenAdminConnection := openAdminConnectionFunc
	defaultRunGoose := runGooseFunc
	defaultRunSeed := runSeedFunc

	t.Cleanup(func() {
		findGoModRoot = defaultFindGoModRoot
		runTemplFunc = defaultRunTempl
		runFmtFunc = defaultRunFmt
		runGoFmtFunc = defaultRunGoFmt
		runGolinesFunc = defaultRunGolines
		runTemplFmtFunc = defaultRunTemplFmt
		makeDiagnosticTempDir = defaultMakeDiagnosticTempDir
		removeDiagnosticTempDir = defaultRemoveDiagnosticTempDir
		syncSingleToolFunc = defaultSyncSingleTool
		ensureToolFunc = defaultEnsureTool
		downloadFromLockToolFunc = defaultDownloadFromLockTool
		installToolVersionAndLockFunc = defaultInstallToolVersionAndLock
		resolveToolChecksumsFunc = defaultResolveToolChecksums
		newUpgraderFunc = defaultNewUpgrader
		lookupLatestAndurelVersionFunc = defaultLookupLatestAndurelVersion
		lookupLatestModuleVersionFunc = defaultLookupLatestModuleVersion
		applyAndurelPackageUpdatesFunc = defaultApplyAndurelPackageUpdates
		openAdminConnectionFunc = defaultOpenAdminConnection
		runGooseFunc = defaultRunGoose
		runSeedFunc = defaultRunSeed
		cache.ClearFileSystemCache()
	})
}

func stubLatestAndurelVersion(t *testing.T, version string, err error) {
	t.Helper()

	original := lookupLatestAndurelVersionFunc
	lookupLatestAndurelVersionFunc = func(context.Context) (string, error) {
		return version, err
	}
	t.Cleanup(func() {
		lookupLatestAndurelVersionFunc = original
	})
}
