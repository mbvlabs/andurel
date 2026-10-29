package layout

import "testing"

// The SSR entries must scope route hosts per render. Configuring a module
// global from the request callback is what leaked origins across concurrent
// server-side renders.
func TestGeneratedSSREntriesScopeRouteHostsPerRender(t *testing.T) {
	for _, name := range []string{
		"inertia_assets_ssr.tmpl",
		"inertia_react_assets_ssr.tmpl",
		"inertia_svelte_assets_ssr.tmpl",
	} {
		assertTemplateContains(t, name,
			"import { AsyncLocalStorage } from 'node:async_hooks'",
			"configureRouteHostScope({",
			"withRouteHosts((page.props.hosts ?? {}) as Record<string, string>",
		)
		assertTemplateMissing(t, name, "configureRouteHosts(")
	}
}

// Browser entries keep one boot-time configuration; there is only one page per
// browser session, so no request scope is needed.
func TestGeneratedBrowserEntriesConfigureRouteHostsOnce(t *testing.T) {
	for _, name := range []string{
		"inertia_assets_app.tmpl",
		"inertia_react_assets_app.tmpl",
		"inertia_svelte_assets_app.tmpl",
	} {
		assertTemplateContains(t, name, "configureRouteHosts(")
		assertTemplateMissing(t, name, "withRouteHosts(")
	}
}
