package queue

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/ctxrun"
	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/tui"
)

func TestRequireEntrypointMissing(t *testing.T) {
	err := RequireEntrypoint(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "cmd/queue/main.go") {
		t.Fatalf("expected missing entrypoint error, got %v", err)
	}
}

func TestRunFailsWhenBinaryDirectoryCannotBeCreated(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tmp"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := NewRunner(dir).Run(t.Context(), nil); err == nil {
		t.Fatal("expected directory creation error")
	}
}

func TestRequireEntrypointPresent(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "cmd", "queue")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := RequireEntrypoint(tmp); err != nil {
		t.Fatal(err)
	}
}

func TestRunnerKeepsRunningAfterBuildFailure(t *testing.T) {
	tmp := t.TempDir()
	statuses := make(chan tui.Status, 4)
	r := &Runner{
		binDir:      tmp,
		buildRunner: ctxrun.New(),
		Log:         io.Discard,
		BuildOut:    io.Discard,
		RuntimeOut:  io.Discard,
		OnStatus:    func(s tui.Status) { statuses <- s },
		runBuild: func(context.Context, string) error {
			return os.ErrPermission
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	rebuild := make(chan struct{}, 1)
	done := make(chan struct{})
	var runErr error
	go func() { runErr = r.Run(ctx, rebuild); close(done) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
			if runErr != nil {
				t.Errorf("Run returned: %v", runErr)
			}
		case <-time.After(2 * time.Second):
			t.Error("Run did not return after cancel")
		}
	})

	for attempt := range 2 {
		if attempt > 0 {
			rebuild <- struct{}{}
		}
		for _, want := range []tui.Status{tui.StatusStarting, tui.StatusError} {
			select {
			case got := <-statuses:
				if got != want {
					t.Fatalf("build %d: status = %s, want %s", attempt+1, got, want)
				}
			case <-done:
				t.Fatalf("Run should keep going after build failure, got %v", runErr)
			case <-time.After(2 * time.Second):
				t.Fatalf("build %d: timed out waiting for status %s", attempt+1, want)
			}
		}
	}
	select {
	case <-done:
		t.Fatalf("Run should keep going after build failure, got %v", runErr)
	default:
	}
}

func TestRunnerRestartsOnRebuild(t *testing.T) {
	tmp := t.TempDir()
	var builds atomic.Int32
	r := &Runner{
		binDir:      tmp,
		buildRunner: ctxrun.New(),
		Log:         io.Discard,
		BuildOut:    io.Discard,
		RuntimeOut:  io.Discard,
		runBuild: func(_ context.Context, outputPath string) error {
			builds.Add(1)
			content := "#!/usr/bin/env sh\nsleep 5\n"
			return os.WriteFile(outputPath, []byte(content), 0o755)
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rebuild := make(chan struct{}, 1)
	errCh := make(chan error, 1)
	go func() { errCh <- r.Run(ctx, rebuild) }()

	deadline := time.After(2 * time.Second)
	for builds.Load() < 1 {
		select {
		case <-deadline:
			t.Fatal("initial build did not run")
		case <-time.After(10 * time.Millisecond):
		}
	}
	rebuild <- struct{}{}
	deadline = time.After(2 * time.Second)
	for builds.Load() < 2 {
		select {
		case <-deadline:
			t.Fatal("rebuild did not run")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}
