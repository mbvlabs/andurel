//go:build unix

package devserver

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/proc"
)

// TestCleanupStopsProcessGroupAfterLeaderExits guards F12e: a package
// manager can exit while the Vite process it started keeps running in the
// same process group. Cleanup must still stop the group.
func TestCleanupStopsProcessGroupAfterLeaderExits(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	// The leader starts a child that ignores SIGTERM and records its PID.
	script := "(trap '' TERM; while true; do sleep 0.05; done) & echo $! > " + pidFile + "; wait"
	cmd := exec.Command("sh", "-c", script)
	proc.ConfigureProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { _ = cmd.Wait() }()

	childPID := waitForPIDFile(t, pidFile)
	procs := &processSet{}
	procs.add(cmd)

	// Kill only the leader; the child keeps running in the group.
	_ = cmd.Process.Kill()
	time.Sleep(100 * time.Millisecond)
	if !pidAlive(childPID) {
		t.Fatal("test setup: child should outlive the leader")
	}

	procs.cleanup(io.Discard, nil)

	assertPIDGone(t, childPID, "cleanup left a process-group member running")
}

func TestStopProcessGroupStopsGrandchildren(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	script := "(while true; do sleep 0.05; done) & echo $! > " + pidFile + "; wait"
	cmd := exec.Command("sh", "-c", script)
	proc.ConfigureProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	childPID := waitForPIDFile(t, pidFile)

	stopProcessGroup(cmd, waitDone(done), 500*time.Millisecond)

	assertPIDGone(t, childPID, "stopProcessGroup left a grandchild running")
}

func waitForPIDFile(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(path)
		if err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("pid file %s was not written", path)
	return 0
}

func assertPIDGone(t *testing.T, pid int, message string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && pidAlive(pid) {
		time.Sleep(20 * time.Millisecond)
	}
	if pidAlive(pid) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		t.Fatal(message)
	}
}

func pidAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
