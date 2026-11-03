// Package devserver is the development runner behind `andurel run`.
//
// Run builds and supervises cmd/app and cmd/queue, puts a live-reload proxy in
// front of the app, runs the Vite dev server (which also serves development
// SSR), watches Go files, resources/*.templ and emails/**, and
// starts allowlisted sidecar tools. Cancel the context to stop everything.
package devserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/ctxrun"
	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/proc"
	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/proxy"
	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/queue"
	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/reload"
	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/server"
	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/state"
	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/tools"
	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/tui"
	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/watcher"
)

const (
	// DefaultProxyPort is the browser-facing port when PROXY_PORT is unset.
	DefaultProxyPort = "3000"
	// DefaultAppPort is the app's own port when PORT is unset.
	DefaultAppPort = "8080"
	// DefaultPackageManager runs `<pm> run dev` for Vite when none is given.
	DefaultPackageManager = "pnpm"
)

// Options configures Run.
type Options struct {
	// Dir is the project root (the directory holding go.mod). Required.
	Dir string
	// PackageManager runs `<PackageManager> run dev` for Vite.
	PackageManager string
	// Tools lists allowlisted sidecars to start (mailpit). Callers validate
	// that the binaries exist; Run rejects names it does not know.
	Tools []string
	// Inline prints interleaved logs instead of the interactive TUI.
	Inline bool
	// Verbose logs watcher and templ details.
	Verbose bool
	// ClearLogs clears the terminal before each rebuild (inline mode only).
	ClearLogs bool
	// Version is shown in the startup line and the TUI header.
	Version string
	// ProxyPort and AppPort override PROXY_PORT and PORT from the
	// environment (after .env is loaded).
	ProxyPort string
	AppPort   string
	// SyncEmails runs andurel sync emails (props/manifest generation plus the
	// project's email:export script). When set, Run syncs once before the
	// first build and again whenever emails/** changes, then
	// rebuilds the app and queue with the new embedded templates. Nil
	// disables email handling.
	SyncEmails func(ctx context.Context, log io.Writer) error
	// Stdin and Stdout decide whether the TUI can take over the terminal and
	// receive inline logs. They default to os.Stdin and os.Stdout.
	Stdin  *os.File
	Stdout *os.File
	// Stderr receives shutdown messages. Defaults to os.Stderr.
	Stderr io.Writer
}

func (opts *Options) normalize() error {
	if opts.Dir == "" {
		return errors.New("devserver: project directory is required")
	}
	dir, err := filepath.Abs(opts.Dir)
	if err != nil {
		return fmt.Errorf("devserver: resolve project directory: %w", err)
	}
	opts.Dir = dir
	if opts.PackageManager == "" {
		opts.PackageManager = DefaultPackageManager
	}
	if opts.Version == "" {
		opts.Version = "dev"
	}
	for _, name := range opts.Tools {
		if !tools.Allowed(name) {
			return fmt.Errorf("devserver: unknown tool %q (allowed: mailpit)", name)
		}
	}
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	return nil
}

// Run starts the development runner and blocks until ctx is cancelled, the
// TUI quits, or a core component (proxy, watchers, app server) fails. On
// return every child process it started has been stopped. A cancelled ctx is
// a normal shutdown and returns nil.
func Run(ctx context.Context, opts Options) error {
	if err := opts.normalize(); err != nil {
		return err
	}

	if err := godotenv.Load(filepath.Join(opts.Dir, ".env")); err != nil && !errors.Is(err, os.ErrNotExist) {
		// Diagnostics are best-effort and must not prevent startup or cleanup.
		_, _ = fmt.Fprintf(opts.Stderr, "[run] warning: could not load .env: %v\n", err)
	}
	if err := queue.RequireEntrypoint(opts.Dir); err != nil {
		return err
	}

	proxyPort := firstNonEmpty(opts.ProxyPort, os.Getenv("PROXY_PORT"), DefaultProxyPort)
	appPort := firstNonEmpty(opts.AppPort, os.Getenv("PORT"), DefaultAppPort)

	procs := &processSet{}
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		procs.cleanup(opts.Stderr, opts.Dir, []string{
			net.JoinHostPort("127.0.0.1", proxyPort),
			net.JoinHostPort("127.0.0.1", appPort),
		})
	}()

	hub := tui.NewHub()
	for _, name := range opts.Tools {
		hub.Enable(name)
	}
	useTUI := tui.UseTUI(opts.Inline, opts.Stdin, opts.Stdout)
	buildLog := hub.Writer(tui.StreamBuild)
	appLog := hub.Writer(tui.StreamApp)
	queueLog := hub.Writer(tui.StreamQueue)
	if !useTUI {
		hub.SetFallback(opts.Stdout)
	}

	// The proxy and websocket handlers log through the standard logger; keep
	// those lines in the build stream so they never draw over the TUI.
	previousLogOutput := log.Writer()
	log.SetOutput(buildLog)
	defer log.SetOutput(previousLogOutput)

	var clearLogs func()
	if !useTUI && opts.ClearLogs {
		// Clearing the terminal is cosmetic and must not interrupt a rebuild.
		clearLogs = func() { _, _ = fmt.Fprint(opts.Stdout, "\033[2J\033[H") }
	}

	// Runner log writes are best-effort; they must not interrupt supervision.
	_, _ = fmt.Fprintf(buildLog, "[run] Starting andurel run (version %s)\n", opts.Version)

	var syncEmails func(context.Context) error
	if opts.SyncEmails != nil {
		syncEmails = func(ctx context.Context) error {
			return opts.SyncEmails(ctx, buildLog)
		}
		// Sync before the first build so the app and queue start with the
		// current embedded email templates.
		if err := syncEmails(ctx); err != nil {
			if useTUI {
				hub.Dump(opts.Stdout)
			}
			return err
		}
	}

	broadcaster := reload.NewBroadcaster()
	rebuildChan := make(chan struct{}, 1)
	appRebuild := make(chan struct{}, 1)
	queueRebuild := make(chan struct{}, 1)
	templChange := make(chan watcher.TemplChange, 64)

	trk := state.New()
	var wg sync.WaitGroup
	errChan := make(chan error, 8)
	var rebuildInProgress atomic.Bool

	requestRebuild := func() {
		select {
		case rebuildChan <- struct{}{}:
		default:
		}
	}

	go ctxrun.Fanout(ctx, rebuildChan, appRebuild, queueRebuild)

	wg.Go(func() {
		if err := runProxyServer(ctx, opts.Dir, proxyPort, appPort, broadcaster, rebuildInProgress.Load); err != nil {
			errChan <- fmt.Errorf("proxy-server: %w", err)
		}
	})

	// When the emails watcher owns emails/*.go, it syncs first and requests
	// the rebuild after the export, so the Go watcher must not request a
	// second rebuild for the same edit.
	var skipGoRebuild func(path string) bool
	if syncEmails != nil {
		skipGoRebuild = func(path string) bool {
			return watcher.IsEmailsSource(opts.Dir, path)
		}
	}

	wg.Go(func() {
		if err := watcher.RunGoWatcher(
			ctx,
			opts.Dir,
			rebuildChan,
			skipGoRebuild,
			opts.Verbose,
			buildLog,
		); err != nil {
			errChan <- fmt.Errorf("go-watcher: %w", err)
		}
	})

	wg.Go(func() {
		cfg := watcher.TemplWatcherConfig{
			Dir:        opts.Dir,
			Verbose:    opts.Verbose,
			AddProcess: procs.add,
			Log:        buildLog,
			OnTemplErr: func(msg string) {
				trk.SetError(state.IndexTempl, msg)
				if msg != "" {
					hub.SetStatus(tui.StreamBuild, tui.StatusError)
				}
			},
		}
		if err := watcher.RunTemplWatcher(ctx, templChange, cfg); err != nil {
			errChan <- fmt.Errorf("live-templ: %w", err)
		}
	})

	if syncEmails != nil {
		wg.Go(func() {
			cfg := watcher.EmailsWatcherConfig{
				Dir:  opts.Dir,
				Sync: syncEmails,
				OnSynced: func() {
					_, _ = fmt.Fprintln(buildLog, "[run] Email templates changed, rebuilding")
					requestRebuild()
				},
				OnTemplatesChanged: func() {
					_, _ = fmt.Fprintln(buildLog, "[run] Email templates changed outside sync, rebuilding")
					requestRebuild()
				},
				OnSyncError: func(msg string) {
					trk.SetError(state.IndexEmails, msg)
					if msg != "" {
						hub.SetStatus(tui.StreamBuild, tui.StatusError)
					}
				},
				Log: buildLog,
			}
			if err := watcher.RunEmailsWatcher(ctx, cfg); err != nil {
				errChan <- fmt.Errorf("emails-watcher: %w", err)
			}
		})
	}

	_, _ = fmt.Fprintf(buildLog, "[run] Starting %s run dev (Vite)\n", opts.PackageManager)
	wg.Go(func() {
		if err := runJsDev(ctx, opts.Dir, opts.PackageManager, buildLog, procs.add); err != nil {
			_, _ = fmt.Fprintf(buildLog, "[run] %s run dev: %v\n", opts.PackageManager, err)
			<-ctx.Done()
		}
	})

	readyChan := make(chan struct{}, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-readyChan:
				rebuildInProgress.Store(false)
			}
		}
	}()

	appServer := server.NewAppServer(server.Config{
		Dir:          opts.Dir,
		AppPort:      appPort,
		Broadcaster:  broadcaster,
		AddProcess:   procs.add,
		ReadyChan:    readyChan,
		StateTracker: trk,
		ClearLogs:    clearLogs,
		Stdout:       appLog,
		Stderr:       appLog,
		BuildOut:     buildLog,
		Log:          appLog,
		OnStatus: func(status string) {
			switch status {
			case "starting":
				hub.SetStatus(tui.StreamApp, tui.StatusStarting)
			case "ready":
				hub.SetStatus(tui.StreamApp, tui.StatusReady)
			case "error":
				hub.SetStatus(tui.StreamApp, tui.StatusError)
			}
		},
		OnRebuildStateChanged: func(inProgress bool) {
			rebuildInProgress.Store(inProgress)
			if inProgress {
				hub.SetStatus(tui.StreamBuild, tui.StatusStarting)
			} else if !trk.HasErrorAt(state.IndexGoBuild) && !trk.HasErrorAt(state.IndexTempl) &&
				!trk.HasErrorAt(state.IndexEmails) {
				hub.SetStatus(tui.StreamBuild, tui.StatusReady)
			}
		},
	})
	wg.Go(func() {
		if err := appServer.Run(ctx, appRebuild); err != nil {
			errChan <- fmt.Errorf("app-server: %w", err)
		}
	})

	queueRunner := queue.NewRunner(opts.Dir)
	queueRunner.AddProcess = procs.add
	queueRunner.BuildOut = buildLog
	queueRunner.RuntimeOut = queueLog
	queueRunner.Log = queueLog
	queueRunner.OnStatus = func(status tui.Status) {
		hub.SetStatus(tui.StreamQueue, status)
		if status == tui.StatusError {
			hub.SetStatus(tui.StreamBuild, tui.StatusError)
		}
	}
	wg.Go(func() {
		if err := queueRunner.Run(ctx, queueRebuild); err != nil {
			_, _ = fmt.Fprintf(queueLog, "[run] queue runner: %v\n", err)
			<-ctx.Done()
		}
	})

	for _, name := range opts.Tools {
		toolLog := hub.Writer(name)
		toolRunner := tools.NewRunner(name)
		toolRunner.Dir = opts.Dir
		toolRunner.AddProcess = procs.add
		toolRunner.Out = toolLog
		toolRunner.OnStatus = func(status tui.Status) {
			hub.SetStatus(name, status)
		}
		wg.Go(func() {
			if err := toolRunner.Run(ctx); err != nil {
				_, _ = fmt.Fprintf(toolLog, "[run] %s runner: %v\n", name, err)
				<-ctx.Done()
			}
		})
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case change := <-templChange:
				handleTemplChange(change, trk, broadcaster, requestRebuild, clearLogs, buildLog)
			}
		}
	}()

	_, _ = fmt.Fprintf(buildLog, "\n  Proxy server: http://localhost:%s\n", proxyPort)
	_, _ = fmt.Fprintf(buildLog, "  App server:   http://localhost:%s (internal)\n", appPort)
	_, _ = fmt.Fprintf(buildLog, "  TEMPL_DEV_MODE: enabled (fast template reloads)\n")
	_, _ = fmt.Fprintf(buildLog, "  Inertia frontend: %s run dev (Vite dev server)\n", opts.PackageManager)
	_, _ = fmt.Fprintf(buildLog, "  Inertia SSR:      Vite /__inertia_ssr\n")
	if syncEmails != nil {
		_, _ = fmt.Fprintf(buildLog, "  Email templates:  synced on change (emails/**)\n")
	}
	if len(opts.Tools) > 0 {
		_, _ = fmt.Fprintf(buildLog, "  Sidecar tools:  %s\n", formatToolList(opts.Tools))
	}
	_, _ = fmt.Fprintln(buildLog)

	var fatalMu sync.Mutex
	var fatal error
	go func() {
		select {
		case <-ctx.Done():
			_, _ = fmt.Fprintf(buildLog, "\n[run] Shutting down...\n")
		case err := <-errChan:
			if err != nil {
				fatalMu.Lock()
				fatal = err
				fatalMu.Unlock()
				_, _ = fmt.Fprintf(buildLog, "[run] Error: %v\n", err)
				_, _ = fmt.Fprintf(buildLog, "[run] Shutting down all processes...\n")
				cancel()
			}
		}
	}()

	if useTUI {
		header := tui.Header{
			Version:  opts.Version,
			ProxyURL: fmt.Sprintf("http://localhost:%s", proxyPort),
		}
		if err := tui.Run(ctx, hub, header); err != nil && ctx.Err() == nil {
			_, _ = fmt.Fprintf(opts.Stderr, "[run] tui: %v\n", err)
		}
		cancel()
	}

	wg.Wait()
	close(errChan)

	fatalMu.Lock()
	errs := []error{fatal}
	fatalMu.Unlock()
	for err := range errChan {
		errs = append(errs, err)
	}

	if useTUI {
		hub.Dump(opts.Stdout)
	}

	return errors.Join(errs...)
}

// handleTemplChange reacts to a templ watch event: content-only changes
// reload the browser, generated Go changes rebuild the app and queue.
func handleTemplChange(
	change watcher.TemplChange,
	trk *state.Tracker,
	broadcaster *reload.Broadcaster,
	requestRebuild func(),
	clearLogs func(),
	log io.Writer,
) {
	// Watcher messages are best-effort; reloads and rebuilds must still run.
	switch change {
	case watcher.TemplChangeNeedsBrowserReload:
		if clearLogs != nil {
			clearLogs()
		}
		if trk.HasErrorAt(state.IndexTempl) {
			_, _ = fmt.Fprintln(log, "[run] Templ has errors, skipping browser reload")
			return
		}
		_, _ = fmt.Fprintln(log, "[run] Template changed, reloading browser")
		broadcaster.Broadcast()
	case watcher.TemplChangeNeedsRestart:
		if trk.HasErrorAt(state.IndexTempl) {
			_, _ = fmt.Fprintln(log, "[run] Templ has errors, skipping rebuild")
			return
		}
		_, _ = fmt.Fprintln(log, "[run] Template Go code changed, rebuilding")
		requestRebuild()
	}
}

// processSet tracks child processes so shutdown can stop anything a runner
// did not stop on its own.
type processSet struct {
	mu        sync.Mutex
	processes []*exec.Cmd
}

func (p *processSet) add(cmd *exec.Cmd) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.compactLocked()
	p.processes = append(p.processes, cmd)
}

func (p *processSet) compactLocked() {
	compacted := p.processes[:0]
	for _, cmd := range p.processes {
		if trackedAlive(cmd) {
			compacted = append(compacted, cmd)
		}
	}
	p.processes = compacted
}

// trackedAlive reports whether cmd, or for a process-group command any
// member of its group, is still running. A package manager can exit while
// the Vite process it started keeps running in the same group.
func trackedAlive(cmd *exec.Cmd) bool {
	if cmd == nil || cmd.Process == nil {
		return false
	}
	if proc.HasProcessGroup(cmd) {
		return proc.GroupAlive(cmd)
	}
	// Cmd.Wait writes ProcessState concurrently with watcher and cleanup
	// bookkeeping. Use Process.Signal instead of reading that field here.
	return processAlive(cmd.Process)
}

func (p *processSet) snapshot() []*exec.Cmd {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.compactLocked()
	processes := make([]*exec.Cmd, len(p.processes))
	copy(processes, p.processes)
	p.processes = nil
	return processes
}

// cleanup SIGTERMs every tracked process (its whole group when it has one),
// SIGKILLs what is still alive, waits for the given ports and removes the
// templ dev-mode temp files.
func (p *processSet) cleanup(out io.Writer, dir string, ports []string) {
	// Cleanup must continue even when diagnostics cannot be written.
	_, _ = fmt.Fprintf(out, "\n[run] Cleaning up processes...\n")

	processes := p.snapshot()
	for _, cmd := range processes {
		signalTrackedProcess(cmd, syscall.SIGTERM)
	}
	waitUntilExited(processes, 500*time.Millisecond)

	for _, cmd := range processes {
		if trackedAlive(cmd) {
			signalTrackedProcess(cmd, syscall.SIGKILL)
		}
	}
	waitUntilExited(processes, time.Second)

	for _, addr := range ports {
		if err := proc.WaitForPortRelease(context.Background(), addr, 2*time.Second); err != nil {
			_, _ = fmt.Fprintf(out, "[run] %v\n", err)
		}
	}

	_ = os.RemoveAll(filepath.Join(dir, "tmp", "templ"))

	_, _ = fmt.Fprintf(out, "[run] Cleanup complete.\n")
}

func waitUntilExited(processes []*exec.Cmd, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !anyProcessAlive(processes) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func anyProcessAlive(processes []*exec.Cmd) bool {
	return slices.ContainsFunc(processes, trackedAlive)
}

func processAlive(process *os.Process) bool {
	if process == nil {
		return false
	}

	err := process.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}

func signalTrackedProcess(cmd *exec.Cmd, sig syscall.Signal) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if proc.HasProcessGroup(cmd) {
		_ = proc.SignalProcessGroup(cmd, sig)
		return
	}
	_ = cmd.Process.Signal(sig)
}

func runProxyServer(
	ctx context.Context,
	dir, proxyPort, appPort string,
	broadcaster *reload.Broadcaster,
	isRebuilding func() bool,
) error {
	targetURL := fmt.Sprintf("http://localhost:%s", appPort)

	proxyServer, err := proxy.NewServer(targetURL, reload.WebSocketPath, dir, isRebuilding)
	if err != nil {
		return err
	}
	if err := proxyServer.ConfigureVite(firstNonEmpty(os.Getenv("INERTIA_VITE_DEV_URL"), "http://localhost:5173/assets/dist")); err != nil {
		log.Printf("[run] Vite proxy disabled: %v", err)
	}
	hosts := []string{os.Getenv("HOST_PRIMARY"), net.JoinHostPort("localhost", proxyPort), net.JoinHostPort("127.0.0.1", proxyPort)}
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if ok && strings.HasPrefix(key, "HOST_") && value != "" {
			hosts = append(hosts, value)
		}
	}
	proxyServer.SetViteHosts(hosts)

	wsHandler := reload.NewWebSocketHandler(broadcaster)
	handler := proxyServer.Handler(wsHandler)

	httpServer := &http.Server{
		Addr:              ":" + proxyPort,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	listener, err := net.Listen("tcp", httpServer.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", httpServer.Addr, err)
	}

	serveErr := make(chan error, 1)
	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			serveErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	case err := <-serveErr:
		return fmt.Errorf("serve proxy on %s: %w", httpServer.Addr, err)
	}
}

func runJsDev(ctx context.Context, dir, runtime string, out io.Writer, addProcess func(*exec.Cmd)) error {
	if out == nil {
		out = io.Discard
	}

	cmd := exec.Command(runtime, "run", "dev")
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = out
	// Own process group so cleanup can SIGKILL Vite (and other grandchildren)
	// that actually bind the frontend ports — not just the npm/pnpm parent.
	proc.ConfigureProcessGroup(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting %s run dev: %w", runtime, err)
	}

	if addProcess != nil {
		addProcess(cmd)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		stopProcessGroup(cmd, waitDone(done), proc.DefaultTermWait)
		return nil
	case err := <-done:
		if ctx.Err() != nil {
			return nil
		}
		// A failed diagnostic must not interfere with process supervision.
		_, _ = fmt.Fprintf(out, "[run] %s run dev exited: %v\n", runtime, err)
		<-ctx.Done()
		return nil
	}
}

// stopProcessGroup stops a package-manager process and everything in its
// group. The manager (pnpm/npm/bun) can exit before Vite does, so the whole
// group gets SIGTERM, and whatever is left after termWait gets SIGKILL.
func stopProcessGroup(cmd *exec.Cmd, done <-chan struct{}, termWait time.Duration) {
	_ = proc.SignalProcessGroup(cmd, syscall.SIGTERM)

	deadline := time.Now().Add(termWait)
	select {
	case <-done:
	case <-time.After(termWait):
	}
	for time.Now().Before(deadline) && proc.GroupAlive(cmd) {
		time.Sleep(20 * time.Millisecond)
	}
	if proc.GroupAlive(cmd) {
		_ = proc.SignalProcessGroup(cmd, syscall.SIGKILL)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
	}
}

func formatToolList(names []string) string {
	if len(names) == 0 {
		return ""
	}
	var out strings.Builder
	out.WriteString(names[0])
	for _, name := range names[1:] {
		out.WriteString(", " + name)
	}
	return out.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// waitDone adapts an error channel into a structural done signal for proc.Stop.
func waitDone(errs <-chan error) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		<-errs
		close(ch)
	}()
	return ch
}
