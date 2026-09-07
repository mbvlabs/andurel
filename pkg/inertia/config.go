package inertia

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
)

// Config controls stable renderer behavior.
type Config struct {
	ContainerID   string
	ProtocolDebug bool
	SSRFailFast   bool
}

// DefaultConfig returns the renderer's protocol defaults.
func DefaultConfig() Config {
	return Config{ContainerID: "app"}
}

// Validate verifies required renderer configuration.
func (config Config) Validate() error {
	if strings.TrimSpace(config.ContainerID) == "" {
		return fmt.Errorf("inertia: container ID cannot be empty")
	}
	return nil
}

// New constructs a reusable renderer. Applications must supply their compiled
// document with WithRoot and any other settings they need.
func New(options ...Option) (*Renderer, error) {
	config := DefaultConfig()
	renderer := &Renderer{
		containerID:   config.ContainerID,
		protocolDebug: config.ProtocolDebug,
		ssrFailFast:   config.SSRFailFast,
		managedConfig: DefaultManagedConfig(),
		buildPathURL:  "/assets/dist/vite/*",
		entryPoint:    "resources/js/app.ts",
		viteDevURL:    "http://localhost:5173/assets/dist",
		shared:        make(Props),
		requestFlash: []func(*echo.Context) any{
			func(etx *echo.Context) any { return FlashFromContext(etx.Request().Context()) },
		},
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(renderer); err != nil {
			return nil, fmt.Errorf("inertia: create renderer: %w", err)
		}
	}
	if strings.TrimSpace(renderer.containerID) == "" {
		return nil, fmt.Errorf("inertia: container ID cannot be empty")
	}
	if err := renderer.configureRoot(); err != nil {
		return nil, err
	}
	if err := renderer.configureSSR(); err != nil {
		return nil, err
	}
	return renderer, nil
}

// Start is retained for lifecycle hooks. The HTTP app does not own the Node
// SSR process; use cmd/ssr (or an operator) for that. This method is a no-op
// unless a custom managed runtime was attached.
func (renderer *Renderer) Start(ctx context.Context) error {
	if renderer == nil || renderer.runtime == nil {
		return nil
	}
	return renderer.runtime.Start(ctx)
}

// Shutdown stops an attached managed SSR runtime. It is a no-op for the
// default HTTP-client configuration used by cmd/app.
func (renderer *Renderer) Shutdown(ctx context.Context) error {
	if renderer == nil || renderer.runtime == nil {
		return nil
	}
	return renderer.runtime.Stop(ctx)
}

func (renderer *Renderer) configureSSR() error {
	if renderer.customSSR {
		return nil
	}
	httpRenderer, err := NewHTTPRenderer(renderer.managedConfig.HTTP)
	if err != nil {
		return err
	}
	renderer.ssr = httpRenderer
	return nil
}

// WithAssetFS supplies the embedded application asset filesystem used for the
// production Vite manifest.
func WithAssetFS(assetFS fs.FS) Option {
	return func(renderer *Renderer) error {
		if assetFS == nil {
			return fmt.Errorf("inertia: asset filesystem is nil")
		}
		renderer.assetFS = assetFS
		return nil
	}
}

func WithProjectName(name string) Option {
	return func(renderer *Renderer) error {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("inertia: project name cannot be empty")
		}
		renderer.projectName = strings.TrimSpace(name)
		return nil
	}
}

func WithEnvironment(environment string) Option {
	return func(renderer *Renderer) error {
		if strings.TrimSpace(environment) == "" {
			return fmt.Errorf("inertia: environment cannot be empty")
		}
		renderer.environment = strings.TrimSpace(environment)
		return nil
	}
}

func WithBuildPathURL(path string) Option {
	return func(renderer *Renderer) error {
		if strings.TrimSpace(path) == "" {
			return fmt.Errorf("inertia: Vite build path URL cannot be empty")
		}
		renderer.buildPathURL = strings.TrimSpace(path)
		renderer.version = renderer.buildPathURL
		return nil
	}
}

func WithEntryPoint(entryPoint string) Option {
	return func(renderer *Renderer) error {
		if strings.TrimSpace(entryPoint) == "" {
			return fmt.Errorf("inertia: Vite entry point cannot be empty")
		}
		renderer.entryPoint = strings.TrimSpace(entryPoint)
		return nil
	}
}

func WithViteDevURL(rawURL string) Option {
	return func(renderer *Renderer) error {
		if strings.TrimSpace(rawURL) == "" {
			return fmt.Errorf("inertia: Vite development URL cannot be empty")
		}
		renderer.viteDevURL = strings.TrimRight(strings.TrimSpace(rawURL), "/")
		return nil
	}
}

func WithRoot(root RootFunc) Option {
	return func(renderer *Renderer) error {
		if root == nil {
			return fmt.Errorf("inertia: root constructor cannot be nil")
		}
		renderer.root = root
		return nil
	}
}

func WithSSRURL(rawURL string) Option {
	return func(renderer *Renderer) error {
		renderer.managedConfig.HTTP.URL = rawURL
		return nil
	}
}

func WithSSRRuntime(executable string) Option {
	return func(renderer *Renderer) error {
		renderer.managedConfig.Executable = executable
		return nil
	}
}

func WithSSRBundle(path string) Option {
	return func(renderer *Renderer) error {
		renderer.managedConfig.BundlePath = path
		return nil
	}
}

func WithSSRRequestTimeout(timeout time.Duration) Option {
	return func(renderer *Renderer) error {
		renderer.managedConfig.HTTP.Timeout = timeout
		return nil
	}
}

func WithSSRStartupTimeout(timeout time.Duration) Option {
	return func(renderer *Renderer) error {
		renderer.managedConfig.StartupTimeout = timeout
		return nil
	}
}

func WithSSRMaxResponseBytes(size int64) Option {
	return func(renderer *Renderer) error {
		renderer.managedConfig.HTTP.MaxResponseBytes = size
		return nil
	}
}
