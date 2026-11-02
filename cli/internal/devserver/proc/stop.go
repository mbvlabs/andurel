package proc

import (
	"os/exec"
	"syscall"
	"time"
)

const (
	DefaultTermWait = 3 * time.Second
	DefaultKillWait = 1 * time.Second
)

// StopOptions controls graceful-then-forceful process teardown.
type StopOptions struct {
	// TermWait is how long to wait after SIGTERM before SIGKILL.
	TermWait time.Duration
	// KillWait is how long to wait after SIGKILL for the process to exit.
	KillWait time.Duration
	// Done, when non-nil, is preferred over spawning a Wait goroutine.
	Done <-chan struct{}
	// GroupKill, when true, sends SIGKILL to the process group (requires Setpgid).
	GroupKill bool
}

// Stop sends SIGTERM, waits, then SIGKILLs (optionally the process group) and
// waits again for exit. It does not wait for listen ports — callers that own a
// known port should follow with WaitForPortRelease.
func Stop(cmd *exec.Cmd, opts StopOptions) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	termWait := opts.TermWait
	if termWait <= 0 {
		termWait = DefaultTermWait
	}
	killWait := opts.KillWait
	if killWait <= 0 {
		killWait = DefaultKillWait
	}

	_ = cmd.Process.Signal(syscall.SIGTERM)

	done := opts.Done
	var localDone chan error
	if done == nil {
		localDone = make(chan error, 1)
		go func() { localDone <- cmd.Wait() }()
	}

	if waitExit(done, localDone, termWait) {
		return
	}

	if opts.GroupKill {
		_ = SignalProcessGroup(cmd, syscall.SIGKILL)
	} else {
		_ = cmd.Process.Kill()
	}

	_ = waitExit(done, localDone, killWait)
}

func waitExit(done <-chan struct{}, localDone <-chan error, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	if done != nil {
		select {
		case <-done:
			return true
		case <-timer.C:
			return false
		}
	}
	select {
	case <-localDone:
		return true
	case <-timer.C:
		return false
	}
}
