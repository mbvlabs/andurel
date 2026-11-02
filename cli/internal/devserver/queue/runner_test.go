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
	var status tui.Status
	r := &Runner{
		binDir:      tmp,
		buildRunner: ctxrun.New(),
		Log:         io.Discard,
		BuildOut:    io.Discard,
		RuntimeOut:  io.Discard,
		OnStatus:    func(s tui.Status) { status = s },
		runBuild: func(context.Context, string) error {
			return os.ErrPermission
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rebuild := make(chan struct{}, 1)
	errCh := make(chan error, 1)
	go func() { errCh <- r.Run(ctx, rebuild) }()

	time.Sleep(50 * time.Millisecond)
	rebuild <- struct{}{}
	time.Sleep(50 * time.Millisecond)

	select {
	case err := <-errCh:
		t.Fatalf("Run should keep going after build failure, got %v", err)
	default:
	}
	if status != tui.StatusError {
		t.Fatalf("status = %s, want error", status)
	}
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not return after cancel")
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
