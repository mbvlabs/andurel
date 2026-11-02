//go:build !unix

package proc

import (
	"os/exec"
	"syscall"
)

// ConfigureProcessGroup is a no-op on non-unix platforms.
func ConfigureProcessGroup(cmd *exec.Cmd) {}

// SignalProcessGroup falls back to signaling the single process.
func SignalProcessGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Signal(sig)
}

// HasProcessGroup is always false on non-unix platforms.
func HasProcessGroup(cmd *exec.Cmd) bool {
	return false
}

// GroupAlive reports whether cmd has not been reaped yet.
func GroupAlive(cmd *exec.Cmd) bool {
	return cmd != nil && cmd.Process != nil && cmd.ProcessState == nil
}
