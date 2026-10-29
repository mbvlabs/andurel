# Composition

`internal/runtime` is the composition root. It constructs the dependency graph
explicitly, bottom-up, with one constructor per process: `New` owns the HTTP
`Handler`, the `anduin.Server`, and every dependency between them. There is no
`NewServer` to glue, no `middleware.NewJar`, and no Fx: every dependency and
every lifecycle hook is readable in one file.

`middleware/` exports application policy only (auth guard, session
definition); generic middleware such as rate limiting and CSRF comes from
`pkg/anduin`. The cookie jar and the cache store are built here, in `newJar`
and `cfg.Cache.Store()`.

The snippet below is the shape of the scaffold's `internal/runtime/runtime.go`.
`<module>` stands for your `go.mod` module path (for example
`example.com/app`); substitute it for the imports. Keep the rest of the file's
helpers (`newRenderer`, `newJar`, `newTelemetry`, `validateEmailConfig`) beside
it.

## Files

### internal/runtime/runtime.go
```go fragment
package runtime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"<module>/config"
	"<module>/domains"
	"<module>/handlers"
	"<module>/middleware"
	"<module>/routes"
	"<module>/services"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/lifecycle"
	"github.com/mbvlabs/andurel/pkg/routing"
	"github.com/mbvlabs/andurel/pkg/storage"
	"github.com/mbvlabs/andurel/pkg/telemetry"
)

type App struct {
	Config  config.Config
	DB      storage.Connection
	Handler http.Handler
	Server  anduin.Server
	closers lifecycle.Stack
}

func New(ctx context.Context, cfg config.Config) (_ *App, err error) {
	if err := validateEmailConfig(cfg.Mail, cfg.App); err != nil {
		return nil, err
	}

	hosts, err := routing.NewHosts(cfg.App.Hosts)
	if err != nil {
		return nil, fmt.Errorf("runtime: configure hosts: %w", err)
	}

	// Each dependency registers its cleanup as soon as it exists, so a failure
	// part way through releases everything built so far.
	app := &App{Config: cfg}
	defer func() {
		if err != nil {
			err = errors.Join(err, app.Close(context.WithoutCancel(ctx)))
		}
	}()

	tel, err := newTelemetry(ctx, cfg.App, cfg.Telemetry)
	if err != nil {
		return nil, err
	}
	app.closers.Add(tel.Shutdown)

	db, err := storage.NewPostgres(ctx, cfg.Database, storage.WithTelemetry(tel))
	if err != nil {
		return nil, err
	}
	app.DB = db
	app.closers.Add(func(context.Context) error { return db.Close() })

	cacheStore, err := cfg.Cache.Store()
	if err != nil {
		return nil, err
	}
	app.closers.Add(func(context.Context) error { return cacheStore.Close() })

	queueInsert, err := storage.NewQueueInsert(db, cfg.QueueInsert.Config)
	if err != nil {
		return nil, err
	}

	transactor := storage.NewTransactor(db)
	users := domains.NewUsers(db)
	tokens := domains.NewTokens(db)

	identity := services.NewIdentity(
		transactor,
		users,
		tokens,
		queueInsert,
		cfg.Auth,
		cfg.Mail,
		hosts,
	)

	renderer, err := newRenderer(cfg, hosts)
	if err != nil {
		return nil, err
	}

	jar, err := newJar(cfg.App, cfg.Session)
	if err != nil {
		return nil, err
	}

	assetsHandler, err := handlers.NewAssetsHandler(cfg.App)
	if err != nil {
		return nil, err
	}

	router := anduin.NewRouter(
		anduin.WithHosts(hosts),
		anduin.WithErrorHandler(handlers.MapError(renderer)),
		anduin.WithTelemetry(tel),
	)
	// Global CORS trusts explicit CORS_ALLOWED_ORIGINS only; host origins stay
	// out so a sibling host cannot read authenticated responses. Vite build
	// files answer with their own uncredentialed CORS from AssetsHandler.
	cors := anduin.DefaultCORS(cfg.HTTP.CORSAllowedOrigins...)
	cors.Skipper = isViteAssetRequest
	csrf := anduin.DefaultCSRF(
		cfg.HTTP.CSRFStrategy,
		cfg.App.IsProduction(),
		anduin.MergeOrigins(cfg.App.Origins(), cfg.HTTP.CSRFTrustedOrigins)...,
	)
	bearerSkipper := anduin.SkipCSRFForBearer(routes.APIPrefix, cfg.Session.Name)
	csrf.Skipper = func(ctx context.Context) bool {
		return bearerSkipper(ctx) || isSafeViteAssetRequest(ctx)
	}

	router.Use(
		anduin.Recover(),
		anduin.RealIP(cfg.HTTP.TrustedProxies...),
		anduin.Telemetry(tel),
		jar.Middleware(),
		renderer.Middleware(),
		anduin.CORS(cors),
		anduin.CSRF(csrf),
	)

	// The welcome page is public: a new project must be reachable before anyone
	// signs in. Pass middleware.AuthOnly(users) to Register to protect a page.
	handlers.NewPagesHandler(renderer).Register(router)
	assetsHandler.Register(router)
	handlers.NewAuthHandler(identity, renderer, cacheStore).Register(router)
	handlers.NewAPIHandler(db, newReadiness(db, renderer)).Register(router)

	app.Handler = router.Handler()
	app.Server = anduin.NewServer(
		ctx,
		cfg.HTTP.Host,
		cfg.HTTP.Port,
		app.Handler,
		anduin.WithTimeouts(
			cfg.HTTP.IdleTimeout,
			cfg.HTTP.ReadTimeout,
			cfg.HTTP.WriteTimeout,
		),
	)

	return app, nil
}

// viteAssetPrefix is the Vite build route subtree without its wildcard.
var viteAssetPrefix = strings.TrimSuffix(routes.ViteBuild.Pattern(), "/*")

func isViteAssetRequest(ctx context.Context) bool {
	request := anduin.Request(ctx)

	return request != nil && isViteAssetPath(request.URL.Path)
}

// isSafeViteAssetRequest limits the CSRF skip to safe methods.
func isSafeViteAssetRequest(ctx context.Context) bool {
	request := anduin.Request(ctx)
	if request == nil || !isViteAssetPath(request.URL.Path) {
		return false
	}

	switch request.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

func isViteAssetPath(path string) bool {
	return strings.HasPrefix(path, viteAssetPrefix+"/")
}

// readiness reports whether the process can serve traffic. Liveness stays
// separate: the database must answer, and the renderer decides whether a
// required SSR sidecar must answer too. The renderer treats a disabled
// renderer-wide default, an explicit CSR fallback, and an unsupported
// transport as not gating readiness, so this type only maps its typed reason.
type readiness struct {
	database storage.Connection
	renderer *inertia.Renderer
}

func newReadiness(db storage.Connection, renderer *inertia.Renderer) readiness {
	return readiness{database: db, renderer: renderer}
}

// readinessError carries a stable, secret-free reason for /api/ready.
type readinessError struct{ reason string }

func (err readinessError) Error() string { return err.reason }

func (err readinessError) ReadinessReason() string { return err.reason }

// Health returns the first reason the process is not ready. The reasons are
// stable and carry no configuration values, so readiness output cannot leak a
// connection string.
func (r readiness) Health(ctx context.Context) error {
	if err := r.database.Health(ctx); err != nil {
		return readinessError{reason: "database_unavailable"}
	}
	if r.renderer == nil {
		return nil
	}
	if err := r.renderer.Health(ctx); err != nil {
		reason := "unavailable"
		var ssrErr *inertia.ReadinessError
		if errors.As(err, &ssrErr) && ssrErr.Status != "" {
			reason = string(ssrErr.Status)
		}

		return readinessError{reason: reason}
	}

	return nil
}

func (a *App) Start() error {
	return a.Server.Start()
}

func (a *App) Shutdown(ctx context.Context) error {
	return a.Server.Shutdown(ctx)
}

func (a *App) Close(ctx context.Context) error {
	return a.closers.Close(ctx)
}
```

## Rules

- Construct dependencies bottom-up in `internal/runtime/runtime.go` with one `New`.
- Build the jar in `newJar` and the cache with `cfg.Cache.Store()`; middleware is policy only.
- Register flat handlers with `handlers.NewXHandler(...).Register(router)`; there is no reflection. `PagesHandler.Register` takes the router plus optional middleware and mounts the public home page when none is passed, `AssetsHandler.Register` takes only the router, and `APIHandler` takes the liveness and readiness checkers.
- `/api/health` is liveness and `/api/ready` is readiness: readiness requires the database and, when the renderer-wide SSR default is on with fail-fast, a healthy Inertia SSR sidecar. `/api/ready` answers 200 `{"status":"ready"}` or 503 `{"status":"unready","reason":"database_unavailable"}` / `{"status":"unready","reason":"unavailable"}`. Keep the two separate so orchestration can hold traffic until the process can render a page.
- Construct the router with `anduin.NewRouter(anduin.WithHosts(hosts), ...)`. The host registry is what makes `routing.Host(...)`, `rt.Host(...)`, and route `FullURL(hosts, ...)` calls agree; without it every request is served by the primary mux.
- Global CORS trusts only `CORS_ALLOWED_ORIGINS`. Never add `cfg.App.Origins()` to CORS: host origins belong in CSRF trusted origins, not in a credentialed read policy. Public Vite assets carry their own uncredentialed `*` CORS on the asset route.
- Host-scoped policy (framing headers, analytics) branches on `anduin.RequestHost(ctx)` in global middleware; never re-parse `Request.Host`.
- Add each dependency's cleanup to a `lifecycle.Stack` as soon as it is built; `Close` releases them in reverse, and the same path cleans up a failed `New`.
- `cmd/app` is env load + `runtime.New` + `lifecycle.Run`, with `lifecycle.WithBlockingStart()` for the HTTP serve loop, `app.Shutdown` as stop and a deferred `closeApp` releasing the rest. Ready-returning starters such as the queue worker use the default, which finishes startup before shutdown.
- Add a new dependency by constructing it here and passing it down; never reach for a container or a global.
- Build one `storage.NewTransactor(db)` and each domain receiver with `domains.NewX(db)`, then pass them individually to the services and handlers that use them. Handlers that need only health take a narrow interface (`healthChecker`, satisfied by `db`).
- Only the wiring layers (`internal/runtime`, `cmd`, `seeds`, `migrations`) and `domains` may name `storage.Connection`; services and handlers never do.
- A new domain is one `domains.NewX(db)` line here, passed to whoever uses it.
- Validate boot-time configuration before building the graph (the scaffold's `validateEmailConfig` is the worked example), so a misconfigured process fails at startup instead of on the first request.

## Checklist

- [ ] New dependency constructed explicitly in `runtime.New`.
- [ ] `anduin.WithHosts(hosts)` is passed to `anduin.NewRouter`.
- [ ] Handler registered with a constructor call plus `Register(router)` and the right middleware arguments.
- [ ] Shutdown function added to the `lifecycle.Stack` if needed.
- [ ] `andurel verify` passes and the app boots.
