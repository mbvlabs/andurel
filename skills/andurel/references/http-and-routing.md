# HTTP And Routing

Andurel apps speak `net/http`. The single HTTP package is
`github.com/mbvlabs/andurel/pkg/anduin` — a thin layer over `net/http` and
`http.ServeMux` (Go 1.22+ method and wildcard patterns). There is no Echo, no
Echo middleware, and no `pkg/hypermedia`. Request and response access travels on
`context.Context`.

Read this together with [layer-placement.md](layer-placement.md) for where HTTP
code lives (`handlers/`, `routes/`, `middleware/`, `internal/runtime/`).

## The handler model

A handler is ordinary Go over a context:

```go
type Handler func(ctx context.Context) error
type Middleware func(Handler) Handler
type ErrorHandler func(ctx context.Context, err error) error
```

A handler returns `nil` on success or an error. Returning a non-nil error routes
to the router's `ErrorHandler`. Only transport data — the `*http.Request`, the
`http.ResponseWriter`, and path wildcards — lives on the context. Dependencies
and database handles never do.

## Router

`Router` owns registration and dispatch. It keeps one `http.ServeMux` per
virtual host and is not itself a listener (see Server).

```go
func NewRouter(opts ...Option) *Router
func WithErrorHandler(h ErrorHandler) Option
func WithTelemetry(tel *telemetry.Telemetry) Option

func (rt *Router) Get(pattern string, h Handler, mw ...Middleware)
func (rt *Router) Post(pattern string, h Handler, mw ...Middleware)
func (rt *Router) Put(pattern string, h Handler, mw ...Middleware)
func (rt *Router) Patch(pattern string, h Handler, mw ...Middleware)
func (rt *Router) Delete(pattern string, h Handler, mw ...Middleware)

func (rt *Router) Host(name routing.HostName) *Host
func (rt *Router) Use(mw ...Middleware)
func (rt *Router) Handler() http.Handler
```

- Per-route middleware is passed after the handler; global middleware is
  appended with `Use`.
- `Handler()` returns an `http.Handler` for `anduin.Server` or any `net/http`
  consumer, applying the middleware chain, dispatching on `Host`, calling the
  matched `Handler`, and routing returned errors to the `ErrorHandler`. The
  result is cached: call `Use` and register routes before the first call.
- `MuxPattern(pattern)` converts an andurel/routing path to an `http.ServeMux`
  pattern: `":id"` → `"{id}"`, a trailing `"*"` → `"{path...}"`, existing
  `"{...}"` wildcards untouched.

Registration:

```go
func (h Handler) Register(rt *anduin.Router) {
	rt.Get(routes.UserIndex.Pattern(), h.Index)
	rt.Get(routes.UserShow.Pattern(), h.Show)
	rt.Post(routes.UserCreate.Pattern(), h.Create, anduin.RateLimit(h.cache, 5, 10*time.Minute, nil))
}
```

`anduin.Serve(w, r, handler)` is the low-level entry point behind
`Router.Handler`, useful when embedding an `anduin.Handler` in another
`http.Handler` and in tests.

### Exact routes, precedence, and HEAD

Register routes with `Pattern()`, not `Path()`. `Path()` returns the URL used
in links and redirects (`"/"`); `Pattern()` returns the value to register with
the router. They differ only when the route is declared with `routing.Exact()`:
the URL path stays `"/"` and the pattern becomes `"/{$}"`, because an
`http.ServeMux` pattern ending in `/` matches every path with that prefix.

```go
var HomePage = routing.NewSimpleRoute(
	"/",
	"pages.home",
	"",
	routing.Exact(),
	routing.TypeScriptRoute(),
)

rt.Get(routes.HomePage.Pattern(), h.Home) // registers GET /{$}
```

Pattern precedence follows `http.ServeMux`: the most specific pattern wins.
Exact paths (`/about`) beat wildcards (`/posts/{id}`), which beat a trailing
catch-all (`/{path...}`); a method-specific pattern beats a method-less one.
Prefer `routing.Exact()` over a hand-written catch-all. Requests that match no
route are passed to the router's `ErrorHandler` as `anduin.ErrNotFound`, so a
scaffolded app maps it once:

```go
inertia.MapError(anduin.ErrNotFound, http.StatusNotFound, "Errors/NotFound")
```

`Get` registers `GET`, and `http.ServeMux` serves `HEAD` with the `GET` handler
(Go treats a `GET` pattern as matching both methods). Do not register a
separate `HEAD` route. A `POST` to a path that only has `GET` still returns
`405 Method Not Allowed`, not the not-found page.

## Host dispatch

`Router.Host(name)` returns a `*Host`: a per-host `ServeMux` with the same
`Get/Post/Put/Patch/Delete` registration surface. Routes registered on a host
are served only when the request `Host` matches that hostname (or one of its
aliases).

```go
rt.Host(config.HostAdmin).Get(routes.AdminDashboard.Pattern(), h.Dashboard)
```

Build the hostname registry at boot with `routing.NewHosts`, using
`routing.HostName` and `routing.HostPrimary`, then pass it to `anduin.NewRouter`
with `anduin.WithHosts`. `routing.Host(name)` binds a route value to a host, and
route `FullURL(hosts, ...)` calls take the same value. Unknown `Host` values are
served by the primary mux. A hostname without a port is also aliased as
`hostname:80` and `hostname:443`.

## Request access

```go
func Request(ctx context.Context) *http.Request
func ResponseWriter(ctx context.Context) http.ResponseWriter

func PathValue(ctx context.Context, key string) string
func SetPathValue(ctx context.Context, key, value string)
func RoutePattern(ctx context.Context) string

func Query(ctx context.Context, key string) string
func QueryValues(ctx context.Context, key string) []string
func QueryInt(ctx context.Context, key string, def int) (int, error)
func QueryBool(ctx context.Context, key string, def bool) (bool, error)
func FormValue(ctx context.Context, key string) string
func Header(ctx context.Context, key string) string
func Cookie(ctx context.Context, name string) (*http.Cookie, error)

func DecodeJSON(ctx context.Context, dst any) error
func DecodeForm(ctx context.Context, dst any) error
```

- `QueryInt` / `QueryBool` return the default when the parameter is absent and
  a `400` `*HTTPError` when it is present but malformed.
- `DecodeJSON` rejects a body that is not JSON and trailing data; `DecodeForm`
  binds `application/x-www-form-urlencoded` or multipart bodies using `form`
  struct tags (falling back to the field name).
- `RoutePattern` returns the matched `http.ServeMux` pattern (the analogue of
  the old `c.RouteInfo()`), used by telemetry and logging. `RoutePath(pattern)`
  strips the method prefix (`"GET /users/{id}"` → `"/users/{id}"`).

## Responses

```go
func JSON(ctx context.Context, status int, v any) error
func JSONBlob(ctx context.Context, status int, blob []byte) error
func Text(ctx context.Context, status int, value string) error
func HTML(ctx context.Context, status int, value string) error
func HTMLBlob(ctx context.Context, status int, blob []byte) error
func Blob(ctx context.Context, status int, contentType string, body []byte) error
func NoContent(ctx context.Context, status int) error
func Redirect(ctx context.Context, location string, status int) error
func ServeContent(ctx context.Context, name string, body []byte) error

func SetHeader(ctx context.Context, key, value string)
func AddHeader(ctx context.Context, key, value string)
```

`Redirect` requires a `3xx` status. Set response headers with `SetHeader` before
writing a body. `ServeContent` is the ETag / conditional-GET helper `StaticFS`
uses: it sets the content type from `name`'s extension, an `ETag` derived from
`body`, and answers a matching `If-None-Match` with `304`. Use it for generated
documents such as sitemaps and feeds.

## Error model

```go
type HTTPError struct {
	Status  int
	Message string
	Err     error
}
func (e *HTTPError) Error() string
func (e *HTTPError) Unwrap() error

func Error(ctx context.Context, status int, err error) error
func Errorf(status int, format string, args ...any) error
func NotFound(err error) error
func BadRequest(err error) error
func Unauthorized(err error) error
func Forbidden(err error) error
func Conflict(err error) error

func StatusCode(err error) int   // unclassified -> 500
func Message(err error) string   // falls back to status text

var ErrNotFound error            // unmatched request sentinel
```

Handlers return domain errors unchanged and let the router map them. The
`ErrorHandler` is installed once in `internal/runtime`:

```go
rt := anduin.NewRouter(
	anduin.WithErrorHandler(handlers.MapError(renderer)),
)
```

`handlers.MapError(*inertia.Renderer) anduin.ErrorHandler` translates the shared
domain vocabulary (`domains.ErrNotFound`, `domains.ErrConflict`), the router's
unmatched-request sentinel (`anduin.ErrNotFound`), and `pkg/validation` errors
(`validation.As(err)`) into HTTP responses, validation props, and statuses,
falling back to `anduin.Error(ctx, http.StatusInternalServerError, err)`.
Without an installed handler, the default writes the status text as plain text.

## Middleware

Generic middleware ships in `pkg/anduin`:

```go
func Recover() Middleware
func RealIP(trusted ...netip.Prefix) Middleware
func Logger(log *slog.Logger, skipPrefixes ...string) Middleware
func Telemetry(tel *telemetry.Telemetry) Middleware
func CORS(cfg CORSConfig) Middleware
func CSRF(cfg CSRFConfig) Middleware
```

- `Recover` turns a panic into a `500` so the `ErrorHandler` can render it.
- `RealIP` resolves `anduin.ClientIP(ctx)`. `X-Forwarded-For` is read only
  when the TCP peer is in `trusted` (`anduin.ParseTrustedProxies`, fed from
  `HTTP_TRUSTED_PROXIES`); otherwise `ClientIP` is the peer address. Never
  read `RemoteAddr` or forwarding headers directly for client identity.
- `Logger` logs one line per request, skipping the given path prefixes.
- `Telemetry` attaches the telemetry pipeline and records the matched route.
- `CORS` answers preflight and annotates actual responses. `CORSConfig` fields:
  `AllowOrigins`, `AllowMethods`, `AllowHeaders`, `ExposeHeaders`,
  `AllowCredentials`, `MaxAge`, `Skipper func(ctx) bool`. Wildcard origins are
  allowed only without credentials.
- `CSRF` issues and verifies double-submit tokens. `CSRFConfig` fields:
  `TokenLookup` (`"header:X-CSRF-Token"` by default;
  `"header:X-CSRF-Token,form:_csrf"` with `header_or_legacy_token`),
  `CookieName`,
  `CookiePath`, `CookieDomain`, `CookieSecure`, `CookieHTTPOnly`,
  `CookieSameSite`, `TrustedOrigins`, `TokenLength`, `Skipper`. The current
  token is available to handlers via `anduin.CSRFToken(ctx)`.

App middleware (auth and session definitions) lives in `middleware/` and uses
the same `anduin.Middleware` type. `middleware.AuthOnly(users)` reloads the
session's user from the database on every request, rejects a missing user or a
stale `SessionVersion` (clearing the cookie and redirecting to sign-in), and
attaches the database-backed user for `middleware.ViewerFrom(ctx)`, so admin
checks never trust a value in the cookie. `pkg/inertia`'s
`(*Renderer).Middleware()` also returns an `anduin.Middleware`.

Install global middleware in `internal/runtime`:

```go
cors := anduin.DefaultCORS(
	anduin.MergeOrigins(cfg.App.Origins(), cfg.HTTP.CORSAllowedOrigins)...,
)
csrf := anduin.DefaultCSRF(
	cfg.HTTP.CSRFStrategy,
	cfg.App.IsProduction(),
	anduin.MergeOrigins(cfg.App.Origins(), cfg.HTTP.CSRFTrustedOrigins)...,
)
csrf.Skipper = anduin.SkipCSRFForBearer(routes.APIPrefix, cfg.Session.Name)

rt.Use(
	anduin.Recover(),
	anduin.RealIP(cfg.HTTP.TrustedProxies...),
	anduin.Telemetry(tel),
	jar.Middleware(),
	renderer.Middleware(),
	anduin.CORS(cors),
	anduin.CSRF(csrf),
)
```

Responses are gzipped by `anduin.Compress`, an `http.Handler` wrapper installed
outside `rt.Handler()` in `internal/runtime` behind `HTTP_COMPRESS` (default
on). It covers `ErrorHandler` pages because it wraps the whole router, skips
`HEAD`, upgrades and event streams, already-encoded and non-compressible
responses, and bodies under 1 KiB, and sets `Vary: Accept-Encoding`.

### CSRF: origin trust vs the token

`anduin.CSRF` is a double-submit token middleware with an origin allow-list.
Safe methods (`GET`, `HEAD`, `OPTIONS`, `TRACE`) skip verification. An unsafe
method passes in either of two ways:

1. **Trusted origin.** `Origin` (or `Referer` when `Origin` is absent) exactly
   matches one of `CSRFConfig.TrustedOrigins`. The scaffold trusts every
   configured app origin (`cfg.App.Origins()`) plus `CSRF_TRUSTED_ORIGINS`, so
   a same-origin browser form or `fetch` passes without a token. Browsers set
   `Origin` themselves and a cross-site page cannot forge it; never add an
   origin you do not control, and do not add `*`.
2. **Double-submit token.** The value of the `_csrf` cookie is echoed in the
   `X-CSRF-Token` header (`CSRF_STRATEGY=header_only`, the default) or in the
   `_csrf` form field (`header_or_legacy_token`). The middleware writes the
   `_csrf` cookie on every request; it is `HttpOnly`, so a handler that needs a
   JavaScript client to send the header exposes `anduin.CSRFToken(ctx)` as a
   prop. A missing or mismatched token returns `403 CSRF token verification
   failed`.

`anduin.SkipCSRFForBearer(routes.APIPrefix, cfg.Session.Name)` skips CSRF only
for `/api` requests that carry a non-empty Bearer token **and** no session
cookie; a cookie-authenticated request on the same path is still checked.

How to curl an unsafe request:

```bash
# 1. Same origin as the app: the configured origins are trusted.
curl -i -X POST http://localhost:8080/users/sign-in \
  -H 'Content-Type: application/json' \
  -H 'Origin: http://localhost:8080' \
  -d '{"email":"you@example.com","password":"your password"}'

# 2. Double-submit token, no Origin header.
curl -sS -c /tmp/andurel-cookies http://localhost:8080/users/sign-in -o /dev/null
token=$(awk '$6 == "_csrf" { print $7 }' /tmp/andurel-cookies)
curl -i -b /tmp/andurel-cookies -X POST http://localhost:8080/users/sign-in \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $token" \
  -d '{"email":"you@example.com","password":"your password"}'
```

The trusted origin in the example must match `PROTOCOL` + `HOST_PRIMARY`
(including the port) or an entry in `CSRF_TRUSTED_ORIGINS`; otherwise the first
request has no token and gets `403`.

## Server lifecycle

`Server` owns the HTTP listener and nothing else. The router stays
independently testable; only the process pays for a `Server`.

```go
type ServerOptions struct {
	IdleTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}
type ServerOption func(*ServerOptions)
type Shutdowner interface { Shutdown(ctx context.Context) error }

func DefaultConfig() ServerOptions
func (config ServerOptions) Validate() error
func WithTimeouts(idle, read, write time.Duration) ServerOption

func NewServer(
	ctx context.Context,
	host, port string,
	handler http.Handler,
	opts ...ServerOption,
) Server
func (s *Server) Addr() string
func (s *Server) Start() error
func (s *Server) Shutdown(ctx context.Context) error
```

`Shutdown` stops only the HTTP listener; `runtime.App.Close` releases the
other dependencies afterwards. `cmd/app` gets its server from `runtime.New`, which
builds the router and wraps it with `anduin.NewServer(...)` internally, then
calls `lifecycle.Run(ctx, start, app.Shutdown, lifecycle.WithBlockingStart())`
and defers `closeApp` for `Close`; lifecycle ordering is explicit code, not an
Fx hook. Without `WithBlockingStart`, startup must finish before shutdown starts.

## Inertia

`pkg/inertia` obtains request/response through `anduin` and takes a
`context.Context`:

```go
func (r *Renderer) Page(ctx context.Context, component string, props Props) *PageBuilder
func (r *Renderer) Redirect(ctx context.Context, location string, status int) error
func (r *Renderer) Location(ctx context.Context, location string) error
func (r *Renderer) FreshRedirect(ctx context.Context, location string) error
func (r *Renderer) Middleware() anduin.Middleware
```

A handler renders a page with:

```go
return h.renderer.Page(ctx, "User/Index", inertia.Props{"users": result.Items}).Render()
```

`Page` returns a `*PageBuilder`; `SSR()` opts that render into server-side
rendering, `Status` sets the HTTP status, and `Errors`/`ValidationErrors` set
the protected `errors` prop. Without `X-Inertia`, the response is the full HTML
document; an Inertia visit gets the same page as JSON.

Client rendering is the default. `.SSR()` opts a single render into
server-side rendering:

```go
return h.renderer.Page(ctx, "User/Show", inertia.FromStruct(user)).SSR().Render()
```

SSR is per `Render()` call, not per route or renderer, so every re-render
branch needs its own `.SSR()` to stay server-rendered: validation errors
(`Page(...).Errors(err)`), rate-limit responses, and error pages. In
development the renderer posts to the Vite dev server's `/__inertia_ssr`
endpoint (`andurel run` / `pnpm dev` must be running); in production it posts
to `INERTIA_SSR_URL`, served by `cmd/ssr` from `assets/dist/ssr/ssr.js`.
`INERTIA_SSR_FAIL_FAST` (default true in production) turns an SSR failure into a
500 instead of a client-rendered fallback.

The root document is `resources/root.templ` (package `resources`). `RootData`
exposes:

- `Page` (`inertia.Page`) and `PageJSON` (the encoded page object)
- `ContainerID`, `ProjectName`, `Environment`
- `ViteHead` / `ViteBody` (rendered asset tags)
- `SSR` (`*inertia.SSRResponse`, nil for a client-rendered response)

Render the fallback `<title>` only when `data.SSR == nil`: SSR pages get their
head from the page's Inertia `<Head>`, and a static title would otherwise be a
duplicate. The scaffold shares `appVersion`, `siteName`, `hosts`, `auth`
(`isAuthenticated`), and `errors` with every page (`errors` is protected).

Map field errors with `Page(...).Errors(err)`, which sets the validation errors
prop and reports whether the error carried a field map. `inertia.ErrorHandler`
does the same for errors returned from handlers; configure it with
`inertia.MapError` for domain sentinels. Error pages are client-rendered like
every page: set `ErrorMapping.SSR` on an individual mapping, and use
`inertia.NewErrorHandler` with `ErrorHandlerOptions` (`ValidationComponent`,
`ValidationSSR`, `FallbackComponent`, `FallbackSSR`) for the built-in validation
and fallback pages. The fallback page renders unmapped 5xx errors (including a
fail-fast SSR failure) as a themed page instead of the router's plain-text
default.

### Inertia form semantics for tests and curl

An Inertia visit sends `X-Inertia: true`, `X-Inertia-Version: <version>`, and
`Accept: text/html`.

- **Version.** On a `GET`, the middleware compares `X-Inertia-Version` with the
  renderer's current asset version. A mismatch returns `409` with
  `X-Inertia-Location: <url>` and `X-Inertia-Version`, and no body; the client
  performs a full page visit. The scaffold's renderer has no version configured
  (the empty string), so a test may omit the header; if you set
  `inertia.WithVersion`, send the same value or every visit becomes a 409.
- **Validation failures.** Re-render the same page with `.Errors(err)` (a
  `pkg/validation` error) or `.ValidationErrors(map[string]string{...})`. The
  response keeps the page's status (`200` unless you call `.Status`) and carries
  `props.errors` keyed by field. `inertia.ErrorHandler` does the same for field
  errors returned from a handler, at `422`. Both are valid Inertia form
  responses; the first is what the scaffold's auth handlers use.
- **Redirects.** `renderer.Redirect` writes a normal `3xx`; the middleware
  upgrades `302` to `303` for POST/PUT/PATCH/DELETE so the browser re-GETs.
  `renderer.Location` sends `409` + `X-Inertia-Location` for a full browser
  visit, and `renderer.FreshRedirect` sends `409` + `X-Inertia-Redirect` for a
  fresh Inertia visit. A redirect whose `Location` contains `#` also becomes a
  `409` + `X-Inertia-Redirect` control response.
- **Content type.** Inertia page responses are JSON with `Vary: X-Inertia`.
  `anduintest.AssertInertia` decodes either that JSON or the HTML document.

```bash
# Read a page as an Inertia visit (JSON) instead of the HTML document.
curl -i http://localhost:8080/users/sign-in \
  -H 'X-Inertia: true' \
  -H 'Accept: text/html'

# A failed form submission re-renders the page: 200 with props.errors.
curl -i -X POST http://localhost:8080/users/sign-in \
  -H 'Content-Type: application/json' \
  -H 'Accept: text/html' \
  -H 'X-Inertia: true' \
  -H 'Origin: http://localhost:8080' \
  -d '{"email":"nobody@example.com","password":"wrong password"}'
```

Add `-H 'X-Inertia-Version: <version>'` when the renderer configures a
version. Unsafe requests also need CSRF, either the trusted `Origin` above or
`X-CSRF-Token`; see [CSRF](#csrf-origin-trust-vs-the-token).

## Testing handlers

`github.com/mbvlabs/andurel/pkg/anduin/anduintest` builds an `httptest` server
from real handler registrations so tests exercise the same `Router` a production
process does:

```go
srv := anduintest.New(t, func(rt *anduin.Router) {
	handlers.NewUserHandler(users, testRenderer(t)).Register(rt)
})

resp := srv.Get(routes.UserIndex.Path() + "?page=1")
resp.AssertStatus(http.StatusOK)
resp.AssertInertia("User/Index")
```

`anduintest` provides `Get`, `PostForm`, `PostJSON`, and `Do`; responses expose
`StatusCode`, `Header`, `Body`, `Text()`, `JSON(dst)`, `AssertStatus`, and
`AssertInertia`. `AssertInertia(component string)` takes only the component
name: the response keeps the `*testing.T` from the server, so there is no `t`
argument. It accepts either an Inertia JSON response or the HTML document,
finding the component in `data-page`. It keeps a cookie jar across requests.
Pass `anduintest.WithErrorHandler(handlers.MapError(renderer))` to exercise
mapped errors such as the themed 404 page.

For full-graph tests, boot `runtime.New(ctx, testConfig)` and call
`app.Handler`. For domain tests, use `pkg/storage/storagetest.Cluster` and
`NewTestDB(t, migrations.Migrations, ".")`.

## Discovery and verification

- `andurel inspect handlers --json` lists each handler with its constructor
  params and the routes its `Register` method mounts (name, method, path).
- `andurel inspect routes --json` lists the route manifest (host-aware).
- `andurel verify` runs format, `go fix`, `go vet`, the typed layer rules, and
  generated-code drift. The rule list is in
  [cli-catalog.md](cli-catalog.md#verify-rules). Add `--with-tests` to also run
  `go test ./...`.
