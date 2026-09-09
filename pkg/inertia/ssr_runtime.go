package inertia

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const ssrHealthMaxResponseBytes = 64 << 10

// SSRRuntimeOption configures optional SSR process I/O.
type SSRRuntimeOption func(*SSRRuntime) error

// SSRRuntime owns one JavaScript process. Its HTTP client is only used for
// /health and /shutdown on the listen address, never for application /render.
type SSRRuntime struct {
	executable     string
	bundlePath     string
	listenHost     string
	listenPort     string
	startupTimeout time.Duration
	minimumMajor   int
	bundleFS       fs.FS
	logger         *slog.Logger
	stdout         io.Writer
	stderr         io.Writer
	renderer       *HTTPRenderer
	errors         chan error

	mu            sync.Mutex
	command       *exec.Cmd
	done          chan error
	stopping      bool
	bundleCleanup func()
}

// WithSSRLogger sets the logger used when the child process exits unexpectedly.
func WithSSRLogger(logger *slog.Logger) SSRRuntimeOption {
	return func(runtime *SSRRuntime) error {
		if logger == nil {
			return fmt.Errorf("inertia: SSR logger cannot be nil")
		}

		runtime.logger = logger
		return nil
	}
}

// WithSSRStdout sets the child process stdout. Defaults to os.Stdout.
func WithSSRStdout(stdout io.Writer) SSRRuntimeOption {
	return func(runtime *SSRRuntime) error {
		if stdout == nil {
			return fmt.Errorf("inertia: SSR stdout cannot be nil")
		}

		runtime.stdout = stdout
		return nil
	}
}

// WithSSRStderr sets the child process stderr. Defaults to os.Stderr.
func WithSSRStderr(stderr io.Writer) SSRRuntimeOption {
	return func(runtime *SSRRuntime) error {
		if stderr == nil {
			return fmt.Errorf("inertia: SSR stderr cannot be nil")
		}

		runtime.stderr = stderr
		return nil
	}
}

// NewSSRRuntime constructs an SSR runtime without starting it.
// listenURL is where Node binds (INERTIA_SSR_LISTEN), not the app client URL.
// bundleFS is optional; nil means disk only. Generated cmd/ssr passes assets.Files.
func NewSSRRuntime(
	executable string,
	bundlePath string,
	listenURL string,
	startupTimeout time.Duration,
	minimumMajor int,
	healthTimeout time.Duration,
	bundleFS fs.FS,
	options ...SSRRuntimeOption,
) (*SSRRuntime, error) {
	if strings.TrimSpace(executable) == "" {
		return nil, fmt.Errorf("inertia: SSR executable cannot be empty")
	}

	if strings.TrimSpace(bundlePath) == "" {
		return nil, fmt.Errorf("inertia: SSR bundle path cannot be empty")
	}

	if startupTimeout <= 0 {
		return nil, fmt.Errorf("inertia: SSR startup timeout must be positive")
	}

	if minimumMajor <= 0 {
		return nil, fmt.Errorf("inertia: SSR minimum runtime major must be positive")
	}

	if healthTimeout <= 0 {
		return nil, fmt.Errorf("inertia: SSR timeout must be positive")
	}

	listen, err := parseSSRListen(listenURL)
	if err != nil {
		return nil, err
	}

	renderer, err := NewHTTPRenderer(
		listen.healthURL,
		healthTimeout,
		ssrHealthMaxResponseBytes,
	)
	if err != nil {
		return nil, err
	}

	runtime := &SSRRuntime{
		executable:     strings.TrimSpace(executable),
		bundlePath:     strings.TrimSpace(bundlePath),
		listenHost:     listen.host,
		listenPort:     listen.port,
		startupTimeout: startupTimeout,
		minimumMajor:   minimumMajor,
		bundleFS:       bundleFS,
		logger:         slog.Default(),
		stdout:         os.Stdout,
		stderr:         os.Stderr,
		renderer:       renderer,
		errors:         make(chan error, 1),
	}

	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(runtime); err != nil {
			return nil, err
		}
	}

	return runtime, nil
}

// Errors reports unexpected child-process exits. The channel remains valid if
// the runtime is restarted.
func (runtime *SSRRuntime) Errors() <-chan error {
	if runtime == nil {
		return nil
	}

	return runtime.errors
}

// Start starts the configured JavaScript process and waits for health readiness.
func (runtime *SSRRuntime) Start(ctx context.Context) (err error) {
	if runtime == nil {
		return nil
	}

	bundlePath, cleanup, err := resolveSSRBundle(runtime.bundlePath, runtime.bundleFS)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil && cleanup != nil {
			runtime.mu.Lock()
			runtime.bundleCleanup = nil
			runtime.mu.Unlock()
			cleanup()
		}
	}()

	executable, err := exec.LookPath(runtime.executable)
	if err != nil {
		return fmt.Errorf("inertia SSR runtime %q: %w", runtime.executable, err)
	}

	output, err := exec.CommandContext(ctx, executable, "--version").Output()
	if err != nil {
		return fmt.Errorf("inspect SSR runtime version: %w", err)
	}

	var major int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(output)), "v%d.", &major); err != nil {
		return fmt.Errorf(
			"inspect SSR runtime version %q: %w",
			strings.TrimSpace(string(output)),
			err,
		)
	}

	if major < runtime.minimumMajor {
		return fmt.Errorf(
			"inertia SSR requires runtime major %d or newer (found %q)",
			runtime.minimumMajor,
			strings.TrimSpace(string(output)),
		)
	}

	runtime.mu.Lock()
	if runtime.command != nil {
		runtime.mu.Unlock()
		return fmt.Errorf("inertia SSR runtime is already started")
	}

	command := exec.Command(executable, bundlePath)
	command.Env = append(os.Environ(),
		"INERTIA_SSR_HOST="+runtime.listenHost,
		"INERTIA_SSR_PORT="+runtime.listenPort,
	)
	command.Stdout = runtime.stdout
	command.Stderr = runtime.stderr

	done := make(chan error, 1)
	if err := command.Start(); err != nil {
		runtime.mu.Unlock()
		return fmt.Errorf("start inertia SSR runtime: %w", err)
	}

	runtime.command = command
	runtime.done = done
	runtime.stopping = false
	runtime.bundleCleanup = cleanup
	runtime.mu.Unlock()

	go func() {
		waitErr := command.Wait()
		done <- waitErr
		close(done)

		runtime.mu.Lock()
		expected := runtime.stopping
		if runtime.command == command {
			runtime.command = nil
			runtime.done = nil
			runtime.stopping = false
		}
		runtime.mu.Unlock()

		if expected {
			return
		}

		err := fmt.Errorf("inertia SSR runtime stopped unexpectedly: %w", normalizeWaitError(waitErr))
		if runtime.logger != nil {
			runtime.logger.Error("inertia SSR runtime stopped", "error", err)
		}

		select {
		case runtime.errors <- err:
		default:
		}
	}()

	startupCtx, cancel := context.WithTimeout(ctx, runtime.startupTimeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		if err := runtime.renderer.Health(startupCtx); err == nil {
			cleanup = nil
			return nil
		}

		select {
		case waitErr := <-done:
			return fmt.Errorf(
				"inertia SSR runtime exited during startup: %w",
				normalizeWaitError(waitErr),
			)
		case <-ticker.C:
		case <-startupCtx.Done():
			_ = command.Process.Kill()
			return fmt.Errorf("inertia SSR runtime health check: %w", startupCtx.Err())
		}
	}
}

// Stop gracefully stops the managed process and kills it when the context ends.
func (runtime *SSRRuntime) Stop(ctx context.Context) error {
	if runtime == nil {
		return nil
	}
	defer runtime.removeExtractedBundle()

	runtime.mu.Lock()
	command := runtime.command
	done := runtime.done
	if command == nil {
		runtime.mu.Unlock()
		return nil
	}

	runtime.stopping = true
	runtime.mu.Unlock()

	shutdownErr := runtime.renderer.Shutdown(ctx)
	select {
	case waitErr := <-done:
		if waitErr != nil && shutdownErr == nil {
			shutdownErr = waitErr
		}

		return shutdownErr
	case <-ctx.Done():
		killErr := command.Process.Kill()
		return errors.Join(shutdownErr, ctx.Err(), killErr)
	}
}

func (runtime *SSRRuntime) removeExtractedBundle() {
	runtime.mu.Lock()
	cleanup := runtime.bundleCleanup
	runtime.bundleCleanup = nil
	runtime.mu.Unlock()
	if cleanup != nil {
		cleanup()
	}
}

func resolveSSRBundle(bundlePath string, bundleFS fs.FS) (string, func(), error) {
	if _, err := os.Stat(bundlePath); err == nil {
		return bundlePath, nil, nil
	} else if bundleFS == nil {
		return "", nil, fmt.Errorf("inertia SSR bundle %q: %w", bundlePath, err)
	}

	embedPath := embedPathForBundle(bundlePath)
	data, err := fs.ReadFile(bundleFS, embedPath)
	if err != nil {
		return "", nil, fmt.Errorf("inertia SSR bundle %q: %w", bundlePath, err)
	}

	dir, err := os.MkdirTemp("", "inertia-ssr-")
	if err != nil {
		return "", nil, fmt.Errorf("inertia SSR bundle %q: %w", bundlePath, err)
	}

	cleanup := func() { _ = os.RemoveAll(dir) }
	dest := filepath.Join(dir, filepath.Base(bundlePath))
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("inertia SSR bundle %q: %w", bundlePath, err)
	}

	if mapData, mapErr := fs.ReadFile(bundleFS, embedPath+".map"); mapErr == nil {
		_ = os.WriteFile(dest+".map", mapData, 0o644)
	}

	return dest, cleanup, nil
}

func embedPathForBundle(bundle string) string {
	return strings.TrimPrefix(filepath.ToSlash(bundle), "assets/")
}

func normalizeWaitError(err error) error {
	if err == nil {
		return errors.New("process exited")
	}

	return err
}
