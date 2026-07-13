package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestCollectCommandsAndFlags(t *testing.T) {
	root := &cobra.Command{Use: "root", Version: "v1.2.3"}
	root.PersistentFlags().StringP("config", "c", "default.toml", "config file")
	root.PersistentFlags().String("hidden-global", "", "hidden")
	if err := root.PersistentFlags().MarkHidden("hidden-global"); err != nil {
		t.Fatalf("hide persistent flag: %v", err)
	}

	visible := &cobra.Command{Use: "zebra <name>", Aliases: []string{"z"}, Run: func(*cobra.Command, []string) {}}
	visible.Flags().BoolP("force", "f", false, "force")
	hidden := &cobra.Command{Use: "hidden", Hidden: true, Run: func(*cobra.Command, []string) {}}
	unavailable := &cobra.Command{Use: "unavailable"}
	alpha := &cobra.Command{Use: "alpha", Run: func(*cobra.Command, []string) {}}
	root.AddCommand(visible, hidden, unavailable, alpha)

	commands := collectCommands(root)
	if len(commands) != 3 {
		t.Fatalf("expected root and two visible commands, got %#v", commands)
	}
	if commands[0].Path != "root" || commands[1].Path != "root alpha" || commands[2].Path != "root zebra" {
		t.Fatalf("commands were not sorted by path: %#v", commands)
	}
	if len(commands[2].Aliases) != 1 || commands[2].Aliases[0] != "z" {
		t.Fatalf("aliases were not collected: %#v", commands[2])
	}
	assertContractFlag(t, commands[0].Flags, "config", "c", "string", "default.toml", true)
	assertContractFlag(t, commands[2].Flags, "force", "f", "bool", "false", false)
	for _, command := range commands {
		for _, flag := range command.Flags {
			if flag.Name == "hidden-global" {
				t.Fatalf("hidden flag was included: %#v", command)
			}
		}
	}

	if got := flagsFromSet(nil, false); got != nil {
		t.Fatalf("nil flag set should produce nil, got %#v", got)
	}
	set := pflag.NewFlagSet("test", pflag.ContinueOnError)
	set.Int("count", 3, "count")
	if got := flagsFromSet(set, false); len(got) != 1 || got[0].Name != "count" || got[0].Default != "3" {
		t.Fatalf("unexpected standalone flags: %#v", got)
	}
}

func TestJSONContractDiscoveryHelpers(t *testing.T) {
	type sample struct {
		Name    string `json:"name"`
		Count   int    `json:"count,omitempty"`
		Ignored string `json:"-"`
		Plain   string
	}
	fields := jsonFields(reflect.TypeFor[sample]())
	if len(fields) != 2 || fields[0].JSONName != "name" || fields[1].JSONName != "count" || !fields[1].OmitEmpty {
		t.Fatalf("unexpected reflected JSON fields: %#v", fields)
	}

	if got, err := strconvUnquote("`json:\"value\"`"); err != nil || got != "json:\"value\"" {
		t.Fatalf("unquote raw tag: got %q, err %v", got, err)
	}
	for _, value := range []string{"", "x", `"json:\"value\""`, "`unterminated"} {
		if _, err := strconvUnquote(value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}

	root := t.TempDir()
	path := filepath.Join(root, "models", "types.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create models directory: %v", err)
	}
	source := `package models

type Payload struct {
	ID string ` + "`json:\"id\"`" + `
	DisplayName string ` + "`json:\",omitempty\"`" + `
	Ignored string ` + "`json:\"-\"`" + `
	NoTag string
}

type Alias string
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	var contracts []jsonStructContract
	if err := collectFileJSONStructs(root, path, &contracts); err != nil {
		t.Fatalf("collect file contracts: %v", err)
	}
	if len(contracts) != 1 || contracts[0].Type != modulePath+"/models.Payload" {
		t.Fatalf("unexpected contracts: %#v", contracts)
	}
	if got := contracts[0].Fields; len(got) != 2 || got[0].JSONName != "id" || got[1].JSONName != "DisplayName" || !got[1].OmitEmpty {
		t.Fatalf("unexpected source fields: %#v", got)
	}

	badPath := filepath.Join(root, "models", "bad.go")
	if err := os.WriteFile(badPath, []byte("package models\ntype"), 0o644); err != nil {
		t.Fatalf("write invalid source: %v", err)
	}
	if err := collectFileJSONStructs(root, badPath, &contracts); err == nil {
		t.Fatal("expected invalid Go source to fail")
	}
}

func TestEmitContractsFromRepository(t *testing.T) {
	packagesOutput, err := captureStdout(t, emitPackages)
	if err != nil {
		t.Fatalf("emit packages: %v", err)
	}
	packages := strings.Fields(packagesOutput)
	if !containsString(packages, modulePath+"/cli") || !containsString(packages, modulePath+"/layout") {
		t.Fatalf("expected public packages in output, got %q", packagesOutput)
	}
	for _, name := range packages {
		if strings.Contains(name, "/internal/") || strings.HasSuffix(name, "/e2e") {
			t.Fatalf("excluded package was emitted: %s", name)
		}
	}

	cliOutput, err := captureStdout(t, emitCLIContract)
	if err != nil {
		t.Fatalf("emit CLI contract: %v", err)
	}
	var value contract
	if err := json.Unmarshal([]byte(cliOutput), &value); err != nil {
		t.Fatalf("decode CLI contract: %v\n%s", err, cliOutput)
	}
	if value.SchemaVersion != 1 || len(value.Commands) == 0 || len(value.JSONStructs) == 0 {
		t.Fatalf("incomplete CLI contract: %#v", value)
	}
	if len(value.Success.Fields) == 0 || len(value.Failure.Fields) == 0 {
		t.Fatalf("wire envelope fields were not emitted: %#v", value)
	}
	if len(value.Projections) != 3 {
		t.Fatalf("unexpected projections: %#v", value.Projections)
	}
	if len(value.Errors) == 0 || value.Errors[0].Code > value.Errors[len(value.Errors)-1].Code {
		t.Fatalf("errors were not sorted: %#v", value.Errors)
	}
	if value.Errors[0].ExitCode == 0 || output.ExitUsage == 0 {
		t.Fatalf("invalid error exit codes: %#v", value.Errors)
	}
}

func TestRepositoryRootAndDirectoryFiltering(t *testing.T) {
	repoRoot, err := repositoryRoot()
	if err != nil {
		t.Fatalf("find repository root: %v", err)
	}
	if !shouldSkipDirectory(repoRoot, filepath.Join(repoRoot, ".git"), ".git") {
		t.Fatal("dot directory should be skipped")
	}
	for _, name := range []string{"testdata", "vendor", "e2e"} {
		if !shouldSkipDirectory(repoRoot, filepath.Join(repoRoot, name), name) {
			t.Fatalf("%s should be skipped", name)
		}
	}
	if !shouldSkipDirectory(repoRoot, filepath.Join(repoRoot, "generator", "internal", "ddl"), "ddl") {
		t.Fatal("nested internal directory should be skipped")
	}
	if shouldSkipDirectory(repoRoot, filepath.Join(repoRoot, "generator", "models"), "models") {
		t.Fatal("public package directory should not be skipped")
	}

	tempRoot := t.TempDir()
	nested := filepath.Join(tempRoot, "one", "two")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempRoot, "go.mod"), []byte("module example.com/test\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	withWorkingDir(t, nested, func() {
		got, err := repositoryRoot()
		if err != nil || got != tempRoot {
			t.Fatalf("repositoryRoot() = %q, %v; want %q", got, err, tempRoot)
		}
	})

	withoutModule := t.TempDir()
	withWorkingDir(t, withoutModule, func() {
		if _, err := repositoryRoot(); err == nil || !strings.Contains(err.Error(), "repository root not found") {
			t.Fatalf("expected missing root error, got %v", err)
		}
	})
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	previous := os.Stdout
	os.Stdout = writer

	type result struct {
		output string
		err    error
	}
	done := make(chan result, 1)
	go func() {
		var buffer bytes.Buffer
		_, copyErr := io.Copy(&buffer, reader)
		done <- result{output: buffer.String(), err: copyErr}
	}()

	fnErr := fn()
	os.Stdout = previous
	closeErr := writer.Close()
	readResult := <-done
	readErr := reader.Close()
	return readResult.output, errors.Join(fnErr, closeErr, readResult.err, readErr)
}

func withWorkingDir(t *testing.T, directory string, fn func()) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	}()
	fn()
}

func assertContractFlag(t *testing.T, flags []flagContract, name, shorthand, flagType, defaultValue string, persistent bool) {
	t.Helper()
	for _, flag := range flags {
		if flag.Name == name {
			if flag.Shorthand != shorthand || flag.Type != flagType || flag.Default != defaultValue || flag.Persistent != persistent {
				t.Fatalf("unexpected flag %s: %#v", name, flag)
			}
			return
		}
	}
	t.Fatalf("flag %s not found in %#v", name, flags)
}

func containsString(values []string, target string) bool {
	return slices.Contains(values, target)
}
