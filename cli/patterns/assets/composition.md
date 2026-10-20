# Composition

`internal/runtime` is the composition root. It constructs the dependency graph
explicitly, bottom-up, with one constructor per process: `New` owns the HTTP
`Handler`, the `anduin.Server`, and every dependency between them. There is no
`NewServer` to glue, no `middleware.NewJar`, and no Fx: every dependency and
every lifecycle hook is readable in one file.

`middleware/` exports policy only (auth guard, rate limiter, session
definition). The cookie jar and the cache store are built here, in `newJar`
and `cfg.Cache.Store()`.

## Files

### internal/runtime/runtime.go
```go
package runtime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"app/config"
	"app/domains"
	"app/handlers"
	"app/middleware"
	"app/services"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/routing"
	"github.com/mbvlabs/andurel/pkg/storage"
	"github.com/mbvlabs/andurel/pkg/telemetry"
)

type App struct {
	Config  config.Config
	DB      storage.Connection
	Handler http.Handler
	Server  anduin.Server
	closers []func(context.Context) error
}

func New(ctx context.Context, cfg config.Config) (_ *App, err error) {
	if err := routing.ConfigureHosts(cfg.App.Hosts); err != nil {
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
	app.closers = append(app.closers, tel.Shutdown)

	db, err := newDatabase(ctx, cfg.Database, tel)
	if err != nil {
		return nil, err
	}
	app.DB = db
	app.closers = append(app.closers, func(context.Context) error { return db.Close() })

	cacheStore, err := cfg.Cache.Store()
	if err != nil {
		return nil, err
	}
	app.closers = append(app.closers, func(context.Context) error { return cacheStore.Close() })

	queueInsert, err := storage.NewQueueInsert(db, cfg.QueueInsert.Config)
	if err != nil {
		return nil, err
	}

	transactor := storage.NewTransactor(db)
	users := domains.NewUsers(db)
	tokens := domains.NewTokens(db)

	identity := services.NewIdentity(transactor, users, tokens, queueInsert, cfg.Auth, cfg.Mail)

	renderer, err := newRenderer(cfg)
	if err != nil {
		return nil, err
	}

	jar, err := newJar(cfg.App, cfg.Session)
	if err != nil {
		return nil, err
	}

	assetsHandler, err := handlers.NewAssets(cfg.App)
	if err != nil {
		return nil, err
	}

	rt := anduin.NewRouter(
		anduin.WithErrorHandler(handlers.MapError(renderer)),
		anduin.WithTelemetry(tel),
	)
	rt.Use(
		anduin.Recover(),
		anduin.RealIP(cfg.HTTP.TrustedProxies...),
		anduin.Telemetry(tel),
		jar.Middleware(),
		renderer.Middleware(),
		anduin.CORS(corsConfig(cfg)),
		anduin.CSRF(csrfConfig(cfg)),
	)

	handlers.NewPages(renderer).Register(rt)
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
	var closeErr error
	for _, closeFn := range slices.Backward(a.closers) {
		closeErr = errors.Join(closeErr, closeFn(ctx))
	}
	return closeErr
}
```

## Rules

- Construct dependencies bottom-up in `internal/runtime/runtime.go` with one `New`.
- Build the jar in `newJar` and the cache with `cfg.Cache.Store()`; middleware is policy only.
- Register flat handlers with `handlers.NewXHandler(...).Register(rt)`; there is no reflection.
- Append each dependency's cleanup to `closers` as soon as it is built; `Close` releases them in reverse, and the same path cleans up a failed `New`.
- `cmd/app` is env load + `runtime.New` + `app.Start` + `app.Shutdown`.
- Add a new dependency by constructing it here and passing it down; never reach for a container or a global.
- Build one `storage.NewTransactor(db)` and each domain receiver with `domains.NewX(db)`, then pass them individually to the services and handlers that use them. Handlers that need only health take a narrow interface (`healthChecker`, satisfied by `db`).
- Only the wiring layers (`internal/runtime`, `cmd`, `seeds`, `migrations`) and `domains` may name `storage.Connection`; services and handlers never do.
- A new domain is one `domains.NewX(db)` line here, passed to whoever uses it.

## Checklist

- [ ] New dependency constructed explicitly in `runtime.New`.
- [ ] Handler registered with a constructor call plus `Register`.
- [ ] Shutdown function appended to `closers` if needed.
- [ ] `andurel verify` passes and the app boots.
