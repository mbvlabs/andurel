//go:build unix

package proc

import (
	"errors"
	"os/exec"
	"syscall"
)

// ConfigureProcessGroup puts cmd in its own process group so later
// SignalProcessGroup can reach grandchildren (e.g. Vite under pnpm).
func ConfigureProcessGroup(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// SignalProcessGroup sends sig to cmd's process group when Setpgid was used,
// otherwise falls back to signaling the single process.
func SignalProcessGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	pid := cmd.Process.Pid
	if pid <= 0 {
		return cmd.Process.Signal(sig)
	}

	if cmd.SysProcAttr != nil && cmd.SysProcAttr.Setpgid {
		err := syscall.Kill(-pid, sig)
		if err == nil || errors.Is(err, syscall.ESRCH) {
			return err
		}
	}
	return cmd.Process.Signal(sig)
}

// HasProcessGroup reports whether cmd was started with Setpgid.
func HasProcessGroup(cmd *exec.Cmd) bool {
	return cmd != nil && cmd.SysProcAttr != nil && cmd.SysProcAttr.Setpgid
}

// GroupAlive reports whether any process in cmd's process group is still
// running. Without a process group it reports whether cmd itself runs.
func GroupAlive(cmd *exec.Cmd) bool {
	if cmd == nil || cmd.Process == nil || cmd.Process.Pid <= 0 {
		return false
	}
	pid := cmd.Process.Pid
	if HasProcessGroup(cmd) {
		pid = -pid
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
