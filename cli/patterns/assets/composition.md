# Composition

`internal/runtime` is the composition root. It constructs the dependency graph
explicitly, bottom-up, and exposes an `App` with the HTTP `Handler` and the
database. There is no Fx: every dependency and every lifecycle hook is
readable in one file.

## Files

### internal/runtime/runtime.go
```go
package runtime

import (
	"context"
	"net/http"
	"slices"

	"app/config"
	"app/domains/token"
	"app/domains/user"
	"app/handlers"
	"app/handlers/api"
	"app/handlers/auth"
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
	closers []func(context.Context) error
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	tel, err := newTelemetry(ctx, cfg.App, cfg.Telemetry)
	if err != nil {
		return nil, err
	}

	db, err := newDatabase(ctx, cfg.Database, tel)
	if err != nil {
		_ = tel.Shutdown(ctx)
		return nil, err
	}

	queueInsert, err := storage.NewQueueInsert(db, cfg.QueueInsert.Config)
	if err != nil {
		_ = db.Close()
		_ = tel.Shutdown(ctx)
		return nil, err
	}

	// Domains are islands; each owns its table and never imports another.
	users := user.NewUsers(db)
	tokens := token.NewTokens(db)

	// Services coordinate domains, the queue, and configuration.
	identity := services.NewIdentity(users, tokens, queueInsert, cfg.Auth, cfg.Mail)

	renderer, err := newRenderer(cfg)
	if err != nil {
		_ = db.Close()
		_ = tel.Shutdown(ctx)
		return nil, err
	}

	jar, err := middleware.NewJar(cfg.App, cfg.Session)
	if err != nil {
		_ = db.Close()
		_ = tel.Shutdown(ctx)
		return nil, err
	}

	cache, err := handlers.NewCacheBuilder[string]().Build()
	if err != nil {
		_ = db.Close()
		_ = tel.Shutdown(ctx)
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
	handlers.NewAssets(cache, cfg.App).Register(rt)
	auth.NewHandler(identity, renderer).Register(rt)
	api.NewAPI(db).Register(rt)

	return &App{
		Config:  cfg,
		DB:      db,
		Handler: rt.Handler(),
		closers: []func(context.Context) error{db.Close, tel.Shutdown},
	}, nil
}

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

- Construct dependencies bottom-up in `internal/runtime/runtime.go`.
- Register handlers with `NewX(...).Register(rt)`; there is no reflection.
- Track shutdown with an explicit ordered `closers` list and release in reverse.
- `cmd/app` is env load + `runtime.New` + `runtime.NewServer` + `Start`.
- Add a new dependency by constructing it here and passing it down; never reach for a container or a global.

## Checklist

- [ ] New dependency constructed explicitly in `runtime.New`.
- [ ] Handler registered with a constructor call plus `Register`.
- [ ] Shutdown function appended to `closers` if needed.
- [ ] `andurel verify` passes and the app boots.
