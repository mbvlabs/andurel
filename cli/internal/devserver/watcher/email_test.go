package watcher

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mbvlabs/andurel/v2/emailcompiler"
)

func TestIsEmailCompilerSource(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "email css", path: filepath.Join(root, "css", "email.css"), want: true},
		{name: "theme css", path: filepath.Join(root, "css", "theme.css"), want: true},
		{name: "email template", path: filepath.Join(root, "email", "welcome.templ"), want: true},
		{
			name: "nested email template",
			path: filepath.Join(root, "email", "auth", "verify.templ"),
			want: true,
		},
		{
			name: "generated email",
			path: filepath.Join(root, "email", "welcome_templ.go"),
			want: false,
		},
		{name: "web css", path: filepath.Join(root, "css", "base.css"), want: false},
		{
			name: "outside template",
			path: filepath.Join(root, "resources", "root.templ"),
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsEmailCompilerSource(root, test.path); got != test.want {
				t.Fatalf("IsEmailCompilerSource(%q) = %v, want %v", test.path, got, test.want)
			}
		})
	}
}

func TestRunEmailWatcherCompilesChangedTemplate(t *testing.T) {
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

	var compiled atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunEmailWatcher(ctx, EmailWatcherConfig{
			Dir: root,
			Compile: func(ctx context.Context) error {
				return emailcompiler.Compile(ctx, emailcompiler.Config{
					ProjectRoot:  root,
					TailwindPath: tailwindPath,
				})
			},
			OnCompiled: func() { compiled.Add(1) },
			Log:        io.Discard,
		})
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	// Give the watcher time to register its directories.
	time.Sleep(100 * time.Millisecond)
	templateSource = strings.Replace(templateSource, "Welcome</p>", "Welcome back</p>", 1)
	if err := os.WriteFile(templatePath, []byte(templateSource), 0o644); err != nil {
		t.Fatalf("update email template: %v", err)
	}

	generatedPath := filepath.Join(emailDir, "welcome_templ.go")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		generated, err := os.ReadFile(generatedPath)
		if err == nil && strings.Contains(string(generated), "padding: 16px;") &&
			strings.Contains(string(generated), "Welcome back") && compiled.Load() > 0 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("watcher did not compile %s before timeout", generatedPath)
}

func TestRunEmailWatcherRequiresCompilerDirectories(t *testing.T) {
	root := t.TempDir()
	err := RunEmailWatcher(context.Background(), EmailWatcherConfig{Dir: root, Log: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "email") {
		t.Fatalf("RunEmailWatcher error = %v, want missing email directory error", err)
	}
}

func TestRunEmailWatcherStopsWithContext(t *testing.T) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("create watcher: %v", err)
	}
	t.Cleanup(func() { _ = fsWatcher.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		runEmailWatcher(ctx, fsWatcher, EmailWatcherConfig{Dir: t.TempDir(), Log: io.Discard})
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
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("create watcher: %v", err)
	}
	t.Cleanup(func() { _ = fsWatcher.Close() })

	err = addEmailWatchDirectories(fsWatcher, filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("addEmailWatchDirectories unexpectedly accepted a missing root")
	}
}
