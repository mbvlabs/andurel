package inertia

import (
	"encoding/json"
	"fmt"
	"html"
	"io/fs"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
)

type viteTags struct {
	head string
	body string
}

type viteManifestEntry struct {
	File string   `json:"file"`
	CSS  []string `json:"css"`
}

// NewRenderer constructs the Inertia protocol renderer used by cmd/app.
//
// It always serves the Inertia protocol (CSR by default). Pages that pass
// WithSSR() call the configured SSR HTTP endpoint. Node process ownership
// belongs to NewSSRRuntime / cmd/ssr.
func NewRenderer(options ...Option) (*Renderer, error) {
	renderer := &Renderer{
		containerID: "app",
		ssrConfig: SSRClientConfig{
			URL:              "http://127.0.0.1:13714",
			Timeout:          2 * time.Second,
			MaxResponseBytes: 2 << 20,
		},
		buildPathURL: "/assets/dist/vite/*",
		entryPoint:   "resources/js/app.ts",
		viteDevURL:   "http://localhost:5173/assets/dist",
		shared:       make(Props),
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
	if renderer.root == nil {
		return nil, fmt.Errorf("inertia: root constructor cannot be nil")
	}

	if renderer.environment != "production" {
		base := strings.TrimRight(renderer.viteDevURL, "/")
		tags := viteTags{
			head: `<script type="module" src="` + html.EscapeString(
				base+"/@vite/client",
			) + `"></script>`,
			body: `<script type="module" src="` + html.EscapeString(
				base+"/"+renderer.entryPoint,
			) + `"></script>`,
		}
		if strings.HasSuffix(renderer.entryPoint, ".tsx") {
			tags.head += `<script type="module">
import RefreshRuntime from "` + html.EscapeString(base+"/@react-refresh") + `"
RefreshRuntime.injectIntoGlobalHook(window)
window.$RefreshReg$ = () => {}
window.$RefreshSig$ = () => (type) => type
window.__vite_plugin_react_preamble_installed__ = true
</script>`
		}
		renderer.viteTags = tags
	} else {
		if renderer.assetFS == nil {
			return nil, fmt.Errorf("inertia: production assets require an asset filesystem")
		}
		data, err := fs.ReadFile(renderer.assetFS, "dist/vite/manifest.json")
		if err != nil {
			return nil, fmt.Errorf("inertia: read Vite manifest: %w", err)
		}
		var manifest map[string]viteManifestEntry
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil, fmt.Errorf("inertia: parse Vite manifest: %w", err)
		}
		entry, ok := manifest[renderer.entryPoint]
		if !ok {
			return nil, fmt.Errorf(
				"inertia: Vite entry point %q not found in manifest",
				renderer.entryPoint,
			)
		}
		prefix := strings.TrimSuffix(renderer.buildPathURL, "*")
		tags := viteTags{}
		for _, stylesheet := range entry.CSS {
			tags.head += `<link rel="stylesheet" href="` + html.EscapeString(prefix+stylesheet) + `">`
		}
		tags.body = `<script type="module" src="` + html.EscapeString(prefix+entry.File) + `"></script>`
		renderer.viteTags = tags
	}

	if !renderer.customSSR {
		httpRenderer, err := NewHTTPRenderer(renderer.ssrConfig)
		if err != nil {
			return nil, err
		}
		renderer.ssr = httpRenderer
	}
	return renderer, nil
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
		renderer.ssrConfig.URL = rawURL
		return nil
	}
}

func WithSSRRequestTimeout(timeout time.Duration) Option {
	return func(renderer *Renderer) error {
		renderer.ssrConfig.Timeout = timeout
		return nil
	}
}

func WithSSRMaxResponseBytes(size int64) Option {
	return func(renderer *Renderer) error {
		renderer.ssrConfig.MaxResponseBytes = size
		return nil
	}
}
