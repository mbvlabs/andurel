//go:build linux

package cli

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestVerifyPortOwnedByChildAcceptsIntendedProcessGroup(t *testing.T) {
	port := reservePortForOwnerTest(t)
	ready := filepath.Join(t.TempDir(), "ready")
	child := exec.Command(os.Args[0], "-test.run=TestPortOwnerHelperProcess")
	child.Env = append(
		os.Environ(),
		"ANDUREL_PORT_OWNER_HELPER=1",
		"ANDUREL_PORT_OWNER_PORT="+strconv.Itoa(port),
		"ANDUREL_PORT_OWNER_READY="+ready,
	)
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := child.Start(); err != nil {
		t.Fatalf("start owner helper: %v", err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-child.Process.Pid, syscall.SIGKILL)
		_ = child.Wait()
	})
	waitForOwnerHelper(t, ready)

	if err := verifyPortOwnedByChild("127.0.0.1", port, child.Process.Pid); err != nil {
		t.Fatalf("verify intended child ownership: %v", err)
	}
}

func TestVerifyPortOwnedByChildRejectsForeignListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	port := listener.Addr().(*net.TCPAddr).Port

	child := exec.Command("sleep", "30")
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := child.Start(); err != nil {
		t.Fatalf("start unrelated child: %v", err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-child.Process.Pid, syscall.SIGKILL)
		_ = child.Wait()
	})

	err = verifyPortOwnedByChild("127.0.0.1", port, child.Process.Pid)
	if err == nil {
		t.Fatal("a listener owned by another process must not satisfy the child ownership proof")
	}
	if !strings.Contains(err.Error(), "owned by another process") {
		t.Fatalf("ownership error = %v", err)
	}
}

func TestPortOwnerHelperProcess(t *testing.T) {
	if os.Getenv("ANDUREL_PORT_OWNER_HELPER") != "1" {
		return
	}
	port, err := strconv.Atoi(os.Getenv("ANDUREL_PORT_OWNER_PORT"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "helper: invalid port:", err)
		os.Exit(2)
	}
	ownerHelperListener, err = net.Listen(
		"tcp",
		net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "helper listen:", err)
		os.Exit(2)
	}
	if err := os.WriteFile(os.Getenv("ANDUREL_PORT_OWNER_READY"), []byte("ready"), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "helper ready file:", err)
		os.Exit(2)
	}
	for {
		time.Sleep(time.Hour)
	}
}

// ownerHelperListener keeps the helper's listener reachable so its finalizer
// cannot close the socket while the helper sleeps.
var ownerHelperListener net.Listener

func reservePortForOwnerTest(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("close reserved port: %v", err)
	}
	return port
}

func waitForOwnerHelper(t *testing.T, ready string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(ready); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("owner helper never became ready at %s", ready)
}
