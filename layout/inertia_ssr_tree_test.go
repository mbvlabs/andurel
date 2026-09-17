package layout

import (
	"strings"
	"testing"
)

func TestInertiaClientAndSSRSetupRenderAppOnly(t *testing.T) {
	t.Parallel()

	cases := []struct {
		adapter string
		app     string
		ssr     string
		want    string
		unwant  string
	}{
		{
			adapter: "react",
			app:     "inertia_react_assets_app.tmpl",
			ssr:     "inertia_react_assets_ssr.tmpl",
			want:    "<App {...props} />",
			unwant:  "FlashToasts",
		},
		{
			adapter: "vue",
			app:     "inertia_assets_app.tmpl",
			ssr:     "inertia_assets_ssr.tmpl",
			want:    "h(App, props)",
			unwant:  "renderAppTree",
		},
		{
			adapter: "svelte",
			app:     "inertia_svelte_assets_app.tmpl",
			ssr:     "inertia_svelte_assets_ssr.tmpl",
			want:    "mount(App, { target: el, props })",
			unwant:  "AppTree",
		},
	}

	for _, tc := range cases {
		t.Run(tc.adapter, func(t *testing.T) {
			t.Parallel()

			app := readGeneratedApplicationTemplate(t, tc.app)
			ssr := readGeneratedApplicationTemplate(t, tc.ssr)

			if !strings.Contains(app, tc.want) {
				t.Errorf("%s does not contain %q", tc.app, tc.want)
			}
			if strings.Contains(app, tc.unwant) {
				t.Errorf("%s still references %q", tc.app, tc.unwant)
			}
			if strings.Contains(ssr, tc.unwant) {
				t.Errorf("%s still references %q", tc.ssr, tc.unwant)
			}
			if strings.Contains(app, "flash-toasts") || strings.Contains(ssr, "flash-toasts") {
				t.Errorf("%s still imports flash-toasts UI", tc.adapter)
			}
		})
	}
}

func TestInertiaFlashToastModulesAreRemoved(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"inertia_react_assets_components_flash_toasts.tmpl",
		"inertia_assets_components_flash_toasts.tmpl",
		"inertia_svelte_assets_components_app_tree.tmpl",
		"inertia_svelte_assets_components_flash_toasts.tmpl",
		"views_components_toast.tmpl",
		"css_toasts.tmpl",
	} {
		if _, exists := baseStyleTemplateMappings[TmplTarget(name)]; exists {
			t.Errorf("%s is still mapped in baseStyleTemplateMappings", name)
		}
		if _, exists := inertiaVueTemplateMappings[TmplTarget(name)]; exists {
			t.Errorf("%s is still mapped for vue", name)
		}
		if _, exists := inertiaReactTemplateMappings[TmplTarget(name)]; exists {
			t.Errorf("%s is still mapped for react", name)
		}
		if _, exists := inertiaSvelteTemplateMappings[TmplTarget(name)]; exists {
			t.Errorf("%s is still mapped for svelte", name)
		}
	}
}
