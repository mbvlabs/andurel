package upgrade

import (
	"fmt"
	"strings"

	"github.com/mbvlabs/andurel/layout"
	layouttemplates "github.com/mbvlabs/andurel/layout/templates"
	"github.com/mbvlabs/andurel/layout/versions"
	"golang.org/x/mod/semver"
)

const (
	sessionCookieRecoveryVersion    = "v1.5.4"
	inertiaRendererInjectionVersion = "v1.5.6"
	telemetryPackageVersion         = "v2.0.0"
)

// ManualAction describes an application-owned change that an upgrade cannot
// apply without risking user code.
type ManualAction struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Instructions string `json:"instructions"`
}

func manualActionsForUpgrade(
	fromVersion string,
	toVersion string,
	modulePath string,
	includeInertiaMigration bool,
) ([]ManualAction, error) {
	actions := []ManualAction{}

	if crossesVersion(fromVersion, toVersion, sessionCookieRecoveryVersion) {
		sessionSource, err := renderTemplateToBytes(
			"router_cookies_session.tmpl",
			layouttemplates.Files,
			&layout.TemplateData{ModuleName: modulePath},
		)
		if err != nil {
			return nil, fmt.Errorf("render session-cookie recovery instructions: %w", err)
		}

		var instructions strings.Builder
		instructions.WriteString(
			"The router tree is application-owned, so Andurel did not change it automatically.\n",
		)
		instructions.WriteString("If this recovery is already present, no action is required.\n\n")
		instructions.WriteString("1. Create router/cookies/session.go:\n\n```go\n")
		instructions.Write(sessionSource)
		instructions.WriteString("```\n\n")
		instructions.WriteString("2. In router/cookies/cookies.go and router/cookies/flash.go:\n")
		instructions.WriteString("   - Replace calls to session.Get with getSession.\n")
		instructions.WriteString(
			"   - Remove the now-unused github.com/labstack/echo-contrib/v5/session imports.\n\n",
		)
		instructions.WriteString(
			"3. In router/middleware/middleware.go, add this inside ValidateSession after the assets and API bypass and before calling the next handler:\n\n",
		)
		instructions.WriteString(
			"```go\nif err := cookies.RecoverInvalidSessions(c); err != nil {\n\treturn err\n}\n```\n\n",
		)
		instructions.WriteString("4. Add this direct requirement to go.mod:\n\n")
		instructions.WriteString("```go\ngithub.com/gorilla/securecookie v1.1.2\n```\n\n")
		instructions.WriteString("5. Format and verify the migration:\n\n```text\n")
		instructions.WriteString(
			"gofmt -w router/cookies/session.go router/cookies/cookies.go router/cookies/flash.go router/middleware/middleware.go\n",
		)
		instructions.WriteString("go fix ./...\ngo vet ./...\n```\n")

		actions = append(actions, ManualAction{
			ID:           "session-cookie-recovery-v1.5.4",
			Title:        "Update application-owned session handling",
			Instructions: instructions.String(),
		})
	}

	if includeInertiaMigration &&
		crossesVersion(fromVersion, toVersion, inertiaRendererInjectionVersion) {
		var instructions strings.Builder
		instructions.WriteString(
			"Inertia is now an Fx-managed renderer instance. Framework-owned internal/inertia files update automatically, but Andurel does not rewrite application-owned controllers.\n\n",
		)
		instructions.WriteString(
			"1. In cmd/app/main.go, add these imports and the renderer provider:\n\n```go\n",
		)
		fmt.Fprintf(
			&instructions,
			"import (\n\t\"%s/assets\"\n\t\"%s/router/appctx\"\n\t\"%s/router/routes\"\n)\n\nfunc provideInertia() (*inertia.Renderer, error) {\n\treturn inertia.New(\n\t\tassets.Files,\n\t\t\"inertia/root.go.html\",\n\t\tconfig.ProjectName,\n\t\tconfig.Env,\n\t\troutes.ViteBuild.Path(),\n\t\tinertia.WithSharedProp(\"appVersion\", appVersion),\n\t\tinertia.WithRequestProps(func(ctx context.Context) inertia.Props {\n\t\t\tflashes := appctx.Flashes(ctx)\n\t\t\tif len(flashes) == 0 {\n\t\t\t\treturn nil\n\t\t\t}\n\t\t\treturn inertia.Props{\"flash\": flashes}\n\t\t}),\n\t)\n}\n",
			modulePath,
			modulePath,
			modulePath,
		)
		instructions.WriteString(
			"```\n\n2. Remove the old inertia.Init call and add provideInertia to the existing fx.Provide block.\n\n",
		)
		instructions.WriteString(
			"3. Inject renderer *inertia.Renderer into router.New and pass renderer.Middleware() to the global middleware list.\n\n",
		)
		instructions.WriteString(
			"4. Add renderer *inertia.Renderer to every Inertia controller constructor and struct. Replace package calls with receiver calls, for example:\n\n```go\ntype Sessions struct {\n\tidentity services.Identity\n\trenderer *inertia.Renderer\n}\n\nfunc NewSessions(identity services.Identity, renderer *inertia.Renderer) Sessions {\n\treturn Sessions{identity: identity, renderer: renderer}\n}\n\nfunc (s Sessions) New(etx *echo.Context) error {\n\treturn s.renderer.Page(etx, \"Auth/Login\", inertia.Props{})\n}\n```\n\n",
		)
		instructions.WriteString(
			"Apply the same receiver change to Redirect and Location calls in built-in and generated resource controllers.\n\n",
		)
		instructions.WriteString(
			"5. Format and verify the migration:\n\n```text\ngofmt -w cmd/app/main.go router/router.go\nfind controllers -name '*.go' -type f -exec gofmt -w {} +\ngo fix ./...\ngo vet ./...\n```\n",
		)

		actions = append(actions, ManualAction{
			ID:           "inertia-renderer-injection-v1.5.6",
			Title:        "Inject the Inertia renderer through Fx",
			Instructions: instructions.String(),
		})
	}

	if crossesVersion(fromVersion, toVersion, telemetryPackageVersion) {
		var instructions strings.Builder
		instructions.WriteString(
			"Generated application telemetry/ is replaced by the standalone github.com/mbvlabs/andurel/pkg/telemetry module. Composition roots, routers, and controllers are application-owned, so Andurel does not rewrite them automatically.\n\n",
		)
		instructions.WriteString(
			"1. Delete the copied telemetry/ package from the application.\n\n",
		)
		instructions.WriteString("2. Add this direct requirement to go.mod:\n\n```go\n")
		fmt.Fprintf(
			&instructions,
			"github.com/mbvlabs/andurel/pkg/telemetry %s\n",
			versions.Telemetry,
		)
		instructions.WriteString("```\n\n")
		instructions.WriteString(
			"3. Keep config.Telemetry as env-backed values. In cmd/app/main.go and cmd/queue/main.go, remove telemetry.Module and provide *telemetry.Telemetry with positional New plus With* options:\n\n```go\n",
		)
		instructions.WriteString(`func newTelemetry(
	lifecycle fx.Lifecycle,
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
	tel, err := telemetry.New(ctx, cfg.ServiceName, cfg.ServiceVersion, opts...)
	if err != nil {
		return nil, err
	}
	lifecycle.Append(fx.Hook{OnStop: tel.Shutdown})
	return tel, nil
}
`)
		instructions.WriteString("```\n\n")
		instructions.WriteString(
			"4. Inject tel *telemetry.Telemetry into newDatabase and pass storage.WithOpenTelemetry(storage.TelemetryConfig{TracerProvider: tel.TracerProvider(), MeterProvider: tel.MeterProvider()}).\n\n",
		)
		instructions.WriteString(
			"5. In router.New, wrap the Echo handler with telemetry.WrapHandler(\"http\", router, tel.TracerProvider()) and attach the handle on each request with middleware.Telemetry(tel). Queue workers should start from tel.Context(ctx).\n\n",
		)
		instructions.WriteString(
			"6. Replace slog.ErrorContext / slog.InfoContext / slog.WarnContext calls in application code with telemetry.Error / telemetry.Info / telemetry.Warn. Controllers use telemetry.From(etx, name) then pass ctx. Services use telemetry.Start. Do not call otel.SetTracerProvider or slog.SetDefault.\n\n",
		)
		instructions.WriteString("7. Format and verify the migration:\n\n```text\n")
		instructions.WriteString(
			"gofmt -w cmd/app/main.go cmd/queue/main.go router/router.go router/middleware/middleware.go\n",
		)
		instructions.WriteString("go fix ./...\ngo vet ./...\n```\n")

		actions = append(actions, ManualAction{
			ID:           "telemetry-package-v2.0.0",
			Title:        "Migrate generated telemetry onto pkg/telemetry",
			Instructions: instructions.String(),
		})
	}

	return actions, nil
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
