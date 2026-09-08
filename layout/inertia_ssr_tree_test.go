package layout

import (
	"strings"
	"testing"
)

func TestInertiaClientAndSSRSetupShareFlashToasts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		adapter string
		app     string
		ssr     string
		shared  string
		snippet string
	}{
		{
			adapter: "react",
			app:     "inertia_react_assets_app.tmpl",
			ssr:     "inertia_react_assets_ssr.tmpl",
			shared:  "FlashToasts",
			snippet: "<FlashToasts initialFlashes={pageFlashes(props.initialPage.flash)} />",
		},
		{
			adapter: "vue",
			app:     "inertia_assets_app.tmpl",
			ssr:     "inertia_assets_ssr.tmpl",
			shared:  "renderAppTree",
			snippet: "renderAppTree(App, props)",
		},
		{
			adapter: "svelte",
			app:     "inertia_svelte_assets_app.tmpl",
			ssr:     "inertia_svelte_assets_ssr.tmpl",
			shared:  "AppTree",
			snippet: "AppTree",
		},
	}

	for _, tc := range cases {
		t.Run(tc.adapter, func(t *testing.T) {
			t.Parallel()

			app := readGeneratedApplicationTemplate(t, tc.app)
			ssr := readGeneratedApplicationTemplate(t, tc.ssr)

			if !strings.Contains(app, tc.shared) {
				t.Errorf("%s does not reference %q", tc.app, tc.shared)
			}
			if !strings.Contains(ssr, tc.shared) {
				t.Errorf("%s does not reference %q", tc.ssr, tc.shared)
			}
			if !strings.Contains(app, tc.snippet) {
				t.Errorf("%s does not contain shared tree %q", tc.app, tc.snippet)
			}
			if !strings.Contains(ssr, tc.snippet) {
				t.Errorf("%s does not contain shared tree %q", tc.ssr, tc.snippet)
			}

			assertSSRSetupDoesNotRenderAppOnly(t, tc.adapter, ssr)
		})
	}
}

func TestInertiaSharedFlashToastModulesExist(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"inertia_react_assets_components_flash_toasts.tmpl":  "export function FlashToasts",
		"inertia_assets_components_flash_toasts.tmpl":        "export function renderAppTree",
		"inertia_svelte_assets_components_app_tree.tmpl":     "FlashToasts",
		"inertia_svelte_assets_components_flash_toasts.tmpl": "$effect(() =>",
	}
	for name, want := range files {
		content := readGeneratedApplicationTemplate(t, name)
		if !strings.Contains(content, want) {
			t.Errorf("%s does not contain %q", name, want)
		}
	}

	vueTree := readGeneratedApplicationTemplate(t, "inertia_assets_components_flash_toasts.tmpl")
	if !strings.Contains(vueTree, "h(App, props)") || !strings.Contains(vueTree, "h(FlashToasts,") {
		t.Error("vue renderAppTree must render App and FlashToasts as siblings")
	}

	svelteTree := readGeneratedApplicationTemplate(t, "inertia_svelte_assets_components_app_tree.tmpl")
	if !strings.Contains(svelteTree, "<App {initialPage} {...rest} />") ||
		!strings.Contains(svelteTree, "<FlashToasts") {
		t.Error("svelte AppTree must render App and FlashToasts as siblings")
	}
}

func assertSSRSetupDoesNotRenderAppOnly(t *testing.T, adapter, ssr string) {
	t.Helper()

	appOnly := []string{
		"setup: ({ App, props }) => <App {...props} />",
		"h('div', [h(App, props)])",
		"return render(App, { props })",
	}
	for _, pattern := range appOnly {
		if strings.Contains(ssr, pattern) {
			t.Errorf("%s SSR setup renders only App (%q)", adapter, pattern)
		}
	}
}
