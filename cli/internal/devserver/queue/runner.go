package queue

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/ctxrun"
	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/tui"
)

const entrypoint = "cmd/queue/main.go"

// RequireEntrypoint fails when the Andurel queue worker is missing from the
// project rooted at dir.
func RequireEntrypoint(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, entrypoint)); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("missing %s: andurel run always runs the Andurel queue worker", entrypoint)
		}
		return fmt.Errorf("stat %s: %w", entrypoint, err)
	}
	return nil
}

// Runner builds and supervises cmd/queue. A worker crash does not cancel parent ctx.
type Runner struct {
	AddProcess func(*exec.Cmd)
	BuildOut   io.Writer
	RuntimeOut io.Writer
	Log        io.Writer
	OnStatus   func(tui.Status)

	cmd         *exec.Cmd
	binPath     string
	dir         string
	binDir      string
	cmdMu       sync.Mutex
	done        chan struct{}
	buildRunner *ctxrun.Runner
	runBuild    func(context.Context, string) error
}

// NewRunner creates a queue runner for the project rooted at dir.
func NewRunner(dir string) *Runner {
	binDir := filepath.Join(dir, "tmp", "bin")
	return &Runner{
		dir:         dir,
		binDir:      binDir,
		buildRunner: ctxrun.New(),
	}
}

func (r *Runner) Run(ctx context.Context, rebuildChan <-chan struct{}) error {
	if err := os.MkdirAll(r.binDir, 0o755); err != nil {
		return fmt.Errorf("create queue binary directory %s: %w", r.binDir, err)
	}
	r.buildRunner.Go(ctx, func(buildCtx context.Context) {
		r.rebuild(buildCtx, ctx)
	})

	for {
		select {
		case <-ctx.Done():
			r.stop()
			return nil
		case <-rebuildChan:
			r.buildRunner.Go(ctx, func(buildCtx context.Context) {
				r.rebuild(buildCtx, ctx)
			})
		}
	}
}

func (r *Runner) rebuild(buildCtx context.Context, runCtx context.Context) {
	r.setStatus(tui.StatusStarting)
	candidate := filepath.Join(r.binDir, "queue_"+strconv.FormatInt(time.Now().UnixNano(), 16))
	r.logBuildf("[run] Building queue...\n")

	runBuild := r.runBuild
	if runBuild == nil {
		runBuild = r.goBuild
	}
	if err := runBuild(buildCtx, candidate); err != nil {
		r.removeBinary(candidate)
		if buildCtx.Err() != nil {
			return
		}
		r.logBuildf("[run] Queue build failed: %v\n", err)
		r.setStatus(tui.StatusError)
		return
	}
	if buildCtx.Err() != nil {
		r.removeBinary(candidate)
		return
	}

	r.cmdMu.Lock()
	if buildCtx.Err() != nil {
		r.removeBinary(candidate)
		r.cmdMu.Unlock()
		return
	}
	previous := r.binPath
	r.stopLocked()

	r.logf("[run] Starting queue worker...\n")
	cmd := exec.CommandContext(runCtx, candidate)
	cmd.Dir = r.dir
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		r.removeBinary(candidate)
		r.cmdMu.Unlock()
		r.logf("[run] Queue stdout pipe failed: %v\n", err)
		r.setStatus(tui.StatusError)
		return
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		r.removeBinary(candidate)
		r.cmdMu.Unlock()
		r.logf("[run] Queue stderr pipe failed: %v\n", err)
		r.setStatus(tui.StatusError)
		return
	}
	if err := cmd.Start(); err != nil {
		r.removeBinary(candidate)
		r.cmdMu.Unlock()
		r.logf("[run] Queue start failed: %v\n", err)
		r.setStatus(tui.StatusError)
		return
	}

	out := r.runtimeWriter()
	go copyLines(stdoutPipe, out)
	go copyLines(stderrPipe, out)

	r.cmd = cmd
	r.binPath = candidate
	done := make(chan struct{})
	r.done = done
	r.cmdMu.Unlock()

	if r.AddProcess != nil {
		r.AddProcess(cmd)
	}
	r.setStatus(tui.StatusReady)
	if previous != "" {
		r.removeBinary(previous)
	}

	go func() {
		waitErr := cmd.Wait()
		close(done)
		r.cmdMu.Lock()
		same := r.cmd == cmd
		if same {
			r.cmd = nil
			r.done = nil
		}
		r.cmdMu.Unlock()
		if waitErr != nil && runCtx.Err() == nil && same {
			r.logf("[run] Queue worker exited: %v\n", waitErr)
			r.setStatus(tui.StatusExited)
		}
	}()
}

func (r *Runner) goBuild(ctx context.Context, outputPath string) error {
	cmd := exec.CommandContext(ctx, "go", "build", "-o", outputPath, entrypoint)
	cmd.Dir = r.dir
	out := r.buildWriter()
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}

func (r *Runner) stop() {
	r.cmdMu.Lock()
	defer r.cmdMu.Unlock()
	r.stopLocked()
}

func (r *Runner) stopLocked() {
	if r.cmd == nil || r.cmd.Process == nil {
		return
	}
	_ = r.cmd.Process.Signal(syscall.SIGTERM)
	done := r.done
	if done != nil {
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = r.cmd.Process.Kill()
			select {
			case <-done:
			case <-time.After(time.Second):
			}
		}
	}
	r.cmd = nil
	r.done = nil
}

func (r *Runner) setStatus(status tui.Status) {
	if r.OnStatus != nil {
		r.OnStatus(status)
	}
}

func (r *Runner) logf(format string, args ...any) {
	_, _ = fmt.Fprintf(r.logWriter(), format, args...)
}

func (r *Runner) removeBinary(path string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		r.logf("[run] Removing queue binary %s: %v\n", path, err)
	}
}

func (r *Runner) logBuildf(format string, args ...any) {
	_, _ = fmt.Fprintf(r.buildWriter(), format, args...)
}

func (r *Runner) logWriter() io.Writer {
	if r.Log != nil {
		return r.Log
	}
	return os.Stdout
}

func (r *Runner) buildWriter() io.Writer {
	if r.BuildOut != nil {
		return r.BuildOut
	}
	return os.Stdout
}

func (r *Runner) runtimeWriter() io.Writer {
	if r.RuntimeOut != nil {
		return r.RuntimeOut
	}
	return os.Stdout
}

func copyLines(reader io.Reader, writer io.Writer) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		_, _ = fmt.Fprintln(writer, scanner.Text())
	}
}
