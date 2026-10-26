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

## Files

### internal/runtime/runtime.go
```go fragment
package runtime

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"app/config"
	"app/domains"
	"app/handlers"
	"app/middleware"
	"app/routes"
	"app/services"

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

	rt := anduin.NewRouter(
		anduin.WithErrorHandler(handlers.MapError(renderer)),
		anduin.WithTelemetry(tel),
	)
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

	handlers.NewPagesHandler(renderer).Register(rt, middleware.AuthOnly(users))
	assetsHandler.Register(rt)
	handlers.NewAuthHandler(identity, renderer, cacheStore).Register(rt)
	handlers.NewAPIHandler(db).Register(rt)

	app.Handler = rt.Handler()
	app.Server = anduin.NewServer(ctx, cfg.HTTP.Host, cfg.HTTP.Port, app.Handler,
		anduin.WithTimeouts(cfg.HTTP.IdleTimeout, cfg.HTTP.ReadTimeout, cfg.HTTP.WriteTimeout))

	return app, nil
}

func (a *App) Start() error                       { return a.Server.Start() }
func (a *App) Shutdown(ctx context.Context) error { return a.Server.Shutdown(ctx) }

func (a *App) Close(ctx context.Context) error {
	return a.closers.Close(ctx)
}
```

## Rules

- Construct dependencies bottom-up in `internal/runtime/runtime.go` with one `New`.
- Build the jar in `newJar` and the cache with `cfg.Cache.Store()`; middleware is policy only.
- Register flat handlers with `handlers.NewXHandler(...).Register(rt)`; there is no reflection.
- Add each dependency's cleanup to a `lifecycle.Stack` as soon as it is built; `Close` releases them in reverse, and the same path cleans up a failed `New`.
- `cmd/app` is env load + `runtime.New` + `lifecycle.Run`, with `lifecycle.WithBlockingStart()` for the HTTP serve loop, `app.Shutdown` as stop and a deferred `closeApp` releasing the rest. Ready-returning starters such as the queue worker use the default, which finishes startup before shutdown.
- Add a new dependency by constructing it here and passing it down; never reach for a container or a global.
- Build one `storage.NewTransactor(db)` and each domain receiver with `domains.NewX(db)`, then pass them individually to the services and handlers that use them. Handlers that need only health take a narrow interface (`healthChecker`, satisfied by `db`).
- Only the wiring layers (`internal/runtime`, `cmd`, `seeds`, `migrations`) and `domains` may name `storage.Connection`; services and handlers never do.
- A new domain is one `domains.NewX(db)` line here, passed to whoever uses it.

## Checklist

- [ ] New dependency constructed explicitly in `runtime.New`.
- [ ] Handler registered with a constructor call plus `Register`.
- [ ] Shutdown function added to the `lifecycle.Stack` if needed.
- [ ] `andurel verify` passes and the app boots.
