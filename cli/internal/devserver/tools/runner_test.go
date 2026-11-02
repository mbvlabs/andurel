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
	statuses := make(chan tui.Status, 2)
	missingBinary := filepath.Join(t.TempDir(), "missing-binary")
	r := &Runner{
		Name: "mailpit",
		Out:  io.Discard,
		OnStatus: func(s tui.Status) {
			statuses <- s
		},
		commandContext: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			// Path that cannot exist / execute.
			return exec.CommandContext(ctx, missingBinary)
		},
	}

	done := runForTest(t, r)
	wantStatuses(t, statuses, done, tui.StatusStarting, tui.StatusError)
}

func TestRunnerReadyThenExited(t *testing.T) {
	statuses := make(chan tui.Status, 3)
	r := &Runner{
		Name: "mailpit",
		Out:  io.Discard,
		OnStatus: func(s tui.Status) {
			statuses <- s
		},
		commandContext: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "true")
		},
	}

	done := runForTest(t, r)
	wantStatuses(t, statuses, done, tui.StatusStarting, tui.StatusReady, tui.StatusExited)
}

func runForTest(t *testing.T, r *Runner) <-chan struct{} {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var runErr error
	go func() { runErr = r.Run(ctx); close(done) }()
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
	return done
}

func wantStatuses(t *testing.T, statuses <-chan tui.Status, done <-chan struct{}, want ...tui.Status) {
	t.Helper()
	for _, expected := range want {
		select {
		case got := <-statuses:
			if got != expected {
				t.Fatalf("status = %s, want %s", got, expected)
			}
		case <-done:
			t.Fatal("Run returned before cancellation")
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for status %s", expected)
		}
	}
	select {
	case <-done:
		t.Fatal("Run returned before cancellation")
	default:
	}
}
