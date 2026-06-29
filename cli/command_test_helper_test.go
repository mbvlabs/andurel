package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mbvlabs/andurel/pkg/cache"
	"github.com/spf13/cobra"
)

type cliTestResult struct {
	cmd    *cobra.Command
	stdout string
	stderr string
	err    error
}

func runCLITest(t *testing.T, args ...string) cliTestResult {
	t.Helper()
	resetCLITestSeams(t)
	return executeCLITest(t, args...)
}

func executeCLITest(t *testing.T, args ...string) cliTestResult {
	t.Helper()

	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "go.mod"), []byte("module example.com/app\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
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
	defaultNewGenerator := newGenerator
	defaultRunModelUpdate := runModelUpdateFunc
	defaultRunTempl := runTemplFunc
	defaultRunFmt := runFmtFunc
	defaultRunGoFmt := runGoFmtFunc
	defaultRunGolines := runGolinesFunc
	defaultRunTemplFmt := runTemplFmtFunc
	defaultGenerateController := generateControllerWithActionsFunc

	t.Cleanup(func() {
		findGoModRoot = defaultFindGoModRoot
		newGenerator = defaultNewGenerator
		runModelUpdateFunc = defaultRunModelUpdate
		runTemplFunc = defaultRunTempl
		runFmtFunc = defaultRunFmt
		runGoFmtFunc = defaultRunGoFmt
		runGolinesFunc = defaultRunGolines
		runTemplFmtFunc = defaultRunTemplFmt
		generateControllerWithActionsFunc = defaultGenerateController
		cache.ClearFileSystemCache()
	})
}

type fakeGenerator struct {
	modelCalls       []modelCall
	modelWithPKCalls []modelWithPKCall
	scaffoldCalls    []scaffoldCall
	controllerCalls  []controllerCall
	err              error
	onGenerateModel  func()
}

type modelCall struct {
	name        string
	tableName   string
	skipFactory bool
}

type modelWithPKCall struct {
	name        string
	tableName   string
	skipFactory bool
	primaryKey  string
}

type scaffoldCall struct {
	name        string
	tableName   string
	skipFactory bool
	primaryKey  string
	inertia     string
}

type controllerCall struct {
	name      string
	modelName string
	tableName string
	withViews bool
	actions   []string
	inertia   string
}

func (f *fakeGenerator) GenerateModel(resourceName string, tableNameOverride string, skipFactory bool) error {
	if f.onGenerateModel != nil {
		f.onGenerateModel()
	}
	f.modelCalls = append(f.modelCalls, modelCall{
		name:        resourceName,
		tableName:   tableNameOverride,
		skipFactory: skipFactory,
	})
	return f.err
}

func (f *fakeGenerator) GenerateModelWithPK(resourceName string, tableNameOverride string, skipFactory bool, primaryKeyColumn string) error {
	f.modelWithPKCalls = append(f.modelWithPKCalls, modelWithPKCall{
		name:        resourceName,
		tableName:   tableNameOverride,
		skipFactory: skipFactory,
		primaryKey:  primaryKeyColumn,
	})
	return f.err
}

func (f *fakeGenerator) GenerateControllerWithActions(resourceName, tableName string, withViews bool, actions []string, inertia string) error {
	f.controllerCalls = append(f.controllerCalls, controllerCall{
		name:      resourceName,
		modelName: resourceName,
		tableName: tableName,
		withViews: withViews,
		actions:   append([]string(nil), actions...),
		inertia:   inertia,
	})
	return f.err
}

func (f *fakeGenerator) GenerateControllerWithActionsForModel(resourceName, modelName, tableName string, withViews bool, actions []string, inertia string) error {
	f.controllerCalls = append(f.controllerCalls, controllerCall{
		name:      resourceName,
		modelName: modelName,
		tableName: tableName,
		withViews: withViews,
		actions:   append([]string(nil), actions...),
		inertia:   inertia,
	})
	return f.err
}

func (f *fakeGenerator) GenerateScaffold(resourceName, tableName string, skipFactory bool, primaryKeyColumn string, inertia string) error {
	f.scaffoldCalls = append(f.scaffoldCalls, scaffoldCall{
		name:        resourceName,
		tableName:   tableName,
		skipFactory: skipFactory,
		primaryKey:  primaryKeyColumn,
		inertia:     inertia,
	})
	return f.err
}

func installFakeGenerator(t *testing.T) *fakeGenerator {
	t.Helper()
	fake := &fakeGenerator{}
	newGenerator = func() (cliGenerator, error) {
		if fake.err != nil && errors.Is(fake.err, errGeneratorFactory) {
			return nil, fake.err
		}
		return fake, nil
	}
	return fake
}

var errGeneratorFactory = errors.New("generator factory failed")
