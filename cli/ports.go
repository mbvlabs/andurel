package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/spf13/cobra"
)

const (
	// PortLeaseDirEnv overrides the directory that stores active port leases.
	PortLeaseDirEnv = "ANDUREL_PORT_LEASE_DIR"
	// DefaultPortLeaseStartupTimeout bounds waiting for the launched child's
	// health endpoint.
	DefaultPortLeaseStartupTimeout = 15 * time.Second
	// DefaultPortLeaseStopTimeout bounds graceful child shutdown.
	DefaultPortLeaseStopTimeout = 10 * time.Second
)

// portAssignment is the machine-readable handoff published by the launcher.
type portAssignment struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	ListenURL string `json:"listen_url"`
	URL       string `json:"url"`
	Mode      string `json:"mode"`
	LeaseFile string `json:"lease_file"`
	PID       int    `json:"pid"`
	ChildPID  int    `json:"child_pid"`
}

// portLease is one held, flock-backed loopback port reservation. The lock is
// held for the launcher process lifetime; the kernel releases it on crash, so
// an unlocked lease file is stale and reusable.
type portLease struct {
	file      *os.File
	path      string
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	ListenURL string    `json:"listen_url"`
	URL       string    `json:"url"`
	PID       int       `json:"pid"`
	CreatedAt time.Time `json:"created_at"`
}

type portLaunchOptions struct {
	Host           string
	Port           int
	ListenURL      string
	LeaseDir       string
	ReadyFile      string
	StartupTimeout time.Duration
	StopTimeout    time.Duration
	Command        []string
	ChildStdout    io.Writer
	ChildStderr    io.Writer
	ChildStdin     io.Reader
}

func newPortsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ports",
		Short: "Allocate and supervise collision-safe loopback ports",
		Long: `Allocate a loopback port for a supervised child process and publish the
matching INERTIA_SSR_LISTEN and INERTIA_SSR_URL values.

A port is reserved with an exclusive, machine-local lease that is held for the
whole child lifetime. A lease file is never treated as proof by itself: the
launcher verifies that the port is bindable before starting the child and only
publishes the URL after the intended child answers /health. An unrelated
listener can therefore never satisfy the readiness check.

Leases use flock on the local filesystem; keep --lease-dir on a local disk, not
a network mount. A crashed launcher releases the kernel lock automatically, so
an unlocked lease file is stale and reusable. The launcher never kills an
unrelated listener. Auto allocation re-checks bindability before start; if an
unrelated process takes the port between that probe and the child bind, the
child exits nonzero and the launcher reports it. Start the launcher again to
allocate a fresh port.

Use --listen for explicit fixed-address operation. Without it the launcher
allocates a free loopback port. Concurrent launchers receive distinct ports.`,
		Example: `  andurel ports launch -- ./dist/ssr
  andurel ports launch --json --ready-file .andurel/ssr-port.json -- ./dist/ssr
  andurel ports launch --listen http://127.0.0.1:13714 -- ./dist/ssr`,
	}
	cmd.AddCommand(newPortsLaunchCommand())
	setStandardHelp(cmd)
	return cmd
}

func newPortsLaunchCommand() *cobra.Command {
	var (
		hostFlag      string
		portFlag      int
		listenFlag    string
		leaseDirFlag  string
		readyFileFlag string
		startupFlag   time.Duration
		stopFlag      time.Duration
	)

	cmd := &cobra.Command{
		Use:   "launch [--] <command> [args...]",
		Short: "Supervise a child on a leased loopback port",
		Long: `Run a child process with INERTIA_SSR_LISTEN and INERTIA_SSR_URL set to
the same leased loopback URL. The assignment is printed only after the child
answers GET /health with {"status":"ok"} and the launcher proves the child owns
the listening socket on that exact address. The lease is released when the
child exits or the launcher receives SIGINT/SIGTERM.

The child defaults to ./dist/ssr. Pass --listen to keep a fixed address
instead of allocating one; the port must be free and unleased.

Wildcard addresses are rejected because they cannot prove which socket answered
the health probe. localhost resolves to 127.0.0.1; pass ::1 explicitly for IPv6.
On platforms other than Linux the launcher fails closed because it cannot read
/proc socket ownership; run the child directly with a fixed INERTIA_SSR_LISTEN
there instead.`,
		Example: `  andurel ports launch -- ./dist/ssr
  andurel ports launch --json -- ./dist/ssr
  andurel ports launch --listen http://127.0.0.1:13714 -- ./dist/ssr`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := output.ParseOptions(cmd)
			if err != nil {
				return err
			}

			command := append([]string(nil), args...)
			if len(command) == 0 {
				command = []string{filepath.Join(".", "dist", "ssr")}
			}

			childStdout := io.Writer(os.Stdout)
			if output.SuppressesHumanOutput(opts) {
				childStdout = cmd.ErrOrStderr()
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			return runPortLauncher(ctx, portLaunchOptions{
				Host:           hostFlag,
				Port:           portFlag,
				ListenURL:      listenFlag,
				LeaseDir:       leaseDirFlag,
				ReadyFile:      readyFileFlag,
				StartupTimeout: startupFlag,
				StopTimeout:    stopFlag,
				Command:        command,
				ChildStdout:    childStdout,
				ChildStderr:    cmd.ErrOrStderr(),
				ChildStdin:     os.Stdin,
			}, func(assignment portAssignment) error {
				return output.OK(
					cmd,
					assignment,
					fmt.Sprintf(
						"SSR port %d ready at %s (child pid %d)",
						assignment.Port,
						assignment.URL,
						assignment.ChildPID,
					),
				)
			})
		},
	}

	cmd.Flags().StringVar(
		&hostFlag,
		"host",
		"127.0.0.1",
		"Loopback host to allocate on (localhost uses 127.0.0.1; use ::1 for IPv6)",
	)
	cmd.Flags().IntVar(&portFlag, "port", 0, "Fixed port to allocate (0 allocates a free port)")
	cmd.Flags().StringVar(
		&listenFlag,
		"listen",
		"",
		"Fixed listen URL, e.g. http://127.0.0.1:13714 (overrides --host/--port)",
	)
	cmd.Flags().StringVar(
		&leaseDirFlag,
		"lease-dir",
		"",
		"Directory for port lease files (defaults to "+PortLeaseDirEnv+" or a per-user runtime directory)",
	)
	cmd.Flags().StringVar(
		&readyFileFlag,
		"ready-file",
		"",
		"Atomically write the ready assignment to this file and remove it on exit",
	)
	cmd.Flags().DurationVar(
		&startupFlag,
		"startup-timeout",
		DefaultPortLeaseStartupTimeout,
		"Maximum time to wait for the child /health endpoint",
	)
	cmd.Flags().DurationVar(
		&stopFlag,
		"stop-timeout",
		DefaultPortLeaseStopTimeout,
		"Maximum time to wait for graceful child shutdown",
	)

	return cmd
}

// runPortLauncher acquires a lease, starts the child, waits for readiness,
// publishes the assignment, and supervises the child until it exits or ctx is
// cancelled. The lease is held for the entire child lifetime.
func runPortLauncher(
	ctx context.Context,
	opts portLaunchOptions,
	ready func(portAssignment) error,
) (err error) {
	if len(opts.Command) == 0 {
		return fmt.Errorf("ports launch requires a child command")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if opts.StartupTimeout <= 0 {
		return fmt.Errorf("--startup-timeout must be positive")
	}
	if opts.StopTimeout <= 0 {
		return fmt.Errorf("--stop-timeout must be positive")
	}

	requested, fixed, err := resolvePortLaunchAddress(opts)
	if err != nil {
		return err
	}

	leaseDir := strings.TrimSpace(opts.LeaseDir)
	if leaseDir == "" {
		leaseDir, err = defaultPortLeaseDir()
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(leaseDir, 0o700); err != nil {
		return fmt.Errorf("create port lease directory: %w", err)
	}

	opts.ReadyFile = strings.TrimSpace(opts.ReadyFile)

	attempts := 1
	if !fixed {
		attempts = 3
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		lease, err := acquirePortLease(leaseDir, requested.Host, requested.Port, !fixed)
		if err != nil {
			lastErr = err
			if fixed {
				return err
			}
			continue
		}

		err = supervisePortLease(ctx, opts, lease, ready, fixed)
		releaseErr := lease.Release()
		if err == nil {
			return releaseErr
		}
		lastErr = errors.Join(err, releaseErr)
		if fixed || !errors.Is(err, errPortBindRace) {
			return lastErr
		}
	}

	return lastErr
}

// errPortBindRace reports a child that lost the bind race after the lease
// preflight probe. Auto allocation retries with a fresh port.
var errPortBindRace = errors.New("port bind race")

func supervisePortLease(
	ctx context.Context,
	opts portLaunchOptions,
	lease *portLease,
	ready func(portAssignment) error,
	fixed bool,
) error {
	var readyPublication *portReadyFilePublication
	defer func() {
		if readyPublication != nil {
			_ = readyPublication.removeIfUnchanged()
		}
	}()

	listenURL := lease.ListenURL
	child := exec.Command(opts.Command[0], opts.Command[1:]...)
	child.Env = envWithOverrides(map[string]string{
		"INERTIA_SSR_LISTEN":      listenURL,
		"INERTIA_SSR_URL":         listenURL,
		"ANDUREL_PORT_LEASE_FILE": lease.path,
	})
	child.Stdout = opts.ChildStdout
	stderr := opts.ChildStderr
	if stderr == nil {
		stderr = io.Discard
	}
	childStderr := newTailBuffer(16 << 10)
	child.Stderr = io.MultiWriter(stderr, childStderr)
	child.Stdin = opts.ChildStdin
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := child.Start(); err != nil {
		return fmt.Errorf("start SSR launcher child %s: %w", opts.Command[0], err)
	}

	var (
		childMu       sync.Mutex
		childErr      error
		childFinished bool
	)
	childDone := make(chan struct{})
	go func() {
		waitErr := child.Wait()
		childMu.Lock()
		childErr = waitErr
		childFinished = true
		childMu.Unlock()
		close(childDone)
	}()
	defer stopPortChild(child, childDone, opts.StopTimeout)

	childExitedError := func() error {
		childMu.Lock()
		defer childMu.Unlock()
		if childErr == nil {
			return fmt.Errorf("SSR launcher child %s exited unexpectedly", opts.Command[0])
		}
		return fmt.Errorf("SSR launcher child %s exited unexpectedly: %w", opts.Command[0], childErr)
	}

	startupCtx, cancelStartup := context.WithTimeout(ctx, opts.StartupTimeout)
	healthErr := waitForChildHealth(startupCtx, lease.URL, childDone)
	cancelStartup()
	if healthErr != nil {
		stopPortChild(child, childDone, opts.StopTimeout)
		if tailReportsBindConflict(childStderr.String()) {
			return fmt.Errorf("%w: %v", errPortBindRace, healthErr)
		}
		select {
		case <-childDone:
			childMu.Lock()
			exited := childFinished
			waitErr := childErr
			childMu.Unlock()
			if exited && waitErr != nil {
				return errors.Join(healthErr, childExitedError())
			}
		default:
		}
		return healthErr
	}

	// A probe listener necessarily closes before the child binds, so an
	// unrelated process can win the bind race and answer /health. Refuse to
	// publish until the intended child process group owns the listening socket
	// on the exact address the health probe used and the child is still alive.
	if err := verifyPortOwnedByChild(lease.Host, lease.Port, child.Process.Pid); err != nil {
		stopPortChild(child, childDone, opts.StopTimeout)
		return err
	}
	if childExited(childDone) {
		return childExitedError()
	}

	mode := "allocated"
	if fixed {
		mode = "fixed"
	}
	assignment := portAssignment{
		Host:      lease.Host,
		Port:      lease.Port,
		ListenURL: lease.ListenURL,
		URL:       lease.URL,
		Mode:      mode,
		LeaseFile: lease.path,
		PID:       os.Getpid(),
		ChildPID:  child.Process.Pid,
	}
	if opts.ReadyFile != "" {
		publication, err := publishPortReadyFile(opts.ReadyFile, assignment)
		if err != nil {
			return err
		}
		readyPublication = publication
	}
	if ready != nil {
		if err := ready(assignment); err != nil {
			return err
		}
	}

	select {
	case <-ctx.Done():
		return nil
	case <-childDone:
		return childExitedError()
	}
}

func tailReportsBindConflict(tail string) bool {
	lower := strings.ToLower(tail)
	return strings.Contains(lower, "eaddrinuse") ||
		strings.Contains(lower, "address already in use")
}

type tailBuffer struct {
	mu    sync.Mutex
	limit int
	buf   []byte
}

func newTailBuffer(limit int) *tailBuffer {
	return &tailBuffer{limit: limit}
}

func (buffer *tailBuffer) Write(data []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()

	buffer.buf = append(buffer.buf, data...)
	if len(buffer.buf) > buffer.limit {
		buffer.buf = append([]byte(nil), buffer.buf[len(buffer.buf)-buffer.limit:]...)
	}
	return len(data), nil
}

func (buffer *tailBuffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return string(buffer.buf)
}

func resolvePortLaunchAddress(opts portLaunchOptions) (portAssignment, bool, error) {
	if strings.TrimSpace(opts.ListenURL) != "" {
		host, port, listenURL, err := parsePortListenURL(opts.ListenURL)
		if err != nil {
			return portAssignment{}, false, err
		}
		if opts.Port != 0 && opts.Port != port {
			return portAssignment{}, false, fmt.Errorf(
				"--listen port %d conflicts with --port %d",
				port,
				opts.Port,
			)
		}
		return portAssignment{Host: host, Port: port, ListenURL: listenURL, URL: listenURL}, true, nil
	}

	host := strings.TrimSpace(opts.Host)
	if host == "" {
		host = "127.0.0.1"
	}
	canonicalHost, err := canonicalPortLeaseHost(host)
	if err != nil {
		return portAssignment{}, false, err
	}
	host = canonicalHost
	if opts.Port < 0 || opts.Port > 65535 {
		return portAssignment{}, false, fmt.Errorf("--port must be between 1 and 65535 (or 0)")
	}

	listenURL := portListenURL(host, opts.Port)
	return portAssignment{
		Host:      host,
		Port:      opts.Port,
		ListenURL: listenURL,
		URL:       listenURL,
	}, opts.Port != 0, nil
}

func parsePortListenURL(raw string) (string, int, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "http" || parsed.Host == "" {
		return "", 0, "", fmt.Errorf("--listen must be an HTTP URL such as http://127.0.0.1:13714")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", 0, "", fmt.Errorf("--listen must not include a path")
	}
	host := parsed.Hostname()
	portText := parsed.Port()
	if portText == "" {
		return "", 0, "", fmt.Errorf("--listen must include a port")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", 0, "", fmt.Errorf("--listen has an invalid port %q", portText)
	}
	canonicalHost, err := canonicalPortLeaseHost(host)
	if err != nil {
		return "", 0, "", err
	}

	return canonicalHost, port, portListenURL(canonicalHost, port), nil
}

// canonicalPortLeaseHost resolves an allowed listen host to one exact address.
// The ownership proof and health probe must use the same address; localhost is
// normalized to IPv4 loopback so the probe cannot silently use a different
// family, and wildcard addresses are rejected because they cannot identify the
// socket that answered the probe.
func canonicalPortLeaseHost(host string) (string, error) {
	trimmed := strings.TrimSpace(host)
	if strings.EqualFold(trimmed, "localhost") {
		return "127.0.0.1", nil
	}
	ip := net.ParseIP(trimmed)
	if ip == nil {
		return "", fmt.Errorf("listen host must be localhost or an IP address, got %q", host)
	}
	if ip.IsUnspecified() {
		return "", fmt.Errorf(
			"%q is a wildcard address; ports launch requires a specific loopback address such as 127.0.0.1 or ::1 so it can prove child ownership",
			trimmed,
		)
	}
	return ip.String(), nil
}

func portListenURL(host string, port int) string {
	if port == 0 {
		return "http://" + net.JoinHostPort(host, "0")
	}
	return "http://" + net.JoinHostPort(host, strconv.Itoa(port))
}

func defaultPortLeaseDir() (string, error) {
	if configured := strings.TrimSpace(os.Getenv(PortLeaseDirEnv)); configured != "" {
		return configured, nil
	}
	if runtimeDir := strings.TrimSpace(os.Getenv("XDG_RUNTIME_DIR")); runtimeDir != "" {
		return filepath.Join(runtimeDir, "andurel", "ports"), nil
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("andurel-%d", os.Getuid()), "ports"), nil
}

func acquirePortLease(leaseDir, host string, requestedPort int, allocate bool) (*portLease, error) {
	if err := os.MkdirAll(leaseDir, 0o700); err != nil {
		return nil, fmt.Errorf("create port lease directory: %w", err)
	}

	if !allocate {
		if requestedPort == 0 {
			return nil, fmt.Errorf("fixed port allocation requires a nonzero port")
		}
		lease, err := tryAcquirePortLease(leaseDir, host, requestedPort)
		if err != nil {
			return nil, err
		}
		if err := checkPortBindable(host, requestedPort); err != nil {
			_ = lease.Release()
			return nil, fmt.Errorf("port %d is not available: %w", requestedPort, err)
		}
		return lease, nil
	}

	var lastErr error
	for range 32 {
		port := requestedPort
		if port == 0 {
			candidate, err := reserveEphemeralPort(host)
			if err != nil {
				return nil, err
			}
			port = candidate
		}

		lease, err := tryAcquirePortLease(leaseDir, host, port)
		if err != nil {
			lastErr = err
			if requestedPort != 0 {
				return nil, err
			}
			continue
		}
		if err := checkPortBindable(host, port); err != nil {
			lastErr = err
			_ = lease.Release()
			if requestedPort != 0 {
				return nil, fmt.Errorf("port %d is not available: %w", port, err)
			}
			continue
		}
		return lease, nil
	}

	if lastErr == nil {
		lastErr = errors.New("no free loopback port found")
	}
	return nil, fmt.Errorf("allocate loopback port: %w", lastErr)
}

func tryAcquirePortLease(leaseDir, host string, port int) (*portLease, error) {
	path := portLeasePath(leaseDir, host, port)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open port lease %s: %w", path, err)
	}

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		holder := readPortLeaseHolder(path)
		_ = file.Close()
		if holder != nil && holder.PID != 0 {
			return nil, fmt.Errorf(
				"port %d is leased by pid %d (%s)",
				port,
				holder.PID,
				holder.ListenURL,
			)
		}
		return nil, fmt.Errorf("port %d is leased by another process", port)
	}

	listenURL := portListenURL(host, port)
	lease := &portLease{
		file:      file,
		path:      path,
		Host:      host,
		Port:      port,
		ListenURL: listenURL,
		URL:       listenURL,
		PID:       os.Getpid(),
		CreatedAt: time.Now().UTC(),
	}
	data, err := json.Marshal(lease)
	if err != nil {
		_ = lease.Release()
		return nil, err
	}
	if err := file.Truncate(0); err != nil {
		_ = lease.Release()
		return nil, fmt.Errorf("truncate port lease %s: %w", path, err)
	}
	if _, err := file.WriteAt(data, 0); err != nil {
		_ = lease.Release()
		return nil, fmt.Errorf("write port lease %s: %w", path, err)
	}
	if err := file.Sync(); err != nil {
		_ = lease.Release()
		return nil, fmt.Errorf("sync port lease %s: %w", path, err)
	}

	return lease, nil
}

func (lease *portLease) Release() error {
	if lease == nil || lease.file == nil {
		return nil
	}

	unlockErr := syscall.Flock(int(lease.file.Fd()), syscall.LOCK_UN)
	closeErr := lease.file.Close()
	lease.file = nil

	return errors.Join(unlockErr, closeErr)
}

func portLeasePath(leaseDir, host string, port int) string {
	digest := sha256.Sum256([]byte(strings.ToLower(host)))
	return filepath.Join(
		leaseDir,
		fmt.Sprintf("port-%s-%d.json", hex.EncodeToString(digest[:6]), port),
	)
}

func readPortLeaseHolder(path string) *portLease {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil
	}
	var holder portLease
	if err := json.Unmarshal(data, &holder); err != nil {
		return nil
	}
	return &holder
}

func reserveEphemeralPort(host string) (int, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return 0, fmt.Errorf("find free port on %s: %w", host, err)
	}
	defer listener.Close()

	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("find free port on %s: unexpected listener address", host)
	}
	return address.Port, nil
}

func checkPortBindable(host string, port int) error {
	listener, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return err
	}
	return listener.Close()
}

func waitForChildHealth(ctx context.Context, baseURL string, childDone <-chan struct{}) error {
	healthURL := strings.TrimSuffix(baseURL, "/") + "/health"
	client := &http.Client{Timeout: time.Second}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		healthy := false
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
		if err == nil {
			response, requestErr := client.Do(request)
			if requestErr == nil {
				healthy = response.StatusCode >= 200 && response.StatusCode < 300
				var body struct {
					Status string `json:"status"`
				}
				if healthy {
					decodeErr := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&body)
					healthy = decodeErr == nil && strings.EqualFold(body.Status, "ok")
				}
				_ = response.Body.Close()
			}
		}

		if healthy {
			// An unrelated listener may already answer /health. Give the child
			// one more interval to fail to bind, then require the intended child
			// to still be alive and healthy.
			select {
			case <-childDone:
				return fmt.Errorf("SSR launcher child exited before readiness was confirmed")
			case <-time.After(150 * time.Millisecond):
			}
			if childExited(childDone) {
				return fmt.Errorf("SSR launcher child exited before readiness was confirmed")
			}
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
			if err == nil {
				response, requestErr := client.Do(request)
				if requestErr == nil {
					var body struct {
						Status string `json:"status"`
					}
					decodeErr := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&body)
					_ = response.Body.Close()
					if response.StatusCode >= 200 && response.StatusCode < 300 &&
						decodeErr == nil && strings.EqualFold(body.Status, "ok") {
						return nil
					}
				}
			}
		}

		select {
		case <-childDone:
			return fmt.Errorf("SSR launcher child exited before becoming healthy")
		case <-ctx.Done():
			return fmt.Errorf("SSR launcher readiness: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func childExited(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}

func stopPortChild(child *exec.Cmd, done <-chan struct{}, timeout time.Duration) {
	if child == nil || child.Process == nil {
		return
	}
	if childExited(done) {
		return
	}

	_ = signalPortProcessGroup(child, syscall.SIGTERM)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return
	case <-timer.C:
		_ = signalPortProcessGroup(child, syscall.SIGKILL)
		<-done
	}
}

func signalPortProcessGroup(child *exec.Cmd, signal syscall.Signal) error {
	if child == nil || child.Process == nil {
		return nil
	}
	// The child is started in its own process group so a supervisor stop also
	// reaches helpers such as the Node process under cmd/ssr.
	if err := syscall.Kill(-child.Process.Pid, signal); err == nil {
		return nil
	}
	return child.Process.Signal(signal)
}

// portReadyFilePublication records the exact file this launcher published so
// cleanup cannot remove an arbitrary pre-existing file, a replacement from a
// competing launcher, or a file edited in place after publication.
type portReadyFilePublication struct {
	path   string
	info   os.FileInfo
	digest [sha256.Size]byte
}

func publishPortReadyFile(
	path string,
	assignment portAssignment,
) (*portReadyFilePublication, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create ready-file directory: %w", err)
	}
	data, err := json.Marshal(assignment)
	if err != nil {
		return nil, err
	}

	temp, err := os.CreateTemp(dir, ".andurel-port-ready-*")
	if err != nil {
		return nil, fmt.Errorf("create ready file: %w", err)
	}
	tempPath := temp.Name()
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempPath)
		return nil, fmt.Errorf("write ready file: %w", err)
	}
	// Capture identity before the rename so a competing launcher replacing the
	// path immediately after publication cannot become this launcher's cleanup
	// target.
	info, err := temp.Stat()
	if err != nil {
		_ = temp.Close()
		_ = os.Remove(tempPath)
		return nil, fmt.Errorf("stat ready file: %w", err)
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return nil, fmt.Errorf("close ready file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return nil, fmt.Errorf("publish ready file: %w", err)
	}
	return &portReadyFilePublication{
		path:   path,
		info:   info,
		digest: sha256.Sum256(data),
	}, nil
}

// removeIfUnchanged removes the published ready file only when the path still
// names the inode this launcher published and its contents are byte-identical.
func (publication *portReadyFilePublication) removeIfUnchanged() error {
	if publication == nil {
		return nil
	}
	info, err := os.Lstat(publication.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !os.SameFile(publication.info, info) {
		return nil
	}
	data, err := os.ReadFile(publication.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if sha256.Sum256(data) != publication.digest {
		return nil
	}
	return os.Remove(publication.path)
}
