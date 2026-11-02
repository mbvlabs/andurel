package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/ctxrun"
	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/reload"
)

func TestClearLogsWiredFromConfig(t *testing.T) {
	var called bool
	cfg := Config{
		Dir:       t.TempDir(),
		ClearLogs: func() { called = true },
	}
	s := NewAppServer(cfg)
	if s.clearLogs == nil {
		t.Fatal("expected clearLogs callback to be set")
	}
	s.clearLogs()
	if !called {
		t.Fatal("expected clearLogs callback to be invoked")
	}
}

func TestRunFailsWhenDirectoriesCannotBeCreated(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tmp"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewAppServer(Config{Dir: dir})
	if err := s.Run(t.Context(), nil); err == nil {
		t.Fatal("expected directory creation error")
	}
}

func TestSetRebuildStateInvokesCallback(t *testing.T) {
	var got atomic.Bool
	s := &AppServer{
		onRebuildStateChanged: func(inProgress bool) {
			got.Store(inProgress)
		},
	}

	s.setRebuildState(true)

	if !got.Load() {
		t.Fatal("expected onRebuildStateChanged callback to receive true")
	}
}

func TestStartHealthMonitorSignalsReadyAndClearsRebuildState(t *testing.T) {
	port, closeServer := startHealthyServer(t)
	defer closeServer()

	readyChan := make(chan struct{}, 1)
	rebuildState := make(chan bool, 2)

	s := &AppServer{
		appPort:     port,
		broadcaster: reload.NewBroadcaster(),
		readyChan:   readyChan,
		onRebuildStateChanged: func(inProgress bool) {
			rebuildState <- inProgress
		},
	}

	s.setRebuildState(true)
	select {
	case got := <-rebuildState:
		if !got {
			t.Fatal("expected initial rebuild state to be true")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for initial rebuild state callback")
	}

	ctx := t.Context()
	s.startHealthMonitor(ctx, "")
	defer s.cancelHealthMonitor()

	select {
	case <-readyChan:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for ready signal from health monitor")
	}

	select {
	case got := <-rebuildState:
		if got {
			t.Fatal("expected rebuild state to be cleared (false) after health check")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for rebuild state clear callback")
	}
}

func TestCancelHealthMonitorPreventsReadySignal(t *testing.T) {
	unhealthyPort := getUnusedPort(t)
	readyChan := make(chan struct{}, 1)
	rebuildState := make(chan bool, 1)

	s := &AppServer{
		appPort:     unhealthyPort,
		broadcaster: reload.NewBroadcaster(),
		readyChan:   readyChan,
		onRebuildStateChanged: func(inProgress bool) {
			rebuildState <- inProgress
		},
	}

	ctx := t.Context()
	s.startHealthMonitor(ctx, "")
	s.cancelHealthMonitor()

	time.Sleep(250 * time.Millisecond)

	select {
	case <-readyChan:
		t.Fatal("did not expect ready signal after canceling health monitor")
	default:
	}

	select {
	case got := <-rebuildState:
		t.Fatalf("did not expect rebuild state callback after cancel, got %v", got)
	default:
	}
}

func TestCanceledRebuildOnlyRemovesItsCandidateBinary(t *testing.T) {
	tmpDir := t.TempDir()

	oldActiveBin := filepath.Join(tmpDir, "server_old_active")
	if err := os.WriteFile(oldActiveBin, []byte("old active"), 0644); err != nil {
		t.Fatalf("failed to write old active binary: %v", err)
	}

	s := &AppServer{
		binDir:   tmpDir,
		templDir: filepath.Join(tmpDir, "templ"),
		binPath:  oldActiveBin,
	}

	buildCtx, cancelBuild := context.WithCancel(context.Background())
	defer cancelBuild()
	newActiveBin := filepath.Join(tmpDir, "server_new_active")

	s.runBuild = func(ctx context.Context, candidateBinPath string) error {
		if err := os.WriteFile(candidateBinPath, []byte("candidate"), 0644); err != nil {
			t.Fatalf("failed to write candidate binary: %v", err)
		}
		if err := os.WriteFile(newActiveBin, []byte("new active"), 0644); err != nil {
			t.Fatalf("failed to write new active binary: %v", err)
		}
		s.binPath = newActiveBin
		cancelBuild()
		return ctx.Err()
	}

	if err := s.rebuild(buildCtx, context.Background()); err == nil {
		t.Fatal("expected canceled rebuild to return an error")
	}

	if _, err := os.Stat(newActiveBin); err != nil {
		t.Fatalf("expected active binary to survive canceled rebuild cleanup: %v", err)
	}

	candidates, err := filepath.Glob(filepath.Join(tmpDir, "server_*"))
	if err != nil {
		t.Fatalf("failed to glob candidate binaries: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected only active binaries to remain, got %v", candidates)
	}
}

func TestStartHealthMonitorRemovesCapturedPreviousBinaryOnly(t *testing.T) {
	port, closeServer := startHealthyServer(t)
	defer closeServer()

	tmpDir := t.TempDir()
	previousBin := filepath.Join(tmpDir, "server_previous")
	currentBin := filepath.Join(tmpDir, "server_current")
	if err := os.WriteFile(previousBin, []byte("previous"), 0644); err != nil {
		t.Fatalf("failed to write previous binary: %v", err)
	}
	if err := os.WriteFile(currentBin, []byte("current"), 0644); err != nil {
		t.Fatalf("failed to write current binary: %v", err)
	}

	readyChan := make(chan struct{}, 1)
	s := &AppServer{
		appPort:     port,
		broadcaster: reload.NewBroadcaster(),
		readyChan:   readyChan,
		binPath:     currentBin,
	}

	ctx := t.Context()
	s.startHealthMonitor(ctx, previousBin)
	defer s.cancelHealthMonitor()

	select {
	case <-readyChan:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for ready signal from health monitor")
	}

	if _, err := os.Stat(previousBin); !os.IsNotExist(err) {
		t.Fatalf("expected previous binary to be removed, stat err: %v", err)
	}
	if _, err := os.Stat(currentBin); err != nil {
		t.Fatalf("expected current binary to remain: %v", err)
	}
}

func TestRuntimeOutputGuardSuppressesRepeatedMissingFileLines(t *testing.T) {
	restarts := 0
	guard := newRuntimeOutputGuard(3, func() { restarts++ })

	for i := range 2 {
		emit, notice := guard.handleLine("open tmp/bin/server: no such file or directory")
		if !emit || notice != "" {
			t.Fatalf("line %d should emit without notice, emit=%v notice=%q", i+1, emit, notice)
		}
	}

	emit, notice := guard.handleLine("open tmp/bin/server: no such file or directory")
	if !emit {
		t.Fatal("threshold line should still be emitted")
	}
	if notice == "" {
		t.Fatal("threshold line should produce suppression notice")
	}
	if restarts != 1 {
		t.Fatalf("expected one restart request, got %d", restarts)
	}

	emit, notice = guard.handleLine("open tmp/bin/server: no such file or directory")
	if emit || notice != "" {
		t.Fatalf("post-threshold line should be suppressed without extra notice, emit=%v notice=%q", emit, notice)
	}
	if restarts != 1 {
		t.Fatalf("expected restart request to remain one, got %d", restarts)
	}

	emit, notice = guard.handleLine("normal app log")
	if !emit || notice != "" {
		t.Fatalf("normal line should reset and emit, emit=%v notice=%q", emit, notice)
	}

	emit, notice = guard.handleLine("file not found")
	if !emit || notice != "" {
		t.Fatalf("first missing-file line after reset should emit, emit=%v notice=%q", emit, notice)
	}
	if restarts != 1 {
		t.Fatalf("restart should only fire once per guard, got %d", restarts)
	}
}

func TestForwardRuntimeOutputCollapsesMissingFileSpam(t *testing.T) {
	guard := newRuntimeOutputGuard(2, nil)
	var out bytes.Buffer

	forwardRuntimeOutput(
		bytes.NewBufferString("first\nfile not found\nfile not found\nfile not found\nnormal\n"),
		&out,
		guard,
	)

	got := out.String()
	if !bytes.Contains([]byte(got), []byte("first\n")) {
		t.Fatalf("expected normal line to pass through, got %q", got)
	}
	if count := bytes.Count([]byte(got), []byte("file not found\n")); count != 2 {
		t.Fatalf("expected exactly two missing-file lines before suppression, got %d in %q", count, got)
	}
	if !bytes.Contains([]byte(got), []byte("Suppressing repeated missing-file output")) {
		t.Fatalf("expected suppression notice, got %q", got)
	}
	if !bytes.Contains([]byte(got), []byte("normal\n")) {
		t.Fatalf("expected later normal line to pass through, got %q", got)
	}
}

func TestRepeatedRuntimeMissingFileOutputTriggersRebuild(t *testing.T) {
	tmpDir := t.TempDir()
	var buildCount atomic.Int32

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := &AppServer{
		binDir:                 tmpDir,
		templDir:               filepath.Join(tmpDir, "templ"),
		appPort:                getUnusedPort(t),
		broadcaster:            reload.NewBroadcaster(),
		buildRunner:            ctxrun.New(),
		runtimeRestartDelay:    10 * time.Millisecond,
		runtimeOutputThreshold: 3,
		stdout:                 io.Discard,
		stderr:                 io.Discard,
	}
	s.runBuild = func(ctx context.Context, candidateBinPath string) error {
		buildCount.Add(1)
		return writeMissingFileEmitter(candidateBinPath)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Run(ctx, make(chan struct{}, 1))
	}()

	deadline := time.After(2 * time.Second)
	for buildCount.Load() < 2 {
		select {
		case err := <-errCh:
			t.Fatalf("server exited before runtime restart: %v", err)
		case <-deadline:
			t.Fatalf("timed out waiting for runtime restart; build count=%d", buildCount.Load())
		case <-time.After(10 * time.Millisecond):
		}
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected nil after cancel, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not exit after cancel")
	}
}

func TestStopLockedFailsWhenPortStaysBusy(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := ln.Close(); err != nil {
			t.Errorf("close listener: %v", err)
		}
	})

	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	s := &AppServer{
		appPort:  port,
		portWait: 80 * time.Millisecond,
	}

	err = s.stopLocked()
	if err == nil {
		t.Fatal("expected stopLocked to fail while listen port is held")
	}
}

func TestStopLockedWaitsForPortAfterKill(t *testing.T) {
	port := getUnusedPort(t)
	helper := buildPortHolder(t, port, true)
	cmd := exec.Command(helper)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	// Wait until the port is actually held.
	deadline := time.Now().Add(2 * time.Second)
	for procListenAvailable(port) {
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			t.Fatal("port holder never bound")
		}
		time.Sleep(10 * time.Millisecond)
	}

	s := &AppServer{
		cmd:          cmd,
		appPort:      port,
		portWait:     2 * time.Second,
		stopTermWait: 80 * time.Millisecond,
	}

	if err := s.stopLocked(); err != nil {
		t.Fatalf("stopLocked: %v", err)
	}
	if !procListenAvailable(port) {
		t.Fatal("port should be free after stopLocked")
	}
}

func procListenAvailable(port string) bool {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func buildPortHolder(t *testing.T, port string, ignoreTerm bool) string {
	t.Helper()
	src := filepath.Join(t.TempDir(), "holder.go")
	bin := filepath.Join(t.TempDir(), "holder")
	ignore := "false"
	if ignoreTerm {
		ignore = "true"
	}
	code := fmt.Sprintf(`package main
import (
  "net"
  "net/http"
  "os/signal"
  "syscall"
)
func main() {
  if %s {
    signal.Ignore(syscall.SIGTERM)
  }
  ln, err := net.Listen("tcp", "127.0.0.1:%s")
  if err != nil { panic(err) }
  _ = http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(200)
  }))
}
`, ignore, port)
	if err := os.WriteFile(src, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", bin, src)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build port holder: %v", err)
	}
	return bin
}

func writeMissingFileEmitter(path string) error {
	content := `#!/usr/bin/env sh
i=0
while [ "$i" -lt 20 ]; do
  echo "open tmp/bin/server: no such file or directory" 1>&2
  i=$((i + 1))
  sleep 0.01
done
sleep 5
`
	return os.WriteFile(path, []byte(content), 0o755)
}

func startHealthyServer(t *testing.T) (string, func()) {
	t.Helper()

	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to open listener: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{Handler: handler}
	go func() {
		_ = srv.Serve(ln)
	}()

	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	closeFn := func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}

	return port, closeFn
}

func getUnusedPort(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to reserve port: %v", err)
	}
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	_ = ln.Close()
	return port
}
