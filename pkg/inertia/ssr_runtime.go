package inertia

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const ssrMinimumMajor = 22

// SSRConfig controls a JavaScript SSR process owned by cmd/ssr (or an
// equivalent operator entrypoint), not by the HTTP application process.
type SSRConfig struct {
	Enabled        bool
	BundlePath     string
	Executable     string
	StartupTimeout time.Duration
	MinimumMajor   int
	HTTP           SSRClientConfig
	Logger         *slog.Logger
	Stdout         io.Writer
	Stderr         io.Writer
}

// Validate verifies SSR runtime configuration.
func (config SSRConfig) Validate() error {
	if !config.Enabled {
		return nil
	}
	if strings.TrimSpace(config.BundlePath) == "" {
		return fmt.Errorf("inertia: SSR bundle path cannot be empty")
	}
	if strings.TrimSpace(config.Executable) == "" {
		return fmt.Errorf("inertia: SSR executable cannot be empty")
	}
	if config.StartupTimeout <= 0 {
		return fmt.Errorf("inertia: SSR startup timeout must be positive")
	}
	if config.MinimumMajor <= 0 {
		return fmt.Errorf("inertia: SSR minimum runtime major must be positive")
	}
	if err := config.HTTP.Validate(); err != nil {
		return err
	}
	parsed, _ := url.Parse(config.HTTP.URL)
	if !isLoopbackHost(parsed.Hostname()) {
		return fmt.Errorf("inertia: SSR URL must use a loopback host")
	}
	if parsed.Port() == "" {
		return fmt.Errorf("inertia: SSR URL must include a port")
	}
	return nil
}

// SSRRuntime owns one JavaScript process and exposes its HTTP renderer.
type SSRRuntime struct {
	config   SSRConfig
	renderer *HTTPRenderer
	errors   chan error

	mu       sync.Mutex
	command  *exec.Cmd
	done     chan error
	stopping bool
}

// NewSSRRuntime constructs an SSR runtime without starting it.
// Zero-value fields receive package defaults before validation.
func NewSSRRuntime(
	config SSRConfig,
	options ...HTTPRendererOption,
) (*SSRRuntime, error) {
	if strings.TrimSpace(config.Executable) == "" {
		config.Executable = "node"
	}
	if config.StartupTimeout <= 0 {
		config.StartupTimeout = 10 * time.Second
	}
	if config.MinimumMajor == 0 {
		config.MinimumMajor = ssrMinimumMajor
	}
	if strings.TrimSpace(config.HTTP.URL) == "" {
		config.HTTP.URL = "http://127.0.0.1:13714"
	}
	if config.HTTP.Timeout <= 0 {
		config.HTTP.Timeout = 2 * time.Second
	}
	if config.HTTP.MaxResponseBytes <= 0 {
		config.HTTP.MaxResponseBytes = 2 << 20
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if config.Stdout == nil {
		config.Stdout = os.Stdout
	}
	if config.Stderr == nil {
		config.Stderr = os.Stderr
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	runtime := &SSRRuntime{
		config: config,
		errors: make(chan error, 1),
	}
	if !config.Enabled {
		return runtime, nil
	}
	renderer, err := NewHTTPRenderer(config.HTTP, options...)
	if err != nil {
		return nil, err
	}
	if !isLoopbackHost(renderer.baseURL.Hostname()) {
		return nil, fmt.Errorf("inertia: SSR URL must use a loopback host")
	}
	runtime.renderer = renderer
	return runtime, nil
}

// Renderer returns the configured renderer, or nil when the runtime is disabled.
func (runtime *SSRRuntime) Renderer() SSRRenderer {
	if runtime == nil || !runtime.config.Enabled {
		return nil
	}
	return runtime.renderer
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
func (runtime *SSRRuntime) Start(ctx context.Context) error {
	if runtime == nil || !runtime.config.Enabled {
		return nil
	}
	if _, err := os.Stat(runtime.config.BundlePath); err != nil {
		return fmt.Errorf("inertia SSR bundle %q: %w", runtime.config.BundlePath, err)
	}
	executable, err := exec.LookPath(runtime.config.Executable)
	if err != nil {
		return fmt.Errorf("inertia SSR runtime %q: %w", runtime.config.Executable, err)
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
	if major < runtime.config.MinimumMajor {
		return fmt.Errorf(
			"inertia SSR requires runtime major %d or newer (found %q)",
			runtime.config.MinimumMajor,
			strings.TrimSpace(string(output)),
		)
	}

	runtime.mu.Lock()
	if runtime.command != nil {
		runtime.mu.Unlock()
		return fmt.Errorf("inertia SSR runtime is already started")
	}
	parsed, _ := url.Parse(runtime.config.HTTP.URL)
	command := exec.Command(executable, runtime.config.BundlePath)
	port := parsed.Port()
	command.Env = append(os.Environ(),
		"INERTIA_SSR_HOST="+parsed.Hostname(),
		"INERTIA_SSR_PORT="+port,
	)
	command.Stdout = runtime.config.Stdout
	command.Stderr = runtime.config.Stderr
	done := make(chan error, 1)
	if err := command.Start(); err != nil {
		runtime.mu.Unlock()
		return fmt.Errorf("start inertia SSR runtime: %w", err)
	}
	runtime.command = command
	runtime.done = done
	runtime.stopping = false
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
		if runtime.config.Logger != nil {
			runtime.config.Logger.Error("inertia SSR runtime stopped", "error", err)
		}
		select {
		case runtime.errors <- err:
		default:
		}
	}()

	startupCtx, cancel := context.WithTimeout(ctx, runtime.config.StartupTimeout)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := runtime.renderer.Health(startupCtx); err == nil {
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
	if runtime == nil || !runtime.config.Enabled {
		return nil
	}
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

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func normalizeWaitError(err error) error {
	if err == nil {
		return errors.New("process exited")
	}
	return err
}
