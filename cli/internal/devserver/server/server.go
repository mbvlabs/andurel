package server

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/ctxrun"
	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/proc"
	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/reload"
	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/state"
)

type AppServer struct {
	cmd                    *exec.Cmd
	dir                    string
	binPath                string
	binDir                 string
	templDir               string
	appPort                string
	broadcaster            *reload.Broadcaster
	addProcess             func(*exec.Cmd)
	readyChan              chan<- struct{}
	onRebuildStateChanged  func(bool)
	stateTracker           *state.Tracker
	clearLogs              func()
	healthMu               sync.Mutex
	healthCancel           context.CancelFunc
	buildRunner            *ctxrun.Runner
	cmdMu                  sync.Mutex
	runBuild               func(context.Context, string) error
	runtimeRestartChan     chan<- struct{}
	runtimeRestartDelay    time.Duration
	runtimeOutputThreshold int
	stdout                 io.Writer
	stderr                 io.Writer
	buildOut               io.Writer
	log                    io.Writer
	onStatus               func(string)
	portWait               time.Duration
	stopTermWait           time.Duration
}

type Config struct {
	// Dir is the project root. Builds and the app process run there.
	Dir                   string
	AppPort               string
	Broadcaster           *reload.Broadcaster
	AddProcess            func(*exec.Cmd)
	ReadyChan             chan<- struct{}
	OnRebuildStateChanged func(bool)
	StateTracker          *state.Tracker
	ClearLogs             func()
	Stdout                io.Writer
	Stderr                io.Writer
	BuildOut              io.Writer
	Log                   io.Writer
	OnStatus              func(string)
}

func (s *AppServer) makeBinaryPath() string {
	return filepath.Join(s.binDir, "server_"+strconv.FormatInt(time.Now().UnixNano(), 16))
}

func NewAppServer(cfg Config) *AppServer {
	binDir := filepath.Join(cfg.Dir, "tmp", "bin")
	templDir := filepath.Join(cfg.Dir, "tmp", "templ")
	s := &AppServer{
		dir:                   cfg.Dir,
		binDir:                binDir,
		templDir:              templDir,
		appPort:               cfg.AppPort,
		broadcaster:           cfg.Broadcaster,
		addProcess:            cfg.AddProcess,
		readyChan:             cfg.ReadyChan,
		onRebuildStateChanged: cfg.OnRebuildStateChanged,
		stateTracker:          cfg.StateTracker,
		clearLogs:             cfg.ClearLogs,
		buildRunner:           ctxrun.New(),
		stdout:                cfg.Stdout,
		stderr:                cfg.Stderr,
		buildOut:              cfg.BuildOut,
		log:                   cfg.Log,
		onStatus:              cfg.OnStatus,
	}
	s.runBuild = s.goBuild
	return s
}

func (s *AppServer) Run(ctx context.Context, rebuildChan <-chan struct{}) error {
	for _, dir := range []string{s.binDir, s.templDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}
	runtimeRestartChan := make(chan struct{}, 1)
	s.runtimeRestartChan = runtimeRestartChan

	s.setRebuildState(true)
	s.setStatus("starting")
	s.buildRunner.Go(ctx, func(buildCtx context.Context) {
		if err := s.rebuild(buildCtx, ctx); err != nil {
			s.logBuildf("[run] Initial build failed: %v\n", err)
			s.setRebuildState(false)
			s.setStatus("error")
		}
	})

	for {
		select {
		case <-ctx.Done():
			s.cancelHealthMonitor()
			return nil
		case <-rebuildChan:
			s.setRebuildState(true)
			s.setStatus("starting")
			s.buildRunner.Go(ctx, func(buildCtx context.Context) {
				if err := s.rebuild(buildCtx, ctx); err != nil {
					s.logBuildf("[run] Build failed: %v\n", err)
					s.setRebuildState(false)
					s.setStatus("error")
				}
			})
		case <-runtimeRestartChan:
			s.logf("[run] Repeated missing-file output detected, restarting app...\n")
			s.setRebuildState(true)
			s.setStatus("starting")
			s.buildRunner.Go(ctx, func(buildCtx context.Context) {
				if err := waitForRuntimeRestart(buildCtx, s.runtimeRestartBackoff()); err != nil {
					s.setRebuildState(false)
					return
				}
				if err := s.rebuild(buildCtx, ctx); err != nil {
					s.logBuildf("[run] Runtime restart failed: %v\n", err)
					s.setRebuildState(false)
					s.setStatus("error")
				}
			})
		}
	}
}

func (s *AppServer) rebuild(buildCtx context.Context, appCtx context.Context) error {
	candidateBinPath := s.makeBinaryPath()

	if s.clearLogs != nil {
		s.clearLogs()
	}

	s.logBuildf("[run] Building...\n")

	runBuild := s.runBuild
	if runBuild == nil {
		runBuild = s.goBuild
	}

	if err := runBuild(buildCtx, candidateBinPath); err != nil {
		s.removeBinary(candidateBinPath)
		if s.stateTracker != nil {
			s.stateTracker.SetError(state.IndexGoBuild, err.Error())
		}
		return fmt.Errorf("build failed: %w", err)
	}

	if buildCtx.Err() != nil {
		s.removeBinary(candidateBinPath)
		return buildCtx.Err()
	}

	if s.stateTracker != nil {
		s.stateTracker.SetError(state.IndexGoBuild, "")
	}

	s.cmdMu.Lock()

	if buildCtx.Err() != nil {
		s.removeBinary(candidateBinPath)
		s.cmdMu.Unlock()
		return buildCtx.Err()
	}

	previousBinPath := s.binPath
	if err := s.stopLocked(); err != nil {
		s.removeBinary(candidateBinPath)
		s.cmdMu.Unlock()
		s.logf("[run] %v\n", err)
		return err
	}

	s.logf("[run] Starting server...\n")
	cmd := exec.CommandContext(appCtx, candidateBinPath)
	cmd.Dir = s.dir
	cmd.Env = append(os.Environ(), "TEMPL_DEV_MODE=true", "TMPDIR="+s.templDir)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		s.removeBinary(candidateBinPath)
		s.cmdMu.Unlock()
		return fmt.Errorf("stdout pipe failed: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		s.removeBinary(candidateBinPath)
		s.cmdMu.Unlock()
		return fmt.Errorf("stderr pipe failed: %w", err)
	}

	if err := cmd.Start(); err != nil {
		s.removeBinary(candidateBinPath)
		s.cmdMu.Unlock()
		return fmt.Errorf("start failed: %w", err)
	}

	guard := newRuntimeOutputGuard(s.runtimeMissingFileThreshold(), s.requestRuntimeRestart)
	go forwardRuntimeOutput(stdoutPipe, s.stdoutWriter(), guard)
	go forwardRuntimeOutput(stderrPipe, s.stderrWriter(), guard)

	s.cmd = cmd
	s.binPath = candidateBinPath
	s.startHealthMonitor(appCtx, previousBinPath)
	s.cmdMu.Unlock()

	if s.addProcess != nil {
		s.addProcess(cmd)
	}

	return nil
}

func (s *AppServer) goBuild(ctx context.Context, outputPath string) error {
	buildCmd := exec.CommandContext(ctx, "go", "build", "-o", outputPath, "cmd/app/main.go")
	buildCmd.Dir = s.dir
	out := s.buildWriter()
	buildCmd.Stdout = out
	buildCmd.Stderr = out
	return buildCmd.Run()
}

func waitForRuntimeRestart(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *AppServer) requestRuntimeRestart() {
	if s.runtimeRestartChan == nil {
		return
	}
	select {
	case s.runtimeRestartChan <- struct{}{}:
	default:
	}
}

func (s *AppServer) runtimeRestartBackoff() time.Duration {
	if s.runtimeRestartDelay > 0 {
		return s.runtimeRestartDelay
	}
	return time.Second
}

func (s *AppServer) runtimeMissingFileThreshold() int {
	if s.runtimeOutputThreshold > 0 {
		return s.runtimeOutputThreshold
	}
	return 5
}

func (s *AppServer) stdoutWriter() io.Writer {
	if s.stdout != nil {
		return s.stdout
	}
	return os.Stdout
}

func (s *AppServer) stderrWriter() io.Writer {
	if s.stderr != nil {
		return s.stderr
	}
	return os.Stderr
}

func (s *AppServer) buildWriter() io.Writer {
	if s.buildOut != nil {
		return s.buildOut
	}
	return os.Stdout
}

func (s *AppServer) logWriter() io.Writer {
	if s.log != nil {
		return s.log
	}
	return os.Stdout
}

func (s *AppServer) logf(format string, args ...any) {
	_, _ = fmt.Fprintf(s.logWriter(), format, args...)
}

func (s *AppServer) removeBinary(path string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		s.logf("[run] Removing binary %s: %v\n", path, err)
	}
}

func (s *AppServer) logBuildf(format string, args ...any) {
	_, _ = fmt.Fprintf(s.buildWriter(), format, args...)
}

func (s *AppServer) setStatus(status string) {
	if s.onStatus != nil {
		s.onStatus(status)
	}
}

type runtimeOutputGuard struct {
	mu          sync.Mutex
	threshold   int
	consecutive int
	restartOnce sync.Once
	onThreshold func()
}

func newRuntimeOutputGuard(threshold int, onThreshold func()) *runtimeOutputGuard {
	if threshold <= 0 {
		threshold = 5
	}
	return &runtimeOutputGuard{
		threshold:   threshold,
		onThreshold: onThreshold,
	}
}

func forwardRuntimeOutput(reader io.Reader, writer io.Writer, guard *runtimeOutputGuard) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		emit, notice := guard.handleLine(line)
		if emit {
			_, _ = fmt.Fprintln(writer, line)
		}
		if notice != "" {
			_, _ = fmt.Fprintln(writer, notice)
		}
	}

	if err := scanner.Err(); err != nil {
		_, _ = fmt.Fprintf(writer, "[run] error reading app output: %v\n", err)
	}
}

func (g *runtimeOutputGuard) handleLine(line string) (bool, string) {
	if !isMissingFileRuntimeLine(line) {
		g.mu.Lock()
		g.consecutive = 0
		g.mu.Unlock()
		return true, ""
	}

	var triggerRestart bool
	var emit bool
	var notice string

	g.mu.Lock()
	g.consecutive++
	switch {
	case g.consecutive < g.threshold:
		emit = true
	case g.consecutive == g.threshold:
		emit = true
		triggerRestart = true
		notice = fmt.Sprintf("[run] Suppressing repeated missing-file output after %d consecutive lines.", g.threshold)
	default:
		emit = false
	}
	g.mu.Unlock()

	if triggerRestart {
		g.restartOnce.Do(func() {
			if g.onThreshold != nil {
				g.onThreshold()
			}
		})
	}

	return emit, notice
}

func isMissingFileRuntimeLine(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "file not found") ||
		strings.Contains(lower, "no such file or directory") ||
		strings.Contains(lower, "cannot find the file") ||
		strings.Contains(lower, "file does not exist") ||
		strings.Contains(lower, "path does not exist") ||
		strings.Contains(lower, "directory does not exist")
}

func (s *AppServer) stopLocked() error {
	s.cancelHealthMonitor()
	if s.cmd != nil && s.cmd.Process != nil {
		proc.Stop(s.cmd, proc.StopOptions{
			TermWait: s.termWait(),
			KillWait: time.Second,
		})
	}
	s.cmd = nil

	if err := proc.WaitForPortRelease(context.Background(), s.listenAddr(), s.portWaitTimeout()); err != nil {
		return fmt.Errorf("app listen address still busy after stop: %w", err)
	}
	return nil
}

func (s *AppServer) listenAddr() string {
	return net.JoinHostPort("127.0.0.1", s.appPort)
}

func (s *AppServer) portWaitTimeout() time.Duration {
	if s.portWait > 0 {
		return s.portWait
	}
	return proc.DefaultPortWait
}

func (s *AppServer) termWait() time.Duration {
	if s.stopTermWait > 0 {
		return s.stopTermWait
	}
	return proc.DefaultTermWait
}

func (s *AppServer) startHealthMonitor(ctx context.Context, previousBinPath string) {
	s.cancelHealthMonitor()
	healthCtx, cancel := context.WithCancel(ctx)
	s.healthMu.Lock()
	s.healthCancel = cancel
	s.healthMu.Unlock()

	go func() {
		healthURL := fmt.Sprintf("http://localhost:%s/", s.appPort)
		reload.BroadcastWhenHealthy(healthCtx, healthURL, s.broadcaster, s.buildWriter(), func() {
			if previousBinPath != "" {
				s.removeBinary(previousBinPath)
			}
			s.setRebuildState(false)
			s.setStatus("ready")
			if s.readyChan != nil {
				select {
				case s.readyChan <- struct{}{}:
				default:
				}
			}
		})
	}()
}

func (s *AppServer) cancelHealthMonitor() {
	s.healthMu.Lock()
	cancel := s.healthCancel
	s.healthCancel = nil
	s.healthMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *AppServer) setRebuildState(inProgress bool) {
	if s.onRebuildStateChanged != nil {
		s.onRebuildStateChanged(inProgress)
	}
}
