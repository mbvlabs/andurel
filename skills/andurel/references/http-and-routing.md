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
	rt.Get(routes.UserIndex.Path(), h.Index)
	rt.Get(routes.UserShow.Path(), h.Show)
	rt.Post(routes.UserCreate.Path(), h.Create, middleware.RateLimit(h.cache, 5, 10*time.Minute, nil))
}
```

`anduin.Serve(w, r, handler)` is the low-level entry point behind
`Router.Handler`, useful when embedding an `anduin.Handler` in another
`http.Handler` and in tests.

## Host dispatch

`Router.Host(name)` returns a `*Host`: a per-host `ServeMux` with the same
`Get/Post/Put/Patch/Delete` registration surface. Routes registered on a host
are served only when the request `Host` matches that hostname (or one of its
aliases).

```go
rt.Host(config.HostAdmin).Get(routes.AdminDashboard.Path(), h.Dashboard)
```

Configure the hostname registry at boot with `routing.ConfigureHosts`, using
`routing.HostName` and `routing.HostPrimary`. `routing.Host(name)` binds a route
value to a host. Unknown `Host` values are served by the primary mux. A hostname
without a port is also aliased as `hostname:80` and `hostname:443`.

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
func NoContent(ctx context.Context, status int) error
func Redirect(ctx context.Context, location string, status int) error

func SetHeader(ctx context.Context, key, value string)
func AddHeader(ctx context.Context, key, value string)
```

`Redirect` requires a `3xx` status. Set response headers with `SetHeader` before
writing a body.

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
```

Handlers return domain errors unchanged and let the router map them. The
`ErrorHandler` is installed once in `internal/runtime`:

```go
rt := anduin.NewRouter(
	anduin.WithErrorHandler(handlers.MapError(renderer)),
)
```

`handlers.MapError(*inertia.Renderer) anduin.ErrorHandler` translates the shared
domain vocabulary (`domains.ErrNotFound`, `domains.ErrConflict`)
and `pkg/validation` errors (`validation.As(err)`) into HTTP responses, validation
props, and statuses, falling back to `anduin.Error(ctx, http.StatusInternalServerError, err)`.
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
  `TokenLookup` (`"header:X-CSRF-Token,form:_csrf"`), `CookieName`,
  `CookiePath`, `CookieDomain`, `CookieSecure`, `CookieHTTPOnly`,
  `CookieSameSite`, `TrustedOrigins`, `TokenLength`, `Skipper`. The current
  token is available to handlers via `anduin.CSRFToken(ctx)`.

App middleware (auth, rate limiting, cookie-jar wiring) lives in `middleware/`
and uses the same `anduin.Middleware` type. `pkg/inertia`'s
`(*Renderer).Middleware()` also returns an `anduin.Middleware`.

Install global middleware in `internal/runtime`:

```go
rt.Use(
	anduin.Recover(),
	anduin.RealIP(cfg.HTTP.TrustedProxies...),
	anduin.Telemetry(tel),
	jar.Middleware(),
	renderer.Middleware(),
	anduin.CORS(corsConfig(cfg)),
	anduin.CSRF(csrfConfig(cfg)),
)
```

## Server lifecycle

`Server` owns an HTTP listener plus a set of `Shutdowner`s. The router stays
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
	host, port, env string,
	handler http.Handler,
	shutdowners []Shutdowner,
	opts ...ServerOption,
) Server
func (s *Server) Addr() string
func (s *Server) Start(ctx context.Context, _ string) error
func (s *Server) Shutdown(ctx context.Context) error
```

`NewServer` appends the HTTP listener to the supplied `Shutdowner`s (shut down
in order, listener last). `cmd/app` gets its server from `runtime.New`, which
builds the router and wraps it with `anduin.NewServer(...)` internally, then
calls `app.Start`; lifecycle ordering is explicit code, not an Fx hook.

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
return h.renderer.Page(ctx, "User/Index", inertia.FromStruct(result.Items)).Render()
```

The root document is `resources/root.templ` (package `resources`). Map
validation failures with `Page(...).ValidationErrors(fields)`; check with
`validation.As(err)`. There is no separate domain validation sentinel or wrapper.

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
resp.AssertInertia(t, "User/Index")
```

`anduintest` provides `Get`, `PostForm`, `PostJSON`, and `Do`; responses expose
`StatusCode`, `Header`, `Body`, `Text()`, `JSON(dst)`, `AssertStatus`, and
`AssertInertia`. It keeps a cookie jar across requests.

For full-graph tests, boot `runtime.New(ctx, testConfig)` and call
`app.Handler`. For domain tests, use `pkg/storage.TestCluster` and
`NewTestDB(t, migrations.Migrations, ".")`.

## Discovery and verification

- `andurel inspect handlers --json` lists handlers.
- `andurel inspect routes --json` lists the route manifest (host-aware).
- `andurel verify` runs format, `go fix`, `go vet`, and the domain/service
  import-boundary lint. Add `--with-tests` to also run `go test ./...`.
