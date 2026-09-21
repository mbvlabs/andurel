package cmds

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/layout/versions"
)

func TestCommandHelperProcess(t *testing.T) {
	if os.Getenv("ANDUREL_COMMAND_HELPER") != "1" {
		return
	}
	actual := os.Getenv("ANDUREL_COMMAND_ACTUAL")
	expected := os.Getenv("ANDUREL_COMMAND_EXPECTED")
	if actual != expected {
		_, _ = os.Stderr.WriteString("command = " + actual + ", want " + expected)
		os.Exit(2)
	}
	if expectedDir := os.Getenv("ANDUREL_COMMAND_DIR"); expectedDir != "" {
		actualDir, err := os.Getwd()
		if err != nil || actualDir != expectedDir {
			_, _ = os.Stderr.WriteString(
				"working directory = " + actualDir + ", want " + expectedDir,
			)
			os.Exit(2)
		}
	}
	if os.Getenv("ANDUREL_COMMAND_FAIL") == "1" {
		_, _ = os.Stderr.WriteString("intentional failure")
		os.Exit(1)
	}
	os.Exit(0)
}

func installCommandHelper(t *testing.T) {
	t.Helper()
	originalCommand := newCommand
	newCommand = func(name string, args ...string) *exec.Cmd {
		actual := strings.Join(append([]string{name}, args...), "\x1f")
		cmd := exec.Command(os.Args[0], "-test.run=TestCommandHelperProcess")
		cmd.Env = append(os.Environ(),
			"ANDUREL_COMMAND_HELPER=1",
			"ANDUREL_COMMAND_ACTUAL="+actual,
		)
		return cmd
	}
	t.Cleanup(func() { newCommand = originalCommand })
}

func expectCommand(t *testing.T, parts ...string) {
	t.Helper()
	t.Setenv("ANDUREL_COMMAND_EXPECTED", strings.Join(parts, "\x1f"))
}

// isolateProjectTools clears ANDUREL_TOOL_BIN and PATH so resolveProjectTool
// cannot find templ/goose/narsilc and the go-run fallback is exercised.
func isolateProjectTools(t *testing.T) {
	t.Helper()
	t.Setenv("ANDUREL_TOOL_BIN", "")
	t.Setenv("PATH", t.TempDir())
}

func TestRunCommands(t *testing.T) {
	tests := []struct {
		name    string
		command []string
		run     func(string) error
	}{
		{name: "go mod tidy", command: []string{"go", "mod", "tidy"}, run: RunGoModTidy},
		{name: "go fmt", command: []string{"go", "fmt", "./..."}, run: RunGoFmt},
		{
			name:    "go fmt path",
			command: []string{"go", "fmt", "./models"},
			run: func(dir string) error {
				return RunGoFmtPath(dir, "./models")
			},
		},
		{name: "golines", command: []string{"golines", "-w", "-m", "100", "."}, run: RunGolines},
		{
			name: "templ generate",
			command: []string{
				"go",
				"run",
				"github.com/a-h/templ/cmd/templ@" + versions.Templ,
				"generate",
				"-path",
				".",
			},
			run: RunTemplGenerate,
		},
		{
			name: "templ fmt",
			command: []string{
				"go",
				"run",
				"github.com/a-h/templ/cmd/templ@" + versions.Templ,
				"fmt",
				"views",
			},
			run: RunTemplFmt,
		},
		{
			name: "goose fix",
			command: []string{
				"go",
				"run",
				"github.com/pressly/goose/v3/cmd/goose@" + versions.Goose,
				"-dir",
				"migrations",
				"fix",
			},
			run: RunGooseFix,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installCommandHelper(t)
			isolateProjectTools(t)
			expectCommand(t, tt.command...)
			targetDir := t.TempDir()
			t.Setenv("ANDUREL_COMMAND_DIR", targetDir)
			if err := tt.run(targetDir); err != nil {
				t.Fatalf("command failed: %v", err)
			}
		})
	}
}

func TestRunNarsilcGenerate(t *testing.T) {
	t.Run("no queries is no-op", func(t *testing.T) {
		installCommandHelper(t)
		targetDir := t.TempDir()
		if err := RunNarsilcGenerate(targetDir); err != nil {
			t.Fatalf("expected no-op, got %v", err)
		}
	})

	t.Run("generates when queries exist", func(t *testing.T) {
		targetDir := t.TempDir()
		queriesDir := filepath.Join(targetDir, "models", "queries")
		if err := os.MkdirAll(queriesDir, 0o755); err != nil {
			t.Fatalf("mkdir queries: %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(queriesDir, "users.sql"),
			[]byte("-- name: GetUser :one\nSELECT 1;\n"),
			0o644,
		); err != nil {
			t.Fatalf("write query file: %v", err)
		}
		binDir := filepath.Join(targetDir, "bin")
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			t.Fatalf("mkdir bin: %v", err)
		}
		narsilcPath := filepath.Join(binDir, "narsilc")
		if err := os.WriteFile(narsilcPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("write narsilc stub: %v", err)
		}

		var invoked []string
		originalCommand := newCommand
		newCommand = func(name string, args ...string) *exec.Cmd {
			invoked = append(invoked, strings.Join(append([]string{name}, args...), " "))
			if name == "go" && len(args) >= 1 && (args[0] == "fmt" || args[0] == "mod") {
				return exec.Command("true")
			}
			cmd := exec.Command(os.Args[0], "-test.run=TestCommandHelperProcess")
			cmd.Env = append(os.Environ(),
				"ANDUREL_COMMAND_HELPER=1",
				"ANDUREL_COMMAND_ACTUAL="+strings.Join(append([]string{name}, args...), "\x1f"),
				"ANDUREL_COMMAND_EXPECTED="+strings.Join([]string{narsilcPath, "generate"}, "\x1f"),
				"ANDUREL_COMMAND_DIR="+targetDir,
			)
			return cmd
		}
		t.Cleanup(func() { newCommand = originalCommand })

		if err := RunNarsilcGenerate(targetDir); err != nil {
			t.Fatalf("narsilc generate failed: %v", err)
		}
		if len(invoked) != 3 {
			t.Fatalf(
				"invoked commands = %#v, want narsilc generate, go mod tidy, and go fmt",
				invoked,
			)
		}
		if !strings.HasPrefix(invoked[0], narsilcPath) || !strings.Contains(invoked[0], "generate") {
			t.Fatalf("first command = %q, want narsilc generate", invoked[0])
		}
		if !strings.Contains(invoked[1], "go mod tidy") {
			t.Fatalf("second command = %q, want go mod tidy", invoked[1])
		}
		if !strings.Contains(invoked[2], "go fmt") {
			t.Fatalf("third command = %q, want go fmt", invoked[2])
		}
	})

	t.Run("falls back to go run when binary is missing", func(t *testing.T) {
		// Clear tool-bin override only; keep PATH so the tidy/fmt stubs can
		// invoke `true`. install-dev-tools does not put narsilc on PATH.
		t.Setenv("ANDUREL_TOOL_BIN", "")
		targetDir := t.TempDir()
		queriesDir := filepath.Join(targetDir, "models", "queries")
		if err := os.MkdirAll(queriesDir, 0o755); err != nil {
			t.Fatalf("mkdir queries: %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(queriesDir, "users.sql"),
			[]byte("-- name: GetUser :one\nSELECT 1;\n"),
			0o644,
		); err != nil {
			t.Fatalf("write query file: %v", err)
		}

		var invoked []string
		originalCommand := newCommand
		newCommand = func(name string, args ...string) *exec.Cmd {
			invoked = append(invoked, strings.Join(append([]string{name}, args...), " "))
			if name == "go" && len(args) >= 1 && (args[0] == "fmt" || args[0] == "mod") {
				return exec.Command("true")
			}
			cmd := exec.Command(os.Args[0], "-test.run=TestCommandHelperProcess")
			cmd.Env = append(os.Environ(),
				"ANDUREL_COMMAND_HELPER=1",
				"ANDUREL_COMMAND_ACTUAL="+strings.Join(append([]string{name}, args...), "\x1f"),
				"ANDUREL_COMMAND_EXPECTED="+strings.Join([]string{
					"go",
					"run",
					"github.com/mbvlabs/narsilc/cmd/narsilc@" + versions.NarsilcModule,
					"generate",
				}, "\x1f"),
				"ANDUREL_COMMAND_DIR="+targetDir,
			)
			return cmd
		}
		t.Cleanup(func() { newCommand = originalCommand })

		if err := RunNarsilcGenerate(targetDir); err != nil {
			t.Fatalf("narsilc generate fallback failed: %v", err)
		}
		if len(invoked) != 3 {
			t.Fatalf(
				"invoked commands = %#v, want go run narsilc, go mod tidy, and go fmt",
				invoked,
			)
		}
		if !strings.Contains(invoked[0], "github.com/mbvlabs/narsilc/cmd/narsilc@") ||
			!strings.Contains(invoked[0], "generate") {
			t.Fatalf("first command = %q, want go run narsilc generate", invoked[0])
		}
	})
}

func TestRunCommandErrors(t *testing.T) {
	t.Run("absolute path", func(t *testing.T) {
		originalAbsolutePath := absolutePath
		expectedErr := errors.New("absolute path failed")
		absolutePath = func(string) (string, error) { return "", expectedErr }
		t.Cleanup(func() { absolutePath = originalAbsolutePath })

		runners := []func(string) error{
			RunGoModTidy,
			RunGoFmt,
			func(dir string) error { return RunGoFmtPath(dir, ".") },
			RunGolines,
			RunTemplGenerate,
			RunTemplFmt,
			RunGooseFix,
			RunNarsilcGenerate,
			RunNarsilcGenerateOptional,
		}
		for _, run := range runners {
			err := run("project")
			if !errors.Is(err, expectedErr) ||
				!strings.Contains(err.Error(), "failed to get absolute path") {
				t.Fatalf("unexpected absolute path error: %v", err)
			}
		}
	})

	t.Run("command", func(t *testing.T) {
		installCommandHelper(t)
		isolateProjectTools(t)
		t.Setenv("ANDUREL_COMMAND_FAIL", "1")
		targetDir := t.TempDir()
		t.Setenv("ANDUREL_COMMAND_DIR", targetDir)

		expectCommand(t, "go", "mod", "tidy")
		if err := RunGoModTidy(targetDir); err == nil {
			t.Fatal("expected go mod tidy failure")
		}

		expectCommand(t, "golines", "-w", "-m", "100", ".")
		if err := RunGolines(targetDir); err == nil {
			t.Fatal("expected golines failure")
		}

		expectCommand(
			t,
			"go",
			"run",
			"github.com/a-h/templ/cmd/templ@"+versions.Templ,
			"generate",
			"-path",
			".",
		)
		if err := RunTemplGenerate(targetDir); err == nil {
			t.Fatal("expected templ generate failure")
		}

		expectCommand(
			t,
			"go",
			"run",
			"github.com/a-h/templ/cmd/templ@"+versions.Templ,
			"fmt",
			"views",
		)
		if err := RunTemplFmt(targetDir); err == nil {
			t.Fatal("expected templ fmt failure")
		}

		expectCommand(
			t,
			"go",
			"run",
			"github.com/pressly/goose/v3/cmd/goose@"+versions.Goose,
			"-dir",
			"migrations",
			"fix",
		)
		if err := RunGooseFix(targetDir); err == nil {
			t.Fatal("expected goose fix failure")
		}
	})

	for _, tt := range []struct {
		name string
		run  func(string) error
	}{
		{name: "go fmt", run: RunGoFmt},
		{name: "go fmt path", run: func(dir string) error { return RunGoFmtPath(dir, "./models") }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			installCommandHelper(t)
			t.Setenv("ANDUREL_COMMAND_FAIL", "1")
			targetDir := t.TempDir()
			t.Setenv("ANDUREL_COMMAND_DIR", targetDir)
			if tt.name == "go fmt" {
				expectCommand(t, "go", "fmt", "./...")
			} else {
				expectCommand(t, "go", "fmt", "./models")
			}
			err := tt.run(targetDir)
			if err == nil || !strings.Contains(err.Error(), "intentional failure") {
				t.Fatalf("expected formatted command output, got %v", err)
			}
		})
	}
}
