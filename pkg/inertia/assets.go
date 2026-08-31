package inertia

import (
	"encoding/json"
	"fmt"
	"html"
	"io/fs"
	"strings"
)

type viteTags struct {
	head string
	body string
}

type viteManifestEntry struct {
	File string   `json:"file"`
	CSS  []string `json:"css"`
}

func (renderer *Renderer) configureRoot() error {
	if renderer.root == nil {
		return fmt.Errorf("inertia: root constructor cannot be nil")
	}
	tags, err := renderer.resolveViteTags()
	if err != nil {
		return err
	}
	renderer.viteTags = tags
	return nil
}

func (renderer *Renderer) resolveViteTags() (viteTags, error) {
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
		return tags, nil
	}
	if renderer.assetFS == nil {
		return viteTags{}, fmt.Errorf("inertia: production assets require an asset filesystem")
	}
	data, err := fs.ReadFile(renderer.assetFS, "dist/vite/manifest.json")
	if err != nil {
		return viteTags{}, fmt.Errorf("inertia: read Vite manifest: %w", err)
	}
	var manifest map[string]viteManifestEntry
	if err := json.Unmarshal(data, &manifest); err != nil {
		return viteTags{}, fmt.Errorf("inertia: parse Vite manifest: %w", err)
	}
	entry, ok := manifest[renderer.entryPoint]
	if !ok {
		return viteTags{}, fmt.Errorf(
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
	return tags, nil
}
