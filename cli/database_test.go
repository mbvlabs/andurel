package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
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

func TestBuildDatabaseURLSafelyEncodesReservedCredentials(t *testing.T) {
	t.Setenv("DB_KIND", "postgres")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_HOST", "2001:db8::1")
	t.Setenv("DB_NAME", "app/name?")
	t.Setenv("DB_USER", "user:name@example")
	t.Setenv("DB_PASSWORD", "p@ss:/?#[]")
	t.Setenv("DB_SSL_MODE", "verify-full")

	_, rawURL, err := buildDatabaseURL()
	if err != nil {
		t.Fatalf("buildDatabaseURL: %v", err)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse database URL: %v", err)
	}
	if parsed.User.Username() != "user:name@example" {
		t.Fatalf("username = %q", parsed.User.Username())
	}
	password, ok := parsed.User.Password()
	if !ok || password != "p@ss:/?#[]" {
		t.Fatalf("password = %q, present = %t", password, ok)
	}
	if parsed.Host != "[2001:db8::1]:5432" || parsed.Path != "/app/name?" {
		t.Fatalf("unexpected endpoint: host=%q path=%q", parsed.Host, parsed.Path)
	}
	if parsed.Query().Get("sslmode") != "verify-full" {
		t.Fatalf("sslmode = %q", parsed.Query().Get("sslmode"))
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
	confirmed, err := confirmDestructive("drop", "example")
	if err != nil {
		t.Fatalf("confirmDestructive yes: %v", err)
	}
	if !confirmed {
		t.Fatalf("expected yes to confirm")
	}

	os.Stdin = tempInputFile(t, "n\n")
	confirmed, err = confirmDestructive("drop", "example")
	if err != nil {
		t.Fatalf("confirmDestructive no: %v", err)
	}
	if confirmed {
		t.Fatalf("expected n to abort")
	}

	if _, err := confirmDestructive("drop", " "); err == nil {
		t.Fatalf("expected empty database name error")
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

func TestDatabaseLifecycleWithFakeAdminConnection(t *testing.T) {
	resetCLITestSeams(t)
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
	unsetEnvForTest(t, "DB_KIND", "DB_PORT", "DB_HOST", "DB_NAME", "DB_USER", "DB_PASSWORD", "DB_SSL_MODE")

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	fake := &fakeAdminConnection{}
	openAdminConnectionFunc = func() (dbConfig, adminConnection, context.Context, context.CancelFunc, error) {
		cfg, err := loadDatabaseConfig()
		ctx, cancel := context.WithCancel(context.Background())
		return cfg, fake, ctx, cancel, err
	}

	if err := createDatabase(); err != nil {
		t.Fatalf("createDatabase: %v", err)
	}
	if !fake.closed || !containsSQL(fake.execs, `CREATE DATABASE "app_db"`) {
		t.Fatalf("createDatabase execs=%#v closed=%v", fake.execs, fake.closed)
	}

	fake.reset()
	originalStdin := os.Stdin
	t.Cleanup(func() { os.Stdin = originalStdin })
	os.Stdin = tempInputFile(t, "yes\n")
	if err := dropDatabase(false); err != nil {
		t.Fatalf("dropDatabase: %v", err)
	}
	if !containsSQL(fake.execs, "pg_terminate_backend") || !containsSQL(fake.execs, `DROP DATABASE IF EXISTS "app_db"`) {
		t.Fatalf("dropDatabase execs=%#v", fake.execs)
	}

	fake.reset()
	os.Stdin = tempInputFile(t, "n\n")
	if err := nukeDatabase(false); !errors.Is(err, errDatabaseOperationAborted) {
		t.Fatalf("nukeDatabase abort: %v", err)
	}
	if len(fake.execs) != 0 {
		t.Fatalf("aborted nuke should not connect or exec, got %#v", fake.execs)
	}

	fake.reset()
	os.Stdin = tempInputFile(t, "yes\n")
	var gooseArgs []string
	var seedName string
	runGooseFunc = func(args ...string) error {
		gooseArgs = append([]string(nil), args...)
		return nil
	}
	runSeedFunc = func(cmd *cobra.Command, name string, list bool) error {
		seedName = name
		return nil
	}
	cmd := newStructuredTestCommand(&bytes.Buffer{})
	if err := rebuildDatabase(cmd, false, false, "development"); err != nil {
		t.Fatalf("rebuildDatabase: %v", err)
	}
	if !reflect.DeepEqual(gooseArgs, []string{"up"}) || seedName != "development" {
		t.Fatalf("rebuild orchestration goose=%#v seed=%q", gooseArgs, seedName)
	}
	if !containsSQL(fake.execs, `DROP DATABASE IF EXISTS "app_db"`) || !containsSQL(fake.execs, `CREATE DATABASE "app_db"`) {
		t.Fatalf("rebuild execs=%#v", fake.execs)
	}
}

func TestDeclinedRebuildDoesNotMutateOrContinue(t *testing.T) {
	resetCLITestSeams(t)
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/app\n")
	writeTestFile(t, root, ".env", strings.Join([]string{
		"DB_KIND=postgres",
		"DB_PORT=5432",
		"DB_HOST=localhost",
		"DB_NAME=app_db",
		"DB_USER=app_user",
		"DB_PASSWORD=never-print-this-password",
		"DB_SSL_MODE=disable",
		"",
	}, "\n"))
	unsetEnvForTest(t, "DB_KIND", "DB_PORT", "DB_HOST", "DB_NAME", "DB_USER", "DB_PASSWORD", "DB_SSL_MODE")

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() { findGoModRoot = originalFindGoModRoot })

	fake := &fakeAdminConnection{}
	openAdminConnectionFunc = func() (dbConfig, adminConnection, context.Context, context.CancelFunc, error) {
		cfg, err := loadDatabaseConfig()
		ctx, cancel := context.WithCancel(context.Background())
		return cfg, fake, ctx, cancel, err
	}

	gooseCalled := false
	seedCalled := false
	runGooseFunc = func(...string) error {
		gooseCalled = true
		return nil
	}
	runSeedFunc = func(*cobra.Command, string, bool) error {
		seedCalled = true
		return nil
	}

	originalStdin := os.Stdin
	t.Cleanup(func() { os.Stdin = originalStdin })
	os.Stdin = tempInputFile(t, "no\n")
	outputText := captureProcessOutput(t, &os.Stdout)

	err := rebuildDatabase(newStructuredTestCommand(&bytes.Buffer{}), false, false, "development")
	if !errors.Is(err, errDatabaseOperationAborted) {
		t.Fatalf("rebuildDatabase error = %v", err)
	}
	if gooseCalled || seedCalled || len(fake.execs) != 0 {
		t.Fatalf("declined rebuild mutated state: goose=%t seed=%t sql=%#v", gooseCalled, seedCalled, fake.execs)
	}
	if output := outputText(); strings.Contains(output, "never-print-this-password") {
		t.Fatalf("confirmation exposed database password: %q", output)
	}
}

func TestDatabaseLifecycleProtectionsAndExecErrors(t *testing.T) {
	cfg := dbConfig{Name: "postgres"}
	fake := &fakeAdminConnection{}
	if err := dropDatabaseWithConn(context.Background(), cfg, fake, false); err == nil ||
		!strings.Contains(err.Error(), "refusing to drop system database") {
		t.Fatalf("expected system drop protection, got %v", err)
	}
	if err := createDatabaseWithConn(context.Background(), cfg, fake); err == nil ||
		!strings.Contains(err.Error(), "refusing to create system database") {
		t.Fatalf("expected system create protection, got %v", err)
	}

	fake.err = errors.New("exec failed")
	cfg.Name = "app_db"
	if err := dropDatabaseWithConn(context.Background(), cfg, fake, false); err == nil ||
		!strings.Contains(err.Error(), "exec failed") {
		t.Fatalf("expected terminate exec error, got %v", err)
	}

	fake.reset()
	fake.errOn = `DROP DATABASE`
	fake.err = errors.New("drop failed")
	if err := dropDatabaseWithConn(context.Background(), cfg, fake, false); err == nil ||
		!strings.Contains(err.Error(), "drop failed") {
		t.Fatalf("expected drop exec error, got %v", err)
	}

	fake.reset()
	fake.err = errors.New("create failed")
	if err := createDatabaseWithConn(context.Background(), cfg, fake); err == nil ||
		!strings.Contains(err.Error(), "create failed") {
		t.Fatalf("expected create exec error, got %v", err)
	}
}

func TestCreateDatabaseReturnsConnectionCloseFailure(t *testing.T) {
	resetCLITestSeams(t)
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/app\n")
	setDatabaseEnv(t)
	findGoModRoot = func() (string, error) { return root, nil }

	closeErr := errors.New("close failed")
	fake := &fakeAdminConnection{closeErr: closeErr}
	openAdminConnectionFunc = func() (dbConfig, adminConnection, context.Context, context.CancelFunc, error) {
		ctx, cancel := context.WithCancel(context.Background())
		return dbConfig{Name: "andurel_test"}, fake, ctx, cancel, nil
	}

	if err := createDatabase(); !errors.Is(err, closeErr) {
		t.Fatalf("createDatabase error = %v, want close failure", err)
	}
	if !fake.closed {
		t.Fatal("database connection was not closed")
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

type fakeAdminConnection struct {
	execs    []string
	closed   bool
	err      error
	errOn    string
	closeErr error
}

func (f *fakeAdminConnection) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	f.execs = append(f.execs, sql)
	if f.err != nil && (f.errOn == "" || strings.Contains(sql, f.errOn)) {
		return pgconn.CommandTag{}, f.err
	}
	return pgconn.CommandTag{}, nil
}

func (f *fakeAdminConnection) Close(ctx context.Context) error {
	f.closed = true
	return f.closeErr
}

func (f *fakeAdminConnection) reset() {
	f.execs = nil
	f.closed = false
	f.err = nil
	f.errOn = ""
	f.closeErr = nil
}

func containsSQL(statements []string, want string) bool {
	for _, statement := range statements {
		if strings.Contains(statement, want) {
			return true
		}
	}
	return false
}
