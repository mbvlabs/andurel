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
	"net/http"
	"slices"

	"app/config"
	"app/domains"
	"app/handlers"
	"app/middleware"
	"app/services"

	"github.com/mbvlabs/andurel/pkg/anduin"
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

func New(ctx context.Context, cfg config.Config) (*App, error) {
	tel, err := newTelemetry(ctx, cfg.App, cfg.Telemetry)
	if err != nil {
		return nil, err
	}

	db, err := newDatabase(ctx, cfg.Database, tel)
	if err != nil {
		return nil, err
	}

	queueInsert, err := storage.NewQueueInsert(db, cfg.QueueInsert.Config)
	if err != nil {
		return nil, err
	}

	// Domains live in one flat package: one file per domain.
	users := domains.NewUsers(db)
	tokens := domains.NewTokens(db)

	// Services coordinate domains, the queue, and configuration.
	identity := services.NewIdentity(users, tokens, queueInsert, cfg.Auth, cfg.Mail)

	renderer, err := newRenderer(cfg)
	if err != nil {
		return nil, err
	}

	jar, err := newJar(cfg.App, cfg.Session)
	if err != nil {
		return nil, err
	}

	cacheStore, err := cfg.Cache.Store()
	if err != nil {
		return nil, err
	}

	rt := anduin.NewRouter(
		anduin.WithErrorHandler(handlers.MapError(renderer)),
		anduin.WithTelemetry(tel),
	)
	rt.Use(
		anduin.Recover(),
		anduin.Telemetry(tel),
		jar.Middleware(),
		renderer.Middleware(),
		anduin.CORS(corsConfig(cfg)),
		anduin.CSRF(csrfConfig(cfg)),
	)

	handlers.NewPages(renderer).Register(rt)
	handlers.NewAssets(cacheStore, cfg.App).Register(rt)
	handlers.NewAuthHandler(identity, renderer, cacheStore).Register(rt)
	handlers.NewAPIHandler(db).Register(rt)

	handler := rt.Handler()
	srv := anduin.NewServer(ctx, cfg.HTTP.Host, cfg.HTTP.Port, cfg.App.Environment,
		handler, nil, anduin.WithTimeouts(cfg.HTTP.IdleTimeout, cfg.HTTP.ReadTimeout, cfg.HTTP.WriteTimeout))

	return &App{
		Config:  cfg,
		DB:      db,
		Handler: handler,
		Server:  srv,
		closers: []func(context.Context) error{db.Close, tel.Shutdown},
	}, nil
}

func (a *App) Start(ctx context.Context) error    { return a.Server.Start(ctx, a.Config.App.Environment) }
func (a *App) Shutdown(ctx context.Context) error { return a.Server.Shutdown(ctx) }

func (a *App) Close(ctx context.Context) error {
	for _, closeFn := range slices.Backward(a.closers) {
		if err := closeFn(ctx); err != nil {
			return err
		}
	}
	return nil
}
```

## Rules

- Construct dependencies bottom-up in `internal/runtime/runtime.go` with one `New`.
- Build the jar in `newJar` and the cache with `cfg.Cache.Store()`; middleware is policy only.
- Register flat handlers with `handlers.NewXHandler(...).Register(rt)`; there is no reflection.
- Track shutdown with an explicit ordered `closers` list and release in reverse.
- `cmd/app` is env load + `runtime.New` + `app.Start` + `app.Shutdown`.
- Add a new dependency by constructing it here and passing it down; never reach for a container or a global.

## Checklist

- [ ] New dependency constructed explicitly in `runtime.New`.
- [ ] Handler registered with a constructor call plus `Register`.
- [ ] Shutdown function appended to `closers` if needed.
- [ ] `andurel verify` passes and the app boots.
