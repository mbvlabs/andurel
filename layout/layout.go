// Package layout provides functionality to scaffold a new Go web application project
package layout

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
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

type Element struct {
	RootDir string
	SubDirs []Element
}

var (
	registerBuiltinOnce sync.Once
	registerBuiltinErr  error
)

func Scaffold(
	targetDir, projectName, database, cssFramework, version string,
	extensionNames []string,
	diMode string,
	inertia string,
) error {
	if diMode == "" {
		diMode = "uberfx"
	}

	fmt.Printf("Scaffolding new project in %s...\n", targetDir)

	moduleName := projectName

	blueprint := initializeBaseBlueprint(moduleName, diMode, inertia)
	templateData := TemplateData{
		AppName:              projectName,
		ProjectName:          projectName,
		ModuleName:           moduleName,
		Database:             database,
		CSSFramework:         cssFramework,
		GoVersion:            goVersion,
		SessionKey:           generateRandomHex(64),
		SessionEncryptionKey: generateRandomHex(32),
		TokenSigningKey:      generateRandomHex(32),
		Pepper:               generateRandomHex(12),
		Extensions:           extensionNames,
		RunToolVersion:       GetRunToolVersion(),
		FrameworkVersion:     normalizeFrameworkVersion(version),
		DIMode:               diMode,
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
	if err := processTemplatedFiles(targetDir, templateData.CSSFramework, &templateData); err != nil {
		return fmt.Errorf("failed to process templated files: %w", err)
	}

	fmt.Print("Processing database migrations...\n")
	nextMigrationTime, err := processMigrations(targetDir, &templateData)
	if err != nil {
		return fmt.Errorf("failed to process migrations: %w", err)
	}

	// if err := os.Mkdir(filepath.Join(targetDir, "bin"), constants.DirPermissionDefault); err != nil {
	// 	return fmt.Errorf("failed to create bin directory: %w", err)
	// }

	fmt.Print("Generating andurel.lock file...\n")
	scaffoldConfig := &ScaffoldConfig{
		ProjectName:  projectName,
		Database:     database,
		CSSFramework: cssFramework,
		DIMode:       diMode,
		Inertia:      inertia,
	}
	if err := generateLockFile(targetDir, version, templateData.CSSFramework == "tailwind", scaffoldConfig, extensionNames); err != nil {
		fmt.Printf("Warning: failed to generate lock file: %v\n", err)
	}

	// fmt.Print("Running go mod tidy...\n")
	// if err := cmds.RunGoModTidy(targetDir); err != nil {
	// 	return fmt.Errorf("failed to run go mod tidy: %w", err)
	// }

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
			DIMode:    diMode,
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

	return nil
}

type (
	TmplTarget     string
	TmplTargetPath string
)

// FrameworkManagedFile identifies a template-backed file that Andurel owns.
type FrameworkManagedFile struct {
	TemplateName string
	TargetPath   string
}

var baseTailwindTemplateMappings = map[TmplTarget]TmplTargetPath{
	"tw_css_base.tmpl": "css/base.css",

	// Views
	"tw_views_layout.tmpl": "views/layout.templ",
	"tw_views_home.tmpl":   "views/home.templ",

	// Views — Pages
	"tw_views_bad_request.tmpl":    "views/bad_request.templ",
	"tw_views_internal_error.tmpl": "views/internal_error.templ",
	"tw_views_not_found.tmpl":      "views/not_found.templ",
	"tw_views_confirm_email.tmpl":  "views/confirm_email.templ",
	"tw_views_login.tmpl":          "views/login.templ",
	"tw_views_registration.tmpl":   "views/registration.templ",
	"tw_views_reset_password.tmpl": "views/reset_password.templ",

	// Views
	"tw_views_head.tmpl": "views/head.templ",
}

var baseVanillaCSSTemplateMappings = map[TmplTarget]TmplTargetPath{
	"assets_vanilla_css_reset.tmpl":     "assets/css/reset.css",
	"assets_vanilla_css_tokens.tmpl":    "assets/css/tokens.css",
	"assets_vanilla_css_base.tmpl":      "assets/css/base.css",
	"assets_vanilla_css_objects.tmpl":   "assets/css/objects.css",
	"assets_vanilla_css_utilities.tmpl": "assets/css/utilities.css",

	// Views
	"vanilla_views_layout.tmpl":         "views/layout.templ",
	"vanilla_views_head.tmpl":           "views/head.templ",
	"vanilla_views_home.tmpl":           "views/home.templ",
	"vanilla_views_bad_request.tmpl":    "views/bad_request.templ",
	"vanilla_views_internal_error.tmpl": "views/internal_error.templ",
	"vanilla_views_not_found.tmpl":      "views/not_found.templ",
	"vanilla_views_confirm_email.tmpl":  "views/confirm_email.templ",
	"vanilla_views_login.tmpl":          "views/login.templ",
	"vanilla_views_registration.tmpl":   "views/registration.templ",
	"vanilla_views_reset_password.tmpl": "views/reset_password.templ",
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
	"deprecated_cmd_app_main.tmpl": "cmd/app/main.go",

	// Config
	"config_app.tmpl":       "config/app.go",
	"config_config.tmpl":    "config/config.go",
	"config_database.tmpl":  "config/database.go",
	"config_telemetry.tmpl": "config/telemetry.go",
	"config_email.tmpl":     "config/email.go",

	// Clients
	"clients_email_mailpit.tmpl": "clients/email/mailpit.go",

	// Controllers
	"deprecated_controllers_api.tmpl":        "controllers/api.go",
	"deprecated_controllers_assets.tmpl":     "controllers/assets.go",
	"controllers_cache.tmpl":      "controllers/cache.go",
	"deprecated_controllers_controller.tmpl": "controllers/controller.go",
	"deprecated_controllers_pages.tmpl":      "controllers/pages.go",

	// Database
	"database_migrations_gitkeep.tmpl": "database/migrations/.gitkeep",
	"database_seeds_main.tmpl":         "database/seeds/main.go",
	"psql_database.tmpl":               "database/database.go",

	// Queue package
	"psql_queue_queue.tmpl":                            "queue/queue.go",
	"psql_queue_jobs_send_transactional_email.tmpl":    "queue/jobs/send_transactional_email.go",
	"psql_queue_jobs_send_marketing_email.tmpl":        "queue/jobs/send_marketing_email.go",
	"psql_queue_workers_workers.tmpl":                  "queue/workers/workers.go",
	"psql_queue_workers_send_transactional_email.tmpl": "queue/workers/send_transactional_email.go",
	"psql_queue_workers_send_marketing_email.tmpl":     "queue/workers/send_marketing_email.go",

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
	"deprecated_router_router.tmpl":                         "router/router.go",
	"deprecated_router_connect_api_routes.tmpl":             "router/connect_api_routes.go",
	"deprecated_router_connect_assets_routes.tmpl":          "router/connect_assets_routes.go",
	"deprecated_router_connect_pages_routes.tmpl":           "router/connect_pages_routes.go",
	"deprecated_router_connect_sessions_routes.tmpl":        "router/connect_sessions_routes.go",
	"deprecated_router_connect_registrations_routes.tmpl":   "router/connect_registrations_routes.go",
	"deprecated_router_connect_confirmations_routes.tmpl":   "router/connect_confirmations_routes.go",
	"deprecated_router_connect_reset_passwords_routes.tmpl": "router/connect_reset_passwords_routes.go",
	"router_cookies_cookies.tmpl":                "router/cookies/cookies.go",
	"router_cookies_flash.tmpl":                  "router/cookies/flash.go",
	"router_middleware_middleware.tmpl":          "router/middleware/middleware.go",

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
	"deprecated_controllers_confirmations.tmpl":   "controllers/confirmations.go",
	"deprecated_controllers_registrations.tmpl":   "controllers/registrations.go",
	"deprecated_controllers_reset_passwords.tmpl": "controllers/reset_passwords.go",
	"deprecated_controllers_sessions.tmpl":        "controllers/sessions.go",

	// Auth - Config
	"config_auth.tmpl": "config/auth.go",

	// Auth - Services
	"services_authentication.tmpl": "services/authentication.go",
	"services_registration.tmpl":   "services/registration.go",
	"services_reset_password.tmpl": "services/reset_password.go",

	// Auth - Router
	"router_routes_users.tmpl":    "router/routes/users.go",
	"router_middleware_auth.tmpl": "router/middleware/auth.go",

	// Auth - Email
	"email_reset_password.tmpl": "email/reset_password.templ",
	"email_verify_email.tmpl":   "email/verify_email.templ",
}

// fxTemplateOverrides maps base template names to their uberfx variants.
// In uberfx mode, these entries replace the manual-mode templates.
var fxTemplateOverrides = map[TmplTarget]TmplTargetPath{
	"cmd_app_main_fx.tmpl":                             "cmd/app/main.go",
	"router_router_fx.tmpl":                            "router/router.go",
	"controllers_api_fx.tmpl":                          "controllers/api.go",
	"controllers_assets_fx.tmpl":                       "controllers/assets.go",
	"controllers_controller_fx.tmpl":                   "controllers/controller.go",
	"controllers_pages_fx.tmpl":                        "controllers/pages.go",
	"controllers_sessions_fx.tmpl":                     "controllers/sessions.go",
	"controllers_registrations_fx.tmpl":                "controllers/registrations.go",
	"controllers_confirmations_fx.tmpl":                "controllers/confirmations.go",
	"controllers_reset_passwords_fx.tmpl":              "controllers/reset_passwords.go",
	"services_service_fx.tmpl":                         "services/service.go",
	"services_identity_fx.tmpl":                        "services/identity.go",
	"psql_queue_workers_workers.tmpl":                  "queue/workers.go",
	"psql_queue_workers_send_transactional_email.tmpl": "queue/send_transactional_email.go",
	"psql_queue_workers_send_marketing_email.tmpl":     "queue/send_marketing_email.go",
}

// fxSkippedTemplates lists base template entries skipped in uberfx mode.
var fxSkippedTemplates = map[TmplTarget]bool{
	"deprecated_cmd_app_main.tmpl":                          true,
	"deprecated_router_router.tmpl":                         true,
	"deprecated_controllers_api.tmpl":                       true,
	"deprecated_controllers_assets.tmpl":                    true,
	"deprecated_controllers_controller.tmpl":                true,
	"deprecated_controllers_pages.tmpl":                     true,
	"deprecated_controllers_sessions.tmpl":                  true,
	"deprecated_controllers_registrations.tmpl":             true,
	"deprecated_controllers_confirmations.tmpl":             true,
	"deprecated_controllers_reset_passwords.tmpl":           true,
	"deprecated_router_connect_api_routes.tmpl":             true,
	"deprecated_router_connect_assets_routes.tmpl":          true,
	"deprecated_router_connect_pages_routes.tmpl":           true,
	"deprecated_router_connect_sessions_routes.tmpl":        true,
	"deprecated_router_connect_registrations_routes.tmpl":   true,
	"deprecated_router_connect_confirmations_routes.tmpl":   true,
	"deprecated_router_connect_reset_passwords_routes.tmpl": true,
}

var inertiaTemplateMappings = map[TmplTarget]TmplTargetPath{
	"inertia_framework_root_html.tmpl":  "views/root.go.html",
	"inertia_assets_app.tmpl":           "resources/js/app.ts",
	"inertia_assets_pages_welcome.tmpl": "resources/js/Pages/Welcome.vue",
	"inertia_assets_vite_config.tmpl":   "vite.config.ts",
	"inertia_assets_package_json.tmpl":  "package.json",
	"inertia_assets_tsconfig.tmpl":      "tsconfig.json",
	"inertia_page_options.tmpl":         "internal/inertia/page_options.go",
	"inertia_render.tmpl":               "internal/inertia/render.go",
	"inertia_vite.tmpl":                 "internal/inertia/vite.go",
}

var inertiaTemplateOverrides = map[TmplTarget]TmplTargetPath{
	"deprecated_controllers_pages_inertia.tmpl":    "controllers/pages.go",
	"controllers_pages_inertia_fx.tmpl": "controllers/pages.go",
}

var inertiaSkippedTemplates = map[TmplTarget]bool{
	"tw_views_home.tmpl":      true,
	"vanilla_views_home.tmpl": true,
}

// GetInternalFrameworkFiles returns the internal package files expected for a project config.
func GetInternalFrameworkFiles(config *ScaffoldConfig) []FrameworkManagedFile {
	mappings := make(map[TmplTarget]TmplTargetPath)
	for templateName, targetPath := range baseTemplateMappings {
		if strings.HasPrefix(string(targetPath), "internal/") {
			mappings[templateName] = targetPath
		}
	}

	if config != nil && config.Inertia == "vue" {
		for templateName, targetPath := range inertiaTemplateMappings {
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
	for templateName, targetPath := range inertiaTemplateMappings {
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

func processTemplatedFiles(
	targetDir string,
	cssFramework string,
	data extensions.TemplateData,
) error {
	mappings := make(map[TmplTarget]TmplTargetPath, len(baseTemplateMappings)+len(fxTemplateOverrides)+len(inertiaTemplateMappings))
	for k, v := range baseTemplateMappings {
		mappings[k] = v
	}

	if td, ok := data.(*TemplateData); ok && td.DIMode == "uberfx" {
		for k := range fxSkippedTemplates {
			delete(mappings, k)
		}
		for k, v := range fxTemplateOverrides {
			mappings[k] = v
		}
	}

	if td, ok := data.(*TemplateData); ok && td.Inertia == "vue" {
		for k := range inertiaSkippedTemplates {
			delete(mappings, k)
		}
		if td.DIMode == "uberfx" {
			delete(mappings, "controllers_pages_fx.tmpl")
			mappings["controllers_pages_inertia_fx.tmpl"] = inertiaTemplateOverrides["controllers_pages_inertia_fx.tmpl"]
		} else {
			delete(mappings, "deprecated_controllers_pages.tmpl")
			mappings["deprecated_controllers_pages_inertia.tmpl"] = inertiaTemplateOverrides["deprecated_controllers_pages_inertia.tmpl"]
		}
		for k, v := range inertiaTemplateMappings {
			mappings[k] = v
		}
	}

	for templateFile, targetPath := range mappings {
		if templateFile == "assets_js_datastar.tmpl" {
			if err := copyFile(targetDir, string(templateFile), string(targetPath), templates.Files); err != nil {
				return fmt.Errorf("failed to copy file %s: %w", templateFile, err)
			}
			continue
		}
		if strings.HasPrefix(string(templateFile), "inertia_assets_") || templateFile == "inertia_framework_root_html.tmpl" {
			if err := copyFile(targetDir, string(templateFile), string(targetPath), templates.Files); err != nil {
				return fmt.Errorf("failed to copy file %s: %w", templateFile, err)
			}
			continue
		}
		if err := renderTemplate(targetDir, string(templateFile), string(targetPath), templates.Files, data); err != nil {
			return fmt.Errorf("failed to process template %s: %w", templateFile, err)
		}
	}

	if cssFramework == "tailwind" {
		for templateFile, targetPath := range baseTailwindTemplateMappings {
			if td, ok := data.(*TemplateData); ok && td.Inertia == "vue" && inertiaSkippedTemplates[templateFile] {
				continue
			}
			if err := renderTemplate(targetDir, string(templateFile), string(targetPath), templates.Files, data); err != nil {
				return fmt.Errorf("failed to process tailwind template %s: %w", templateFile, err)
			}
		}
	}

	if cssFramework == "vanilla" {
		for templateFile, targetPath := range baseVanillaCSSTemplateMappings {
			if err := renderTemplate(targetDir, string(templateFile), string(targetPath), templates.Files, data); err != nil {
				return fmt.Errorf(
					"failed to process vanilla css template %s: %w",
					templateFile,
					err,
				)
			}
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

	td, ok := data.(*TemplateData)
	if !ok {
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

	// Mode-specific templates
	if td.DIMode == "uberfx" {
		blueprintTemplates = append(blueprintTemplates,
			"cmd_app_main_fx.tmpl",
			"controllers_controller_fx.tmpl",
		)
	} else {
		blueprintTemplates = append(blueprintTemplates,
			"deprecated_cmd_app_main.tmpl",
			"deprecated_controllers_controller.tmpl",
			"deprecated_router_connect_api_routes.tmpl",
			"deprecated_router_connect_assets_routes.tmpl",
			"deprecated_router_connect_pages_routes.tmpl",
		)
	}

	for _, tmplName := range blueprintTemplates {
		// Look up in base mappings first, then fx overrides
		targetPath, ok := baseTemplateMappings[tmplName]
		if !ok {
			targetPath, ok = fxTemplateOverrides[tmplName]
			if !ok {
				return fmt.Errorf("template mapping missing for blueprint template %s", tmplName)
			}
		}

		if err := renderTemplate(targetDir, string(tmplName), string(targetPath), templates.Files, data); err != nil {
			return fmt.Errorf("failed to render blueprint template %s: %w", tmplName, err)
		}
	}

	if err := renderTemplate(targetDir, "go_mod.tmpl", "go.mod", templates.Files, data); err != nil {
		return fmt.Errorf("failed to render go.mod template: %w", err)
	}

	return nil
}

// func processTemplate(
// 	targetDir, templateFile, targetPath string,
// 	data extensions.TemplateData,
// ) error {
// 	return renderTemplate(targetDir, templateFile, targetPath, templates.Files, data)
// }
//
// func ProcessTemplateFromRecipe(
// 	targetDir, templateFile, targetPath string,
// 	data extensions.TemplateData,
// ) error {
// 	return renderTemplate(targetDir, templateFile, targetPath, extensions.Files, data)
// }

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
		tmpFile.Close()
		return fmt.Errorf("failed to execute template %s: %w", templateFile, err)
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
		"lower": strings.ToLower,
	}
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

const goVersion = "1.26.0"

type GoTool struct {
	Name    string
	Source  string
	Version string
}

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

	// Add tailwindcli if using Tailwind CSS
	if config != nil && config.CSSFramework == "tailwind" {
		expectedTools["tailwindcli"] = NewBinaryTool("tailwindcli", versions.TailwindCLI)
	}

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

func generateRandomHex(bytes int) string {
	randomBytes := make([]byte, bytes)
	rand.Read(randomBytes)
	return hex.EncodeToString(randomBytes)
}

// initializeBaseBlueprint creates a blueprint with default base configuration
// for controllers, routes, and other scaffold components.
func initializeBaseBlueprint(moduleName, diMode, inertia string) *blueprint.Blueprint {
	if diMode == "uberfx" {
		return initializeUberFxBlueprint(moduleName)
	}
	return initializeManualBlueprint(moduleName, inertia)
}

func initializeManualBlueprint(moduleName, inertia string) *blueprint.Blueprint {
	builder := blueprint.NewBuilder(nil)

	builder.AddMainImport(fmt.Sprintf("%s/clients/email", moduleName))
	builder.AddControllerImport(fmt.Sprintf("%s/controllers", moduleName))
	builder.AddControllerImport(fmt.Sprintf("%s/email", moduleName))

	builder.AddMainInitialization(
		"emailClient",
		"mailclients.NewMailpit(cfg)",
		"cfg",
	)

	builder.AddConfigField("Email", "email")

	// Auth configuration
	builder.AddConfigField("Auth", "auth")

	// Auth controller dependencies and imports
	builder.AddControllerImport(fmt.Sprintf("%s/config", moduleName))
	builder.AddControllerDependency("cfg", "config.Config")

	builder.AddControllerDependency("db", "storage.Pool")

	// Controller fields - the main sub-controllers
	builder.
		AddControllerField("Assets", "controllers.Assets").
		AddControllerField("API", "controllers.API").
		AddControllerField("Pages", "controllers.Pages").
		AddControllerField("Sessions", "controllers.Sessions").
		AddControllerField("Registrations", "controllers.Registrations").
		AddControllerField("Confirmations", "controllers.Confirmations").
		AddControllerField("ResetPasswords", "controllers.ResetPasswords")

	pagesConstructor := "controllers.NewPages(db, insertOnly, pagesCache)"
	if inertia == "vue" {
		pagesConstructor = "controllers.NewPages(db, insertOnly)"
	}

	// Constructor initializations
	builder.
		AddControllerConstructor("assets", "controllers.NewAssets(assetsCache)").
		AddControllerConstructor("api", "controllers.NewAPI(db)").
		AddControllerConstructor("pages", pagesConstructor).
		AddControllerConstructor("sessions", "controllers.NewSessions(db, cfg)").
		AddControllerConstructor("registrations", "controllers.NewRegistrations(db, insertOnly, cfg)").
		AddControllerConstructor("confirmations", "controllers.NewConfirmations(db, cfg)").
		AddControllerConstructor("resetPasswords", "controllers.NewResetPasswords(db, insertOnly, cfg)")

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

func initializeUberFxBlueprint(moduleName string) *blueprint.Blueprint {
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

func generateLockFile(targetDir, version string, hasTailwind bool, config *ScaffoldConfig, extensions []string) error {
	lock := NewAndurelLock(version)
	lock.ScaffoldConfig = config
	lock.DatabaseConfig = &DatabaseConfig{
		NullType: "sql.Null",
	}

	for _, tool := range DefaultGoTools {
		sourceRepo := extractRepo(tool.Source)
		lock.AddTool(tool.Name, NewGoTool(tool.Name, sourceRepo, tool.Version))
	}

	if hasTailwind {
		lock.AddTool("tailwindcli", NewBinaryTool("tailwindcli", versions.TailwindCLI))
	}

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
