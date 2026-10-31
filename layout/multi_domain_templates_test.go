package layout

import "testing"

func TestGeneratedRoutesRegisterByRouteValue(t *testing.T) {
	assertTemplateContains(t, "handlers_auth.tmpl",
		"rt.Route(routes.SessionNew).Get(h.SessionNew)",
		"rt.Route(routes.SessionCreate).Post(",
	)
	assertTemplateContains(t, "handlers_pages.tmpl", "rt.Route(routes.HomePage).Get(p.Home, middleware...)")
	assertTemplateContains(t, "handlers_api.tmpl", "rt.Route(routes.Health).Get(a.health)")
	assertTemplateMissing(t, "handlers_auth.tmpl", "rt.Get(routes.")
	assertTemplateMissing(t, "handlers_pages.tmpl", "rt.Get(routes.")
	assertTemplateMissing(t, "handlers_api.tmpl", "rt.Get(routes.")
	assertTemplateMissing(t, "handlers_assets.tmpl", "rt.Get(routes.")
}

func TestGeneratedHomeRouteIsNotMarkedExact(t *testing.T) {
	assertTemplateContains(t, "router_routes_pages.tmpl", "routing.TypeScriptRoute(),\n)")
	assertTemplateMissing(t, "router_routes_pages.tmpl", "routing.Exact()")
}

func TestGeneratedAuthOnlyRedirectsToPrimarySignIn(t *testing.T) {
	assertTemplateContains(t, "router_middleware_auth.tmpl",
		"func AuthOnly(users domains.Users, hosts *routing.Hosts) anduin.Middleware",
		"func signInURL(ctx context.Context, hosts *routing.Hosts) string",
		"anduin.RequestHost(ctx) == routing.HostPrimary",
		"routes.SessionNew.FullURL(hosts)",
	)
}

func TestGeneratedSessionCookieDomainIsConfigurable(t *testing.T) {
	assertTemplateContains(t, "config_session.tmpl",
		`strings.TrimSpace(e.String("SESSION_COOKIE_DOMAIN", ""))`,
		"CookieDomain string",
	)
	assertTemplateContains(t, "router_cookies_app.tmpl", "kiks.Domain(cfg.CookieDomain)")
	assertTemplateContains(t, "runtime_runtime.tmpl", "csrf.CookieDomain = cfg.Session.CookieDomain")
	assertTemplateContains(t, "env.tmpl", "# SESSION_COOKIE_DOMAIN=")
}

func TestGeneratedExtraHostsAreReadAndValidatedWithConfig(t *testing.T) {
	assertTemplateContains(t, "config_app.tmpl",
		"for _, name := range extraHostNames",
		"e.String(hostEnvKey(name), defaultHostname(name, primaryHost))",
		"routing.NewHosts(c.Hosts)",
	)
	assertTemplateContains(t, "config_hosts.tmpl",
		"var extraHostNames []routing.HostName",
		"func hostEnvKey(name routing.HostName) string",
		"func defaultHostname(name routing.HostName, primaryHost string) string",
	)
	assertTemplateContains(t, "env.tmpl", "# HOST_ADMIN=admin.localhost:8080")
}
