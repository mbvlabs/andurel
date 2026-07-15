// Package layout provides functionality to scaffold a new Go web application project
package layout

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/mbvlabs/andurel/layout/blueprint"
	"github.com/mbvlabs/andurel/layout/cmds"
	"github.com/mbvlabs/andurel/layout/extensions"
	"github.com/mbvlabs/andurel/layout/templates"
	"github.com/mbvlabs/andurel/layout/versions"
	"github.com/mbvlabs/andurel/pkg/constants"
)

// Element describes a directory tree node to create during scaffolding.
type Element struct {
	RootDir string
	SubDirs []Element
}

var (
	registerBuiltinOnce sync.Once
	registerBuiltinErr  error
)

// Scaffold creates a new Andurel project in the target directory.
func Scaffold(
	targetDir, projectName, database, version string,
	extensionNames []string,
	inertia, javascriptRuntime string,
) error {
	fmt.Printf("Scaffolding new project in %s...\n", targetDir)

	moduleName := projectName
	secrets, err := generateScaffoldSecrets(rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate scaffold secrets: %w", err)
	}

	blueprint := initializeBlueprint(moduleName)
	templateData := TemplateData{
		AppName:              projectName,
		ProjectName:          projectName,
		ModuleName:           moduleName,
		Database:             database,
		GoVersion:            goVersion,
		SessionKey:           secrets.sessionKey,
		SessionEncryptionKey: secrets.sessionEncryptionKey,
		TokenSigningKey:      secrets.tokenSigningKey,
		Pepper:               secrets.pepper,
		Extensions:           extensionNames,
		RunToolVersion:       GetRunToolVersion(),
		FrameworkVersion:     normalizeFrameworkVersion(version),
		Inertia:              inertia,
		blueprint:            blueprint,
	}

	if err := registerBuiltinExtensions(); err != nil {
		return fmt.Errorf("failed to register builtin extensions: %w", err)
	}

	requestedExtensions, err := resolveExtensions(extensionNames)
	if err != nil {
		return err
	}

	fmt.Print("Creating project structure...\n")
	if err := os.MkdirAll(targetDir, constants.DirPermissionDefault); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	fmt.Print("Initializing git repository...\n")
	if err := initializeGit(targetDir); err != nil {
		return fmt.Errorf("failed to initialize git: %w", err)
	}

	fmt.Print("Creating go.mod file...\n")
	if err := createGoMod(targetDir, &templateData); err != nil {
		return fmt.Errorf("failed to create go.mod: %w", err)
	}

	fmt.Print("Processing templated files...\n")
	if err := processTemplatedFiles(targetDir, &templateData); err != nil {
		return fmt.Errorf("failed to process templated files: %w", err)
	}

	fmt.Print("Processing database migrations...\n")
	nextMigrationTime, err := processMigrations(targetDir, &templateData)
	if err != nil {
		return fmt.Errorf("failed to process migrations: %w", err)
	}

	fmt.Print("Generating andurel.lock file...\n")
	scaffoldConfig := &ScaffoldConfig{
		ProjectName:       projectName,
		Database:          database,
		Inertia:           inertia,
		JavaScriptRuntime: javascriptRuntime,
	}
	if err := generateLockFile(targetDir, version, scaffoldConfig, extensionNames); err != nil {
		fmt.Printf("Warning: failed to generate lock file: %v\n", err)
	}

	type postStep struct {
		extensionName string
		fn            func(targetDir string) error
	}

	var postExtensionSteps []postStep

	if len(requestedExtensions) > 0 {
		fmt.Print("Applying extensions...\n")
	}

	for _, ext := range requestedExtensions {
		currentExt := ext
		fmt.Printf(" - %s\n", currentExt.Name())

		ctx := extensions.Context{
			TargetDir: targetDir,
			Data:      &templateData,
			Inertia:   inertia,
			ProcessTemplate: func(templateFile, targetPath string, data extensions.TemplateData) error {
				if data == nil {
					data = &templateData
				}

				return renderTemplate(targetDir, templateFile, targetPath, extensions.Files, data)
			},
			AddPostStep: func(fn func(targetDir string) error) {
				if fn == nil {
					return
				}

				postExtensionSteps = append(postExtensionSteps, postStep{
					extensionName: currentExt.Name(),
					fn:            fn,
				})
			},
			NextMigrationTime: &nextMigrationTime,
		}

		if err := currentExt.Apply(&ctx); err != nil {
			return fmt.Errorf("failed to apply extension %s: %w", currentExt.Name(), err)
		}

		nextMigrationTime = nextMigrationTime.Add(10 * time.Second)
	}

	// Re-render templates that use blueprint data after extensions have been applied
	if len(requestedExtensions) > 0 {
		if err := rerenderBlueprintTemplates(targetDir, &templateData); err != nil {
			return fmt.Errorf("failed to re-render blueprint templates: %w", err)
		}
	}

	for _, step := range postExtensionSteps {
		if err := step.fn(targetDir); err != nil {
			return fmt.Errorf("extension %s post-step failed: %w", step.extensionName, err)
		}
	}

	fmt.Print("Fixing migration timestamps...\n")
	if err := cmds.RunGooseFix(targetDir); err != nil {
		slog.Error(
			"failed to run goose fix",
			"error",
			err,
			"fix",
			"run 'andurel tool sync' then 'goose -dir database/migrations fix' after sync",
		)
	}

	fmt.Print("Running templ generate...\n")
	if err := cmds.RunTemplGenerate(targetDir); err != nil {
		slog.Error(
			"failed to run templ generate",
			"error",
			err,
			"fix",
			"run 'andurel template generate' after sync",
		)
	}

	fmt.Print("Running go mod tidy...\n")
	if err := cmds.RunGoModTidy(targetDir); err != nil {
		slog.Error(
			"failed to run go mod tidy",
			"error",
			err,
			"fix",
			"run 'go mod tidy' after sync",
		)
	}

	fmt.Print("Running go fmt...\n")
	if err := cmds.RunGoFmt(targetDir); err != nil {
		slog.Error(
			"failed to run go fmt",
			"error",
			err,
		)
	}

	return nil
}

type (
	// TmplTarget represents tmpl target.
	TmplTarget string
	// TmplTargetPath represents tmpl target path.
	TmplTargetPath string
)

// FrameworkManagedFile identifies a template-backed file that Andurel owns.
type FrameworkManagedFile struct {
	TemplateName string
	TargetPath   string
}

var baseStyleTemplateMappings = map[TmplTarget]TmplTargetPath{
	"css_base.tmpl": "css/base.css",

	// Views
	"views_layout.tmpl":  "views/layout.templ",
	"views_welcome.tmpl": "views/welcome.templ",

	// Views - Pages
	"views_bad_request.tmpl":    "views/bad_request.templ",
	"views_internal_error.tmpl": "views/internal_error.templ",
	"views_not_found.tmpl":      "views/not_found.templ",
	"views_confirm_email.tmpl":  "views/confirm_email.templ",
	"views_login.tmpl":          "views/login.templ",
	"views_registration.tmpl":   "views/registration.templ",
	"views_reset_password.tmpl": "views/reset_password.templ",

	// Views
	"views_head.tmpl": "views/head.templ",
}

var baseTemplateMappings = map[TmplTarget]TmplTargetPath{
	"env.tmpl":       ".env.example",
	"gitignore.tmpl": ".gitignore",
	"readme.tmpl":    "README.md",

	// Core files
	"framework_elements_request_context.tmpl":        "internal/request/context.go",
	"framework_elements_request_request.tmpl":        "internal/request/request.go",
	"framework_elements_routing_definitions.tmpl":    "internal/routing/definitions.go",
	"framework_elements_routing_routes.tmpl":         "internal/routing/routes.go",
	"framework_elements_server_server.tmpl":          "internal/server/server.go",
	"framework_elements_storage_psql.tmpl":           "internal/storage/psql.go",
	"framework_elements_storage_queue.tmpl":          "internal/storage/queue.go",
	"framework_elements_hypermedia_signals.tmpl":     "internal/hypermedia/signals.go",
	"framework_elements_hypermedia_core.tmpl":        "internal/hypermedia/core.go",
	"framework_elements_hypermedia_options.tmpl":     "internal/hypermedia/options.go",
	"framework_elements_hypermedia_render.tmpl":      "internal/hypermedia/render.go",
	"framework_elements_hypermedia_script.tmpl":      "internal/hypermedia/script.go",
	"framework_elements_hypermedia_sse.tmpl":         "internal/hypermedia/sse.go",
	"framework_elements_hypermedia_broadcaster.tmpl": "internal/hypermedia/broadcaster.go",
	"framework_elements_hypermedia_helpers.tmpl":     "internal/hypermedia/helpers.go",

	// Validation
	"framework_elements_validation_validation.tmpl": "internal/validation/validation.go",
	"framework_elements_validation_rules.tmpl":      "internal/validation/rules.go",
	"framework_elements_validation_helpers.tmpl":    "internal/validation/helpers.go",

	// Assets
	"assets_assets.tmpl":      "assets/assets.go",
	"assets_css_style.tmpl":   "assets/css/style.css",
	"assets_js_scripts.tmpl":  "assets/js/scripts.js",
	"assets_js_datastar.tmpl": "assets/js/datastar_1-0-1.min.js",

	// Commands
	"cmd_app_main.tmpl":      "cmd/app/main.go",
	"cmd_app_main_test.tmpl": "cmd/app/main_test.go",
	"cmd_seeds_main.tmpl":    "cmd/seeds/main.go",

	// Config
	"config_app.tmpl":       "config/app.go",
	"config_config.tmpl":    "config/config.go",
	"config_database.tmpl":  "config/database.go",
	"config_telemetry.tmpl": "config/telemetry.go",
	"config_email.tmpl":     "config/email.go",

	// Clients
	"clients_email_mailpit.tmpl": "clients/email/mailpit.go",

	// Controllers
	"controllers_api.tmpl":        "controllers/api/api.go",
	"controllers_assets.tmpl":     "controllers/assets.go",
	"controllers_cache.tmpl":      "controllers/cache.go",
	"controllers_controller.tmpl": "controllers/controller.go",
	"controllers_pages.tmpl":      "controllers/pages.go",

	// Database
	"database_migrations_gitkeep.tmpl": "database/migrations/.gitkeep",
	"database_seeds_seeds.tmpl":        "database/seeds/seeds.go",
	"psql_database.tmpl":               "database/database.go",

	// Queue package
	"psql_queue_queue.tmpl":                            "queue/queue.go",
	"psql_queue_jobs_send_transactional_email.tmpl":    "queue/jobs/send_transactional_email.go",
	"psql_queue_jobs_send_marketing_email.tmpl":        "queue/jobs/send_marketing_email.go",
	"psql_queue_workers_workers.tmpl":                  "queue/workers.go",
	"psql_queue_workers_send_transactional_email.tmpl": "queue/send_transactional_email.go",
	"psql_queue_workers_send_marketing_email.tmpl":     "queue/send_marketing_email.go",

	// Email
	"email_email.tmpl":       "email/email.go",
	"email_base_layout.tmpl": "email/base_layout.templ",
	"email_components.tmpl":  "email/components.templ",

	// Models
	"models_errors.tmpl": "models/errors.go",
	"models_model.tmpl":  "models/model.go",
	"models_token.tmpl":  "models/token.go",
	"models_user.tmpl":   "models/user.go",

	"models_factories_factories.tmpl": "models/factories/factories.go",
	"models_factories_user.tmpl":      "models/factories/user.go",
	"models_factories_token.tmpl":     "models/factories/token.go",

	// Router
	"router_router.tmpl":                     "router/router.go",
	"router_router_test.tmpl":                "router/router_test.go",
	"router_cookies_cookies.tmpl":            "router/cookies/cookies.go",
	"router_cookies_flash.tmpl":              "router/cookies/flash.go",
	"router_middleware_middleware.tmpl":      "router/middleware/middleware.go",
	"router_middleware_middleware_test.tmpl": "router/middleware/middleware_test.go",

	// Routes
	"router_routes_api.tmpl":    "router/routes/api.go",
	"router_routes_assets.tmpl": "router/routes/assets.go",
	"router_routes_pages.tmpl":  "router/routes/pages.go",

	// Telemetry
	"telemetry_telemetry.tmpl":        "telemetry/telemetry.go",
	"telemetry_options.tmpl":          "telemetry/options.go",
	"telemetry_logger.tmpl":           "telemetry/logger.go",
	"telemetry_log_exporters.tmpl":    "telemetry/log_exporters.go",
	"telemetry_metrics.tmpl":          "telemetry/metrics.go",
	"telemetry_metric_exporters.tmpl": "telemetry/metric_exporters.go",
	"telemetry_tracer.tmpl":           "telemetry/tracer.go",
	"telemetry_trace_exporters.tmpl":  "telemetry/trace_exporters.go",
	"telemetry_helpers.tmpl":          "telemetry/helpers.go",

	// Auth - Controllers
	"controllers_confirmations.tmpl":   "controllers/confirmations.go",
	"controllers_registrations.tmpl":   "controllers/registrations.go",
	"controllers_reset_passwords.tmpl": "controllers/reset_passwords.go",
	"controllers_sessions.tmpl":        "controllers/sessions.go",

	// Auth - Config
	"config_auth.tmpl": "config/auth.go",

	// Auth - Services
	"services_service.tmpl":             "services/service.go",
	"services_identity.tmpl":            "services/identity.go",
	"services_authentication.tmpl":      "services/authentication.go",
	"services_authentication_test.tmpl": "services/authentication_test.go",
	"services_registration.tmpl":        "services/registration.go",
	"services_reset_password.tmpl":      "services/reset_password.go",

	// Auth - Router
	"router_routes_users.tmpl":         "router/routes/users.go",
	"router_middleware_auth.tmpl":      "router/middleware/auth.go",
	"router_middleware_auth_test.tmpl": "router/middleware/auth_test.go",

	// Auth - Email
	"email_reset_password.tmpl": "email/reset_password.templ",
	"email_verify_email.tmpl":   "email/verify_email.templ",
}

var inertiaSharedTemplateMappings = map[TmplTarget]TmplTargetPath{
	"inertia_framework_root_html.tmpl": "assets/inertia/root.go.html",
	"inertia_page_options.tmpl":        "internal/inertia/page_options.go",
	"inertia_render.tmpl":              "internal/inertia/render.go",
	"inertia_assets_routes.tmpl":       "resources/js/routes.ts",
	"inertia_vite.tmpl":                "internal/inertia/vite.go",
}

var inertiaVueTemplateMappings = map[TmplTarget]TmplTargetPath{
	"inertia_assets_app.tmpl":                               "resources/js/app.ts",
	"inertia_assets_layouts_layout.tmpl":                    "resources/js/Layouts/Layout.vue",
	"inertia_assets_pages_auth_confirm_email.tmpl":          "resources/js/Pages/Auth/ConfirmEmail.vue",
	"inertia_assets_pages_auth_login.tmpl":                  "resources/js/Pages/Auth/Login.vue",
	"inertia_assets_pages_auth_registration.tmpl":           "resources/js/Pages/Auth/Registration.vue",
	"inertia_assets_pages_auth_reset_password.tmpl":         "resources/js/Pages/Auth/ResetPassword.vue",
	"inertia_assets_pages_auth_reset_password_request.tmpl": "resources/js/Pages/Auth/ResetPasswordRequest.vue",
	"inertia_assets_pages_errors_bad_request.tmpl":          "resources/js/Pages/Errors/BadRequest.vue",
	"inertia_assets_pages_errors_internal_error.tmpl":       "resources/js/Pages/Errors/InternalError.vue",
	"inertia_assets_pages_errors_not_found.tmpl":            "resources/js/Pages/Errors/NotFound.vue",
	"inertia_assets_vite_config.tmpl":                       "vite.config.ts",
	"inertia_assets_package_json.tmpl":                      "package.json",
	"inertia_assets_tsconfig.tmpl":                          "tsconfig.json",
}

var inertiaReactTemplateMappings = map[TmplTarget]TmplTargetPath{
	"inertia_react_assets_app.tmpl":                               "resources/js/app.tsx",
	"inertia_react_assets_layouts_layout.tmpl":                    "resources/js/Layouts/Layout.tsx",
	"inertia_react_assets_pages_auth_confirm_email.tmpl":          "resources/js/Pages/Auth/ConfirmEmail.tsx",
	"inertia_react_assets_pages_auth_login.tmpl":                  "resources/js/Pages/Auth/Login.tsx",
	"inertia_react_assets_pages_auth_registration.tmpl":           "resources/js/Pages/Auth/Registration.tsx",
	"inertia_react_assets_pages_auth_reset_password.tmpl":         "resources/js/Pages/Auth/ResetPassword.tsx",
	"inertia_react_assets_pages_auth_reset_password_request.tmpl": "resources/js/Pages/Auth/ResetPasswordRequest.tsx",
	"inertia_react_assets_pages_errors_bad_request.tmpl":          "resources/js/Pages/Errors/BadRequest.tsx",
	"inertia_react_assets_pages_errors_internal_error.tmpl":       "resources/js/Pages/Errors/InternalError.tsx",
	"inertia_react_assets_pages_errors_not_found.tmpl":            "resources/js/Pages/Errors/NotFound.tsx",
	"inertia_react_assets_vite_config.tmpl":                       "vite.config.ts",
	"inertia_react_assets_package_json.tmpl":                      "package.json",
	"inertia_react_assets_tsconfig.tmpl":                          "tsconfig.json",
}

var inertiaSvelteTemplateMappings = map[TmplTarget]TmplTargetPath{
	"inertia_svelte_assets_app.tmpl":                               "resources/js/app.ts",
	"inertia_svelte_assets_components_flash_toasts.tmpl":           "resources/js/Components/FlashToasts.svelte",
	"inertia_svelte_assets_layouts_layout.tmpl":                    "resources/js/Layouts/Layout.svelte",
	"inertia_svelte_assets_pages_auth_confirm_email.tmpl":          "resources/js/Pages/Auth/ConfirmEmail.svelte",
	"inertia_svelte_assets_pages_auth_login.tmpl":                  "resources/js/Pages/Auth/Login.svelte",
	"inertia_svelte_assets_pages_auth_registration.tmpl":           "resources/js/Pages/Auth/Registration.svelte",
	"inertia_svelte_assets_pages_auth_reset_password.tmpl":         "resources/js/Pages/Auth/ResetPassword.svelte",
	"inertia_svelte_assets_pages_auth_reset_password_request.tmpl": "resources/js/Pages/Auth/ResetPasswordRequest.svelte",
	"inertia_svelte_assets_pages_errors_bad_request.tmpl":          "resources/js/Pages/Errors/BadRequest.svelte",
	"inertia_svelte_assets_pages_errors_internal_error.tmpl":       "resources/js/Pages/Errors/InternalError.svelte",
	"inertia_svelte_assets_pages_errors_not_found.tmpl":            "resources/js/Pages/Errors/NotFound.svelte",
	"inertia_svelte_assets_vite_config.tmpl":                       "vite.config.ts",
	"inertia_svelte_assets_package_json.tmpl":                      "package.json",
	"inertia_svelte_assets_tsconfig.tmpl":                          "tsconfig.json",
	"inertia_svelte_assets_svelte_config.tmpl":                     "svelte.config.js",
}

var inertiaSkippedTemplates = map[TmplTarget]bool{
	"views_confirm_email.tmpl":  true,
	"views_login.tmpl":          true,
	"views_registration.tmpl":   true,
	"views_reset_password.tmpl": true,
}

func inertiaAdapterTemplateMappings(adapter string) map[TmplTarget]TmplTargetPath {
	switch adapter {
	case "vue":
		return inertiaVueTemplateMappings
	case "react":
		return inertiaReactTemplateMappings
	case "svelte":
		return inertiaSvelteTemplateMappings
	default:
		return nil
	}
}

func isStaticInertiaAssetTemplate(templateFile TmplTarget) bool {
	return strings.HasPrefix(string(templateFile), "inertia_assets_") ||
		strings.HasPrefix(string(templateFile), "inertia_react_assets_") ||
		strings.HasPrefix(string(templateFile), "inertia_svelte_assets_") ||
		templateFile == "inertia_framework_root_html.tmpl"
}

// GetInternalFrameworkFiles returns the internal package files expected for a project config.
func GetInternalFrameworkFiles(config *ScaffoldConfig) []FrameworkManagedFile {
	mappings := make(map[TmplTarget]TmplTargetPath)
	for templateName, targetPath := range baseTemplateMappings {
		if strings.HasPrefix(string(targetPath), "internal/") {
			mappings[templateName] = targetPath
		}
	}

	if config != nil && IsSupportedInertiaAdapter(config.Inertia) {
		for templateName, targetPath := range inertiaSharedTemplateMappings {
			if strings.HasPrefix(string(targetPath), "internal/") {
				mappings[templateName] = targetPath
			}
		}
	}

	return sortedFrameworkManagedFiles(mappings)
}

// GetAllManagedInternalFrameworkFiles returns every internal package file Andurel can manage.
func GetAllManagedInternalFrameworkFiles() []FrameworkManagedFile {
	mappings := make(map[TmplTarget]TmplTargetPath)
	for templateName, targetPath := range baseTemplateMappings {
		if strings.HasPrefix(string(targetPath), "internal/") {
			mappings[templateName] = targetPath
		}
	}
	for templateName, targetPath := range inertiaSharedTemplateMappings {
		if strings.HasPrefix(string(targetPath), "internal/") {
			mappings[templateName] = targetPath
		}
	}

	return sortedFrameworkManagedFiles(mappings)
}

func sortedFrameworkManagedFiles(mappings map[TmplTarget]TmplTargetPath) []FrameworkManagedFile {
	files := make([]FrameworkManagedFile, 0, len(mappings))
	for templateName, targetPath := range mappings {
		files = append(files, FrameworkManagedFile{
			TemplateName: string(templateName),
			TargetPath:   string(targetPath),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].TargetPath < files[j].TargetPath
	})

	return files
}

func processTemplatedFiles(targetDir string, data extensions.TemplateData) error {
	mappings := make(map[TmplTarget]TmplTargetPath, len(baseTemplateMappings)+len(inertiaSharedTemplateMappings)+len(inertiaVueTemplateMappings))
	maps.Copy(mappings, baseTemplateMappings)

	if td, ok := data.(*TemplateData); ok && IsSupportedInertiaAdapter(td.Inertia) {
		for k := range inertiaSkippedTemplates {
			delete(mappings, k)
		}
		delete(mappings, "controllers_pages.tmpl")
		delete(mappings, "controllers_confirmations.tmpl")
		delete(mappings, "controllers_registrations.tmpl")
		delete(mappings, "controllers_reset_passwords.tmpl")
		delete(mappings, "controllers_sessions.tmpl")
		mappings["controllers_pages_inertia.tmpl"] = "controllers/pages.go"
		mappings["controllers_confirmations_inertia.tmpl"] = "controllers/confirmations.go"
		mappings["controllers_registrations_inertia.tmpl"] = "controllers/registrations.go"
		mappings["controllers_reset_passwords_inertia.tmpl"] = "controllers/reset_passwords.go"
		mappings["controllers_sessions_inertia.tmpl"] = "controllers/sessions.go"
		maps.Copy(mappings, inertiaSharedTemplateMappings)
		maps.Copy(mappings, inertiaAdapterTemplateMappings(td.Inertia))
	}

	for templateFile, targetPath := range mappings {
		if templateFile == "assets_js_datastar.tmpl" {
			if err := copyFile(targetDir, string(templateFile), string(targetPath), templates.Files); err != nil {
				return fmt.Errorf("failed to copy file %s: %w", templateFile, err)
			}
			continue
		}
		if isStaticInertiaAssetTemplate(templateFile) {
			if err := copyFile(targetDir, string(templateFile), string(targetPath), templates.Files); err != nil {
				return fmt.Errorf("failed to copy file %s: %w", templateFile, err)
			}
			continue
		}
		if err := renderTemplate(targetDir, string(templateFile), string(targetPath), templates.Files, data); err != nil {
			return fmt.Errorf("failed to process template %s: %w", templateFile, err)
		}
	}

	for templateFile, targetPath := range baseStyleTemplateMappings {
		if td, ok := data.(*TemplateData); ok && IsSupportedInertiaAdapter(td.Inertia) && inertiaSkippedTemplates[templateFile] {
			continue
		}
		if err := renderTemplate(targetDir, string(templateFile), string(targetPath), templates.Files, data); err != nil {
			return fmt.Errorf("failed to process style template %s: %w", templateFile, err)
		}
	}

	return nil
}

func processMigrations(
	targetDir string,
	data extensions.TemplateData,
) (time.Time, error) {
	baseTime := time.Now()

	if os.Getenv("ANDUREL_TEST_MODE") == "true" {
		baseTime = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	migrations := []struct {
		template string
		name     string
		offset   time.Duration
	}{
		// River queue migrations
		{"psql_riverqueue_migration_one.tmpl", "create_river_migration_table", 0},
		{
			"psql_riverqueue_migration_two.tmpl",
			"create_river_job_and_leader_tables",
			1 * time.Second,
		},
		{"psql_riverqueue_migration_three.tmpl", "alter_river_job_tags", 2 * time.Second},
		{
			"psql_riverqueue_migration_four.tmpl",
			"alter_river_job_args_metadata_add_queue",
			3 * time.Second,
		},
		{
			"psql_riverqueue_migration_five.tmpl",
			"add_river_job_unique_key_and_clients",
			4 * time.Second,
		},
		{"psql_riverqueue_migration_six.tmpl", "add_river_job_unique_states", 5 * time.Second},
		// Auth migrations
		{"database_migrations_users.tmpl", "create_users_table", 6 * time.Second},
		{"database_migrations_tokens.tmpl", "create_tokens_table", 7 * time.Second},
	}

	var lastTime time.Time
	for _, migration := range migrations {
		lastTime = baseTime.Add(migration.offset)
		timestamp := lastTime.Format("20060102150405")
		targetPath := fmt.Sprintf("database/migrations/%s_%s.sql", timestamp, migration.name)

		if err := renderTemplate(targetDir, migration.template, targetPath, templates.Files, data); err != nil {
			return time.Time{}, fmt.Errorf(
				"failed to process migration %s: %w",
				migration.template,
				err,
			)
		}
	}

	return lastTime.Add(1 * time.Second), nil
}

func rerenderBlueprintTemplates(targetDir string, data extensions.TemplateData) error {
	if data == nil {
		return fmt.Errorf("template data is nil")
	}

	if _, ok := data.(*TemplateData); !ok {
		return fmt.Errorf("template data is not *TemplateData")
	}

	// Templates to re-render after extensions have been applied
	blueprintTemplates := []TmplTarget{
		"config_config.tmpl",
		"env.tmpl",
		"framework_elements_request_context.tmpl",
		"framework_elements_request_request.tmpl",
		"router_cookies_cookies.tmpl",
	}

	blueprintTemplates = append(blueprintTemplates,
		"cmd_app_main.tmpl",
		"controllers_controller.tmpl",
	)

	for _, tmplName := range blueprintTemplates {
		targetPath, ok := baseTemplateMappings[tmplName]
		if !ok {
			return fmt.Errorf("template mapping missing for blueprint template %s", tmplName)
		}

		if err := renderTemplate(targetDir, string(tmplName), string(targetPath), templates.Files, data); err != nil {
			return fmt.Errorf("failed to render blueprint template %s: %w", tmplName, err)
		}
	}

	if err := renderTemplate(targetDir, "go_mod.tmpl", "go.mod", templates.Files, data); err != nil {
		return fmt.Errorf("failed to render go.mod template: %w", err)
	}

	if err := renderTemplate(targetDir, "css_base.tmpl", "css/base.css", templates.Files, data); err != nil {
		return fmt.Errorf("failed to render css base template: %w", err)
	}

	return nil
}

func copyFile(
	targetDir, sourceFile, targetPath string,
	fsys fs.FS,
) error {
	content, err := fs.ReadFile(fsys, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", sourceFile, err)
	}

	fullTargetPath := filepath.Join(targetDir, targetPath)
	dir := filepath.Dir(fullTargetPath)
	if err := os.MkdirAll(dir, constants.DirPermissionDefault); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", targetPath, err)
	}

	if err := os.WriteFile(fullTargetPath, content, constants.FilePermissionPublic); err != nil {
		return fmt.Errorf("failed to write file %s: %w", targetPath, err)
	}

	return nil
}

func renderTemplate(
	targetDir, templateFile, targetPath string,
	fsys fs.FS,
	data extensions.TemplateData,
) error {
	content, err := fs.ReadFile(fsys, templateFile)
	if err != nil {
		return fmt.Errorf("failed to read template %s: %w", templateFile, err)
	}

	if data == nil {
		data = &TemplateData{}
	}

	contentStr := string(content)

	tmpl, err := template.New(templateFile).
		Funcs(templateFuncMap()).
		Parse(contentStr)
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", templateFile, err)
	}

	fullTargetPath := filepath.Join(targetDir, targetPath)
	dir := filepath.Dir(fullTargetPath)
	if err := os.MkdirAll(dir, constants.DirPermissionDefault); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", targetPath, err)
	}

	tmpFile, err := os.CreateTemp(dir, ".layout-tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file for %s: %w", targetPath, err)
	}
	tmpPath := tmpFile.Name()
	shouldCleanup := true
	defer func() {
		if shouldCleanup {
			if removeErr := os.Remove(tmpPath); removeErr != nil &&
				!errors.Is(removeErr, os.ErrNotExist) {
				slog.Debug(
					"layout: failed to cleanup temporary file",
					"path",
					tmpPath,
					"error",
					removeErr,
				)
			}
		}
	}()

	if err := tmpl.Execute(tmpFile, data); err != nil {
		return errors.Join(
			fmt.Errorf("failed to execute template %s: %w", templateFile, err),
			tmpFile.Close(),
		)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file for %s: %w", targetPath, err)
	}

	if err := os.Chmod(tmpPath, constants.FilePermissionPublic); err != nil {
		return fmt.Errorf("failed to set permissions for %s: %w", targetPath, err)
	}

	if err := os.Rename(tmpPath, fullTargetPath); err != nil {
		if removeErr := os.Remove(fullTargetPath); removeErr == nil ||
			errors.Is(removeErr, os.ErrNotExist) {
			if renameErr := os.Rename(tmpPath, fullTargetPath); renameErr == nil {
				shouldCleanup = false
				return nil
			}
		}

		return fmt.Errorf("failed to move temporary file into place for %s: %w", targetPath, err)
	}

	shouldCleanup = false
	return nil
}

func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"hasExtension": hasExtension,
		"lower":        strings.ToLower,
	}
}

func hasExtension(extensions []string, name string) bool {
	return slices.Contains(extensions, name)
}

func registerBuiltinExtensions() error {
	registerBuiltinOnce.Do(func() {
		builtin := []extensions.Extension{
			extensions.AwsSes{},
			extensions.Docker{},
			extensions.CssComponents{},
		}

		for _, ext := range builtin {
			if err := extensions.Register(ext); err != nil {
				registerBuiltinErr = fmt.Errorf("register %s: %w", ext.Name(), err)
				return
			}
		}
	})

	return registerBuiltinErr
}

// AvailableExtensionNames returns the sorted names of built-in extensions.
func AvailableExtensionNames() ([]string, error) {
	if err := registerBuiltinExtensions(); err != nil {
		return nil, err
	}
	return extensions.Names(), nil
}

func resolveExtensions(names []string) ([]extensions.Extension, error) {
	if len(names) == 0 {
		return nil, nil
	}

	// First pass: collect all requested extensions and validate they exist
	requested := make(map[string]struct{})
	for _, rawName := range names {
		name := strings.TrimSpace(rawName)
		if name == "" {
			return nil, fmt.Errorf("extension name cannot be empty")
		}

		if _, ok := extensions.Get(name); !ok {
			available := strings.Join(extensions.Names(), ", ")
			if available == "" {
				return nil, fmt.Errorf("unknown extension %q", name)
			}
			return nil, fmt.Errorf(
				"unknown extension %q. available extensions: %s",
				name,
				available,
			)
		}

		requested[name] = struct{}{}
	}

	// Build complete dependency graph (includes transitive dependencies)
	allNeeded := make(map[string]struct{})
	if err := collectDependencies(requested, allNeeded); err != nil {
		return nil, err
	}

	// Topologically sort extensions so dependencies come before dependents
	sorted, err := topologicalSort(allNeeded)
	if err != nil {
		return nil, err
	}

	// Convert names to extension instances
	resolved := make([]extensions.Extension, 0, len(sorted))
	for _, name := range sorted {
		ext, _ := extensions.Get(name)
		resolved = append(resolved, ext)
	}

	return resolved, nil
}

// collectDependencies recursively gathers all extensions needed, including transitive dependencies
func collectDependencies(requested map[string]struct{}, allNeeded map[string]struct{}) error {
	for name := range requested {
		if _, seen := allNeeded[name]; seen {
			continue
		}

		ext, ok := extensions.Get(name)
		if !ok {
			return fmt.Errorf("unknown extension %q", name)
		}

		allNeeded[name] = struct{}{}

		// Recursively add dependencies
		deps := ext.Dependencies()
		if len(deps) > 0 {
			depsMap := make(map[string]struct{}, len(deps))
			for _, dep := range deps {
				dep = strings.TrimSpace(dep)
				if dep == "" {
					continue
				}
				if dep == name {
					return fmt.Errorf("extension %q cannot depend on itself", name)
				}
				depsMap[dep] = struct{}{}
			}

			if err := collectDependencies(depsMap, allNeeded); err != nil {
				return err
			}
		}
	}

	return nil
}

// topologicalSort orders extensions so dependencies are applied before dependents.
// Returns an error if a circular dependency is detected.
func topologicalSort(extSet map[string]struct{}) ([]string, error) {
	const (
		unvisited = 0
		visiting  = 1
		visited   = 2
	)

	state := make(map[string]int)
	var result []string
	var path []string // for cycle detection

	var visit func(string) error
	visit = func(name string) error {
		switch state[name] {
		case visited:
			return nil
		case visiting:
			// Found a cycle
			cycle := append(path, name)
			return fmt.Errorf("circular dependency detected: %s", strings.Join(cycle, " -> "))
		}

		state[name] = visiting
		path = append(path, name)

		ext, ok := extensions.Get(name)
		if !ok {
			return fmt.Errorf("unknown extension %q", name)
		}

		// Visit dependencies first
		for _, dep := range ext.Dependencies() {
			dep = strings.TrimSpace(dep)
			if dep == "" {
				continue
			}
			if err := visit(dep); err != nil {
				return err
			}
		}

		path = path[:len(path)-1]
		state[name] = visited
		result = append(result, name)
		return nil
	}

	// Visit all extensions
	for name := range extSet {
		if err := visit(name); err != nil {
			return nil, err
		}
	}

	return result, nil
}

const goVersion = "1.26.5"

// GoTool represents go tool.
type GoTool struct {
	Name    string
	Source  string
	Version string
}

// DefaultGoTools provides default go tools.
var DefaultGoTools = []GoTool{
	{Name: "templ", Source: "github.com/a-h/templ/cmd/templ", Version: versions.Templ},
	{Name: "goose", Source: "github.com/pressly/goose/v3/cmd/goose", Version: versions.Goose},
	{Name: "mailpit", Source: "github.com/axllent/mailpit", Version: versions.Mailpit},
	{Name: "usql", Source: "github.com/xo/usql", Version: versions.Usql},
	{Name: "dblab", Source: "github.com/danvergara/dblab", Version: versions.Dblab},
	{Name: "shadowfax", Source: "github.com/mbvlabs/shadowfax", Version: versions.Shadowfax},
}

var defaultTools = []string{
	"github.com/a-h/templ/cmd/templ",
	"github.com/pressly/goose/v3/cmd/goose",
	"github.com/axllent/mailpit",
	"github.com/xo/usql",
	"github.com/danvergara/dblab",
	"github.com/mbvlabs/shadowfax",
}

// GetExpectedTools returns the list of tools that should exist for a given scaffold config
func GetExpectedTools(config *ScaffoldConfig) map[string]*Tool {
	expectedTools := make(map[string]*Tool)

	// Add all default Go tools
	for _, tool := range DefaultGoTools {
		sourceRepo := extractRepo(tool.Source)
		expectedTools[tool.Name] = NewGoTool(tool.Name, sourceRepo, tool.Version)
	}

	expectedTools["tailwindcli"] = NewBinaryTool("tailwindcli", versions.TailwindCLI)

	return expectedTools
}

// GetRunToolVersion returns the version of the run tool
func GetRunToolVersion() string {
	return versions.Shadowfax
}

func normalizeFrameworkVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "dev"
	}
	return version
}

func createGoMod(targetDir string, data *TemplateData) error {
	if data == nil {
		return fmt.Errorf("template data is nil")
	}

	if err := renderTemplate(targetDir, "go_mod.tmpl", "go.mod", templates.Files, data); err != nil {
		return fmt.Errorf("failed to render go.mod template: %w", err)
	}

	return nil
}

func initializeGit(targetDir string) error {
	cmd := exec.Command("git", "init")
	cmd.Dir = targetDir
	return cmd.Run()
}

type scaffoldSecrets struct {
	sessionKey           string
	sessionEncryptionKey string
	tokenSigningKey      string
	pepper               string
}

func generateScaffoldSecrets(reader io.Reader) (scaffoldSecrets, error) {
	var secrets scaffoldSecrets
	var err error

	secrets.sessionKey, err = generateRandomHex(reader, 64)
	if err != nil {
		return scaffoldSecrets{}, fmt.Errorf("generate session key: %w", err)
	}

	secrets.sessionEncryptionKey, err = generateRandomHex(reader, 32)
	if err != nil {
		return scaffoldSecrets{}, fmt.Errorf("generate session encryption key: %w", err)
	}

	secrets.tokenSigningKey, err = generateRandomHex(reader, 32)
	if err != nil {
		return scaffoldSecrets{}, fmt.Errorf("generate token signing key: %w", err)
	}

	secrets.pepper, err = generateRandomHex(reader, 12)
	if err != nil {
		return scaffoldSecrets{}, fmt.Errorf("generate pepper: %w", err)
	}

	return secrets, nil
}

func generateRandomHex(reader io.Reader, bytes int) (string, error) {
	randomBytes := make([]byte, bytes)
	if _, err := io.ReadFull(reader, randomBytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(randomBytes), nil
}

// initializeBlueprint creates a blueprint with default base configuration
// for controllers, routes, and other scaffold components.
func initializeBlueprint(moduleName string) *blueprint.Blueprint {
	builder := blueprint.NewBuilder(nil)

	builder.AddControllerImport(fmt.Sprintf("%s/controllers", moduleName))
	builder.AddControllerImport(fmt.Sprintf("%s/config", moduleName))

	builder.AddConfigField("Email", "email")
	builder.AddConfigField("Auth", "auth")

	builder.AddWorkerDependency("transactionalSender", "email.TransactionalSender")
	builder.AddWorkerDependency("marketingSender", "email.MarketingSender")

	// Auth cookies configuration
	builder.AddCookiesImport("github.com/google/uuid")
	builder.AddCookiesImport(fmt.Sprintf("%s/models", moduleName))

	builder.AddCookiesConstant("isAuthenticated", "is_authenticated")
	builder.AddCookiesConstant("isAdmin", "is_admin")
	builder.AddCookiesConstant("userID", "user_id")

	builder.AddCookiesAppField("UserID", "uuid.UUID")
	builder.AddCookiesAppField("IsAdmin", "bool")
	builder.AddCookiesAppField("IsAuthenticated", "bool")

	builder.SetCookiesCreateSessionCode(`	sess.Values[isAuthenticated] = true
	sess.Values[isAdmin] = user.IsAdmin
	sess.Values[userID] = user.ID.String()`)

	builder.SetCookiesGetSessionCode(`	if v, ok := sess.Values[isAuthenticated].(bool); ok {
		app.IsAuthenticated = v
	}
	if v, ok := sess.Values[isAdmin].(bool); ok {
		app.IsAdmin = v
	}
	if v, ok := sess.Values[userID].(string); ok {
		app.UserID, _ = uuid.Parse(v)
	}`)

	for _, tool := range defaultTools {
		builder.AddTool(tool)
	}

	return builder.Blueprint()
}

func generateLockFile(targetDir, version string, config *ScaffoldConfig, extensions []string) error {
	lock := NewAndurelLock(version)
	lock.ScaffoldConfig = config
	lock.DatabaseConfig = &DatabaseConfig{
		NullType: "sql.Null",
	}

	for _, tool := range DefaultGoTools {
		sourceRepo := extractRepo(tool.Source)
		lock.AddTool(tool.Name, NewGoTool(tool.Name, sourceRepo, tool.Version))
	}

	lock.AddTool("tailwindcli", NewBinaryTool("tailwindcli", versions.TailwindCLI))

	for _, ext := range extensions {
		lock.AddExtension(ext, time.Now().Format(time.RFC3339))
	}

	return lock.WriteLockFile(targetDir)
}

func extractRepo(module string) string {
	parts := strings.Split(module, "/")
	if len(parts) >= 3 {
		return strings.Join(parts[:3], "/")
	}
	return module
}
