package upgrade

import (
	"fmt"
	"strings"

	"github.com/mbvlabs/andurel/v2/layout/versions"
	"golang.org/x/mod/semver"
)

// inertiaViteFullURLVersion is the first released V2 pre-release that shipped
// the absolute Vite build URL for Inertia (#805). Projects already on or past
// it get no note.
const inertiaViteFullURLVersion = "v2.0.0-alpha.3"

// packageAPIVersion is the first release whose scaffold pins the package APIs
// that take config structs and drop unused arguments.
const packageAPIVersion = "v2.0.0-alpha.5"

// storageTestClusterVersion is the first release whose scaffold imports the
// test-container helpers from pkg/storage/storagetest.
const storageTestClusterVersion = "v2.0.0-alpha.5"

// betaTwoFixesVersion is the first release whose scaffold derives a valid
// default email sender, bundles SSR dependencies into the production build
// only, and renders a single <title> under SSR.
const betaTwoFixesVersion = "v2.0.0-beta.2"

// betaThreeFixesVersion is the first release whose scaffold trusts only
// explicit CORS origins, keeps CSRF cookies off static assets, renders 405
// responses as the themed not-found page, and exposes anduin.RequestHost for
// host-scoped middleware.
const betaThreeFixesVersion = "v2.0.0-beta.3"

// betaFourFixesVersion is the candidate release after v2.0.0-beta.3. It is not
// tagged yet: the actions below gate on it because the matching scaffold change
// must ship in the release that tags it. pkg/inertia v0.13.0 is the companion
// release gate.
const betaFourFixesVersion = "v2.0.0-beta.4"

// goNativeRebuildVersion is the release that replaced Echo and Fx with the
// Go-native domains/services/handlers layout. Projects older than it cannot be
// upgraded automatically; they must be ported by hand.
const goNativeRebuildVersion = "v2.0.0-alpha.4"

// ManualAction describes an application-owned change that an upgrade cannot
// apply without risking user code.
type ManualAction struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Instructions string `json:"instructions"`
}

// ManualActionsBetween returns the manual actions an upgrade from fromVersion
// to toVersion prints. It lets a project re-read them after andurel.toml
// already records toVersion.
func ManualActionsBetween(
	fromVersion string,
	toVersion string,
	includeInertiaMigration bool,
) ([]ManualAction, error) {
	from, fromOK := canonicalUpgradeVersion(fromVersion)
	if !fromOK {
		return nil, fmt.Errorf("invalid from version %q", fromVersion)
	}
	to, toOK := canonicalUpgradeVersion(toVersion)
	if !toOK {
		return nil, fmt.Errorf("invalid to version %q", toVersion)
	}
	if semver.Compare(from, to) >= 0 {
		return nil, fmt.Errorf("from version %s must be older than %s", from, to)
	}

	return manualActionsForUpgrade(from, to, includeInertiaMigration)
}

func manualActionsForUpgrade(
	fromVersion string,
	toVersion string,
	includeInertiaMigration bool,
) ([]ManualAction, error) {
	actions := []ManualAction{}

	if includeInertiaMigration &&
		crossesVersion(fromVersion, toVersion, inertiaViteFullURLVersion) {
		var instructions strings.Builder
		instructions.WriteString(
			"Inertia production tags now need an absolute Vite build URL so secondary-host pages load the entry from the primary origin.\n\n",
		)
		instructions.WriteString(
			"The composition root is application-owned, so Andurel does not rewrite NewInertia automatically.\n\n",
		)
		instructions.WriteString(
			"1. In NewInertia (or the inertia renderer provider), pass routes.ViteBuild.FullURL() instead of Path():\n\n```go\n",
		)
		instructions.WriteString(
			"inertia.NewRenderer(inertia.RendererConfig{\n\tContainerID:  cfg.ContainerID,\n\tBuildPathURL: routes.ViteBuild.FullURL(),\n\t// ...\n})\n",
		)
		instructions.WriteString("```\n\n")
		instructions.WriteString("2. Format and verify the migration:\n\n```text\n")
		instructions.WriteString("gofmt -w internal/runtime\n")
		instructions.WriteString("go fix ./...\ngo vet ./...\n```\n")

		actions = append(actions, ManualAction{
			// The ID predates the alpha.3 gate. It stays unchanged because
			// structured upgrade output exposes it as a stable identifier.
			ID:           "inertia-vite-fullurl-v2.0.0",
			Title:        "Pass ViteBuild.FullURL() to the Inertia renderer",
			Instructions: instructions.String(),
		})
	}

	if crossesVersion(fromVersion, toVersion, packageAPIVersion) {
		actions = append(actions, ManualAction{
			ID:           "package-api-v2.0.0-alpha.5",
			Title:        "Update internal/runtime for the anduin, cache, inertia and storage APIs",
			Instructions: packageAPIInstructions(includeInertiaMigration),
		})
	}

	if crossesVersion(fromVersion, toVersion, storageTestClusterVersion) {
		actions = append(actions, ManualAction{
			ID:           "storage-storagetest-v2.0.0-alpha.5",
			Title:        "Move test-cluster helpers to pkg/storage/storagetest",
			Instructions: storageTestClusterInstructions(),
		})
	}

	if crossesVersion(fromVersion, toVersion, packageAPIVersion) {
		actions = append(actions, ManualAction{
			ID:           "routing-hosts-v2.0.0-alpha.5",
			Title:        "Replace the routing host registry with routing.NewHosts",
			Instructions: routingHostsInstructions(),
		})
	}

	if crossesVersion(fromVersion, toVersion, packageAPIVersion) {
		actions = append(actions, ManualAction{
			ID:           "session-revocation-v2.0.0-alpha.5",
			Title:        "Add session revocation to users, queries and AuthOnly",
			Instructions: sessionRevocationInstructions(),
		})
	}

	if crossesVersion(fromVersion, toVersion, packageAPIVersion) {
		actions = append(actions, ManualAction{
			ID:           "email-smtp-v2.0.0-alpha.5",
			Title:        "Add the SMTP sender and pass config.App to newEmailSenders",
			Instructions: emailSMTPInstructions(),
		})
	}

	if crossesVersion(fromVersion, toVersion, packageAPIVersion) {
		actions = append(actions, ManualAction{
			ID:           "validation-validate-v2.0.0-alpha.5",
			Title:        "Call the domain Validate method instead of validation.Validate",
			Instructions: validationInstructions(),
		})
	}

	if crossesVersion(fromVersion, toVersion, betaTwoFixesVersion) {
		actions = append(actions, ManualAction{
			ID:           "email-sender-v2.0.0-beta.2",
			Title:        "Strip the port from the default email sender and validate it",
			Instructions: emailSenderInstructions(),
		})
	}

	if includeInertiaMigration &&
		crossesVersion(fromVersion, toVersion, betaTwoFixesVersion) {
		actions = append(actions, ManualAction{
			ID:           "vite-ssr-bundle-v2.0.0-beta.2",
			Title:        "Bundle SSR dependencies into the production build only",
			Instructions: viteSSRBundleInstructions(),
		})
		actions = append(actions, ManualAction{
			ID:           "root-title-ssr-v2.0.0-beta.2",
			Title:        "Render the fallback <title> only for client-rendered pages",
			Instructions: rootTitleInstructions(),
		})
	}

	if crossesVersion(fromVersion, toVersion, betaThreeFixesVersion) {
		actions = append(actions, ManualAction{
			ID:           "cors-host-origins-v2.0.0-beta.3",
			Title:        "Scope CORS to CORS_ALLOWED_ORIGINS and serve Vite assets with public CORS",
			Instructions: corsScopeInstructions(),
		})
		actions = append(actions, ManualAction{
			ID:           "csrf-asset-skip-v2.0.0-beta.3",
			Title:        "Skip CSRF cookie issuance for safe asset requests",
			Instructions: csrfAssetSkipInstructions(),
		})
		actions = append(actions, ManualAction{
			ID:           "method-not-allowed-page-v2.0.0-beta.3",
			Title:        "Render the themed not-found page for 405 responses",
			Instructions: methodNotAllowedInstructions(),
		})
		actions = append(actions, ManualAction{
			ID:           "request-host-middleware-v2.0.0-beta.3",
			Title:        "Use anduin.RequestHost for host-scoped middleware",
			Instructions: requestHostInstructions(),
		})
	}

	if crossesVersion(fromVersion, toVersion, betaFourFixesVersion) {
		actions = append(actions,
			ManualAction{
				ID:           "public-home-v2.0.0-beta.4",
				Title:        "Register the welcome page without AuthOnly",
				Instructions: publicHomeInstructions(),
			},
			ManualAction{
				ID:           "configcheck-v2.0.0-beta.4",
				Title:        "Add the read-only internal/configcheck helper",
				Instructions: configCheckInstructions(),
			},
			ManualAction{
				ID:           "runtime-readiness-v2.0.0-beta.4",
				Title:        "Expose /api/ready and wire database plus SSR readiness",
				Instructions: runtimeReadinessInstructions(),
			},
		)
		if includeInertiaMigration {
			actions = append(actions,
				ManualAction{
					ID:           "inertia-ssr-default-v2.0.0-beta.4",
					Title:        "Server-render production pages by default and keep the themed error page",
					Instructions: inertiaSSRDefaultInstructions(),
				},
				ManualAction{
					ID:           "routes-host-scope-v2.0.0-beta.4",
					Title:        "Regenerate routes.ts and scope route hosts per SSR render",
					Instructions: routesHostScopeInstructions(),
				},
			)
		}
	}

	return actions, nil
}

func publicHomeInstructions() string {
	return "The generated home page is public in the beta.4 scaffold: a new project must be reachable before " +
		"anyone signs in. An older scaffold registers it behind middleware.AuthOnly, so anonymous `GET /` redirects " +
		"to sign-in. That is generated registration code in an application-owned file, so change it by hand:\n\n" +
		"In internal/runtime/runtime.go, register the pages handler without the auth middleware:\n\n```go\n" +
		"handlers.NewPagesHandler(renderer).Register(router)\n" +
		"```\n\n" +
		"instead of:\n\n```go\n" +
		"handlers.NewPagesHandler(renderer).Register(router, middleware.AuthOnly(users))\n" +
		"```\n\n" +
		"Keep middleware.AuthOnly(users) on genuinely protected routes; the login and registration routes stay " +
		"public. Run `andurel run` and check anonymous `GET /` returns 200 with the welcome page while a protected " +
		"route still redirects. Format and verify:\n\n```text\ngofmt -w internal/runtime\ngo fix ./...\ngo vet ./...\n```\n"
}

func configCheckInstructions() string {
	return "andurel doctor validates runtime configuration through a generated, read-only helper instead of a " +
		"hard-coded key list, so the check cannot drift from config.New(). `andurel upgrade` does not manage " +
		"internal/ files: create `internal/configcheck/main.go` from a current `andurel new` scaffold and keep it " +
		"in step when config changes. It must:\n\n" +
		"1. call `env.Load()` and `config.New()`;\n" +
		"2. open the database with `storage.NewPostgres(ctx, cfg, storage.WithoutOpenTelemetry())` and run the " +
		"read-only `db.Health(ctx)` check;\n" +
		"3. print one JSON document `{\"ok\":...,\"config\":{\"status\":\"valid|invalid\",\"errors\":[...]},\"database\":{\"status\":\"reachable|unreachable|skipped\",\"reason\":...}}`;\n" +
		"4. never migrate, create a database, or print a password or connection URL.\n\n" +
		"The helper must be string-free of framework-owned paths: it is application code. Once it exists, " +
		"`andurel doctor --json` reports the `Runtime config` check from its output and fails on a missing key, " +
		"malformed value, or unreachable database. A helper that does not build is a doctor failure, not a skip.\n\n" +
		"Format and verify:\n\n```text\ngofmt -w internal/configcheck\ngo fix ./...\ngo vet ./...\ngo run ./internal/configcheck\n```\n"
}

func runtimeReadinessInstructions() string {
	return "beta.4 separates liveness from readiness. `/api/health` stays liveness; `/api/ready` answers 200 " +
		"`{\"status\":\"ready\"}` only when the database answers and a required Inertia SSR sidecar is healthy, " +
		"otherwise 503 `{\"status\":\"unready\",\"reason\":\"...\"}`. `inertia.Renderer.Health` probes the sidecar " +
		"on every call, so readiness recovers without a restart.\n\n" +
		"Application-owned wiring to add:\n\n" +
		"1. In internal/runtime/runtime.go, keep the database health checker and compose the renderer readiness, " +
		"for example:\n\n```go\ntype readiness struct {\n\tdatabase storage.Connection\n\trenderer *inertia.Renderer\n}\n\n" +
		"func (r readiness) Health(ctx context.Context) error {\n" +
		"\tif err := r.database.Health(ctx); err != nil {\n" +
		"\t\treturn readinessError{reason: \"database_unavailable\"}\n\t}\n" +
		"\tif r.renderer == nil {\n\t\treturn nil\n\t}\n" +
		"\tif err := r.renderer.Health(ctx); err != nil {\n" +
		"\t\treason := \"unavailable\"\n\t\tvar ssrErr *inertia.ReadinessError\n" +
		"\t\tif errors.As(err, &ssrErr) && ssrErr.Status != \"\" {\n\t\t\treason = string(ssrErr.Status)\n\t\t}\n" +
		"\t\treturn readinessError{reason: reason}\n\t}\n\treturn nil\n}\n```\n\n" +
		"2. Register `handlers.NewAPIHandler(db, newReadiness(db, renderer)).Register(router)`; handlers/api.go " +
		"writes the 503 body itself so it never becomes the themed 500 page.\n\n" +
		"3. Add `var Ready = routing.NewSimpleRoute(\"GET\", \"/ready\", \"api.ready\")` next to Health in routes/api.go.\n\n" +
		"4. Do not exit the process when the sidecar is down: serve unready until it recovers. Keep the readiness " +
		"reason free of connection strings and passwords.\n\n" +
		"Format and verify:\n\n```text\ngofmt -w internal/runtime handlers routes\ngo fix ./...\ngo vet ./...\n```\n"
}

func inertiaSSRDefaultInstructions() string {
	return "beta.4 server-renders every production page by default. Production no longer needs a `.SSR()` call on " +
		"each page, and development keeps the per-page opt-in so a bare `go run ./cmd/app` does not require the Vite " +
		"SSR endpoint. This needs `pkg/inertia v0.13.0` (release gate).\n\n" +
		"1. In internal/runtime/runtime.go, add the renderer-wide default next to the other renderer options:\n\n```go\n" +
		"inertia.WithSSRDefault(cfg.App.IsProduction()),\n```\n\n" +
		"2. Remove per-page `.SSR()` calls that only existed to opt production pages in; keep `.SSR()` where a " +
		"development page needs SSR, and use `.CSR()` to opt a production page out explicitly.\n\n" +
		"3. Keep the themed error page: configure `FallbackComponent: \"Errors/InternalError\"` on " +
		"`inertia.NewErrorHandler` and leave error mappings client-rendered (the default) so a failed SSR render " +
		"never retries SSR for the page it caused. A production `/` request with the sidecar down must return the " +
		"themed 500, not an empty 200.\n\n" +
		"4. Do not set `INERTIA_SSR_FAIL_FAST=false` in production unless CSR fallback is the deliberate policy; " +
		"that mode is explicit and readiness stops gating on the sidecar.\n\n" +
		"Format and verify:\n\n```text\ngofmt -w internal/runtime handlers\ngo fix ./...\ngo vet ./...\n```\n"
}

func routesHostScopeInstructions() string {
	return "beta.4 regenerates `resources/js/routes.ts` so full-URL helpers throw before route hosts are " +
		"configured instead of silently returning a relative URL, and isolates SSR route hosts per render. Application " +
		"bootstrap code must adopt the new shape.\n\n" +
		"1. Run `andurel sync routes` and commit the regenerated `resources/js/routes.ts`. If it reports " +
		"a duplicate route name, rename the duplicate first: route names are application-owned. The alpha.4 " +
		"scaffold reused `users.user_password` for PasswordUpdate; the current scaffold names it " +
		"`users.update_user_password`.\n\n" +
		"2. In `resources/js/app` (browser entry), keep one boot configuration:\n\n```ts\n" +
		"import { configureRouteHosts } from './routes'\n" +
		"import { resolveHosts } from './hosts' // your existing shared-prop reader\n\n" +
		"configureRouteHosts(resolveHosts())\n" +
		"```\n\n" +
		"3. In the SSR entry, configure the scope once and wrap each render with the request's hosts:\n\n```ts\n" +
		"import { configureRouteHostScope, withRouteHosts } from './routes'\n\n" +
		"configureRouteHostScope()\n" +
		"// per request:\n" +
		"const html = await render(createElement(App, { ...pageProps, hosts: requestHosts }), {\n" +
		"  wrap: (element) => withRouteHosts(requestHosts, () => element),\n" +
		"})\n" +
		"```\n\n" +
		"4. Remove module-scope caches of full-URL helpers (`const href = routes.homePage()`). Call helpers at " +
		"render time, or use `routes.name.path()` for a relative URL. `andurel verify` reports module-scope calls, " +
		"including imports through the `@/*` tsconfig alias and immediately invoked functions.\n\n" +
		"Format and verify:\n\n```text\nandurel sync routes\npnpm run typecheck\nandurel verify\n```\n"
}

func corsScopeInstructions() string {
	var instructions strings.Builder
	instructions.WriteString(
		"The beta.3 scaffold stops trusting application host origins in global CORS. A sibling host must not be " +
			"able to read authenticated responses through a browser, so only CORS_ALLOWED_ORIGINS is trusted and " +
			"Vite build files answer with wildcard, uncredentialed CORS. This is security-relevant for apps with " +
			"more than one configured host.\n\n",
	)
	instructions.WriteString(
		"internal/runtime and handlers are application-owned, so update them by hand:\n\n" +
			"1. Replace the host-origin merge in the global policy:\n\n```go\n" +
			"cors := anduin.DefaultCORS(\n" +
			"\tanduin.MergeOrigins(cfg.App.Origins(), cfg.HTTP.CORSAllowedOrigins)...,\n" +
			")\n```\n\nwith\n\n```go\n" +
			"cors := anduin.DefaultCORS(cfg.HTTP.CORSAllowedOrigins...)\n" +
			"cors.Skipper = isViteAssetRequest\n```\n\n" +
			"Add the helpers next to New in internal/runtime/runtime.go. Import \"context\" and \"strings\" " +
			"if the file does not import them yet. In the alpha.4 corsConfig helper, replace the merged " +
			"AllowOrigins field with cfg.HTTP.CORSAllowedOrigins and add Skipper: isViteAssetRequest.\n\n```go\n" +
			"// viteAssetPrefix is the Vite build route subtree without its wildcard. The\n" +
			"// static files are public: they load from a secondary host without the\n" +
			"// visitor's session, so neither credentialed CORS nor a CSRF cookie belongs on\n" +
			"// them.\n" +
			"var viteAssetPrefix = strings.TrimSuffix(routes.ViteBuild.Pattern(), \"/*\")\n\n" +
			"func isViteAssetRequest(ctx context.Context) bool {\n" +
			"\trequest := anduin.Request(ctx)\n\n" +
			"\treturn request != nil && isViteAssetPath(request.URL.Path)\n" +
			"}\n\n" +
			"func isViteAssetPath(path string) bool {\n" +
			"\treturn strings.HasPrefix(path, viteAssetPrefix+\"/\")\n" +
			"}\n```\n\n" +
			"2. Register the asset route with its own uncredentialed CORS in handlers/assets.go. " +
			"Import \"net/http\" in that file for the method constants:\n\n```go\n" +
			"rt.Get(routes.ViteBuild.Pattern(), a.dist, anduin.CORS(anduin.CORSConfig{\n" +
			"\tAllowOrigins:     []string{\"*\"},\n" +
			"\tAllowMethods:     []string{http.MethodGet, http.MethodHead},\n" +
			"\tAllowCredentials: false,\n" +
			"}))\n```\n\n" +
			"For an alpha.4 Assets handler, use a.ViteBuild instead of a.dist. Alpha.4/alpha.5/beta.1 " +
			"scaffolds may register the route with Path() instead of Pattern(); either works for this wildcard route.\n\n" +
			"Keep cfg.App.Origins() in the CSRF trusted origins; same-site form posts between hosts are deliberate.\n\n",
	)
	instructions.WriteString(
		"Already fixed this yourself? Your version is equivalent when a credentialed cross-origin request " +
			"from a sibling application host gets no Access-Control-Allow-Origin header, only CORS_ALLOWED_ORIGINS " +
			"are echoed with credentials, and Vite build files answer without credentials. Keep it if it holds all " +
			"three; the scaffold code does not need to replace it.\n\n",
	)
	instructions.WriteString(
		"Format and verify:\n\n```text\ngofmt -w .\ngo fix ./...\ngo vet ./...\n```\n",
	)

	return instructions.String()
}

func csrfAssetSkipInstructions() string {
	return "The beta.3 scaffold keeps the CSRF cookie off safe requests for Vite build files. A static asset " +
		"request has no session to protect, and issuing a cookie there pollutes responses that should be cacheable.\n\n" +
		"Compose the existing bearer-token skip with an asset and safe-method check in internal/runtime/runtime.go:\n\n```go\n" +
		"bearerSkipper := anduin.SkipCSRFForBearer(routes.APIPrefix, cfg.Session.Name)\n" +
		"csrf.Skipper = func(ctx context.Context) bool {\n" +
		"\treturn bearerSkipper(ctx) || isSafeViteAssetRequest(ctx)\n" +
		"}\n```\n\n" +
		"Add the helper next to isViteAssetRequest from the CORS action. Import \"net/http\" for the method " +
		"constants if the file does not import it yet.\n\n```go\n" +
		"// isSafeViteAssetRequest limits the CSRF skip to safe methods. An unsafe request\n" +
		"// to a static path still carries an Origin and stays protected.\n" +
		"func isSafeViteAssetRequest(ctx context.Context) bool {\n" +
		"\trequest := anduin.Request(ctx)\n" +
		"\tif request == nil || !isViteAssetPath(request.URL.Path) {\n" +
		"\t\treturn false\n" +
		"\t}\n\n" +
		"\tswitch request.Method {\n" +
		"\tcase http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:\n" +
		"\t\treturn true\n" +
		"\tdefault:\n" +
		"\t\treturn false\n" +
		"\t}\n" +
		"}\n```\n\n" +
		"isViteAssetPath comes from the CORS action. Unsafe requests to static paths stay protected. " +
		"In the alpha.4 csrfSkipper helper, keep its existing bearer/session checks and add " +
		"if isSafeViteAssetRequest(ctx) { return true } at the start of the returned function.\n\n" +
		"Already fixed this yourself? Your version is equivalent when a GET or HEAD for a Vite build file " +
		"carries no Set-Cookie for the CSRF cookie and a POST to the same path is still rejected without a " +
		"valid token. Keep it if both hold.\n\n" +
		"Format and verify:\n\n```text\ngofmt -w internal/runtime\ngo fix ./...\ngo vet ./...\n```\n"
}

func methodNotAllowedInstructions() string {
	return "pkg/anduin beta.3 reports a ServeMux-generated 405 as anduin.ErrMethodNotAllowed and preserves the " +
		"Allow header. Map it to the themed not-found page in handlers/errors.go so a method mismatch is not a " +
		"plain-text response:\n\n```go\n" +
		"inertia.MapError(\n" +
		"\tanduin.ErrMethodNotAllowed,\n" +
		"\thttp.StatusMethodNotAllowed,\n" +
		"\t\"Errors/NotFound\",\n" +
		"),\n```\n\n" +
		"If the alpha.4 MapError is still a handwritten switch, add this case before its " +
		"not-found case instead:\n\n```go\n" +
		"case errors.Is(err, anduin.ErrMethodNotAllowed):\n" +
		"\treturn renderer.Page(ctx, \"Errors/NotFound\", inertia.Props{}).\n" +
		"\t\tStatus(http.StatusMethodNotAllowed).\n" +
		"\t\tRender()\n```\n\n" +
		"Already fixed this yourself? Your version is equivalent when a request with the wrong method on an " +
		"existing route renders the themed not-found page with status 405 and keeps the Allow header. Keep it " +
		"if both hold.\n\n" +
		"Then format and verify:\n\n```text\ngofmt -w handlers\ngo fix ./...\ngo vet ./...\n```\n"
}

func requestHostInstructions() string {
	return "pkg/anduin beta.3 exposes anduin.RequestHost(ctx): the routing.HostName the dispatcher resolved for " +
		"the request. Host matching is case-insensitive, aliases resolve to their HostName, and an unknown authority " +
		"reports routing.HostPrimary. Global middleware sees the value before dispatch.\n\n" +
		"Replace code that mirrors host matching by re-parsing Request.Host (for example middleware/host_authority.go) " +
		"with a check on anduin.RequestHost(ctx). For host-scoped security headers, add pkg/anduin middleware like " +
		"middleware.DenyFraming from the beta.3 scaffold and install it in internal/runtime:\n\n```go\n" +
		"router.Use(middleware.DenyFraming(config.HostAdmin))\n```\n\n" +
		"The headers are set before the handler runs, so error pages, 404s and 405s carry them too.\n\n" +
		"Already fixed this yourself? This action is a refactor, not a security fix. Host middleware that " +
		"matches hosts case-insensitively, resolves aliases and treats an unknown host as the primary host " +
		"behaves the same; you can keep it. anduin.RequestHost only removes the duplicate host parsing, so " +
		"the two cannot disagree when the host configuration changes.\n\n" +
		"Format and verify:\n\n```text\ngofmt -w .\ngo fix ./...\ngo vet ./...\n```\n"
}

func emailSenderInstructions() string {
	var instructions strings.Builder
	instructions.WriteString(
		"The default DEFAULT_SENDER_SIGNATURE was \"noreply@\"+app.Domain, and app.Domain is HOST_PRIMARY " +
			"with its port (noreply@localhost:8080). pkg/email rejects that address, so every queued auth email " +
			"was cancelled. config/email.go is application-owned, so fix it by hand:\n\n",
	)
	instructions.WriteString(
		"1. Derive the default from the host without its port:\n\n```go\n" +
			"DefaultSenderSignature: e.String(\"DEFAULT_SENDER_SIGNATURE\", \"noreply@\"+senderHost(app.Domain)),\n\n" +
			"// senderHost strips the port from the app domain. HOST_PRIMARY carries a port\n" +
			"// and an email address cannot contain one.\n" +
			"func senderHost(domain string) string {\n" +
			"\thost, _, err := net.SplitHostPort(domain)\n" +
			"\tif err == nil && host != \"\" {\n\t\treturn host\n\t}\n\n" +
			"\treturn domain\n}\n```\n\n",
	)
	instructions.WriteString(
		"2. Fail at startup on an invalid sender instead of cancelling every email job. " +
			"Replace the empty check in NewMail with:\n\n```go\n" +
			"sender := strings.TrimSpace(cfg.DefaultSenderSignature)\n" +
			"address, err := mail.ParseAddress(sender)\n" +
			"if err != nil || address.Name != \"\" || address.Address != sender {\n" +
			"\treturn Mail{}, fmt.Errorf(\"config: mail: DEFAULT_SENDER_SIGNATURE %q must be a bare email address\", sender)\n" +
			"}\n" +
			"cfg.DefaultSenderSignature = sender\n```\n\n" +
			"Import \"net\" and \"net/mail\". The sender is also the SMTP envelope sender, so a display name is rejected.\n\n",
	)
	instructions.WriteString("Then format and verify:\n\n```text\ngofmt -w config\ngo fix ./...\ngo vet ./...\n```\n")

	return instructions.String()
}

func viteSSRBundleInstructions() string {
	return "cmd/ssr extracts the embedded assets/dist/ssr/ssr.js to a temporary directory, where Node cannot " +
		"resolve react, @inertiajs/* or the other bare imports Vite leaves external. Bundle them into the " +
		"production SSR build only; setting noExternal for the dev server breaks SSR under `andurel run`.\n\n" +
		"In vite.config.ts, inside `defineConfig(({ command }) => ({ ... }))`, add:\n\n```ts\n" +
		"ssr: command === 'build' ? { noExternal: true } : undefined,\n```\n\n" +
		"Then rebuild and check that the bundle runs on its own:\n\n```text\nandurel build\n```\n"
}

func rootTitleInstructions() string {
	return "With SSR, the renderer writes the page's Inertia <Head> after the root template's static head, so a " +
		"page that sets its own title ended up with two <title> elements and browsers showed the first one. " +
		"In resources/root.templ, render the fallback only for client-rendered pages:\n\n```templ\n" +
		"if data.SSR == nil {\n\t<title>{ data.ProjectName }</title>\n}\n```\n\n" +
		"Then regenerate the root template and verify:\n\n```text\nandurel build\ngo vet ./...\n```\n"
}

func storageTestClusterInstructions() string {
	var instructions strings.Builder
	instructions.WriteString(
		"pkg/storage no longer imports testing or testcontainers. The test-cluster helpers moved to pkg/storage/storagetest with their methods unchanged:\n\n",
	)
	instructions.WriteString(
		"1. In your test files, import the new package and rename the helpers:\n\n```go\n" +
			"import \"github.com/mbvlabs/andurel/pkg/storage/storagetest\"\n\n" +
			"var testCluster *storagetest.Cluster\n\n" +
			"cluster, err := storagetest.NewCluster(context.Background())\n```\n\n",
	)
	instructions.WriteString(
		"2. There is no compatibility alias. Replace `storage.TestCluster` with `storagetest.Cluster` and " +
			"`storage.NewTestCluster` with `storagetest.NewCluster`; `Close` and `NewTestDB` keep their signatures.\n\n",
	)
	fmt.Fprintf(&instructions,
		"3. The upgrade pins pkg/storage %s. Format and verify:\n\n```text\n"+
			"gofmt -w .\ngo fix ./...\ngo vet ./...\ngolangci-lint run ./...\n```\n",
		versions.Storage,
	)

	return instructions.String()
}

func packageAPIInstructions(inertia bool) string {
	var instructions strings.Builder
	fmt.Fprintf(&instructions,
		"The upgrade pins pkg/anduin %s, pkg/cache %s, pkg/inertia %s, pkg/kiks %s and pkg/storage %s. "+
			"internal/runtime is application-owned, so update it by hand:\n\n",
		versions.Anduin,
		versions.Cache,
		versions.Inertia,
		versions.Kiks,
		versions.Storage,
	)
	instructions.WriteString(
		"1. anduin.NewServer drops the environment and shutdowner arguments, and Server.Start takes none:\n\n```go\n" +
			"srv := anduin.NewServer(ctx, cfg.HTTP.Host, cfg.HTTP.Port, handler, anduin.WithTimeouts(/* ... */))\n" +
			"err := srv.Start()\n```\n\n",
	)
	instructions.WriteString(
		"2. cache.Store has Close. Close the store on shutdown, after the server stops:\n\n```go\n" +
			"closers = append(closers, func(context.Context) error { return cacheStore.Close() })\n```\n\n",
	)
	instructions.WriteString(
		"3. storage.RunInTransaction is gone. Use storage.AtomicOps with a storage.Transactor.\n\n",
	)
	instructions.WriteString(
		"River v0.40 adds main-line migration 7 (river_notification, drops river_client). " +
			"New projects ship it; existing projects add it with `andurel generate migration add_river_notification_drop_river_client` " +
			"and copy the up and down SQL from River's riverdriver/riverpgxv5/migration/main/007_*.sql.\n\n",
	)
	if inertia {
		instructions.WriteString(
			"4. In internal/runtime, inertia.NewRenderer takes inertia.RendererConfig; in cmd/ssr/main.go, inertia.NewSSRRuntime takes inertia.SSRRuntimeConfig. Both replace positional arguments:\n\n```go\n" +
				"inertia.NewRenderer(inertia.RendererConfig{\n\tContainerID:  cfg.Inertia.ContainerID,\n\tBuildPathURL: routes.ViteBuild.FullURL(),\n\t// ...\n})\n\n" +
				"inertia.NewSSRRuntime(inertia.SSRRuntimeConfig{\n\tExecutable:     cfg.SSRRuntime,\n\tBundlePath:     cfg.SSRBundle,\n\tListenURL:      cfg.SSRListen,\n\tStartupTimeout: cfg.SSRStartupTimeout,\n\tMinimumMajor:   cfg.SSRMinimumMajor,\n\tHealthTimeout:  cfg.SSRRequestTimeout,\n\tBundleFS:       assets.Files,\n})\n```\n\n",
		)
		instructions.WriteString(
			"5. pnpm 10+ blocks esbuild's install script, so `pnpm install` fails. Add pnpm-workspace.yaml:\n\n```yaml\n" +
				"allowBuilds:\n  esbuild: true\nonlyBuiltDependencies:\n  - esbuild\n```\n\n",
		)
		instructions.WriteString(
			"6. Svelte projects: @inertiajs/svelte v3 `useForm` returns a runes object, not a store. " +
				"In resources/js/Pages/Auth/*.svelte replace `$form.` with `form.`.\n\n",
		)
	}
	instructions.WriteString("Then format and verify:\n\n```text\ngofmt -w internal/runtime\ngo fix ./...\ngo vet ./...\n```\n")

	return instructions.String()
}

func routingHostsInstructions() string {
	var instructions strings.Builder
	fmt.Fprintf(&instructions,
		"pkg/routing %s replaces the package-level host registry with an explicit *routing.Hosts. "+
			"routing.ConfigureHosts, routing.LookupHost, routing.Hosts and routing.HostBaseURL are gone, "+
			"and every FullURL now takes the registry as its first argument:\n\n",
		versions.Routing,
	)
	instructions.WriteString(
		"1. Delete the ConfigureHosts call from NewApp in config/app.go. The runtime builds and validates the registry instead:\n\n```go\n" +
			"// remove\nif err := routing.ConfigureHosts(cfg.Hosts); err != nil {\n\treturn App{}, fmt.Errorf(\"config: app: %w\", err)\n}\n```\n\n",
	)
	instructions.WriteString(
		"2. Build the registry in internal/runtime/runtime.go and pass it to the router:\n\n```go\n" +
			"hosts, err := routing.NewHosts(cfg.App.Hosts)\n" +
			"if err != nil {\n\treturn nil, fmt.Errorf(\"runtime: configure hosts: %w\", err)\n}\n\n" +
			"router := anduin.NewRouter(\n\tanduin.WithHosts(hosts),\n\tanduin.WithErrorHandler(handlers.MapError(renderer)),\n\t// ...\n)\n```\n\n",
	)
	instructions.WriteString(
		"3. Do the same in NewQueue in internal/runtime/queue.go before it builds services.NewIdentity.\n\n",
	)
	instructions.WriteString(
		"4. Thread the registry through the identity service and every FullURL call:\n\n```go\n" +
			"// services/identity.go\n" +
			"type Identity struct {\n\t// ...\n\thosts *routing.Hosts\n}\n\n" +
			"func NewIdentity(\n\ttransactor storage.Transactor,\n\tusers domains.Users,\n\ttokens domains.Tokens,\n\tqueue storage.InsertQueue,\n\tauthCfg config.Auth,\n\tmailCfg config.Mail,\n\thosts *routing.Hosts,\n) Identity {\n\treturn Identity{\n\t\t// ...\n\t\thosts: hosts,\n\t}\n}\n\n" +
			"// services/reset_password.go\nrpEmail := appemail.ResetPassword{ResetURL: routes.PasswordEdit.FullURL(i.hosts, token)}\n\n" +
			"// internal/runtime/runtime.go\nrenderer, err := newRenderer(cfg, hosts)\n\n" +
			"func newRenderer(cfg config.Config, hosts *routing.Hosts) (*inertia.Renderer, error) {\n" +
			"\treturn inertia.NewRenderer(\n\t\tinertia.RendererConfig{\n" +
			"\t\t\tContainerID:  cfg.Inertia.ContainerID,\n" +
			"\t\t\tBuildPathURL: routes.ViteBuild.FullURL(hosts),\n" +
			"\t\t\t// ...\n\t\t},\n\t\t// ...\n\t)\n}\n```\n\n",
	)
	instructions.WriteString(
		"5. Update every other FullURL call to put the registry first, for example " +
			"`routes.PasswordEdit.FullURL(token)` becomes `routes.PasswordEdit.FullURL(hosts, token)` " +
			"and `Route.FullURL(id)` becomes `Route.FullURL(hosts, id)`. Replace " +
			"`routing.LookupHost(name)` with `hosts.Lookup(name)`, `routing.Hosts()` with `hosts.All()` " +
			"and `routing.HostBaseURL(name)` with `hosts.BaseURL(name)`.\n\n",
	)
	instructions.WriteString(
		"Run `andurel patterns show composition` for the full runtime wiring, then format and verify:\n\n```text\n" +
			"gofmt -w .\ngo fix ./...\ngo vet ./...\n```\n",
	)

	return instructions.String()
}

func sessionRevocationInstructions() string {
	var instructions strings.Builder
	fmt.Fprintf(&instructions,
		"pkg/auth %s and the alpha.5 scaffold make sessions revocable: a per-user session_version is stored "+
			"in the cookie and checked against the database on every authenticated request. Password resets bump "+
			"the version, so every other session stops working immediately.\n\n",
		versions.Auth,
	)
	instructions.WriteString(
		"This is a recommended security fix, not a compile requirement: an alpha.4 app keeps building without it.\n\n",
	)
	instructions.WriteString(
		"1. Add a migration:\n\n```text\nandurel generate migration add_session_version_to_users\n```\n\n" +
			"Use this SQL:\n\n```sql\n-- +goose Up\n-- +goose StatementBegin\n" +
			"ALTER TABLE users ADD COLUMN session_version INTEGER NOT NULL DEFAULT 1;\n" +
			"-- +goose StatementEnd\n\n-- +goose Down\n-- +goose StatementBegin\n" +
			"ALTER TABLE users DROP COLUMN session_version;\n-- +goose StatementEnd\n```\n\n",
	)
	instructions.WriteString(
		"2. Update domains/user.sql: include session_version in the CreateUser INSERT and UpdateUser SET, then add:\n\n```sql\n" +
			"-- name: BumpSessionVersion :exec\nUPDATE users\nSET session_version = session_version + 1\nWHERE id = $1;\n```\n\n" +
			"Run `andurel sync queries` to regenerate domains/internal/queries. Run `andurel patterns show domain` " +
			"to see the full users domain with SessionVersion in the User struct and its Create, Save and " +
			"ChangePassword parameter structs.\n\n",
	)
	instructions.WriteString(
		"3. Add the domain method and revoke on password change:\n\n```go\n" +
			"func (u Users) BumpSessionVersion(ctx context.Context, id uuid.UUID) error {\n" +
			"\treturn u.queries.BumpSessionVersion(ctx, id)\n}\n\n" +
			"// In ChangePassword, after the password write succeeds (Save in the alpha.5 domain, Update\n" +
			"// in an alpha.4 domain):\n" +
			"if err := users.BumpSessionVersion(ctx, id); err != nil {\n\treturn err\n}\n```\n\n",
	)
	instructions.WriteString(
		"4. Update the session cookie fields: keep UserID and PendingUserID, replace IsAdmin with SessionVersion, " +
			"and keep IsAuthenticated:\n\n```go\ntype Session struct {\n" +
			"\tUserID          string `json:\"user_id,omitempty\"`\n" +
			"\tPendingUserID   string `json:\"pending_user_id,omitempty\"`\n" +
			"\tSessionVersion  int32  `json:\"session_version,omitempty\"`\n" +
			"\tIsAuthenticated bool   `json:\"is_authenticated,omitempty\"`\n}\n```\n\n" +
			"If your alpha.4 cookie is still named App, keep the name or rename it to Session; the fields are what matter. " +
			"In handlers/auth.go set `SessionVersion: entity.SessionVersion` instead of `IsAdmin: entity.IsAdmin` " +
			"wherever the session cookie is created.\n\n",
	)
	instructions.WriteString(
		"5. Replace AuthOnly with the database-backed version and pass the users repository at the call site:\n\n```go\n" +
			"func AuthOnly(users domains.Users) anduin.Middleware {\n" +
			"\treturn func(next anduin.Handler) anduin.Handler {\n" +
			"\t\treturn func(ctx context.Context) error {\n" +
			"\t\t\tsession, err := kiks.Get[*Session](ctx)\n" +
			"\t\t\tif err != nil {\n\t\t\t\treturn err\n\t\t\t}\n" +
			"\t\t\tif session == nil || !session.IsAuthenticated {\n\t\t\t\treturn rejectSession(ctx)\n\t\t\t}\n\n" +
			"\t\t\tuserID, err := uuid.Parse(session.UserID)\n" +
			"\t\t\tif err != nil {\n\t\t\t\treturn rejectSession(ctx)\n\t\t\t}\n\n" +
			"\t\t\tuser, err := users.Find(ctx, userID)\n" +
			"\t\t\tif err != nil {\n\t\t\t\tif errors.Is(err, domains.ErrNotFound) {\n\t\t\t\t\treturn rejectSession(ctx)\n\t\t\t\t}\n\n" +
			"\t\t\t\treturn err\n\t\t\t}\n\n" +
			"\t\t\tif user.SessionVersion != session.SessionVersion {\n\t\t\t\treturn rejectSession(ctx)\n\t\t\t}\n\n" +
			"\t\t\treturn next(WithViewer(ctx, user))\n\t\t}\n\t}\n}\n```\n\n" +
			"Then register protected pages with `middleware.AuthOnly(users)`; `andurel patterns show composition` " +
			"has the matching WithViewer, Viewer and rejectSession helpers.\n\n",
	)
	instructions.WriteString(
		"Format and verify:\n\n```text\ngofmt -w .\ngo fix ./...\ngo vet ./...\n```\n",
	)

	return instructions.String()
}

func emailSMTPInstructions() string {
	var instructions strings.Builder
	fmt.Fprintf(&instructions,
		"pkg/email %s adds a production SMTP transport and the alpha.5 scaffold rejects Mailpit when "+
			"ENVIRONMENT=production. `newEmailSenders` also takes config.App so it can make that check.\n\n",
		versions.Email,
	)
	instructions.WriteString(
		"This action is optional for local development, but production apps still on Mailpit must switch to SMTP.\n\n",
	)
	instructions.WriteString(
		"1. Add the SMTP fields to config.Mail and read them in NewMail in config/email.go:\n\n```go\n" +
			"const (\n\tMailpitDriver MailDriver = \"mailpit\"\n\tSMTPDriver    MailDriver = \"smtp\"\n)\n\n" +
			"type Mail struct {\n\t// ...\n\tSMTPHost     string\n\tSMTPPort     string\n\tSMTPUsername string\n\tSMTPPassword string\n\tSMTPTLS      string\n}\n```\n\n",
	)
	instructions.WriteString(
		"2. Change newEmailSenders to take config.App and add the SMTP case:\n\n```go\n" +
			"func newEmailSenders(\n\tcfg config.Mail,\n\tapp config.App,\n) (email.TransactionalSender, email.MarketingSender, error) {\n" +
			"\tif err := validateEmailConfig(cfg, app); err != nil {\n\t\treturn nil, nil, err\n\t}\n\n" +
			"\tswitch cfg.Driver {\n\tcase config.MailpitDriver:\n\t\t// ...\n" +
			"\tcase config.SMTPDriver:\n" +
			"\t\tclient, err := email.NewSMTP(email.SMTPConfig{\n" +
			"\t\t\tHost:     cfg.SMTPHost,\n\t\t\tPort:     cfg.SMTPPort,\n" +
			"\t\t\tUsername: cfg.SMTPUsername,\n\t\t\tPassword: cfg.SMTPPassword,\n" +
			"\t\t\tTLS:      email.SMTPTLSMode(cfg.SMTPTLS),\n\t\t})\n" +
			"\t\tif err != nil {\n\t\t\treturn nil, nil, fmt.Errorf(\"create SMTP sender: %w\", err)\n\t\t}\n\n" +
			"\t\treturn client, client, nil\n\tdefault:\n" +
			"\t\treturn nil, nil, fmt.Errorf(\"unsupported email provider %q\", cfg.Driver)\n\t}\n}\n```\n\n",
	)
	instructions.WriteString(
		"3. Pass cfg.App at the call site in internal/runtime/queue.go (the function stays in internal/runtime/runtime.go):\n\n```go\n" +
			"transactional, marketing, err := newEmailSenders(cfg.Mail, cfg.App)\n```\n\n" +
			"Then set EMAIL_PROVIDER=smtp and SMTP_HOST, SMTP_PORT, SMTP_USERNAME, SMTP_PASSWORD and SMTP_TLS " +
			"(starttls, tls, or none). `andurel patterns show composition` has the complete sender setup.\n\n",
	)
	instructions.WriteString("Format and verify:\n\n```text\ngofmt -w .\ngo fix ./...\ngo vet ./...\n```\n")

	return instructions.String()
}

func validationInstructions() string {
	return "pkg/validation " + versions.Validation + " removes the deprecated validation.Validate function and " +
		"the Validatable interface, which the alpha.4 scaffold still calls in domains/user.go and domains/token.go.\n\n" +
		"Replace each call with the value's own Validate method:\n\n```go\n" +
		"// before\nif err := validation.Validate(&entity); err != nil {\n\treturn err\n}\n\n" +
		"// after\nif err := entity.Validate(); err != nil {\n\treturn err\n}\n```\n\n" +
		"Generated domains already expose Validate; `andurel patterns show domain` shows the current shape. " +
		"Then format and verify:\n\n```text\ngofmt -w .\ngo fix ./...\ngo vet ./...\n```\n"
}

func crossesVersion(fromVersion, toVersion, boundary string) bool {
	from, fromOK := canonicalUpgradeVersion(fromVersion)
	to, toOK := canonicalUpgradeVersion(toVersion)
	boundary, boundaryOK := canonicalUpgradeVersion(boundary)
	return fromOK && toOK && boundaryOK &&
		semver.Compare(from, boundary) < 0 &&
		semver.Compare(to, boundary) >= 0
}

func canonicalUpgradeVersion(version string) (string, bool) {
	version = strings.TrimSpace(version)
	if version == "" {
		return "", false
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	version = semver.Canonical(version)
	return version, version != ""
}
