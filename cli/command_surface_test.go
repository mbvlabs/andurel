package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/layout/cmds"
	"github.com/mbvlabs/andurel/layout/upgrade"
	"github.com/spf13/cobra"
)

func TestDatabaseConnectionToolCommands(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)
	writeTestFile(t, root, ".env", strings.Join([]string{
		"DB_KIND=postgres",
		"DB_PORT=5432",
		"DB_HOST=127.0.0.1",
		"DB_NAME=app_db",
		"DB_USER=app_user",
		"DB_PASSWORD=secret",
		"DB_SSL_MODE=disable",
		"",
	}, "\n"))
	unsetEnvForTest(t, "DB_KIND", "DB_PORT", "DB_HOST", "DB_NAME", "DB_USER", "DB_PASSWORD", "DB_SSL_MODE")

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	if err := executeStandaloneCommand(newConsoleCommand()); err == nil || !strings.Contains(err.Error(), "usql binary not found") {
		t.Fatalf("expected missing usql error, got %v", err)
	}
	writeExecutable(t, root, "bin/usql", "#!/bin/sh\nprintf '%s\\n' \"$@\" > usql.args\n")
	if err := executeStandaloneCommand(newConsoleCommand()); err != nil {
		t.Fatalf("console command: %v", err)
	}
	assertTestFileContains(t, root, "usql.args", "postgres://app_user:secret@127.0.0.1:5432/app_db?sslmode=disable")

	if err := executeStandaloneCommand(newDblabCommand()); err == nil || !strings.Contains(err.Error(), "dblab binary not found") {
		t.Fatalf("expected missing dblab error, got %v", err)
	}
	writeExecutable(t, root, "bin/dblab", "#!/bin/sh\nprintf '%s\\n' \"$@\" > dblab.args\n")
	if err := executeStandaloneCommand(newDblabCommand()); err != nil {
		t.Fatalf("dblab command: %v", err)
	}
	assertTestFileContains(t, root, "dblab.args", "--url")
	assertTestFileContains(t, root, "dblab.args", "postgres://app_user:secret@127.0.0.1:5432/app_db?sslmode=disable")
}

func TestDatabaseConnectionToolCommandsReportEnvProblems(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)
	unsetEnvForTest(t, "DB_KIND", "DB_PORT", "DB_HOST", "DB_NAME", "DB_USER", "DB_PASSWORD", "DB_SSL_MODE")

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	if err := executeStandaloneCommand(newConsoleCommand()); err == nil || !strings.Contains(err.Error(), ".env file not found") {
		t.Fatalf("expected missing .env error, got %v", err)
	}

	writeTestFile(t, root, ".env", "DB_KIND=postgres\n")
	if err := executeStandaloneCommand(newDblabCommand()); err == nil || !strings.Contains(err.Error(), "error parsing environment variables") {
		t.Fatalf("expected env parse error, got %v", err)
	}
}

func TestMailpitCommandRunsDefaultBinary(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	if err := executeStandaloneCommand(newMailpitCommand()); err == nil || !strings.Contains(err.Error(), "mailpit binary not found") {
		t.Fatalf("expected missing mailpit error, got %v", err)
	}

	writeExecutable(t, root, "bin/mailpit", "#!/bin/sh\nprintf '%s\\n' \"$@\" > mailpit.args\n")
	if err := executeStandaloneCommand(newMailpitCommand()); err != nil {
		t.Fatalf("mailpit command: %v", err)
	}
	assertTestFileContains(t, root, "mailpit.args", "--smtp=0.0.0.0:1025")
	assertTestFileContains(t, root, "mailpit.args", "--listen=0.0.0.0:8025")
}

func TestRunFmtOrchestratesFormattersAndErrors(t *testing.T) {
	resetCLITestSeams(t)

	var calls []string
	runGoFmtFunc = func(rootDir string, checkMode bool) error {
		calls = append(calls, "gofmt")
		return nil
	}
	runGolinesFunc = func(rootDir string, checkMode bool) error {
		calls = append(calls, "golines")
		return errors.New("wrap failed")
	}
	runTemplFmtFunc = func(rootDir string, checkMode bool) error {
		calls = append(calls, "templ")
		return nil
	}

	err := runFmt(t.TempDir(), true, false, false)
	if err == nil || !strings.Contains(err.Error(), "some files need formatting") {
		t.Fatalf("expected check mode error, got %v", err)
	}
	if !reflect.DeepEqual(calls, []string{"gofmt", "golines", "templ"}) {
		t.Fatalf("formatter calls = %#v", calls)
	}

	calls = nil
	err = runFmt(t.TempDir(), false, true, true)
	if err != nil {
		t.Fatalf("skip all formatters: %v", err)
	}
	if len(calls) != 0 {
		t.Fatalf("expected skipped formatters, got %#v", calls)
	}
}

func TestFormatterHelpers(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)
	writeTestFile(t, root, "main.go", "package main\n\nfunc main() {}\n")
	writeTestFile(t, root, ".git/ignored.go", "package ignored\n")
	writeTestFile(t, root, "vendor/example.com/ignored/ignored.go", "package ignored\n")
	writeTestFile(t, root, ".hidden/ignored.go", "package ignored\n")
	writeTestFile(t, root, "_tools/ignored.go", "package ignored\n")
	writeTestFile(t, root, "testdata/ignored.go", "package ignored\n")

	if err := runGoFmt(root, true); err != nil {
		t.Fatalf("runGoFmt check: %v", err)
	}

	files, err := collectGoFiles(root)
	if err != nil {
		t.Fatalf("collectGoFiles: %v", err)
	}
	if len(files) != 1 || filepath.Base(files[0]) != "main.go" {
		t.Fatalf("collectGoFiles = %#v", files)
	}

	pathDir := t.TempDir()
	t.Setenv("PATH", pathDir)
	if err := runGolines(root, true); err == nil || !strings.Contains(err.Error(), "golines not found") {
		t.Fatalf("missing golines should fail check mode, got: %v", err)
	}

	if err := runTemplFmt(root, false); err != nil {
		t.Fatalf("missing templ should be skipped: %v", err)
	}
	writeExecutable(t, root, "bin/templ", "#!/bin/sh\nprintf '%s\\n' \"$@\" >> templ.args\n")
	writeTestFile(t, root, "views/home.templ", "package views\n")
	writeTestFile(t, root, "email/welcome.templ", "package email\n")
	if err := runTemplFmt(root, false); err != nil {
		t.Fatalf("runTemplFmt: %v", err)
	}
	assertTestFileContains(t, root, "templ.args", "fmt")
	assertTestFileContains(t, root, "templ.args", "views")
	assertTestFileContains(t, root, "templ.args", "email")
}

func TestSyncBinariesCleansStaleToolsAndSkipsCurrentVersions(t *testing.T) {
	resetCLITestSeams(t)
	root := t.TempDir()
	writeGoModule(t, root)
	writeExecutable(t, root, "bin/alpha", "#!/bin/sh\necho alpha v1.2.3\n")
	writeExecutable(t, root, "bin/stale", "#!/bin/sh\n")
	lock := layout.NewAndurelLock("test")
	lock.Tools["alpha"] = &layout.Tool{
		Version:      "v1.2.3",
		VersionCheck: &layout.VersionCheck{Args: []string{"--version"}},
		Download:     validTestDownload("alpha"),
	}
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	if err := syncBinaries(root); err != nil {
		t.Fatalf("syncBinaries: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "bin", "stale")); !os.IsNotExist(err) {
		t.Fatalf("expected stale binary to be removed, stat err=%v", err)
	}
}

func TestSyncSingleToolDownloadsMissingAndOutdatedTools(t *testing.T) {
	resetCLITestSeams(t)
	root := t.TempDir()
	writeGoModule(t, root)

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	var downloads []string
	downloadFromLockToolFunc = func(name string, tool *layout.Tool, goos, goarch, binPath string) error {
		downloads = append(downloads, name+":"+tool.Version)
		if err := os.MkdirAll(filepath.Dir(binPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(binPath, []byte("#!/bin/sh\necho "+tool.Version+"\n"), 0o755)
	}

	tool := &layout.Tool{
		Version:      "v2.0.0",
		VersionCheck: &layout.VersionCheck{Args: []string{"--version"}},
		Download:     validTestDownload("tool"),
	}
	if err := syncSingleTool(root, "alpha", tool, "linux", "amd64"); err != nil {
		t.Fatalf("sync missing tool: %v", err)
	}

	writeExecutable(t, root, "bin/beta", "#!/bin/sh\necho beta v1.0.0\n")
	if err := syncSingleTool(root, "beta", tool, "linux", "amd64"); err != nil {
		t.Fatalf("sync outdated tool: %v", err)
	}

	if !reflect.DeepEqual(downloads, []string{"alpha:v2.0.0", "beta:v2.0.0"}) {
		t.Fatalf("downloads = %#v", downloads)
	}
}

func TestSyncSingleToolVerifiesVersionBeforeAtomicReplacement(t *testing.T) {
	resetCLITestSeams(t)
	root := t.TempDir()
	writeGoModule(t, root)
	writeExecutable(t, root, "bin/tool", "#!/bin/sh\necho v1.0.0\n")
	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() { findGoModRoot = originalFindGoModRoot })
	downloadFromLockToolFunc = func(name string, tool *layout.Tool, goos, goarch, binPath string) error {
		return os.WriteFile(binPath, []byte("#!/bin/sh\necho v9.9.9\n"), 0o755)
	}

	tool := validTestTool("tool", "v2.0.0")
	if err := syncSingleTool(root, "tool", tool, "linux", "amd64"); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("version mismatch error = %v", err)
	}
	assertTestFileContains(t, root, "bin/tool", "v1.0.0")
	entries, err := os.ReadDir(filepath.Join(root, "bin"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".andurel-candidate-") {
			t.Fatalf("candidate remains after verification failure: %s", entry.Name())
		}
	}
}

func TestSyncBinariesHandlesDownloadLookupFailures(t *testing.T) {
	resetCLITestSeams(t)
	root := t.TempDir()
	writeGoModule(t, root)
	lock := layout.NewAndurelLock("test")
	lock.Tools["missing-release"] = validTestTool("missing-release", "v9.9.9")
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	downloadFromLockToolFunc = func(string, *layout.Tool, string, string, string) error {
		return cmds.ErrFailedToGetRleaseURL
	}

	if err := syncBinaries(root); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("release lookup failure should fail synchronization: %v", err)
	}
}

func TestSetVersionAddsManagedToolsAndRejectsInvalidInput(t *testing.T) {
	resetCLITestSeams(t)
	root := t.TempDir()
	writeGoModule(t, root)
	lock := layout.NewAndurelLock("test")
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	var synced []string
	installToolVersionAndLockFunc = func(projectRoot, name string, tool *layout.Tool, lock *layout.AndurelLock, goos, goarch string) error {
		synced = append(synced, name+":"+tool.Version)
		return lock.WriteLockFile(projectRoot)
	}

	if err := setVersion(root, "tailwindcli", "v4.1.0", testChecksumArguments()...); err != nil {
		t.Fatalf("setVersion: %v", err)
	}
	if !reflect.DeepEqual(synced, []string{"tailwindcli:v4.1.0"}) {
		t.Fatalf("synced = %#v", synced)
	}
	read, err := layout.ReadLockFile(root)
	if err != nil {
		t.Fatalf("read lock: %v", err)
	}
	if read.Tools["tailwindcli"].Version != "v4.1.0" || read.Tools["tailwindcli"].Download == nil {
		t.Fatalf("unexpected tool lock entry: %#v", read.Tools["tailwindcli"])
	}

	if err := setVersion(root, "tailwindcli", ""); err == nil {
		t.Fatalf("expected empty version error")
	}
	if err := setVersion(root, "unknown-tool", "1.0.0"); err == nil || !strings.Contains(err.Error(), "unknown tool") {
		t.Fatalf("expected unknown tool error, got %v", err)
	}
}

func TestParseRepeatedChecksumArguments(t *testing.T) {
	checksums, err := parseChecksumArguments(testChecksumArguments())
	if err != nil {
		t.Fatalf("parseChecksumArguments: %v", err)
	}
	if len(checksums) != 4 || checksums["linux/amd64"] != strings.Repeat("1", 64) {
		t.Fatalf("checksums = %#v", checksums)
	}

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing platform", args: testChecksumArguments()[:3], want: "missing --sha256"},
		{name: "duplicate", args: append(testChecksumArguments(), testChecksumArguments()[0]), want: "duplicate"},
		{name: "unsupported", args: []string{"freebsd/amd64=" + strings.Repeat("1", 64)}, want: "unsupported"},
		{name: "malformed assignment", args: []string{"linux/amd64"}, want: "expected os/arch"},
		{name: "malformed digest", args: []string{"linux/amd64=xyz"}, want: "invalid SHA-256"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parseChecksumArguments(test.args); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestCustomToolVersionRequiresChecksumsBeforeMutation(t *testing.T) {
	resetCLITestSeams(t)
	root := t.TempDir()
	writeGoModule(t, root)
	lock := layout.NewAndurelLock("test")
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatal(err)
	}
	installToolVersionAndLockFunc = func(string, string, *layout.Tool, *layout.AndurelLock, string, string) error {
		t.Fatal("sync must not run without complete checksums")
		return nil
	}
	if err := setVersion(root, "tailwindcli", "v4.1.0"); err == nil || !strings.Contains(err.Error(), "requires four repeated") {
		t.Fatalf("missing checksum error = %v", err)
	}
	persisted, err := layout.ReadLockFile(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Tools) != 0 {
		t.Fatalf("lock mutated after checksum failure: %#v", persisted.Tools)
	}
}

func TestSetVersionCommitsLockAndBinaryTogether(t *testing.T) {
	tests := []struct {
		name          string
		downloadedVer string
		wantErr       bool
	}{
		{name: "success", downloadedVer: "v4.1.0"},
		{name: "version mismatch", downloadedVer: "v9.9.9", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetCLITestSeams(t)
			root := t.TempDir()
			writeGoModule(t, root)
			lock := layout.NewAndurelLock("test")
			if err := lock.WriteLockFile(root); err != nil {
				t.Fatal(err)
			}
			writeExecutable(t, root, "bin/tailwindcli", "#!/bin/sh\necho v0.1.0\n")
			downloadFromLockToolFunc = func(name string, tool *layout.Tool, goos, goarch, path string) error {
				return os.WriteFile(path, []byte("#!/bin/sh\necho "+test.downloadedVer+"\n"), 0o755)
			}

			err := setVersion(root, "tailwindcli", "v4.1.0", testChecksumArguments()...)
			if test.wantErr {
				if err == nil || !strings.Contains(err.Error(), "does not match") {
					t.Fatalf("version mismatch error = %v", err)
				}
				persisted, readErr := layout.ReadLockFile(root)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if len(persisted.Tools) != 0 {
					t.Fatalf("lock changed after failed install: %#v", persisted.Tools)
				}
				assertTestFileContains(t, root, "bin/tailwindcli", "v0.1.0")
			} else {
				if err != nil {
					t.Fatalf("setVersion: %v", err)
				}
				persisted, readErr := layout.ReadLockFile(root)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if persisted.Tools["tailwindcli"].Version != "v4.1.0" {
					t.Fatalf("persisted tool = %#v", persisted.Tools["tailwindcli"])
				}
				assertTestFileContains(t, root, "bin/tailwindcli", "v4.1.0")
			}
			entries, readErr := os.ReadDir(root)
			if readErr != nil {
				t.Fatal(readErr)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".andurel-lock-") {
					t.Fatalf("lock staging data remains: %s", entry.Name())
				}
			}
		})
	}
}

func TestDownloadFromLockToolRejectsMissingAndUnsupportedPlatformDigests(t *testing.T) {
	tool := validTestTool("tool", "v1.2.3")
	delete(tool.Download.SHA256, "linux/arm64")
	if err := downloadFromLockTool("tool", tool, "linux", "arm64", filepath.Join(t.TempDir(), "tool")); err == nil || !strings.Contains(err.Error(), "missing SHA-256") {
		t.Fatalf("missing digest error = %v", err)
	}
	if err := downloadFromLockTool("tool", validTestTool("tool", "v1.2.3"), "freebsd", "amd64", filepath.Join(t.TempDir(), "tool")); err == nil || !strings.Contains(err.Error(), "unsupported platform") {
		t.Fatalf("unsupported platform error = %v", err)
	}
}

func TestNewProjectValidatesInertiaAndBuildsReports(t *testing.T) {
	cmd := newProjectCommand("test")
	if err := cmd.Flags().Set("inertia", "svelte"); err != nil {
		t.Fatalf("set inertia: %v", err)
	}
	if err := newProject(cmd, []string{"app"}, "test", true, false); err == nil || !strings.Contains(err.Error(), "invalid inertia adapter") {
		t.Fatalf("expected invalid inertia adapter, got %v", err)
	}

	cmd = newProjectCommand("test")
	if err := cmd.Flags().Set("inertia", "vue/deno"); err != nil {
		t.Fatalf("set inertia runtime: %v", err)
	}
	if err := newProject(cmd, []string{"app"}, "test", true, false); err == nil || !strings.Contains(err.Error(), "invalid JavaScript runtime") {
		t.Fatalf("expected invalid runtime, got %v", err)
	}

	root := t.TempDir()
	report, err := newProjectReport("app", filepath.Join(root, "app"), false, false, func(target string) error {
		writeTestFile(t, target, "go.mod", "module example.com/app\n")
		writeTestFile(t, target, "main.go", "package main\n")
		return nil
	})
	if err != nil {
		t.Fatalf("newProjectReport: %v", err)
	}
	if report.Action != "new project" || report.Resource != "app" || len(report.FilesCreated) != 2 {
		t.Fatalf("unexpected report: %#v", report)
	}

	nonEmpty := t.TempDir()
	writeTestFile(t, nonEmpty, "existing.txt", "content")
	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(nonEmpty); err != nil {
		t.Fatalf("chdir non-empty: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(original)
	})
	cmd = newProjectCommand("test")
	if err := newProject(cmd, []string{"."}, "test", true, false); err == nil || !strings.Contains(err.Error(), "current directory is not empty") {
		t.Fatalf("expected non-empty current directory error, got %v", err)
	}
}

func TestRootHelpAndBinaryChecks(t *testing.T) {
	resetCLITestSeams(t)
	root := t.TempDir()
	writeGoModule(t, root)

	originalFindGoModRoot := findGoModRoot
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	findGoModRoot = func() (string, error) { return "", errors.New("no project") }
	outside := captureProcessOutput(t, &os.Stdout)
	cmd := NewRootCommand("test", "date")
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("root help outside project: %v", err)
	}
	if output := outside(); !strings.Contains(output, "You must specify a command") {
		t.Fatalf("outside project help missing guidance:\n%s", output)
	}
	if isInAndurelProject() {
		t.Fatalf("expected isInAndurelProject false")
	}

	findGoModRoot = func() (string, error) { return root, nil }
	inside := captureProcessOutput(t, &os.Stdout)
	cmd = NewRootCommand("test", "date")
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("root help inside project: %v", err)
	}
	if output := inside(); strings.TrimSpace(output) == "" {
		t.Fatalf("inside project help should print the banner:\n%s", output)
	}
	if !isInAndurelProject() {
		t.Fatalf("expected isInAndurelProject true")
	}

	if err := checkBinaries(root); err != nil {
		t.Fatalf("no lock should not require binaries: %v", err)
	}
	writeTestFile(t, root, "andurel.lock", "")
	if err := checkBinaries(root); err != nil {
		t.Fatalf("empty lock should not require binaries: %v", err)
	}
	writeTestFile(t, root, "andurel.lock", "{}")
	if err := checkBinaries(root); err == nil || !strings.Contains(err.Error(), "bin/shadowfax not found") {
		t.Fatalf("expected missing shadowfax error, got %v", err)
	}
	writeExecutable(t, root, "bin/shadowfax", "#!/bin/sh\nprintf run > shadowfax.args\n")
	if err := checkBinaries(root); err != nil {
		t.Fatalf("shadowfax present: %v", err)
	}
	if err := executeStandaloneCommand(newRunAppCommand()); err != nil {
		t.Fatalf("run app command: %v", err)
	}
	assertTestFileContains(t, root, "shadowfax.args", "run")
}

func TestStandardHelpRendering(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "Tool summary",
		Long:  "Long tool help.",
	}
	cmd.Flags().Bool("local", false, "local flag")
	cmd.AddCommand(&cobra.Command{Use: "child", Short: "Child command"})
	setStandardHelp(cmd, helpCommand{Use: "sync", Description: "Sync tools"})

	capture := captureProcessOutput(t, &os.Stdout)
	if err := cmd.Help(); err != nil {
		t.Fatalf("render owner help: %v", err)
	}
	if got := capture(); !strings.Contains(got, "Long tool help.") || !strings.Contains(got, "sync") {
		t.Fatalf("owner help missing custom sections:\n%s", got)
	}

	child := cmd.Commands()[0]
	capture = captureProcessOutput(t, &os.Stdout)
	if err := child.Help(); err != nil {
		t.Fatalf("render child help: %v", err)
	}
	if got := capture(); !strings.Contains(got, "Child command") {
		t.Fatalf("child help missing short text:\n%s", got)
	}
}

func TestRunTemplAndToolListCommands(t *testing.T) {
	resetCLITestSeams(t)
	root := t.TempDir()
	writeGoModule(t, root)

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	if err := runTempl("generate"); err == nil || !strings.Contains(err.Error(), "templ binary not found") {
		t.Fatalf("expected missing templ error, got %v", err)
	}
	writeExecutable(t, root, "bin/templ", "#!/bin/sh\nprintf '%s\\n' \"$@\" > templ.args\n")
	if err := runTempl("generate", "./views"); err != nil {
		t.Fatalf("runTempl: %v", err)
	}
	assertTestFileContains(t, root, "templ.args", "generate")
	assertTestFileContains(t, root, "templ.args", "./views")

	lock := layout.NewAndurelLock("test")
	lock.Tools["templ"] = validTestTool("templ", "v0.3.0")
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	var out bytes.Buffer
	cmd := newToolCommand()
	output.RegisterPersistentFlags(cmd)
	cmd.SetOut(&out)
	_ = cmd.PersistentFlags().Set("json", "true")
	if err := cmd.Execute(); err != nil {
		t.Fatalf("tool list root command: %v", err)
	}
	if !strings.Contains(out.String(), "Listed tools") || !strings.Contains(out.String(), "templ") {
		t.Fatalf("tool output missing data:\n%s", out.String())
	}

	out.Reset()
	cmd = newToolCommand()
	output.RegisterPersistentFlags(cmd)
	cmd.SetOut(&out)
	_ = cmd.PersistentFlags().Set("json", "true")
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("tool list subcommand: %v", err)
	}
	if !strings.Contains(out.String(), "Listed tools") {
		t.Fatalf("tool list output missing summary:\n%s", out.String())
	}
}

func TestModelPromptAndDiffOutput(t *testing.T) {
	originalStdin := os.Stdin
	t.Cleanup(func() { os.Stdin = originalStdin })

	os.Stdin = tempInputFile(t, "yes\n")
	confirmed, err := confirmModelApply()
	if err != nil {
		t.Fatalf("confirmModelApply yes: %v", err)
	}
	if !confirmed {
		t.Fatalf("expected yes to confirm")
	}

	os.Stdin = tempInputFile(t, "\n")
	confirmed, err = confirmModelApply()
	if err != nil {
		t.Fatalf("confirmModelApply default: %v", err)
	}
	if confirmed {
		t.Fatalf("expected blank response to decline")
	}

	capture := captureProcessOutput(t, &os.Stdout)
	printColoredDiff("--- old\n+++ new\n@@ hunk\n-removed\n+added\n context")
	out := capture()
	for _, want := range []string{"--- old", "+++ new", "@@ hunk", "removed", "added", "context"} {
		if !strings.Contains(out, want) {
			t.Fatalf("diff output missing %q:\n%s", want, out)
		}
	}
}

func TestFrameworkRepairPrompt(t *testing.T) {
	originalStdin := os.Stdin
	t.Cleanup(func() { os.Stdin = originalStdin })

	os.Stdin = tempInputFile(t, "yes\n")
	confirmed, err := confirmFrameworkRepair()
	if err != nil {
		t.Fatalf("confirmFrameworkRepair yes: %v", err)
	}
	if !confirmed {
		t.Fatal("expected yes to confirm framework repair")
	}

	os.Stdin = tempInputFile(t, "\n")
	confirmed, err = confirmFrameworkRepair()
	if err != nil {
		t.Fatalf("confirmFrameworkRepair default: %v", err)
	}
	if confirmed {
		t.Fatal("expected blank response to decline framework repair")
	}
}

func TestRunUpgradeStructuredAndHumanBranches(t *testing.T) {
	resetCLITestSeams(t)
	stubLatestAndurelVersion(t, "v2.0.0", nil)
	root := t.TempDir()
	writeGoModule(t, root)

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	var gotOpts upgrade.UpgradeOptions
	newUpgraderFunc = func(projectRoot string, opts upgrade.UpgradeOptions) (cliUpgrader, error) {
		if projectRoot != root {
			t.Fatalf("projectRoot = %q, want %q", projectRoot, root)
		}
		gotOpts = opts
		return fakeUpgrader{report: &upgrade.UpgradeReport{
			Success:       true,
			ReplacedFiles: []string{"controllers/controller.go"},
			RemovedFiles:  []string{"internal/old.go"},
			ToolsUpdated:  1,
		}}, nil
	}

	var out bytes.Buffer
	cmd := newStructuredTestCommand(&out)
	cmd.Flags().Bool("dry-run", false, "")
	cmd.Flags().Bool("diff", false, "")
	if err := cmd.Flags().Set("dry-run", "true"); err != nil {
		t.Fatalf("set dry-run: %v", err)
	}
	if err := cmd.Flags().Set("diff", "true"); err != nil {
		t.Fatalf("set diff: %v", err)
	}
	if err := runUpgrade(cmd, "v2.0.0"); err != nil {
		t.Fatalf("structured runUpgrade: %v", err)
	}
	if !gotOpts.DryRun || gotOpts.TargetVersion != "v2.0.0" {
		t.Fatalf("upgrade opts = %#v", gotOpts)
	}
	if !strings.Contains(out.String(), "dry run only") || !strings.Contains(out.String(), "controllers/controller.go") {
		t.Fatalf("structured upgrade output missing report data:\n%s", out.String())
	}

	var synced bool
	syncSingleToolFunc = func(projectRoot, name string, tool *layout.Tool, goos, goarch string) error {
		synced = true
		return nil
	}
	lock := layout.NewAndurelLock("v1.0.0")
	lock.Tools["templ"] = validTestTool("templ", "v0.1.0")
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	writeExecutable(t, root, "bin/templ", "#!/bin/sh\necho templ v0.1.0\n")
	human := &cobra.Command{Use: "upgrade"}
	human.Flags().Bool("dry-run", false, "")
	human.Flags().Bool("diff", false, "")
	if err := runUpgrade(human, "v2.0.0"); err != nil {
		t.Fatalf("human runUpgrade: %v", err)
	}
	if !synced {
		t.Fatalf("expected successful human upgrade with tool changes to sync tools")
	}
}

func TestRunUpgradeErrors(t *testing.T) {
	resetCLITestSeams(t)
	stubLatestAndurelVersion(t, "v2.0.0", nil)
	root := t.TempDir()
	writeGoModule(t, root)

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	cmd := &cobra.Command{Use: "upgrade"}
	cmd.Flags().Bool("dry-run", false, "")
	cmd.Flags().Bool("diff", false, "")

	newUpgraderFunc = func(string, upgrade.UpgradeOptions) (cliUpgrader, error) {
		return nil, errors.New("init failed")
	}
	if err := runUpgrade(cmd, "v2.0.0"); err == nil || !strings.Contains(err.Error(), "failed to initialize upgrader") {
		t.Fatalf("expected init error, got %v", err)
	}

	newUpgraderFunc = func(string, upgrade.UpgradeOptions) (cliUpgrader, error) {
		return fakeUpgrader{err: errors.New("execute failed")}, nil
	}
	if err := runUpgrade(cmd, "v2.0.0"); err == nil || !strings.Contains(err.Error(), "execute failed") {
		t.Fatalf("expected execute error, got %v", err)
	}
}

func TestRunUpgradeReleaseRequirementAndRepairBypass(t *testing.T) {
	resetCLITestSeams(t)
	root := t.TempDir()
	writeGoModule(t, root)
	findGoModRoot = func() (string, error) { return root, nil }

	cmd := &cobra.Command{Use: "upgrade"}
	cmd.Flags().Bool("dry-run", false, "")
	cmd.Flags().Bool("diff", false, "")
	cmd.Flags().Bool("repair", false, "")

	stubLatestAndurelVersion(t, "v2.1.0", nil)
	initialized := false
	newUpgraderFunc = func(string, upgrade.UpgradeOptions) (cliUpgrader, error) {
		initialized = true
		return fakeUpgrader{report: &upgrade.UpgradeReport{AlreadyCurrent: true}}, nil
	}

	err := runUpgrade(cmd, "v2.0.0")
	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) || cliErr.Code != output.CodeUpdateRequired {
		t.Fatalf("release requirement error = %v", err)
	}
	if initialized {
		t.Fatal("upgrader initialized before satisfying the CLI release requirement")
	}

	if err := cmd.Flags().Set("repair", "true"); err != nil {
		t.Fatalf("set repair: %v", err)
	}
	if err := runUpgrade(cmd, "v2.0.0"); err != nil {
		t.Fatalf("repair should bypass release requirement: %v", err)
	}
	if !initialized {
		t.Fatal("repair did not initialize the upgrader")
	}
}

func TestExtensionListCommandHumanAndStructured(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)
	lock := layout.NewAndurelLock("test")
	lock.AddExtension("docker", "2026-07-08")
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	capture := captureProcessOutput(t, &os.Stdout)
	cmd := &cobra.Command{Use: "extension"}
	if err := runExtensionList(cmd, false); err != nil {
		t.Fatalf("runExtensionList human: %v", err)
	}
	if out := capture(); !strings.Contains(out, "docker (applied: 2026-07-08)") {
		t.Fatalf("human extension output missing applied extension:\n%s", out)
	}

	var out bytes.Buffer
	cmd = newExtensionListCommand()
	output.RegisterPersistentFlags(cmd)
	cmd.SetOut(&out)
	_ = cmd.PersistentFlags().Set("json", "true")
	if err := cmd.Execute(); err != nil {
		t.Fatalf("extension list command: %v", err)
	}
	if !strings.Contains(out.String(), "Listed extensions") || !strings.Contains(out.String(), "docker") {
		t.Fatalf("structured extension output missing data:\n%s", out.String())
	}

	emptyRoot := t.TempDir()
	writeGoModule(t, emptyRoot)
	emptyLock := layout.NewAndurelLock("test")
	if err := emptyLock.WriteLockFile(emptyRoot); err != nil {
		t.Fatalf("write empty lock: %v", err)
	}
	findGoModRoot = func() (string, error) { return emptyRoot, nil }
	capture = captureProcessOutput(t, &os.Stdout)
	cmd = &cobra.Command{Use: "extension"}
	if err := runExtensionList(cmd, false); err != nil {
		t.Fatalf("runExtensionList empty: %v", err)
	}
	if out := capture(); !strings.Contains(out, "No extensions applied") {
		t.Fatalf("empty extension output missing message:\n%s", out)
	}
}

func executeStandaloneCommand(cmd *cobra.Command, args ...string) error {
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	return cmd.Execute()
}

type fakeUpgrader struct {
	report *upgrade.UpgradeReport
	err    error
}

func (f fakeUpgrader) Execute() (*upgrade.UpgradeReport, error) {
	if f.report != nil {
		return f.report, f.err
	}
	return &upgrade.UpgradeReport{}, f.err
}

func assertTestFileContains(t *testing.T, root, rel, want string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("%s missing %q:\n%s", rel, want, string(data))
	}
}
