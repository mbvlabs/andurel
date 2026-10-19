package runtime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"example.com/app/assets"
	"example.com/app/config"
	"example.com/app/domains"
	"example.com/app/handlers"
	"example.com/app/handlers/api"
	"example.com/app/handlers/auth"
	"example.com/app/middleware"
	"example.com/app/resources"
	"example.com/app/routes"
	"example.com/app/services"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/email"
	"github.com/mbvlabs/andurel/pkg/inertia"
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

	transactor := storage.NewTransactor(db)
	users := domains.NewUsers(db)

	identity := services.NewIdentity(transactor, users, queueInsert, cfg.Auth, cfg.Mail)

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

	router := anduin.NewRouter(
		anduin.WithErrorHandler(handlers.MapError(renderer)),
		anduin.WithTelemetry(tel),
	)
	router.Use(
		anduin.Recover(),
		anduin.Telemetry(tel),
		jar.Middleware(),
		renderer.Middleware(),
		anduin.CORS(corsConfig(cfg)),
		anduin.CSRF(csrfConfig(cfg)),
	)

	handlers.NewPages(renderer).Register(router)
	handlers.NewAssets(cache, cfg.App).Register(router)
	auth.NewHandler(identity, renderer).Register(router)
	api.NewAPI(db).Register(router)

	return &App{
		Config:  cfg,
		DB:      db,
		Handler: router.Handler(),
		closers: []func(context.Context) error{
			func(context.Context) error { return db.Close() },
			tel.Shutdown,
		},
	}, nil
}

func (a *App) Close(ctx context.Context) error {
	var closeErr error
	for _, closeFn := range slices.Backward(a.closers) {
		closeErr = errors.Join(closeErr, closeFn(ctx))
	}

	return closeErr
}

func (a *App) Shutdown(ctx context.Context) error {
	return a.Close(ctx)
}

func newTelemetry(
	ctx context.Context,
	appCfg config.App,
	cfg config.Telemetry,
) (*telemetry.Telemetry, error) {
	opts := []telemetry.Option{
		telemetry.WithTraceSampleRate(cfg.TraceSampleRate),
		telemetry.WithBatchConfig(
			cfg.BatchSize,
			time.Duration(cfg.BatchTimeoutMs)*time.Millisecond,
			2048,
		),
		telemetry.WithLogLevel(cfg.LogLevel),
	}
	if !appCfg.IsProduction() {
		opts = append(opts, telemetry.WithConsole())
	}
	headers := telemetry.ParseHeaders(cfg.OtlpHeaders)
	if cfg.OtlpLogsEndpoint != "" {
		opts = append(opts, telemetry.WithOTLPLogs(cfg.OtlpLogsEndpoint, headers))
	}
	if cfg.OtlpTracesEndpoint != "" {
		opts = append(opts, telemetry.WithOTLPTraces(cfg.OtlpTracesEndpoint, headers))
	}
	if cfg.OtlpMetricsEndpoint != "" {
		opts = append(opts, telemetry.WithOTLPMetrics(cfg.OtlpMetricsEndpoint, headers))
	}

	return telemetry.New(ctx, cfg.ServiceName, cfg.ServiceVersion, opts...)
}

func newDatabase(
	ctx context.Context,
	cfg storage.Config,
	tel *telemetry.Telemetry,
) (*storage.Postgres, error) {
	opts := []storage.Option{}
	if tel != nil {
		opts = append(opts, storage.WithOpenTelemetry(storage.TelemetryConfig{
			TracerProvider: tel.TracerProvider(),
			MeterProvider:  tel.MeterProvider(),
		}))
	}

	return storage.NewPostgres(ctx, cfg, opts...)
}

func newEmailSenders(
	cfg config.Mail,
) (email.TransactionalSender, email.MarketingSender, error) {
	switch cfg.Driver {
	case config.MailpitDriver:
		client, err := email.NewMailpit(email.MailpitConfig{
			Host: cfg.Host,
			Port: cfg.Port,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("create Mailpit client: %w", err)
		}

		return client, client, nil
	default:
		return nil, nil, fmt.Errorf("unsupported email provider %q", cfg.Driver)
	}
}

func newRenderer(cfg config.Config) (*inertia.Renderer, error) {
	return inertia.NewRenderer(
		cfg.Inertia.ContainerID,
		routes.ViteBuild.FullURL(),
		cfg.Inertia.EntryPoint,
		cfg.Inertia.ViteDevURL,
		cfg.Inertia.SSRURL,
		cfg.Inertia.SSRRequestTimeout,
		cfg.Inertia.SSRMaxResponseBytes,
		inertia.WithRoot(resources.Root),
		inertia.WithAssetFS(assets.Files),
		inertia.WithProjectName(cfg.App.ProjectName),
		inertia.WithEnvironment(cfg.App.Environment),
		inertia.WithProtocolDebug(cfg.Inertia.ProtocolDebug),
		inertia.WithShared(inertia.Props{
			"appVersion": cfg.Version,
			"hosts":      cfg.App.HostOrigins(),
		}),
		inertia.WithSSRFailFast(cfg.Inertia.SSRFailFast),
	)
}

func corsConfig(cfg config.Config) anduin.CORSConfig {
	return anduin.CORSConfig{
		AllowOrigins: mergedOrigins(
			cfg.App.Origins(),
			cfg.HTTP.CORSAllowedOrigins,
		),
		AllowMethods: []string{
			http.MethodGet,
			http.MethodHead,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300,
	}
}

func csrfConfig(cfg config.Config) anduin.CSRFConfig {
	tokenLookup := "header:X-CSRF-Token"
	if cfg.HTTP.CSRFStrategy == "header_or_legacy_token" {
		tokenLookup = "header:X-CSRF-Token,form:_csrf"
	}

	return anduin.CSRFConfig{
		TokenLookup:  tokenLookup,
		CookieName:   "_csrf",
		CookiePath:   "/",
		CookieSecure: cfg.App.IsProduction(),
		//nolint:gosec // HttpOnly is safe: the token is only read server-side.
		CookieHTTPOnly: true,
		CookieSameSite: http.SameSiteStrictMode,
		TrustedOrigins: mergedOrigins(
			cfg.App.Origins(),
			cfg.HTTP.CSRFTrustedOrigins,
		),
		Skipper: csrfSkipper(cfg),
	}
}

func csrfSkipper(cfg config.Config) func(context.Context) bool {
	return func(ctx context.Context) bool {
		request := anduin.Request(ctx)
		if request == nil || !isAPIPath(request.URL.Path) {
			return false
		}
		if !hasNonEmptyBearerToken(request.Header.Get("Authorization")) {
			return false
		}
		_, err := request.Cookie(cfg.Session.Name)

		return err != nil
	}
}

func isAPIPath(path string) bool {
	prefix := strings.TrimSuffix(routes.APIPrefix, "/")
	if prefix == "" {
		return false
	}

	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func hasNonEmptyBearerToken(authorization string) bool {
	parts := strings.Fields(authorization)

	return len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") && parts[1] != ""
}

func mergedOrigins(groups ...[]string) []string {
	seen := make(map[string]struct{})
	origins := make([]string, 0)
	for _, group := range groups {
		for _, origin := range group {
			origin = strings.TrimSpace(origin)
			if origin == "" {
				continue
			}
			if _, ok := seen[origin]; ok {
				continue
			}
			seen[origin] = struct{}{}
			origins = append(origins, origin)
		}
	}

	return origins
}
