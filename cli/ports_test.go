package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mbvlabs/andurel/v2/cli/output"
)

func TestPortsCommandSurface(t *testing.T) {
	cmd := newPortsCommand()
	if cmd.Name() != "ports" {
		t.Fatalf("command name = %q", cmd.Name())
	}
	launch, _, err := cmd.Find([]string{"launch"})
	if err != nil {
		t.Fatalf("find launch: %v", err)
	}
	if launch.Name() != "launch" {
		t.Fatalf("launch name = %q", launch.Name())
	}
	for _, flag := range []string{
		"host", "port", "listen", "lease-dir", "ready-file", "startup-timeout", "stop-timeout",
	} {
		if launch.Flags().Lookup(flag) == nil {
			t.Fatalf("launch is missing --%s", flag)
		}
	}
}

func TestPortLaunchAddressValidation(t *testing.T) {
	for _, listen := range []string{
		"https://127.0.0.1:13714",
		"http://127.0.0.1",
		"http://127.0.0.1:13714/path",
		"http://example.com:13714",
	} {
		if _, _, err := resolvePortLaunchAddress(portLaunchOptions{ListenURL: listen}); err == nil {
			t.Fatalf("expected %q to be rejected", listen)
		}
	}

	assignment, fixed, err := resolvePortLaunchAddress(portLaunchOptions{
		ListenURL: "http://127.0.0.1:13714",
	})
	if err != nil {
		t.Fatalf("fixed listen: %v", err)
	}
	if !fixed || assignment.Port != 13714 {
		t.Fatalf("fixed assignment = %#v, fixed=%v", assignment, fixed)
	}

	if _, _, err := resolvePortLaunchAddress(portLaunchOptions{
		ListenURL: "http://127.0.0.1:13714",
		Port:      13715,
	}); err == nil {
		t.Fatal("expected conflicting --listen/--port to be rejected")
	}

	localhost, allocate, err := resolvePortLaunchAddress(portLaunchOptions{
		Host: "localhost",
		Port: 0,
	})
	if err != nil {
		t.Fatalf("localhost host: %v", err)
	}
	if allocate {
		t.Fatal("port 0 must allocate rather than use a fixed address")
	}
	if localhost.Host != "127.0.0.1" || localhost.ListenURL != "http://127.0.0.1:0" {
		t.Fatalf("localhost assignment = %#v, want the exact IPv4 loopback address", localhost)
	}

	ipv6, fixed, err := resolvePortLaunchAddress(portLaunchOptions{
		ListenURL: "http://[::1]:13714",
	})
	if err != nil {
		t.Fatalf("IPv6 listen: %v", err)
	}
	if !fixed || ipv6.Host != "::1" || ipv6.ListenURL != "http://[::1]:13714" {
		t.Fatalf("IPv6 assignment = %#v, fixed=%v", ipv6, fixed)
	}

	for _, wildcard := range []portLaunchOptions{
		{Host: "0.0.0.0", Port: 13714},
		{Host: "::", Port: 13714},
		{ListenURL: "http://0.0.0.0:13714"},
		{ListenURL: "http://[::]:13714"},
	} {
		if _, _, err := resolvePortLaunchAddress(wildcard); err == nil ||
			!strings.Contains(err.Error(), "wildcard") {
			t.Fatalf("wildcard address %#v was not rejected with an actionable error: %v", wildcard, err)
		}
	}
}

func TestPortLeaseAllocatesDistinctConcurrentPorts(t *testing.T) {
	leaseDir := t.TempDir()

	type result struct {
		lease *portLease
		err   error
	}
	results := make(chan result, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Go(func() {
			lease, err := acquirePortLease(leaseDir, "127.0.0.1", 0, true)
			results <- result{lease: lease, err: err}
		})
	}
	wait.Wait()
	close(results)

	var leases []*portLease
	for result := range results {
		if result.err != nil {
			t.Fatalf("acquire lease: %v", result.err)
		}
		leases = append(leases, result.lease)
	}
	if len(leases) != 2 {
		t.Fatalf("leases = %d", len(leases))
	}
	if leases[0].Port == leases[1].Port {
		t.Fatalf("concurrent leases shared port %d", leases[0].Port)
	}

	first, err := net.Listen("tcp", net.JoinHostPort(leases[0].Host, strconv.Itoa(leases[0].Port)))
	if err != nil {
		t.Fatalf("listen on first lease: %v", err)
	}
	defer first.Close()
	second, err := net.Listen("tcp", net.JoinHostPort(leases[1].Host, strconv.Itoa(leases[1].Port)))
	if err != nil {
		t.Fatalf("listen on second lease: %v", err)
	}
	defer second.Close()

	for _, lease := range leases {
		if err := lease.Release(); err != nil {
			t.Fatalf("release lease: %v", err)
		}
	}
}

func TestPortLeaseFixedPortIsExclusiveAndStaleFilesAreReusable(t *testing.T) {
	leaseDir := t.TempDir()
	port := freeLoopbackPort(t)

	first, err := acquirePortLease(leaseDir, "127.0.0.1", port, false)
	if err != nil {
		t.Fatalf("first lease: %v", err)
	}
	if _, err := acquirePortLease(leaseDir, "127.0.0.1", port, false); err == nil {
		t.Fatal("expected the held fixed port to be rejected")
	}
	if err := first.Release(); err != nil {
		t.Fatalf("release: %v", err)
	}

	// A lease file with metadata but no kernel lock is stale: a crashed
	// launcher leaves it behind, and the next claimant may reuse the port.
	stalePath := portLeasePath(leaseDir, "127.0.0.1", port)
	stale, err := json.Marshal(portLease{
		Host:      "127.0.0.1",
		Port:      port,
		ListenURL: portListenURL("127.0.0.1", port),
		URL:       portListenURL("127.0.0.1", port),
		PID:       999999,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stalePath, stale, 0o600); err != nil {
		t.Fatal(err)
	}

	second, err := acquirePortLease(leaseDir, "127.0.0.1", port, false)
	if err != nil {
		t.Fatalf("stale lease was not reusable: %v", err)
	}
	if err := second.Release(); err != nil {
		t.Fatalf("release second: %v", err)
	}
}

func TestPortLeaseRejectsOccupiedPortWithoutKillingListener(t *testing.T) {
	leaseDir := t.TempDir()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	if _, err := acquirePortLease(leaseDir, "127.0.0.1", port, false); err == nil {
		t.Fatal("expected occupied port to be rejected")
	}

	// The unrelated listener must still be alive and accepting.
	connection, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("unrelated listener was killed: %v", err)
	}
	_ = connection.Close()
}

func TestPortLauncherAllocatesAndReleasesLease(t *testing.T) {
	requirePortHelper(t)
	leaseDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var assignment portAssignment
	err := runPortLauncher(ctx, portLaunchOptions{
		LeaseDir:       leaseDir,
		StartupTimeout: 5 * time.Second,
		StopTimeout:    2 * time.Second,
		Command:        portHelperCommand(),
		ChildStdout:    io.Discard,
		ChildStderr:    io.Discard,
	}, func(ready portAssignment) error {
		assignment = ready
		cancel()
		return nil
	})
	if err != nil {
		t.Fatalf("runPortLauncher: %v", err)
	}
	if assignment.Port == 0 || assignment.URL != assignment.ListenURL {
		t.Fatalf("assignment = %#v", assignment)
	}
	if assignment.Mode != "allocated" {
		t.Fatalf("mode = %q", assignment.Mode)
	}

	lease, err := acquirePortLease(leaseDir, assignment.Host, assignment.Port, false)
	if err != nil {
		t.Fatalf("lease was not released: %v", err)
	}
	_ = lease.Release()
}

func TestPortLauncherHoldsLeaseUntilChildExit(t *testing.T) {
	requirePortHelper(t)
	leaseDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ready := make(chan portAssignment, 1)
	done := make(chan error, 1)
	go func() {
		done <- runPortLauncher(ctx, portLaunchOptions{
			LeaseDir:       leaseDir,
			StartupTimeout: 5 * time.Second,
			StopTimeout:    2 * time.Second,
			Command:        portHelperCommand(),
			ChildStdout:    io.Discard,
			ChildStderr:    io.Discard,
		}, func(assignment portAssignment) error {
			ready <- assignment
			return nil
		})
	}()

	var assignment portAssignment
	select {
	case assignment = <-ready:
	case <-time.After(10 * time.Second):
		t.Fatal("launcher did not become ready")
	}

	if _, err := acquirePortLease(leaseDir, assignment.Host, assignment.Port, false); err == nil {
		t.Fatal("a running launcher's port was claimable")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("launcher: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("launcher did not stop")
	}

	lease, err := acquirePortLease(leaseDir, assignment.Host, assignment.Port, false)
	if err != nil {
		t.Fatalf("lease was not released after child exit: %v", err)
	}
	_ = lease.Release()
}

func TestPortLauncherConcurrentAssignmentsAreDistinct(t *testing.T) {
	requirePortHelper(t)
	leaseDir := t.TempDir()

	assignments := make(chan portAssignment, 2)
	errs := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Go(func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var assignment portAssignment
			err := runPortLauncher(ctx, portLaunchOptions{
				LeaseDir:       leaseDir,
				StartupTimeout: 5 * time.Second,
				StopTimeout:    2 * time.Second,
				Command:        portHelperCommand(),
				ChildStdout:    io.Discard,
				ChildStderr:    io.Discard,
			}, func(ready portAssignment) error {
				assignment = ready
				cancel()
				return nil
			})
			errs <- err
			assignments <- assignment
		})
	}
	wait.Wait()
	close(errs)
	close(assignments)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent launcher: %v", err)
		}
	}
	var ports []int
	for assignment := range assignments {
		ports = append(ports, assignment.Port)
	}
	if len(ports) != 2 || ports[0] == ports[1] {
		t.Fatalf("concurrent assignments = %v", ports)
	}
}

func TestPortLauncherHandsListenAndURLToChild(t *testing.T) {
	requirePortHelper(t)
	envFile := filepath.Join(t.TempDir(), "child.env")
	t.Setenv("ANDUREL_PORT_HELPER_ENV_FILE", envFile)
	// A stale value in the parent environment must not win over the lease.
	t.Setenv("INERTIA_SSR_LISTEN", "http://127.0.0.1:1")
	t.Setenv("INERTIA_SSR_URL", "http://127.0.0.1:1")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var assignment portAssignment
	err := runPortLauncher(ctx, portLaunchOptions{
		LeaseDir:       t.TempDir(),
		StartupTimeout: 5 * time.Second,
		StopTimeout:    2 * time.Second,
		Command:        portHelperCommand(),
		ChildStdout:    io.Discard,
		ChildStderr:    io.Discard,
	}, func(ready portAssignment) error {
		assignment = ready
		data, err := os.ReadFile(envFile)
		if err != nil {
			return err
		}
		want := ready.ListenURL + "\n" + ready.URL
		if string(data) != want {
			return fmt.Errorf("child env = %q, want %q", data, want)
		}
		cancel()
		return nil
	})
	if err != nil {
		t.Fatalf("runPortLauncher: %v", err)
	}
	if assignment.ListenURL != assignment.URL {
		t.Fatalf("listen/url mismatch: %#v", assignment)
	}
}

func TestPortLauncherFixedAddressAndReadyFile(t *testing.T) {
	requirePortHelper(t)
	leaseDir := t.TempDir()
	port := freeLoopbackPort(t)
	readyFile := filepath.Join(t.TempDir(), "ssr.json")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var assignment portAssignment
	err := runPortLauncher(ctx, portLaunchOptions{
		Port:           port,
		LeaseDir:       leaseDir,
		ReadyFile:      readyFile,
		StartupTimeout: 5 * time.Second,
		StopTimeout:    2 * time.Second,
		Command:        portHelperCommand(),
		ChildStdout:    io.Discard,
		ChildStderr:    io.Discard,
	}, func(ready portAssignment) error {
		assignment = ready
		data, err := os.ReadFile(readyFile)
		if err != nil {
			return fmt.Errorf("ready file missing before callback: %w", err)
		}
		var published portAssignment
		if err := json.Unmarshal(data, &published); err != nil {
			return err
		}
		if published.URL != ready.URL || published.Port != port {
			return fmt.Errorf("ready file = %#v, want %#v", published, ready)
		}
		cancel()
		return nil
	})
	if err != nil {
		t.Fatalf("runPortLauncher: %v", err)
	}
	if assignment.Port != port || assignment.Mode != "fixed" {
		t.Fatalf("assignment = %#v", assignment)
	}
	if _, err := os.Stat(readyFile); !os.IsNotExist(err) {
		t.Fatalf("ready file was not removed: %v", err)
	}
}

func TestPortLauncherRefusesChildListeningOnDifferentAddress(t *testing.T) {
	requirePortHelper(t)
	t.Setenv("ANDUREL_PORT_HELPER_BIND_HOST", "127.0.0.2")
	envFile := filepath.Join(t.TempDir(), "child-listen")
	t.Setenv("ANDUREL_PORT_HELPER_ENV_FILE", envFile)
	port := freeLoopbackPortOn(t, "127.0.0.1", "127.0.0.2")

	ctx := t.Context()
	ready := make(chan portAssignment, 1)
	done := make(chan error, 1)
	go func() {
		done <- runPortLauncher(ctx, portLaunchOptions{
			Port:           port,
			LeaseDir:       t.TempDir(),
			StartupTimeout: 10 * time.Second,
			StopTimeout:    2 * time.Second,
			Command:        portHelperCommand(),
			ChildStdout:    io.Discard,
			ChildStderr:    io.Discard,
		}, func(assignment portAssignment) error {
			ready <- assignment
			return nil
		})
	}()

	listenFile := waitForPortHelperEnvFile(t, envFile)
	if !strings.Contains(listenFile, ":"+strconv.Itoa(port)) {
		t.Fatalf("helper listen file = %q, want port %d", listenFile, port)
	}

	// This listener is in the test process, not the launcher child's process
	// group. It answers the health probe on the requested address while the
	// child binds the other loopback address on the same port.
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("listen on unrelated address: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("content-type", "application/json")
		_, _ = io.WriteString(writer, `{"status":"ok"}`)
	})
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	defer func() { _ = server.Close() }()

	select {
	case assignment := <-ready:
		t.Fatalf("launcher published unrelated listener %#v", assignment)
	case err := <-done:
		if err == nil {
			t.Fatal("launcher accepted a child listening on a different address")
		}
		if !strings.Contains(err.Error(), "owned by another process") {
			t.Fatalf("ownership error = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("launcher neither published nor rejected the listener")
	}

	response, err := http.Get(portListenURL("127.0.0.1", port) + "/health")
	if err != nil {
		t.Fatalf("unrelated listener was killed: %v", err)
	}
	_ = response.Body.Close()
}

func TestPortLauncherDoesNotTrustUnrelatedHealthyListener(t *testing.T) {
	requirePortHelper(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("content-type", "application/json")
		_, _ = writer.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	readyCalled := false
	err = runPortLauncher(ctx, portLaunchOptions{
		Port:           port,
		LeaseDir:       t.TempDir(),
		StartupTimeout: 2 * time.Second,
		StopTimeout:    time.Second,
		Command:        portHelperCommand(),
		ChildStdout:    io.Discard,
		ChildStderr:    io.Discard,
	}, func(portAssignment) error {
		readyCalled = true
		return nil
	})
	if err == nil {
		t.Fatal("expected an unrelated listener to block the claim")
	}
	if readyCalled {
		t.Fatal("launcher advertised an unrelated listener as ready")
	}

	response, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("unrelated listener was killed: %v", err)
	}
	_ = response.Body.Close()
}

func TestPortLauncherFailedAcquisitionPreservesPreexistingReadyFile(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	readyFile := filepath.Join(t.TempDir(), "ssr.json")
	sentinel := []byte("{\"owner\":\"someone-else\"}\n")
	if err := os.WriteFile(readyFile, sentinel, 0o600); err != nil {
		t.Fatal(err)
	}

	readyCalled := false
	err = runPortLauncher(t.Context(), portLaunchOptions{
		Port:           port,
		LeaseDir:       t.TempDir(),
		ReadyFile:      readyFile,
		StartupTimeout: 2 * time.Second,
		StopTimeout:    time.Second,
		Command:        portHelperCommand(),
		ChildStdout:    io.Discard,
		ChildStderr:    io.Discard,
	}, func(portAssignment) error {
		readyCalled = true
		return nil
	})
	if err == nil {
		t.Fatal("expected occupied fixed port to fail")
	}
	if readyCalled {
		t.Fatal("failed acquisition published an assignment")
	}
	data, err := os.ReadFile(readyFile)
	if err != nil {
		t.Fatalf("preexisting ready file was removed: %v", err)
	}
	if !bytes.Equal(data, sentinel) {
		t.Fatalf("preexisting ready file changed to %q", data)
	}
}

func TestPortLauncherFailedCompetingLauncherPreservesLiveReadyFile(t *testing.T) {
	requirePortHelper(t)
	leaseDir := t.TempDir()
	port := freeLoopbackPort(t)
	readyFile := filepath.Join(t.TempDir(), "ssr.json")

	cancelFirst, firstAssignments, firstDone := startTestPortLauncher(t, portLaunchOptions{
		Port:           port,
		LeaseDir:       leaseDir,
		ReadyFile:      readyFile,
		StartupTimeout: 5 * time.Second,
		StopTimeout:    2 * time.Second,
		Command:        portHelperCommand(),
		ChildStdout:    io.Discard,
		ChildStderr:    io.Discard,
	})
	defer cancelFirst()
	first := waitTestPortAssignment(t, firstAssignments, firstDone, "first launcher")

	readyCalled := false
	err := runPortLauncher(t.Context(), portLaunchOptions{
		Port:           port,
		LeaseDir:       leaseDir,
		ReadyFile:      readyFile,
		StartupTimeout: 2 * time.Second,
		StopTimeout:    time.Second,
		Command:        portHelperCommand(),
		ChildStdout:    io.Discard,
		ChildStderr:    io.Discard,
	}, func(portAssignment) error {
		readyCalled = true
		return nil
	})
	if err == nil {
		t.Fatal("second launcher acquired the first launcher's port")
	}
	if readyCalled {
		t.Fatal("second launcher published a failed assignment")
	}

	published := readPortAssignmentFile(t, readyFile)
	if published.URL != first.URL || published.ChildPID != first.ChildPID {
		t.Fatalf("failed launcher changed live assignment: got %#v, want %#v", published, first)
	}

	cancelFirst()
	waitTestPortLauncher(t, firstDone, "first launcher")
	if _, err := os.Stat(readyFile); !os.IsNotExist(err) {
		t.Fatalf("first launcher did not clean up its own ready file: %v", err)
	}
}

func TestPortLauncherSharedReadyFileCleanupPreservesReplacement(t *testing.T) {
	requirePortHelper(t)
	leaseDir := t.TempDir()
	readyFile := filepath.Join(t.TempDir(), "ssr.json")

	cancelFirst, firstAssignments, firstDone := startTestPortLauncher(t, portLaunchOptions{
		Port:           freeLoopbackPort(t),
		LeaseDir:       leaseDir,
		ReadyFile:      readyFile,
		StartupTimeout: 5 * time.Second,
		StopTimeout:    2 * time.Second,
		Command:        portHelperCommand(),
		ChildStdout:    io.Discard,
		ChildStderr:    io.Discard,
	})
	defer cancelFirst()
	first := waitTestPortAssignment(t, firstAssignments, firstDone, "first launcher")

	cancelSecond, secondAssignments, secondDone := startTestPortLauncher(t, portLaunchOptions{
		Port:           freeLoopbackPort(t),
		LeaseDir:       leaseDir,
		ReadyFile:      readyFile,
		StartupTimeout: 5 * time.Second,
		StopTimeout:    2 * time.Second,
		Command:        portHelperCommand(),
		ChildStdout:    io.Discard,
		ChildStderr:    io.Discard,
	})
	defer cancelSecond()
	second := waitTestPortAssignment(t, secondAssignments, secondDone, "second launcher")
	if second.URL == first.URL {
		t.Fatalf("competing launchers shared assignment %#v", second)
	}
	published := readPortAssignmentFile(t, readyFile)
	if published.URL != second.URL || published.ChildPID != second.ChildPID {
		t.Fatalf("ready file = %#v, want second launcher %#v", published, second)
	}

	cancelFirst()
	waitTestPortLauncher(t, firstDone, "first launcher")
	published = readPortAssignmentFile(t, readyFile)
	if published.URL != second.URL || published.ChildPID != second.ChildPID {
		t.Fatalf("first launcher removed the second launcher's assignment: %#v", published)
	}

	cancelSecond()
	waitTestPortLauncher(t, secondDone, "second launcher")
	if _, err := os.Stat(readyFile); !os.IsNotExist(err) {
		t.Fatalf("second launcher did not clean up its own ready file: %v", err)
	}
}

func TestPortLauncherReadyFileCleanupPreservesInPlaceReplacement(t *testing.T) {
	requirePortHelper(t)
	readyFile := filepath.Join(t.TempDir(), "ssr.json")
	cancel, assignments, done := startTestPortLauncher(t, portLaunchOptions{
		Port:           freeLoopbackPort(t),
		LeaseDir:       t.TempDir(),
		ReadyFile:      readyFile,
		StartupTimeout: 5 * time.Second,
		StopTimeout:    2 * time.Second,
		Command:        portHelperCommand(),
		ChildStdout:    io.Discard,
		ChildStderr:    io.Discard,
	})
	defer cancel()
	_ = waitTestPortAssignment(t, assignments, done, "launcher")

	replacement := []byte("{\"owner\":\"replacement\"}\n")
	if err := os.WriteFile(readyFile, replacement, 0o600); err != nil {
		t.Fatal(err)
	}
	cancel()
	waitTestPortLauncher(t, done, "launcher")

	data, err := os.ReadFile(readyFile)
	if err != nil {
		t.Fatalf("edited ready file was removed: %v", err)
	}
	if !bytes.Equal(data, replacement) {
		t.Fatalf("ready file = %q, want replacement %q", data, replacement)
	}
}

func TestPortLauncherChildExitBeforeReadyFails(t *testing.T) {
	t.Setenv("ANDUREL_PORT_HELPER", "1")
	t.Setenv("ANDUREL_PORT_HELPER_MODE", "exit")
	ctx := t.Context()

	readyCalled := false
	err := runPortLauncher(ctx, portLaunchOptions{
		LeaseDir:       t.TempDir(),
		StartupTimeout: 3 * time.Second,
		StopTimeout:    time.Second,
		Command:        portHelperCommand(),
		ChildStdout:    io.Discard,
		ChildStderr:    io.Discard,
	}, func(portAssignment) error {
		readyCalled = true
		return nil
	})
	if err == nil {
		t.Fatal("expected a child exit before readiness to fail")
	}
	if readyCalled {
		t.Fatal("launcher published a failed child")
	}
}

func TestPortLauncherChildCrashAfterReadyFailsAndReleasesLease(t *testing.T) {
	t.Setenv("ANDUREL_PORT_HELPER", "1")
	t.Setenv("ANDUREL_PORT_HELPER_MODE", "crash")
	leaseDir := t.TempDir()
	ctx := t.Context()

	var assignment portAssignment
	err := runPortLauncher(ctx, portLaunchOptions{
		LeaseDir:       leaseDir,
		StartupTimeout: 5 * time.Second,
		StopTimeout:    2 * time.Second,
		Command:        portHelperCommand(),
		ChildStdout:    io.Discard,
		ChildStderr:    io.Discard,
	}, func(ready portAssignment) error {
		assignment = ready
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "exited unexpectedly") {
		t.Fatalf("expected post-ready crash error, got %v", err)
	}
	if assignment.Port == 0 {
		t.Fatal("launcher never published the assignment")
	}

	lease, err := acquirePortLease(leaseDir, assignment.Host, assignment.Port, false)
	if err != nil {
		t.Fatalf("lease was not released after crash: %v", err)
	}
	_ = lease.Release()
}

func TestPortsLaunchCommandJSONContract(t *testing.T) {
	requirePortHelper(t)
	port := freeLoopbackPort(t)
	readyFile := filepath.Join(t.TempDir(), "ready.json")

	cmd := newPortsLaunchCommand()
	output.RegisterPersistentFlags(cmd)
	var stdout syncBuffer
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{
		"--json",
		"--port", strconv.Itoa(port),
		"--lease-dir", t.TempDir(),
		"--ready-file", readyFile,
		"--startup-timeout", "5s",
		"--",
		portHelperCommand()[0],
		portHelperCommand()[1],
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- cmd.ExecuteContext(ctx)
	}()

	var assignment portAssignment
	deadline := time.Now().Add(10 * time.Second)
	for {
		data, err := os.ReadFile(readyFile)
		if err == nil {
			if err := json.Unmarshal(data, &assignment); err != nil {
				t.Fatalf("decode ready file: %v", err)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("ready file was not published: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if assignment.Port != port || assignment.URL != portListenURL("127.0.0.1", port) {
		t.Fatalf("assignment = %#v", assignment)
	}
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ports launch --json: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("ports launch did not stop after cancellation")
	}

	var envelope struct {
		OK   bool           `json:"ok"`
		Data portAssignment `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode JSON envelope: %v\n%s", err, stdout.String())
	}
	if !envelope.OK || envelope.Data.Port != port {
		t.Fatalf("JSON envelope = %#v", envelope)
	}
}

func TestPortLauncherRetriesBindRaceOnAutoAllocation(t *testing.T) {
	t.Setenv("ANDUREL_PORT_HELPER", "1")
	t.Setenv("ANDUREL_PORT_HELPER_MODE", "bind-race-once")
	counter := filepath.Join(t.TempDir(), "bind-race")
	t.Setenv("ANDUREL_PORT_HELPER_COUNTER", counter)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var assignment portAssignment
	err := runPortLauncher(ctx, portLaunchOptions{
		LeaseDir:       t.TempDir(),
		StartupTimeout: 5 * time.Second,
		StopTimeout:    2 * time.Second,
		Command:        portHelperCommand(),
		ChildStdout:    io.Discard,
		ChildStderr:    io.Discard,
	}, func(ready portAssignment) error {
		assignment = ready
		cancel()
		return nil
	})
	if err != nil {
		t.Fatalf("runPortLauncher: %v", err)
	}

	first, err := os.ReadFile(counter)
	if err != nil {
		t.Fatalf("read first bind-race URL: %v", err)
	}
	firstURL, err := url.Parse(string(first))
	if err != nil {
		t.Fatalf("parse first bind-race URL: %v", err)
	}
	firstPort, err := strconv.Atoi(firstURL.Port())
	if err != nil {
		t.Fatal(err)
	}
	if assignment.Port == firstPort {
		t.Fatalf("launcher retried on the same port %d", firstPort)
	}
}

// TestPortLauncherHelper is the child process used by launcher tests. It is
// inert unless ANDUREL_PORT_HELPER=1 is set in the environment.
func TestPortLauncherHelper(t *testing.T) {
	if os.Getenv("ANDUREL_PORT_HELPER") != "1" {
		return
	}

	mode := os.Getenv("ANDUREL_PORT_HELPER_MODE")
	if mode == "exit" {
		os.Exit(1)
	}
	if mode == "bind-race-once" {
		counter := os.Getenv("ANDUREL_PORT_HELPER_COUNTER")
		if data, err := os.ReadFile(counter); err != nil || len(data) == 0 {
			_ = os.WriteFile(counter, []byte(os.Getenv("INERTIA_SSR_LISTEN")), 0o644)
			fmt.Fprintln(os.Stderr, "Error: listen EADDRINUSE: address already in use")
			os.Exit(1)
		}
	}

	listen := os.Getenv("INERTIA_SSR_LISTEN")
	parsed, err := url.Parse(listen)
	if err != nil || parsed.Host == "" {
		os.Exit(2)
	}
	if envFile := os.Getenv("ANDUREL_PORT_HELPER_ENV_FILE"); envFile != "" {
		_ = os.WriteFile(envFile, []byte(listen+"\n"+os.Getenv("INERTIA_SSR_URL")), 0o644)
	}
	bindAddress := parsed.Host
	if bindHost := os.Getenv("ANDUREL_PORT_HELPER_BIND_HOST"); bindHost != "" {
		_, port, splitErr := net.SplitHostPort(parsed.Host)
		if splitErr != nil {
			os.Exit(2)
		}
		bindAddress = net.JoinHostPort(bindHost, port)
	}

	shutdown := make(chan struct{})
	var once sync.Once
	server := &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/health":
			writer.Header().Set("content-type", "application/json")
			_, _ = writer.Write([]byte(`{"status":"ok"}`))
		case "/shutdown":
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte(`{}`))
			once.Do(func() { close(shutdown) })
		default:
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte(`{}`))
		}
	})}

	listener, err := net.Listen("tcp", bindAddress)
	if err != nil {
		os.Exit(2)
	}
	go func() { _ = server.Serve(listener) }()

	if mode == "crash" {
		time.Sleep(300 * time.Millisecond)
		os.Exit(1)
	}

	<-shutdown
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func portHelperCommand() []string {
	return []string{os.Args[0], "-test.run=^TestPortLauncherHelper$"}
}

func requirePortHelper(t *testing.T) {
	t.Helper()
	t.Setenv("ANDUREL_PORT_HELPER", "1")
	t.Setenv("ANDUREL_PORT_HELPER_MODE", "steady")
}

func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	return freeLoopbackPortOn(t, "127.0.0.1")
}

func freeLoopbackPortOn(t *testing.T, hosts ...string) int {
	t.Helper()
	for range 32 {
		listener, err := net.Listen("tcp", net.JoinHostPort(hosts[0], "0"))
		if err != nil {
			t.Fatalf("reserve port on %s: %v", hosts[0], err)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		_ = listener.Close()

		available := true
		for _, host := range hosts[1:] {
			probe, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
			if err != nil {
				available = false
				break
			}
			_ = probe.Close()
		}
		if available {
			return port
		}
	}
	t.Fatalf("no port is free on all hosts %v", hosts)
	return 0
}

func waitForPortHelperEnvFile(t *testing.T, path string) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && len(data) > 0 {
			return string(data)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("port helper never wrote %s", path)
	return ""
}

func readPortAssignmentFile(t *testing.T, path string) portAssignment {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ready file: %v", err)
	}
	var assignment portAssignment
	if err := json.Unmarshal(data, &assignment); err != nil {
		t.Fatalf("decode ready file: %v", err)
	}
	return assignment
}

func startTestPortLauncher(
	t *testing.T,
	opts portLaunchOptions,
) (context.CancelFunc, <-chan portAssignment, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	assignments := make(chan portAssignment, 1)
	done := make(chan error, 1)
	go func() {
		done <- runPortLauncher(ctx, opts, func(assignment portAssignment) error {
			assignments <- assignment
			return nil
		})
	}()
	return cancel, assignments, done
}

func waitTestPortAssignment(
	t *testing.T,
	assignments <-chan portAssignment,
	done <-chan error,
	label string,
) portAssignment {
	t.Helper()
	select {
	case assignment := <-assignments:
		return assignment
	case err := <-done:
		t.Fatalf("%s exited before publishing: %v", label, err)
	case <-time.After(10 * time.Second):
		t.Fatalf("%s did not publish within 10s", label)
	}
	return portAssignment{}
}

func waitTestPortLauncher(t *testing.T, done <-chan error, label string) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("%s did not stop within 10s", label)
	}
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (buffer *syncBuffer) Write(data []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.buf.Write(data)
}

func (buffer *syncBuffer) Bytes() []byte {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return append([]byte(nil), buffer.buf.Bytes()...)
}

func (buffer *syncBuffer) String() string {
	return string(buffer.Bytes())
}
