package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/layout"
)

func TestHasEmailCompilerInputs(t *testing.T) {
	root := t.TempDir()
	if hasEmailCompilerInputs(root) {
		t.Fatal("hasEmailCompilerInputs returned true without compiler inputs")
	}
	if err := os.MkdirAll(filepath.Join(root, "email"), 0o755); err != nil {
		t.Fatalf("create email directory: %v", err)
	}
	if hasEmailCompilerInputs(root) {
		t.Fatal("hasEmailCompilerInputs returned true without email CSS")
	}
	if err := os.MkdirAll(filepath.Join(root, "css"), 0o755); err != nil {
		t.Fatalf("create CSS directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "css", "email.css"), nil, 0o644); err != nil {
		t.Fatalf("write email CSS: %v", err)
	}
	if !hasEmailCompilerInputs(root) {
		t.Fatal("hasEmailCompilerInputs returned false with all compiler inputs")
	}
}

func TestCompileEmailProjectUsesManagedTailwindTool(t *testing.T) {
	resetCLITestSeams(t)
	root := t.TempDir()
	lock := layout.NewAndurelLock("test")
	configured := validTestTool("tailwindcli", "v9.8.7")
	lock.Tools["tailwindcli"] = configured
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	wantErr := errors.New("sync unavailable")
	syncSingleToolFunc = func(projectRoot, name string, tool *layout.Tool, goos, goarch string) error {
		if projectRoot != root || name != "tailwindcli" || tool.Version != configured.Version {
			t.Fatalf("sync arguments = %q, %q, %#v", projectRoot, name, tool)
		}
		return wantErr
	}
	_, err := compileEmailProjectReport(context.Background(), root)
	if !errors.Is(err, wantErr) || !strings.Contains(err.Error(), "failed to sync tailwind CLI") {
		t.Fatalf("compileEmailProject error = %v, want sync failure", err)
	}
}

func TestCompileEmailProjectDefaultsTailwindAndReportsCompilerErrors(t *testing.T) {
	resetCLITestSeams(t)
	root := t.TempDir()
	if err := layout.NewAndurelLock("test").WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	var synced *layout.Tool
	syncSingleToolFunc = func(projectRoot, name string, tool *layout.Tool, goos, goarch string) error {
		if projectRoot != root || name != "tailwindcli" {
			t.Fatalf("sync arguments = %q, %q", projectRoot, name)
		}
		synced = tool
		return nil
	}
	_, err := compileEmailProjectReport(context.Background(), root)
	if err == nil || !strings.Contains(err.Error(), "email compilation failed") {
		t.Fatalf("compileEmailProject error = %v, want compiler failure", err)
	}
	if synced == nil || synced.Version == "" || synced.Download == nil {
		t.Fatalf("default Tailwind tool = %#v", synced)
	}
}

func TestCompileEmailProjectRequiresLockFile(t *testing.T) {
	resetCLITestSeams(t)
	_, err := compileEmailProjectReport(context.Background(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "failed to read andurel.toml") {
		t.Fatalf("compileEmailProject error = %v, want lock failure", err)
	}
}

func TestEmailCommandSurface(t *testing.T) {
	command := newEmailCompileCommand()
	if command.Use != "email" {
		t.Fatalf("email compile command use = %q", command.Use)
	}
	if err := command.Args(command, []string{"unexpected"}); err == nil {
		t.Fatal("email compile accepted a positional argument")
	}
	for _, want := range []string{"email/*_templ.go", "templ generate", "--json"} {
		if !strings.Contains(command.Long, want) {
			t.Fatalf("email compile help is missing %q:\n%s", want, command.Long)
		}
	}
}

func TestEmailCompileCommandReportsProjectErrors(t *testing.T) {
	t.Run("go module root", func(t *testing.T) {
		resetCLITestSeams(t)
		wantErr := errors.New("module root unavailable")
		findGoModRoot = func() (string, error) { return "", wantErr }
		command := newEmailCompileCommand()
		if err := command.RunE(command, nil); !errors.Is(err, wantErr) {
			t.Fatalf("RunE error = %v, want module root failure", err)
		}
	})
	t.Run("email project", func(t *testing.T) {
		resetCLITestSeams(t)
		root := t.TempDir()
		findGoModRoot = func() (string, error) { return root, nil }
		command := newEmailCompileCommand()
		if err := command.RunE(command, nil); err == nil || !strings.Contains(err.Error(), "failed to read andurel.toml") {
			t.Fatalf("RunE error = %v, want project compile failure", err)
		}
	})
}
