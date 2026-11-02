package tools

import (
	"context"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/tui"
)

func TestResolveArgvMailpit(t *testing.T) {
	argv, err := ResolveArgv("mailpit")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join("bin", "mailpit"),
		"--smtp=0.0.0.0:1025",
		"--listen=0.0.0.0:8025",
	}
	if len(argv) != len(want) {
		t.Fatalf("argv = %#v", argv)
	}
	for i := range want {
		if argv[i] != want[i] {
			t.Fatalf("argv[%d] = %q, want %q", i, argv[i], want[i])
		}
	}
}

func TestResolveArgvUnknown(t *testing.T) {
	_, err := ResolveArgv("dblab")
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("got %v", err)
	}
	if Allowed("dblab") {
		t.Fatal("dblab should not be allowed")
	}
	if !Allowed("mailpit") {
		t.Fatal("mailpit should be allowed")
	}
}

func TestRunnerStatusOnStartFailure(t *testing.T) {
	var status tui.Status
	r := &Runner{
		Name: "mailpit",
		Out:  io.Discard,
		OnStatus: func(s tui.Status) {
			status = s
		},
		commandContext: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			// Path that cannot exist / execute.
			return exec.CommandContext(ctx, filepath.Join(t.TempDir(), "missing-binary"))
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- r.Run(ctx) }()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if status == tui.StatusError {
			break
		}
		time.Sleep(10 * time.Millisecond)
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

func TestRunnerReadyThenExited(t *testing.T) {
	var statuses []tui.Status
	r := &Runner{
		Name: "mailpit",
		Out:  io.Discard,
		OnStatus: func(s tui.Status) {
			statuses = append(statuses, s)
		},
		commandContext: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "true")
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- r.Run(ctx) }()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(statuses) >= 3 && statuses[len(statuses)-1] == tui.StatusExited {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(statuses) < 3 {
		t.Fatalf("statuses = %#v", statuses)
	}
	if statuses[0] != tui.StatusStarting || statuses[1] != tui.StatusReady || statuses[2] != tui.StatusExited {
		t.Fatalf("statuses = %#v", statuses)
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
