package inertia

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io/fs"
	"strings"
	"time"

	"github.com/mbvlabs/andurel/pkg/anduin"
)

type viteTags struct {
	head string
	body string
}

type viteManifestEntry struct {
	File string   `json:"file"`
	CSS  []string `json:"css"`
}

// RendererConfig holds the Inertia protocol settings of a Renderer.
type RendererConfig struct {
	// ContainerID is the id of the root element Inertia mounts into. Required.
	ContainerID string
	// BuildPathURL is the URL pattern of the Vite build route, e.g.
	// "https://example.com/assets/dist/*". It prefixes production asset tags
	// and is the default asset version.
	BuildPathURL string
	// EntryPoint is the Vite input, e.g. "resources/js/app.ts".
	EntryPoint string
	// ViteDevURL is the Vite dev server URL used outside production.
	ViteDevURL string
	// SSRURL is the SSR client URL (INERTIA_SSR_URL); any http(s) host.
	SSRURL string
	// SSRTimeout bounds one SSR render request.
	SSRTimeout time.Duration
	// SSRMaxResponseBytes caps the SSR response body.
	SSRMaxResponseBytes int64
}

// NewRenderer constructs the Inertia protocol renderer used by cmd/app.
//
// It always serves the Inertia protocol (CSR by default). Pages that pass
// WithSSR() call the configured SSR HTTP endpoint. In development, that is
// Vite's /__inertia_ssr endpoint derived from ViteDevURL. In other
// environments, cmd/app posts to INERTIA_SSR_URL /render. Node process
// ownership belongs to NewSSRRuntime / cmd/ssr in production. Required Inertia
// protocol settings live in RendererConfig; optional behavior is configured
// with options such as WithRoot, WithAssetFS, and WithEnvironment.
func NewRenderer(cfg RendererConfig, options ...Option) (*Renderer, error) {
	if strings.TrimSpace(cfg.ContainerID) == "" {
		return nil, fmt.Errorf("inertia: container ID cannot be empty")
	}

	renderer := &Renderer{
		containerID:         strings.TrimSpace(cfg.ContainerID),
		buildPathURL:        strings.TrimSpace(cfg.BuildPathURL),
		version:             strings.TrimSpace(cfg.BuildPathURL),
		entryPoint:          strings.TrimSpace(cfg.EntryPoint),
		viteDevURL:          strings.TrimRight(strings.TrimSpace(cfg.ViteDevURL), "/"),
		ssrURL:              strings.TrimSpace(cfg.SSRURL),
		ssrTimeout:          cfg.SSRTimeout,
		ssrMaxResponseBytes: cfg.SSRMaxResponseBytes,
		shared:              make(Props),
		requestFlash: []func(context.Context) any{
			func(ctx context.Context) any {
				return FlashFromContext(anduin.Request(ctx).Context())
			},
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

		// The scaffold serves Vite assets with non-credentialed CORS
		// (Access-Control-Allow-Origin: *), so these tags stay in anonymous
		// crossorigin mode; use-credentials would be rejected by the browser.
		prefix := strings.TrimSuffix(renderer.buildPathURL, "*")
		tags := viteTags{}
		for _, stylesheet := range entry.CSS {
			tags.head += `<link rel="stylesheet" crossorigin href="` + html.EscapeString(
				prefix+stylesheet,
			) + `">`
		}

		tags.body = `<script type="module" crossorigin src="` + html.EscapeString(
			prefix+entry.File,
		) + `"></script>`
		renderer.viteTags = tags
	}

	if !renderer.customSSR {
		if useViteDevSSR(renderer.environment, renderer.viteDevURL) {
			viteRenderer, err := NewViteSSRRenderer(
				renderer.viteDevURL,
				renderer.ssrTimeout,
				renderer.ssrMaxResponseBytes,
			)
			if err != nil {
				return nil, err
			}
			renderer.ssr = viteRenderer
		} else {
			httpRenderer, err := NewHTTPRenderer(
				renderer.ssrURL,
				renderer.ssrTimeout,
				renderer.ssrMaxResponseBytes,
			)
			if err != nil {
				return nil, err
			}
			renderer.ssr = httpRenderer
		}
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
