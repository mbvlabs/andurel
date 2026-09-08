package inertia

import (
	"encoding/json"
	"fmt"
	"html"
	"io/fs"
	"strings"

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
// belongs to NewSSRRuntime / cmd/ssr. Required Inertia protocol settings are
// positional arguments; optional behavior is configured with options such as
// WithRoot, WithAssetFS, and WithEnvironment.
func NewRenderer(
	containerID string,
	buildPathURL string,
	entryPoint string,
	viteDevURL string,
	ssrConfig SSRClientConfig,
	options ...Option,
) (*Renderer, error) {
	if strings.TrimSpace(containerID) == "" {
		return nil, fmt.Errorf("inertia: container ID cannot be empty")
	}
	renderer := &Renderer{
		containerID:  strings.TrimSpace(containerID),
		buildPathURL: strings.TrimSpace(buildPathURL),
		version:      strings.TrimSpace(buildPathURL),
		entryPoint:   strings.TrimSpace(entryPoint),
		viteDevURL:   strings.TrimRight(strings.TrimSpace(viteDevURL), "/"),
		ssrConfig:    ssrConfig,
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

func WithRoot(root RootFunc) Option {
	return func(renderer *Renderer) error {
		if root == nil {
			return fmt.Errorf("inertia: root constructor cannot be nil")
		}
		renderer.root = root
		return nil
	}
}
