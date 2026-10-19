// Package layout provides functionality to scaffold a new Go web application project
package layout

import (
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
	"sort"
	"strings"
	"text/template"
	"unicode"

	"github.com/mbvlabs/andurel/v2/internal/constants"
	"github.com/mbvlabs/andurel/v2/internal/testseed"
	"github.com/mbvlabs/andurel/v2/layout/blueprint"
	"github.com/mbvlabs/andurel/v2/layout/cmds"
	"github.com/mbvlabs/andurel/v2/layout/templates"
	"github.com/mbvlabs/andurel/v2/layout/versions"
)

// Element describes a directory tree node to create during scaffolding.
type Element struct {
	RootDir string
	SubDirs []Element
}

// Scaffold creates a new Andurel project in the target directory.
func Scaffold(
	targetDir, projectName, database, version string,
	inertia, javascriptRuntime string,
) error {
	moduleName := projectName
	secrets, err := generateScaffoldSecrets(testseed.RandomReader())
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
		RunToolVersion:       GetRunToolVersion(),
		FrameworkVersion:     normalizeFrameworkVersion(version),
		Inertia:              inertia,
		JavaScriptSSRRuntime: "node",
		blueprint:            blueprint,
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
	if err := processMigrations(targetDir, &templateData); err != nil {
		return fmt.Errorf("failed to process migrations: %w", err)
	}

	fmt.Print("Generating andurel.toml and andurel.lock...\n")
	scaffoldConfig := &ScaffoldConfig{
		ProjectName:              projectName,
		Inertia:                  inertia,
		JavaScriptPackageManager: javascriptRuntime,
		JavaScriptSSRRuntime:     "node",
	}
	if err := generateLockFile(targetDir, version, scaffoldConfig, database); err != nil {
		fmt.Printf("Warning: failed to generate lock file: %v\n", err)
	}

	// Golden CLI builds already seed secrets via testseed. Migration files are
	// written with finished sequential versions (00001–00008), so scaffold
	// never runs goose fix. PR goldens skip network-bound post-scaffold steps
	// so `andurel new` stays offline. Compiled views (*_templ.go) and narsilc
	// query packages are written from embeds above; nightly full-tree goldens
	// call RunCLIFull so go fmt and tidy still run.
	// See internal/testseed and testdata/golden/README.md.
	if testseed.Enabled() && !testseed.FullScaffold() {
		return nil
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
	"css_base.tmpl":  "css/base.css",
	"css_theme.tmpl": "css/theme.css",
	"css_email.tmpl": "css/email.css",
}

var baseTemplateMappings = map[TmplTarget]TmplTargetPath{
	"env.tmpl":       ".env.example",
	"gitignore.tmpl": ".gitignore",
	"readme.tmpl":    "README.md",
	"agents.tmpl":    "AGENTS.md",

	// Assets
	"assets_assets.tmpl":     "assets/assets.go",
	"assets_css_style.tmpl":  "assets/css/style.css",
	"assets_js_scripts.tmpl": "assets/js/scripts.js",

	// Commands
	"cmd_app_main.tmpl":     "cmd/app/main.go",
	"cmd_queue_main.tmpl":   "cmd/queue/main.go",
	"cmd_migrate_main.tmpl": "cmd/migrate/main.go",
	"cmd_seeds_main.tmpl":   "cmd/seeds/main.go",

	// Process composition
	"runtime_runtime.tmpl":     "internal/runtime/runtime.go",
	"runtime_queue.tmpl":       "internal/runtime/queue.go",
	"runtime_server_test.tmpl": "internal/runtime/server_test.go",

	// Config
	"config_app.tmpl":       "config/app.go",
	"config_auth.tmpl":      "config/auth.go",
	"config_cache.tmpl":     "config/cache.go",
	"config_config.tmpl":    "config/config.go",
	"config_database.tmpl":  "config/database.go",
	"config_email.tmpl":     "config/email.go",
	"config_helper.tmpl":    "config/helper.go",
	"config_hosts.tmpl":     "config/hosts.go",
	"config_http.tmpl":      "config/http.go",
	"config_queue.tmpl":     "config/queue.go",
	"config_session.tmpl":   "config/session.go",
	"config_telemetry.tmpl": "config/telemetry.go",

	// Handlers
	"handlers_api.tmpl":    "handlers/api.go",
	"handlers_assets.tmpl": "handlers/assets.go",
	"handlers_pages.tmpl":  "handlers/pages.go",
	"handlers_errors.tmpl": "handlers/errors.go",
	"handlers_auth.tmpl":   "handlers/auth.go",

	// Migrations and seeds
	"database_migrations_gitkeep.tmpl": "migrations/.gitkeep",
	"database_migrations.tmpl":         "migrations/migrations.go",
	"database_seeds_seeds.tmpl":        "seeds/seeds.go",

	// Queue package
	"psql_queue_queue.tmpl":                            "queue/queue.go",
	"psql_queue_jobs_send_transactional_email.tmpl":    "queue/jobs/send_transactional_email.go",
	"psql_queue_jobs_send_marketing_email.tmpl":        "queue/jobs/send_marketing_email.go",
	"psql_queue_workers_workers.tmpl":                  "queue/workers.go",
	"psql_queue_workers_send_transactional_email.tmpl": "queue/send_transactional_email.go",
	"psql_queue_workers_send_marketing_email.tmpl":     "queue/send_marketing_email.go",

	// Email
	"email_email.tmpl":                   "email/email.go",
	"email_base_layout.tmpl":             "email/base_layout.templ",
	"email_base_layout_templ_go.tmpl":    "email/base_layout_templ.go",
	"email_reset_password.tmpl":          "email/reset_password.templ",
	"email_reset_password_templ_go.tmpl": "email/reset_password_templ.go",
	"email_verify_email.tmpl":            "email/verify_email.templ",
	"email_verify_email_templ_go.tmpl":   "email/verify_email_templ.go",

	// Domains
	"domains_errors.tmpl":                  "domains/errors.go",
	"domains_user.tmpl":                    "domains/user.go",
	"domains_token.tmpl":                   "domains/token.go",
	"domains_queries_user.tmpl":            "domains/user.sql",
	"domains_queries_token.tmpl":           "domains/token.sql",
	"domains_internal_queries_db.tmpl":     "domains/internal/queries/db.go",
	"domains_internal_queries_models.tmpl": "domains/internal/queries/models.go",
	"domains_internal_queries_user.tmpl":   "domains/internal/queries/user.sql.go",
	"domains_internal_queries_token.tmpl":  "domains/internal/queries/token.sql.go",

	// Services
	"services_service.tmpl":             "services/service.go",
	"services_identity.tmpl":            "services/identity.go",
	"services_authentication.tmpl":      "services/authentication.go",
	"services_authentication_test.tmpl": "services/authentication_test.go",
	"services_registration.tmpl":        "services/registration.go",
	"services_reset_password.tmpl":      "services/reset_password.go",

	// Routes
	"router_routes_api.tmpl":    "routes/api.go",
	"router_routes_assets.tmpl": "routes/assets.go",
	"router_routes_pages.tmpl":  "routes/pages.go",
	"router_routes_users.tmpl":  "routes/users.go",

	// Middleware
	"router_middleware_middleware.tmpl":      "middleware/middleware.go",
	"router_middleware_middleware_test.tmpl": "middleware/middleware_test.go",
	"router_middleware_auth.tmpl":            "middleware/auth.go",
	"router_middleware_auth_test.tmpl":       "middleware/auth_test.go",
	"router_cookies_app.tmpl":                "middleware/cookies.go",
}

// resources/js/routes.ts and resources/js/types/payloads.ts are not templated:
// andurel new emits them with the sync generators so a fresh project never
// drifts from andurel verify.
var inertiaSharedTemplateMappings = map[TmplTarget]TmplTargetPath{
	"config_inertia.tmpl":         "config/inertia.go",
	"cmd_ssr_main.tmpl":           "cmd/ssr/main.go",
	"inertia_framework_root.tmpl": "resources/root.templ",
	"views_root_templ_go.tmpl":    "resources/root_templ.go",
}

var inertiaVueTemplateMappings = map[TmplTarget]TmplTargetPath{
	"inertia_assets_app.tmpl":                               "resources/js/app.ts",
	"inertia_assets_ssr.tmpl":                               "resources/js/ssr.ts",
	"inertia_assets_layouts_layout.tmpl":                    "resources/js/Layouts/Layout.vue",
	"inertia_assets_pages_auth_confirm_email.tmpl":          "resources/js/Pages/Auth/ConfirmEmail.vue",
	"inertia_assets_pages_auth_login.tmpl":                  "resources/js/Pages/Auth/Login.vue",
	"inertia_assets_pages_auth_registration.tmpl":           "resources/js/Pages/Auth/Registration.vue",
	"inertia_assets_pages_auth_reset_password.tmpl":         "resources/js/Pages/Auth/ResetPassword.vue",
	"inertia_assets_pages_auth_reset_password_request.tmpl": "resources/js/Pages/Auth/ResetPasswordRequest.vue",
	"inertia_assets_pages_welcome.tmpl":                     "resources/js/Pages/Welcome.vue",
	"inertia_assets_pages_errors_bad_request.tmpl":          "resources/js/Pages/Errors/BadRequest.vue",
	"inertia_assets_pages_errors_conflict.tmpl":             "resources/js/Pages/Errors/Conflict.vue",
	"inertia_assets_pages_errors_internal_error.tmpl":       "resources/js/Pages/Errors/InternalError.vue",
	"inertia_assets_pages_errors_not_found.tmpl":            "resources/js/Pages/Errors/NotFound.vue",
	"inertia_assets_pages_errors_validation.tmpl":           "resources/js/Pages/Errors/Validation.vue",
	"inertia_assets_vite_config.tmpl":                       "vite.config.ts",
	"inertia_assets_package_json.tmpl":                      "package.json",
	"inertia_assets_tsconfig.tmpl":                          "tsconfig.json",
}

var inertiaReactTemplateMappings = map[TmplTarget]TmplTargetPath{
	"inertia_react_assets_app.tmpl":                               "resources/js/app.tsx",
	"inertia_react_assets_ssr.tmpl":                               "resources/js/ssr.tsx",
	"inertia_react_assets_layouts_layout.tmpl":                    "resources/js/Layouts/Layout.tsx",
	"inertia_react_assets_pages_auth_confirm_email.tmpl":          "resources/js/Pages/Auth/ConfirmEmail.tsx",
	"inertia_react_assets_pages_auth_login.tmpl":                  "resources/js/Pages/Auth/Login.tsx",
	"inertia_react_assets_pages_auth_registration.tmpl":           "resources/js/Pages/Auth/Registration.tsx",
	"inertia_react_assets_pages_auth_reset_password.tmpl":         "resources/js/Pages/Auth/ResetPassword.tsx",
	"inertia_react_assets_pages_auth_reset_password_request.tmpl": "resources/js/Pages/Auth/ResetPasswordRequest.tsx",
	"inertia_react_assets_pages_welcome.tmpl":                     "resources/js/Pages/Welcome.tsx",
	"inertia_react_assets_pages_errors_bad_request.tmpl":          "resources/js/Pages/Errors/BadRequest.tsx",
	"inertia_react_assets_pages_errors_conflict.tmpl":             "resources/js/Pages/Errors/Conflict.tsx",
	"inertia_react_assets_pages_errors_internal_error.tmpl":       "resources/js/Pages/Errors/InternalError.tsx",
	"inertia_react_assets_pages_errors_not_found.tmpl":            "resources/js/Pages/Errors/NotFound.tsx",
	"inertia_react_assets_pages_errors_validation.tmpl":           "resources/js/Pages/Errors/Validation.tsx",
	"inertia_react_assets_vite_config.tmpl":                       "vite.config.ts",
	"inertia_react_assets_package_json.tmpl":                      "package.json",
	"inertia_react_assets_tsconfig.tmpl":                          "tsconfig.json",
}

var inertiaSvelteTemplateMappings = map[TmplTarget]TmplTargetPath{
	"inertia_svelte_assets_app.tmpl":                               "resources/js/app.ts",
	"inertia_svelte_assets_ssr.tmpl":                               "resources/js/ssr.ts",
	"inertia_svelte_assets_layouts_layout.tmpl":                    "resources/js/Layouts/Layout.svelte",
	"inertia_svelte_assets_pages_auth_confirm_email.tmpl":          "resources/js/Pages/Auth/ConfirmEmail.svelte",
	"inertia_svelte_assets_pages_auth_login.tmpl":                  "resources/js/Pages/Auth/Login.svelte",
	"inertia_svelte_assets_pages_auth_registration.tmpl":           "resources/js/Pages/Auth/Registration.svelte",
	"inertia_svelte_assets_pages_auth_reset_password.tmpl":         "resources/js/Pages/Auth/ResetPassword.svelte",
	"inertia_svelte_assets_pages_auth_reset_password_request.tmpl": "resources/js/Pages/Auth/ResetPasswordRequest.svelte",
	"inertia_svelte_assets_pages_welcome.tmpl":                     "resources/js/Pages/Welcome.svelte",
	"inertia_svelte_assets_pages_errors_bad_request.tmpl":          "resources/js/Pages/Errors/BadRequest.svelte",
	"inertia_svelte_assets_pages_errors_conflict.tmpl":             "resources/js/Pages/Errors/Conflict.svelte",
	"inertia_svelte_assets_pages_errors_internal_error.tmpl":       "resources/js/Pages/Errors/InternalError.svelte",
	"inertia_svelte_assets_pages_errors_not_found.tmpl":            "resources/js/Pages/Errors/NotFound.svelte",
	"inertia_svelte_assets_pages_errors_validation.tmpl":           "resources/js/Pages/Errors/Validation.svelte",
	"inertia_svelte_assets_vite_config.tmpl":                       "vite.config.ts",
	"inertia_svelte_assets_package_json.tmpl":                      "package.json",
	"inertia_svelte_assets_tsconfig.tmpl":                          "tsconfig.json",
	"inertia_svelte_assets_svelte_config.tmpl":                     "svelte.config.js",
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
		strings.HasPrefix(string(templateFile), "inertia_svelte_assets_")
}

// GetInternalFrameworkFiles returns template-backed files that upgrades own
// for a project config. Generated application internal/ is not included.
func GetInternalFrameworkFiles(config *ScaffoldConfig) []FrameworkManagedFile {
	mappings := make(map[TmplTarget]TmplTargetPath)
	for templateName, targetPath := range baseTemplateMappings {
		if isManagedInternalFile(templateName, targetPath) {
			mappings[templateName] = targetPath
		}
	}

	if config != nil && IsSupportedInertiaAdapter(config.Inertia) {
		for templateName, targetPath := range inertiaSharedTemplateMappings {
			if isManagedInternalFile(templateName, targetPath) {
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
		if isManagedInternalFile(templateName, targetPath) {
			mappings[templateName] = targetPath
		}
	}
	for templateName, targetPath := range inertiaSharedTemplateMappings {
		if isManagedInternalFile(templateName, targetPath) {
			mappings[templateName] = targetPath
		}
	}

	return sortedFrameworkManagedFiles(mappings)
}

// isManagedInternalFile reports whether a template maps to a file that
// upgrades own. Generated application internal/ is application-owned.
// Standalone pkg/ sources are not scaffolded into apps. Test files are
// generated on project creation but are not part of the upgrade-managed surface.
func isManagedInternalFile(_ TmplTarget, targetPath TmplTargetPath) bool {
	return strings.HasPrefix(string(targetPath), "pkg/") &&
		!strings.HasSuffix(string(targetPath), "_test.go")
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

func processTemplatedFiles(targetDir string, data *TemplateData) error {
	mappings := make(map[TmplTarget]TmplTargetPath)
	maps.Copy(mappings, baseTemplateMappings)
	maps.Copy(mappings, inertiaSharedTemplateMappings)
	if data != nil {
		maps.Copy(mappings, inertiaAdapterTemplateMappings(data.Inertia))
	}

	for templateFile, targetPath := range mappings {
		if isStaticInertiaAssetTemplate(templateFile) {
			if err := copyFile(
				targetDir,
				string(templateFile),
				string(targetPath),
				templates.Files,
			); err != nil {
				return fmt.Errorf("failed to copy file %s: %w", templateFile, err)
			}
			continue
		}
		if err := renderTemplate(
			targetDir,
			string(templateFile),
			string(targetPath),
			templates.Files,
			data,
		); err != nil {
			return fmt.Errorf("failed to process template %s: %w", templateFile, err)
		}
	}

	for templateFile, targetPath := range baseStyleTemplateMappings {
		if err := renderTemplate(
			targetDir,
			string(templateFile),
			string(targetPath),
			templates.Files,
			data,
		); err != nil {
			return fmt.Errorf("failed to process style template %s: %w", templateFile, err)
		}
	}

	return nil
}

func processMigrations(
	targetDir string,
	data *TemplateData,
) error {
	migrations := []struct {
		template string
		name     string
	}{
		// River queue migrations
		{"psql_riverqueue_migration_one.tmpl", "create_river_migration_table"},
		{"psql_riverqueue_migration_two.tmpl", "create_river_job_and_leader_tables"},
		{"psql_riverqueue_migration_three.tmpl", "alter_river_job_tags"},
		{"psql_riverqueue_migration_four.tmpl", "alter_river_job_args_metadata_add_queue"},
		{"psql_riverqueue_migration_five.tmpl", "add_river_job_unique_key_and_clients"},
		{"psql_riverqueue_migration_six.tmpl", "add_river_job_unique_states"},
		// Auth migrations
		{"database_migrations_users.tmpl", "create_users_table"},
		{"database_migrations_tokens.tmpl", "create_tokens_table"},
	}

	for i, migration := range migrations {
		version := fmt.Sprintf("%05d", i+1)
		targetPath := fmt.Sprintf("migrations/%s_%s.sql", version, migration.name)

		if err := renderTemplate(
			targetDir,
			migration.template,
			targetPath,
			templates.Files,
			data,
		); err != nil {
			return fmt.Errorf(
				"failed to process migration %s: %w",
				migration.template,
				err,
			)
		}
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
	data *TemplateData,
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
		"lower":   strings.ToLower,
		"toSnake": ToSnakeCase,
	}
}

// ToSnakeCase converts PascalCase or camelCase identifiers to snake_case
// (UserID → user_id, IsAuthenticated → is_authenticated).
func ToSnakeCase(name string) string {
	runes := []rune(name)
	if len(runes) == 0 {
		return name
	}
	var result []rune
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 {
				prevLower := unicode.IsLower(runes[i-1])
				nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
				if prevLower || nextLower {
					result = append(result, '_')
				}
			}
			result = append(result, unicode.ToLower(r))
			continue
		}
		result = append(result, r)
	}
	return string(result)
}

const goVersion = "1.27.2"

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

	expectedTools["narsilc"] = NewBinaryTool("narsilc", versions.Narsilc)
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

	if err := renderTemplate(
		targetDir,
		"go_mod.tmpl",
		"go.mod",
		templates.Files,
		data,
	); err != nil {
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

	builder.AddWorkerDependency("transactionalSender", "email.TransactionalSender")
	builder.AddWorkerDependency("marketingSender", "email.MarketingSender")

	// Auth cookies configuration
	builder.AddCookiesAppField("UserID", "string")
	builder.AddCookiesAppField("PendingUserID", "string")
	builder.AddCookiesAppField("IsAdmin", "bool")
	builder.AddCookiesAppField("IsAuthenticated", "bool")

	for _, tool := range defaultTools {
		builder.AddTool(tool)
	}

	return builder.Blueprint()
}

func generateLockFile(
	targetDir, version string,
	config *ScaffoldConfig,
	databaseEngine string,
) error {
	lock := NewAndurelLock(version)
	lock.ScaffoldConfig = config
	lock.DatabaseConfig = &DatabaseConfig{
		Engine:   databaseEngine,
		NullType: NullTypePGType,
	}

	for _, tool := range DefaultGoTools {
		sourceRepo := extractRepo(tool.Source)
		lock.AddTool(tool.Name, NewGoTool(tool.Name, sourceRepo, tool.Version))
	}

	lock.AddTool("narsilc", NewBinaryTool("narsilc", versions.Narsilc))
	lock.AddTool("tailwindcli", NewBinaryTool("tailwindcli", versions.TailwindCLI))

	return lock.WriteLockFile(targetDir)
}

func extractRepo(module string) string {
	parts := strings.Split(module, "/")
	if len(parts) >= 3 {
		return strings.Join(parts[:3], "/")
	}
	return module
}
