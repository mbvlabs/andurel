package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mbvlabs/andurel/layout"
)

func TestIsEmailCompilerSource(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "email css", path: filepath.Join(root, "css", "email.css"), want: true},
		{name: "email template", path: filepath.Join(root, "email", "welcome.templ"), want: true},
		{name: "nested email template", path: filepath.Join(root, "email", "auth", "verify.templ"), want: true},
		{name: "generated email", path: filepath.Join(root, "email", "welcome_templ.go"), want: false},
		{name: "web css", path: filepath.Join(root, "css", "base.css"), want: false},
		{name: "outside template", path: filepath.Join(root, "views", "welcome.templ"), want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isEmailCompilerSource(root, test.path); got != test.want {
				t.Fatalf("isEmailCompilerSource(%q) = %v, want %v", test.path, got, test.want)
			}
		})
	}
}

func TestWatchEmailProjectCompilesChangedTemplate(t *testing.T) {
	root := t.TempDir()
	emailDir := filepath.Join(root, "email")
	cssDir := filepath.Join(root, "css")
	binDir := filepath.Join(root, "bin")
	for _, dir := range []string{emailDir, cssDir, binDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	css := `/* andurel:head:start */
body { margin: 0; }
/* andurel:head:end */
`
	if err := os.WriteFile(filepath.Join(cssDir, "email.css"), []byte(css), 0o644); err != nil {
		t.Fatalf("write email CSS: %v", err)
	}
	templatePath := filepath.Join(emailDir, "welcome.templ")
	templateSource := `package email

templ Welcome() {
	<html>
		<head></head>
		<body><p class="p-4">Welcome</p></body>
	</html>
}
`
	if err := os.WriteFile(templatePath, []byte(templateSource), 0o644); err != nil {
		t.Fatalf("write email template: %v", err)
	}

	tailwindPath := filepath.Join(binDir, "tailwindcli")
	tailwindScript := `#!/bin/sh
output=""
while [ "$#" -gt 0 ]; do
	if [ "$1" = "-o" ]; then
		shift
		output="$1"
	fi
	shift
done
printf '%s\n' '.p-4 { padding: 1rem; }' > "$output"
`
	if err := os.WriteFile(tailwindPath, []byte(tailwindScript), 0o755); err != nil {
		t.Fatalf("write fake Tailwind CLI: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := watchEmailProject(ctx, root, tailwindPath); err != nil {
		t.Fatalf("watchEmailProject: %v", err)
	}

	templateSource = strings.Replace(templateSource, "Welcome</p>", "Welcome back</p>", 1)
	if err := os.WriteFile(templatePath, []byte(templateSource), 0o644); err != nil {
		t.Fatalf("update email template: %v", err)
	}

	generatedPath := filepath.Join(emailDir, "welcome_templ.go")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		generated, err := os.ReadFile(generatedPath)
		if err == nil && strings.Contains(string(generated), "padding: 16px;") && strings.Contains(string(generated), "Welcome back") {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("watcher did not compile %s before timeout", generatedPath)
}

func TestWatchEmailProjectRequiresCompilerDirectories(t *testing.T) {
	root := t.TempDir()
	err := watchEmailProject(context.Background(), root, filepath.Join(root, "tailwindcli"))
	if err == nil || !strings.Contains(err.Error(), "email") {
		t.Fatalf("watchEmailProject error = %v, want missing email directory error", err)
	}
}

func TestRunEmailWatcherStopsWithContext(t *testing.T) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("create watcher: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		runEmailWatcher(ctx, watcher, t.TempDir(), "tailwindcli")
		close(done)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runEmailWatcher did not stop after context cancellation")
	}
}

func TestAddEmailWatchDirectoriesRejectsMissingRoot(t *testing.T) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("create watcher: %v", err)
	}
	t.Cleanup(func() { _ = watcher.Close() })

	err = addEmailWatchDirectories(watcher, filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("addEmailWatchDirectories unexpectedly accepted a missing root")
	}
}

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

	err := compileEmailProject(context.Background(), root)
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

	err := compileEmailProject(context.Background(), root)
	if err == nil || !strings.Contains(err.Error(), "email compilation failed") {
		t.Fatalf("compileEmailProject error = %v, want compiler failure", err)
	}
	if synced == nil || synced.Version == "" || synced.Download == nil {
		t.Fatalf("default Tailwind tool = %#v", synced)
	}
}

func TestCompileEmailProjectRequiresLockFile(t *testing.T) {
	resetCLITestSeams(t)
	err := compileEmailProject(context.Background(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "failed to read andurel.lock") {
		t.Fatalf("compileEmailProject error = %v, want lock failure", err)
	}
}

func TestEmailCommandSurface(t *testing.T) {
	command := newEmailCommand()
	if command.Use != "email" || len(command.Commands()) != 1 || command.Commands()[0].Use != "compile" {
		t.Fatalf("email command surface = %q, %#v", command.Use, command.Commands())
	}
	if err := command.Commands()[0].Args(command.Commands()[0], []string{"unexpected"}); err == nil {
		t.Fatal("email compile accepted a positional argument")
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
		if err := command.RunE(command, nil); err == nil || !strings.Contains(err.Error(), "failed to read andurel.lock") {
			t.Fatalf("RunE error = %v, want project compile failure", err)
		}
	})
}
