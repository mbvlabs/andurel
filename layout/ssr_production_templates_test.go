package layout

import "testing"

func TestGeneratedViteBundlesSSROnlyForProductionBuilds(t *testing.T) {
	for _, name := range []string{
		"inertia_assets_vite_config.tmpl",
		"inertia_react_assets_vite_config.tmpl",
		"inertia_svelte_assets_vite_config.tmpl",
	} {
		assertTemplateContains(t, name,
			"ssr: command === 'build' ? { noExternal: true } : undefined",
		)
	}
}

func TestGeneratedEnvDocumentsSSRFailFastAsOptIn(t *testing.T) {
	assertTemplateContains(t, "env.tmpl",
		"# INERTIA_SSR_FAIL_FAST=false",
		"client-side-rendering fallback",
		"production SSR is served by cmd/ssr",
	)
	// An uncommented override would be copied into .env by every project and
	// would silently turn a production SSR failure into a 200 CSR page.
	assertTemplateMissing(t, "env.tmpl", "\nINERTIA_SSR_FAIL_FAST=false")
	assertTemplateContains(t, "readme.tmpl",
		"`andurel run` (or `pnpm dev` alongside",
		"the `cmd/ssr` sidecar serves renders from",
	)
}

func TestGeneratedRootTitleIsFallbackOnlyWithoutSSR(t *testing.T) {
	assertTemplateContains(t, "inertia_framework_root.tmpl",
		"if data.SSR == nil {",
		"<title>{ data.ProjectName }</title>",
	)
}

func TestGeneratedErrorHandlerRendersThemedServerErrors(t *testing.T) {
	assertTemplateContains(t, "handlers_errors.tmpl",
		"inertia.NewErrorHandler(",
		`FallbackComponent: "Errors/InternalError"`,
		`inertia.MapError(domains.ErrNotFound, http.StatusNotFound, "Errors/NotFound")`,
	)
}

func TestGeneratedQueueLoadsOnlyWorkerConfig(t *testing.T) {
	assertTemplateContains(t, "cmd_queue_main.tmpl", "config.NewWorker()")
	assertTemplateMissing(t, "cmd_queue_main.tmpl", "config.New()")
	assertTemplateContains(t, "config_config.tmpl",
		"type WorkerConfig struct {",
		"func NewWorker() (WorkerConfig, error)",
	)
	assertTemplateContains(t, "runtime_queue.tmpl", "cfg config.WorkerConfig")
}

func TestGeneratedProductionLikeRunCanLogEmail(t *testing.T) {
	assertTemplateContains(t, "config_email.tmpl", `LogDriver MailDriver = "log"`)
	assertTemplateContains(t, "runtime_runtime.tmpl", "email.NewLog(slog.Default())")
	assertTemplateContains(t, "readme.tmpl",
		"EMAIL_PROVIDER=log",
		"logs each message instead of sending it and is allowed in",
	)
}
