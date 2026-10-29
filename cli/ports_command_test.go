package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mbvlabs/andurel/v2/cli/output"
)

// TestPortsCommandDispatch covers the command registration seam: the ports
// group must be reachable through the real root command, keep help successful,
// and turn an unknown subcommand into a usage error like every other group.
func TestPortsCommandDispatch(t *testing.T) {
	help := runCLITest(t, "ports", "--help")
	if help.err != nil {
		t.Fatalf("ports --help: %v\n%s", help.err, help.stderr)
	}
	if !strings.Contains(help.stdout, "ports launch") {
		t.Fatalf("ports --help does not mention ports launch:\n%s", help.stdout)
	}

	unknown := runCLITest(t, "ports", "definitely-not-a-command")
	if unknown.err == nil {
		t.Fatal("ports definitely-not-a-command should fail")
	}
	if code := output.ExitCode(unknown.err); code != output.ExitUsage {
		t.Fatalf("unknown ports subcommand exit code = %d, want %d", code, output.ExitUsage)
	}
	if !strings.Contains(unknown.err.Error(), "definitely-not-a-command") {
		t.Fatalf("unknown subcommand error = %v", unknown.err)
	}
}

// TestPortsLaunchCLIPublishesJSONAndShutsDownCleanly drives the built CLI, not
// the RunE callback: the launcher starts a real child, waits for the intended
// child's /health, publishes the ready file and JSON envelope, and exits cleanly
// on SIGTERM. The child is this test binary running a helper HTTP server.
func TestPortsLaunchCLIPublishesJSONAndShutsDownCleanly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compiled ports launcher test in short mode")
	}
	bin := buildAndurelCLI(t)
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("test executable: %v", err)
	}
	leaseDir := t.TempDir()
	readyFile := filepath.Join(t.TempDir(), "ssr-port.json")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(
		ctx,
		bin,
		"ports", "launch",
		"--json",
		"--lease-dir", leaseDir,
		"--ready-file", readyFile,
		"--startup-timeout", "30s",
		"--",
		self, "-test.run=TestPortsLaunchHelperProcess",
	)
	cmd.Env = append(os.Environ(), "ANDUREL_PORTS_HELPER=1")
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start ports launch: %v", err)
	}
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()

	assignment := waitForPortReadyFile(t, readyFile, waited, &stderr)
	if assignment.Port == 0 || assignment.URL == "" || assignment.ChildPID == 0 {
		t.Fatalf("published assignment = %#v", assignment)
	}
	if assignment.LeaseFile == "" || !strings.HasPrefix(assignment.LeaseFile, leaseDir) {
		t.Fatalf("lease file = %q, want one inside %s", assignment.LeaseFile, leaseDir)
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("signal launcher: %v", err)
	}
	select {
	case err := <-waited:
		if err != nil {
			t.Fatalf("launcher exit after SIGTERM: %v\nstderr:\n%s", err, stderr.String())
		}
	case <-ctx.Done():
		t.Fatalf("launcher did not exit after SIGTERM\nstderr:\n%s", stderr.String())
	}

	if _, err := os.Stat(readyFile); !os.IsNotExist(err) {
		t.Fatalf("ready file %s still exists after clean shutdown (err=%v)", readyFile, err)
	}

	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Port      int    `json:"port"`
			URL       string `json:"url"`
			LeaseFile string `json:"lease_file"`
			ChildPID  int    `json:"child_pid"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout.String())), &envelope); err != nil {
		t.Fatalf("launcher stdout is not one JSON envelope: %v\n%s", err, stdout.String())
	}
	if !envelope.OK || envelope.Data.Port != assignment.Port ||
		envelope.Data.URL != assignment.URL {
		t.Fatalf("JSON envelope = %#v, ready file = %#v", envelope, assignment)
	}
}

// TestPortsLaunchCLIFailureIsNonzero covers the failure path through the real
// command: a child that cannot start must produce a nonzero exit, not a
// successful assignment.
func TestPortsLaunchCLIFailureIsNonzero(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compiled ports launcher test in short mode")
	}
	bin := buildAndurelCLI(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(
		ctx,
		bin,
		"ports", "launch",
		"--json",
		"--lease-dir", t.TempDir(),
		"--startup-timeout", "5s",
		"--",
		filepath.Join(t.TempDir(), "missing-child"),
	)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("launching a missing child succeeded:\nstdout:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "start SSR launcher child") {
		t.Fatalf("stderr does not explain the child start failure:\n%s", stderr.String())
	}
}

type portReadyAssignment struct {
	Port      int    `json:"port"`
	URL       string `json:"url"`
	LeaseFile string `json:"lease_file"`
	ChildPID  int    `json:"child_pid"`
}

func waitForPortReadyFile(
	t *testing.T,
	path string,
	waited <-chan error,
	stderr *strings.Builder,
) portReadyAssignment {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && len(data) > 0 {
			var assignment portReadyAssignment
			if err := json.Unmarshal(data, &assignment); err != nil {
				t.Fatalf("decode ready file: %v\n%s", err, data)
			}
			return assignment
		}
		select {
		case err := <-waited:
			t.Fatalf("launcher exited before publishing readiness: %v\nstderr:\n%s", err, stderr.String())
		default:
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("ready file %s never appeared\nstderr:\n%s", path, stderr.String())
	return portReadyAssignment{}
}

func buildAndurelCLI(t *testing.T) string {
	t.Helper()
	// findGoModRoot is a package seam that tests repoint at a temporary project;
	// the compiled CLI must come from the repository this test runs in.
	repo := repositoryRoot(t)
	bin := filepath.Join(t.TempDir(), "andurel")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build CLI: %v\n%s", err, out)
	}
	return bin
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod above %s", dir)
		}
		dir = parent
	}
}

// TestPortsLaunchHelperProcess is not a normal test. The launcher starts this
// test binary as its child with INERTIA_SSR_LISTEN set; the helper binds that
// address and answers the readiness probe until the launcher stops it.
func TestPortsLaunchHelperProcess(t *testing.T) {
	if os.Getenv("ANDUREL_PORTS_HELPER") != "1" {
		return
	}
	listenURL := os.Getenv("INERTIA_SSR_LISTEN")
	address := strings.TrimPrefix(listenURL, "http://")
	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Fprintln(os.Stderr, "helper listen:", err)
		os.Exit(2)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	for {
		time.Sleep(time.Hour)
	}
}
