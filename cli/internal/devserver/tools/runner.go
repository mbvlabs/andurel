package tools

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/tui"
)

// registry maps allowlisted tool names to argv (path relative to project root).
var registry = map[string][]string{
	"mailpit": {
		filepath.Join("bin", "mailpit"),
		"--smtp=0.0.0.0:1025",
		"--listen=0.0.0.0:8025",
	},
}

// ResolveArgv returns the command argv for an allowlisted tool name.
func ResolveArgv(name string) ([]string, error) {
	argv, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown tool %q", name)
	}
	out := make([]string, len(argv))
	copy(out, argv)
	return out, nil
}

// Allowed reports whether name is a registered sidecar tool.
func Allowed(name string) bool {
	_, ok := registry[name]
	return ok
}

// Runner supervises a single long-lived sidecar. A crash does not cancel ctx;
// the tab is marked exited/error and Run waits until the parent cancels.
type Runner struct {
	Name string
	// Dir is the project root. Relative tool paths resolve against it and
	// the tool runs there. Empty means the current working directory.
	Dir        string
	AddProcess func(*exec.Cmd)
	Out        io.Writer
	Log        io.Writer
	OnStatus   func(tui.Status)

	// commandContext is overridden in tests.
	commandContext func(ctx context.Context, name string, arg ...string) *exec.Cmd
}

// NewRunner creates a sidecar runner for the given allowlisted tool name.
func NewRunner(name string) *Runner {
	return &Runner{Name: name}
}

// Run starts the tool once and waits until ctx is cancelled.
// Start/exit failures only update status; they never cancel the parent.
func (r *Runner) Run(ctx context.Context) error {
	argv, err := ResolveArgv(r.Name)
	if err != nil {
		r.logf("[run] %s: %v\n", r.Name, err)
		r.setStatus(tui.StatusError)
		<-ctx.Done()
		return nil
	}

	r.setStatus(tui.StatusStarting)
	r.logf("[run] Starting %s...\n", r.Name)

	cmdFn := r.commandContext
	if cmdFn == nil {
		cmdFn = exec.CommandContext
	}
	if r.Dir != "" && !filepath.IsAbs(argv[0]) {
		argv[0] = filepath.Join(r.Dir, argv[0])
	}
	cmd := cmdFn(ctx, argv[0], argv[1:]...)
	if r.Dir != "" {
		cmd.Dir = r.Dir
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		r.logf("[run] %s stdout pipe failed: %v\n", r.Name, err)
		r.setStatus(tui.StatusError)
		<-ctx.Done()
		return nil
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		r.logf("[run] %s stderr pipe failed: %v\n", r.Name, err)
		r.setStatus(tui.StatusError)
		<-ctx.Done()
		return nil
	}

	if err := cmd.Start(); err != nil {
		r.logf("[run] %s start failed: %v\n", r.Name, err)
		r.setStatus(tui.StatusError)
		<-ctx.Done()
		return nil
	}

	out := r.writer()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		copyLines(stdoutPipe, out)
	}()
	go func() {
		defer wg.Done()
		copyLines(stderrPipe, out)
	}()

	if r.AddProcess != nil {
		r.AddProcess(cmd)
	}
	r.setStatus(tui.StatusReady)

	waitErr := cmd.Wait()
	wg.Wait()

	if ctx.Err() != nil {
		return nil
	}
	if waitErr != nil {
		r.logf("[run] %s exited: %v\n", r.Name, waitErr)
		r.setStatus(tui.StatusError)
	} else {
		r.logf("[run] %s exited\n", r.Name)
		r.setStatus(tui.StatusExited)
	}
	<-ctx.Done()
	return nil
}

func (r *Runner) setStatus(status tui.Status) {
	if r.OnStatus != nil {
		r.OnStatus(status)
	}
}

func (r *Runner) logf(format string, args ...any) {
	fmt.Fprintf(r.writer(), format, args...)
}

func (r *Runner) writer() io.Writer {
	if r.Out != nil {
		return r.Out
	}
	if r.Log != nil {
		return r.Log
	}
	return os.Stdout
}

func copyLines(reader io.Reader, writer io.Writer) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		fmt.Fprintln(writer, scanner.Text())
	}
}
