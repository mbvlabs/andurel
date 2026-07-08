package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/spf13/cobra"
)

func TestLoadDatabaseConfigAndBuildURL(t *testing.T) {
	setDatabaseEnv(t)

	cfg, err := loadDatabaseConfig()
	if err != nil {
		t.Fatalf("loadDatabaseConfig: %v", err)
	}
	if cfg.Kind != "postgres" || cfg.Name != "andurel_test" || cfg.SslMode != "disable" {
		t.Fatalf("unexpected config: %#v", cfg)
	}

	driver, dbURL, err := buildDatabaseURL()
	if err != nil {
		t.Fatalf("buildDatabaseURL: %v", err)
	}
	if driver != "postgres" {
		t.Fatalf("driver = %q, want postgres", driver)
	}
	want := "postgres://andurel:secret@127.0.0.1:5432/andurel_test?sslmode=disable"
	if dbURL != want {
		t.Fatalf("dbURL = %q, want %q", dbURL, want)
	}
}

func TestLoadDatabaseConfigReportsMissingVariables(t *testing.T) {
	t.Setenv("DB_KIND", "")
	t.Setenv("DB_PORT", "")
	t.Setenv("DB_HOST", "")
	t.Setenv("DB_NAME", "")
	t.Setenv("DB_USER", "")
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("DB_SSL_MODE", "")

	_, err := loadDatabaseConfig()
	if err == nil {
		t.Fatalf("expected missing config error")
	}
	for _, name := range []string{"DB_KIND", "DB_PORT", "DB_HOST", "DB_NAME", "DB_USER", "DB_PASSWORD", "DB_SSL_MODE"} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("missing error did not mention %s: %v", name, err)
		}
	}
}

func TestDatabaseHelpers(t *testing.T) {
	for _, name := range []string{"postgres", "template0", "template1", "POSTGRES"} {
		if !isSystemDatabase(name) {
			t.Fatalf("%q should be a system database", name)
		}
	}
	if isSystemDatabase("app") {
		t.Fatalf("app should not be a system database")
	}
	if got, want := quoteIdentifier(`tenant"one`), `"tenant""one"`; got != want {
		t.Fatalf("quoteIdentifier = %q, want %q", got, want)
	}
	if got := splitNonEmptyLines("\n alpha \n\n beta\n"); !reflect.DeepEqual(got, []string{"alpha", "beta"}) {
		t.Fatalf("splitNonEmptyLines = %#v", got)
	}
}

func TestLoadProjectEnv(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("DB_KIND=postgres\nDB_NAME=from_env\n"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	unsetEnvForTest(t, "DB_KIND", "DB_NAME")

	loadProjectEnv(root)

	if os.Getenv("DB_KIND") != "postgres" || os.Getenv("DB_NAME") != "from_env" {
		t.Fatalf("expected .env values to load, got kind=%q name=%q", os.Getenv("DB_KIND"), os.Getenv("DB_NAME"))
	}
}

func unsetEnvForTest(t *testing.T, names ...string) {
	t.Helper()
	type originalEnv struct {
		name   string
		value  string
		exists bool
	}
	originals := make([]originalEnv, 0, len(names))
	for _, name := range names {
		value, exists := os.LookupEnv(name)
		originals = append(originals, originalEnv{name: name, value: value, exists: exists})
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("unset %s: %v", name, err)
		}
	}
	t.Cleanup(func() {
		for _, original := range originals {
			if original.exists {
				_ = os.Setenv(original.name, original.value)
			} else {
				_ = os.Unsetenv(original.name)
			}
		}
	})
}

func TestConfirmDestructiveUsesStdin(t *testing.T) {
	originalStdin := os.Stdin
	originalStdout := os.Stdout
	t.Cleanup(func() {
		os.Stdin = originalStdin
		os.Stdout = originalStdout
	})

	os.Stdout = tempInputFile(t, "")
	os.Stdin = tempInputFile(t, "yes\n")
	confirmed, err := confirmDestructive("drop", "postgres://example")
	if err != nil {
		t.Fatalf("confirmDestructive yes: %v", err)
	}
	if !confirmed {
		t.Fatalf("expected yes to confirm")
	}

	os.Stdin = tempInputFile(t, "n\n")
	confirmed, err = confirmDestructive("drop", "postgres://example")
	if err != nil {
		t.Fatalf("confirmDestructive no: %v", err)
	}
	if confirmed {
		t.Fatalf("expected n to abort")
	}

	if _, err := confirmDestructive("drop", " "); err == nil {
		t.Fatalf("expected empty database URL error")
	}
}

func TestRunSeedStructuredListAndNamedSeed(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/app\n")
	writeTestFile(t, root, "cmd/seeds/main.go", "package main\n")

	originalFindGoModRoot := findGoModRoot
	originalRunner := runSeedCommandOutput
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
		runSeedCommandOutput = originalRunner
	})

	var calls [][]string
	runSeedCommandOutput = func(rootDir string, args []string, stdin io.Reader, stdout, stderr io.Writer) ([]byte, error) {
		if rootDir != root {
			t.Fatalf("rootDir = %q, want %q", rootDir, root)
		}
		calls = append(calls, append([]string(nil), args...))
		return []byte("alpha\n\nbeta\n"), nil
	}

	var out bytes.Buffer
	cmd := newStructuredTestCommand(&out)
	if err := runSeed(cmd, "", true); err != nil {
		t.Fatalf("runSeed list: %v", err)
	}
	var listEnvelope output.Envelope
	if err := json.Unmarshal(out.Bytes(), &listEnvelope); err != nil {
		t.Fatalf("decode list envelope: %v\n%s", err, out.String())
	}
	if !strings.Contains(listEnvelope.Summary, "Found 2 seed sets") {
		t.Fatalf("unexpected list summary: %q", listEnvelope.Summary)
	}
	if !reflect.DeepEqual(calls[0], []string{"run", "./cmd/seeds", "--list"}) {
		t.Fatalf("list command args = %#v", calls[0])
	}

	out.Reset()
	cmd = newStructuredTestCommand(&out)
	if err := runSeed(cmd, "development", false); err != nil {
		t.Fatalf("runSeed named: %v", err)
	}
	var seedEnvelope output.Envelope
	if err := json.Unmarshal(out.Bytes(), &seedEnvelope); err != nil {
		t.Fatalf("decode seed envelope: %v\n%s", err, out.String())
	}
	if !strings.Contains(seedEnvelope.Summary, `Ran "development" seed`) {
		t.Fatalf("unexpected seed summary: %q", seedEnvelope.Summary)
	}
	if !reflect.DeepEqual(calls[1], []string{"run", "./cmd/seeds", "development"}) {
		t.Fatalf("seed command args = %#v", calls[1])
	}
}

func TestRunSeedStructuredErrorAndMissingEntrypoint(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/app\n")

	originalFindGoModRoot := findGoModRoot
	originalRunner := runSeedCommandOutput
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
		runSeedCommandOutput = originalRunner
	})

	cmd := newStructuredTestCommand(&bytes.Buffer{})
	err := runSeed(cmd, "", false)
	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) || cliErr.Code != output.CodeMissingTool {
		t.Fatalf("missing entrypoint error = %T %[1]v", err)
	}

	writeTestFile(t, root, "cmd/seeds/main.go", "package main\n")
	runSeedCommandOutput = func(string, []string, io.Reader, io.Writer, io.Writer) ([]byte, error) {
		return []byte("boom\n"), errors.New("exit status 1")
	}
	err = runSeed(cmd, "", false)
	if !errors.As(err, &cliErr) || cliErr.Code != output.CodeExternalCommandFailed || !strings.Contains(cliErr.Hint, "boom") {
		t.Fatalf("seed failure error = %#v", err)
	}
}

func TestRunGooseBuildsCommand(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/app\n")
	writeTestFile(t, root, ".env", strings.Join([]string{
		"DB_KIND=postgres",
		"DB_PORT=5432",
		"DB_HOST=localhost",
		"DB_NAME=app_db",
		"DB_USER=app_user",
		"DB_PASSWORD=secret",
		"DB_SSL_MODE=disable",
		"",
	}, "\n"))
	writeTestFile(t, root, "bin/goose", "#!/bin/sh\n")
	writeTestFile(t, root, "database/migrations/0001_init.sql", "-- noop\n")
	unsetEnvForTest(t, "DB_KIND", "DB_PORT", "DB_HOST", "DB_NAME", "DB_USER", "DB_PASSWORD", "DB_SSL_MODE")

	originalFindGoModRoot := findGoModRoot
	originalRunGooseCommand := runGooseCommand
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
		runGooseCommand = originalRunGooseCommand
	})

	var gotRoot string
	var gotPath string
	var gotArgs []string
	runGooseCommand = func(rootDir, goosePath string, args []string) error {
		gotRoot = rootDir
		gotPath = goosePath
		gotArgs = append([]string(nil), args...)
		return nil
	}

	if err := runGoose("up-to", "20260708120000"); err != nil {
		t.Fatalf("runGoose: %v", err)
	}
	if gotRoot != root {
		t.Fatalf("goose root = %q, want %q", gotRoot, root)
	}
	if gotPath != filepath.Join(root, "bin", "goose") {
		t.Fatalf("goose path = %q", gotPath)
	}
	wantPrefix := []string{
		"-dir",
		filepath.Join(root, "database", "migrations"),
		"postgres",
		"postgres://app_user:secret@localhost:5432/app_db?sslmode=disable",
		"up-to",
		"20260708120000",
	}
	if !reflect.DeepEqual(gotArgs, wantPrefix) {
		t.Fatalf("goose args = %#v, want %#v", gotArgs, wantPrefix)
	}
}

func TestRunGooseMissingBinary(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/app\n")
	setDatabaseEnv(t)

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	err := runGoose("status")
	if err == nil || !strings.Contains(err.Error(), "goose binary not found") {
		t.Fatalf("expected missing goose error, got %v", err)
	}
}

func TestMigrationCommandsCallGoose(t *testing.T) {
	originalRunGooseCommand := runGooseCommand
	originalFindGoModRoot := findGoModRoot
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/app\n")
	writeTestFile(t, root, "bin/goose", "#!/bin/sh\n")
	writeTestFile(t, root, "database/migrations/0001_init.sql", "-- noop\n")
	setDatabaseEnv(t)
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
		runGooseCommand = originalRunGooseCommand
	})

	var calls [][]string
	runGooseCommand = func(_ string, _ string, args []string) error {
		calls = append(calls, append([]string(nil), args...))
		return nil
	}

	tests := []struct {
		name string
		cmd  *cobra.Command
		args []string
		want []string
	}{
		{name: "new", cmd: newDBMigrationNewCommand(), args: []string{"create_users"}, want: []string{"create", "create_users", "sql"}},
		{name: "up", cmd: newDBMigrationUpCommand(), want: []string{"up"}},
		{name: "down", cmd: newDBMigrationDownCommand(), want: []string{"down"}},
		{name: "fix", cmd: newDBMigrationFixCommand(), want: []string{"fix"}},
		{name: "reset", cmd: newDBMigrationResetCommand(), want: []string{"reset"}},
		{name: "up-to", cmd: newDBMigrationUpToCommand(), args: []string{"10"}, want: []string{"up-to", "10"}},
		{name: "down-to", cmd: newDBMigrationDownToCommand(), args: []string{"9"}, want: []string{"down-to", "9"}},
		{name: "status", cmd: newDBMigrationStatusCommand(), want: []string{"status"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cmd.RunE(tt.cmd, tt.args); err != nil {
				t.Fatalf("RunE: %v", err)
			}
			got := calls[len(calls)-1]
			gotTail := got[len(got)-len(tt.want):]
			if !reflect.DeepEqual(gotTail, tt.want) {
				t.Fatalf("goose tail args = %#v, want %#v (full %#v)", gotTail, tt.want, got)
			}
		})
	}
}

func setDatabaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DB_KIND", "postgres")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_HOST", "127.0.0.1")
	t.Setenv("DB_NAME", "andurel_test")
	t.Setenv("DB_USER", "andurel")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_SSL_MODE", "disable")
}

func tempInputFile(t *testing.T, value string) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stdin-*")
	if err != nil {
		t.Fatalf("create temp input: %v", err)
	}
	if _, err := f.WriteString(value); err != nil {
		t.Fatalf("write temp input: %v", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("seek temp input: %v", err)
	}
	return f
}

func newStructuredTestCommand(out *bytes.Buffer) *cobra.Command {
	cmd := &cobra.Command{Use: "andurel"}
	output.RegisterPersistentFlags(cmd)
	cmd.SetOut(out)
	_ = cmd.PersistentFlags().Set("json", "true")
	return cmd
}
