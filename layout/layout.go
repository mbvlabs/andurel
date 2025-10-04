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
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"text/template"

	"github.com/mbvlabs/andurel/layout/extensions"
	"github.com/mbvlabs/andurel/layout/templates"
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

// Scaffold TODO: figure out a way to have full repo path on init, i.e. github.com/mbvlabs/andurel
// breaks because go mod tidy will look up that path and not find it
func Scaffold(targetDir, projectName, repo, database string, extensionNames []string) error {
	fmt.Printf("Scaffolding new project in %s...\n", targetDir)

	if strings.Contains(repo, "github.com/") {
		slog.Warn(
			"The 'github.com/' prefix is not supported currently as it breaks the setup process due to the repo not _yet_ existing on GH and will be removed.",
		)
		repo = strings.TrimPrefix(repo, "github.com/")
	}

	moduleName := projectName
	if repo != "" {
		moduleName = repo + "/" + projectName
	}

	templateData := TemplateData{
		ProjectName:          projectName,
		ModuleName:           moduleName,
		Database:             database,
		SessionKey:           generateRandomHex(64),
		SessionEncryptionKey: generateRandomHex(32),
		TokenSigningKey:      generateRandomHex(32),
		PasswordSalt:         generateRandomHex(16),
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
	if err := createGoMod(targetDir, moduleName); err != nil {
		return fmt.Errorf("failed to create go.mod: %w", err)
	}

	fmt.Print("Processing templated files...\n")
	if err := processTemplatedFiles(targetDir, &templateData); err != nil {
		return fmt.Errorf("failed to process templated files: %w", err)
	}

	if database == "sqlite" {
		if err := createSqliteDB(targetDir, projectName); err != nil {
			return fmt.Errorf("failed to create go.mod: %w", err)
		}

		cmd := exec.Command("cp", ".env.example", ".env")
		cmd.Dir = targetDir
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to copy .env.example to .env: %w", err)
		}
	}

	if err := os.Mkdir(filepath.Join(targetDir, "bin"), constants.DirPermissionDefault); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	fmt.Print("Setting up Tailwind CSS...\n")
	// Need to skip download for testing purposes
	if os.Getenv("ANDUREL_SKIP_TAILWIND") != "true" {
		if err := SetupTailwind(targetDir); err != nil {
			return fmt.Errorf("failed to download Tailwind binary: %w", err)
		}
	}

	fmt.Print("Running initial go mod tidy...\n")
	if err := runGoModTidy(targetDir); err != nil {
		return fmt.Errorf("failed to run go mod tidy: %w", err)
	}

	fmt.Print("Building run binary...\n")
	// Need to skip build for testing purposes
	if os.Getenv("ANDUREL_SKIP_BUILD") != "true" {
		if err := runGoRunBin(targetDir); err != nil {
			return fmt.Errorf("failed to build run binary: %w", err)
		}
	}

	fmt.Print("Building migration binary...\n")
	if os.Getenv("ANDUREL_SKIP_BUILD") != "true" {
		if err := runGoMigrationBin(targetDir); err != nil {
			return fmt.Errorf("failed to build migration binary: %w", err)
		}
	}

	fmt.Print("Building console binary...\n")
	if os.Getenv("ANDUREL_SKIP_BUILD") != "true" {
		if err := runConsoleBin(targetDir); err != nil {
			return fmt.Errorf("failed to build console binary: %w", err)
		}
	}

	type postStep struct {
		extensionName string
		fn            func() error
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
			ProcessTemplate: func(templateFile, targetPath string, data extensions.TemplateData) error {
				if data == nil {
					data = &templateData
				}

				return ProcessTemplateFromRecipe(targetDir, templateFile, targetPath, data)
			},
			AddPostStep: func(fn func() error) {
				if fn == nil {
					return
				}

				postExtensionSteps = append(postExtensionSteps, postStep{
					extensionName: currentExt.Name(),
					fn:            fn,
				})
			},
		}

		if err := currentExt.Apply(&ctx); err != nil {
			return fmt.Errorf("failed to apply extension %s: %w", currentExt.Name(), err)
		}
	}

	if err := rerenderSlotTemplates(targetDir, &templateData); err != nil {
		return fmt.Errorf("failed to re-render slot templates: %w", err)
	}

	for _, step := range postExtensionSteps {
		if err := step.fn(); err != nil {
			return fmt.Errorf("extension %s post-step failed: %w", step.extensionName, err)
		}
	}

	fmt.Print("Running templ fmt...\n")
	if err := runTemplFmt(targetDir); err != nil {
		return fmt.Errorf("failed to run templ fmt: %w", err)
	}

	fmt.Print("Running templ generate...\n")
	if err := runTemplGenerate(targetDir); err != nil {
		return fmt.Errorf("failed to run templ generate: %w", err)
	}

	fmt.Print("Running go fmt...\n")
	if err := runGoFmt(targetDir); err != nil {
		return fmt.Errorf("failed to run go fmt: %w", err)
	}

	fmt.Print("Finalizing go mod tidy...\n")
	// calling go mod tidy again to ensure everything is in place after templ generation
	if err := runGoModTidy(targetDir); err != nil {
		return fmt.Errorf("failed to run go mod tidy: %w", err)
	}

	return nil
}

type (
	TmplTarget     string
	TmplTargetPath string
)

var baseTemplateMappings = map[TmplTarget]TmplTargetPath{
	// Core files
	"database.tmpl":  "database/database.go",
	"env.tmpl":       ".env.example",
	"sqlc.tmpl":      "database/sqlc.yaml",
	"gitignore.tmpl": ".gitignore",
	"justfile.tmpl":  "justfile",

	// Assets
	"assets_assets.tmpl":      "assets/assets.go",
	"assets_css_tw.tmpl":      "assets/css/tw.css",
	"assets_js_scripts.tmpl":  "assets/js/scripts.js",
	"assets_js_datastar.tmpl": "assets/js/datastar_1-0-0-rc5.min.js",

	// CSS
	"css_base.tmpl":  "css/base.css",
	"css_theme.tmpl": "css/theme.css",

	// Commands
	"cmd_app_main.tmpl":       "cmd/app/main.go",
	"cmd_migration_main.tmpl": "cmd/migration/main.go",
	"cmd_run_main.tmpl":       "cmd/run/main.go",
	"cmd_console_main.tmpl":   "cmd/console/main.go",

	// Config
	"config_auth.tmpl":     "config/auth.go",
	"config_config.tmpl":   "config/config.go",
	"config_database.tmpl": "config/database.go",

	// Controllers
	"controllers_api.tmpl":        "controllers/api.go",
	"controllers_assets.tmpl":     "controllers/assets.go",
	"controllers_controller.tmpl": "controllers/controller.go",
	"controllers_pages.tmpl":      "controllers/pages.go",

	// Database
	"database_migrations_gitkeep.tmpl": "database/migrations/.gitkeep",
	"database_queries_gitkeep.tmpl":    "database/queries/.gitkeep",

	// Models
	"models_errors.tmpl": "models/errors.go",
	"models_model.tmpl":  "models/model.go",

	// Router
	"router_router.tmpl":                "router/router.go",
	"router_cookies_cookies.tmpl":       "router/cookies/cookies.go",
	"router_cookies_flash.tmpl":         "router/cookies/flash.go",
	"router_middleware_middleware.tmpl": "router/middleware/middleware.go",

	// Routes
	"router_routes_routes.tmpl": "router/routes/routes.go",
	"router_routes_api.tmpl":    "router/routes/api.go",
	"router_routes_assets.tmpl": "router/routes/assets.go",
	"router_routes_pages.tmpl":  "router/routes/pages.go",

	// Views
	"views_layout.tmpl":         "views/layout.templ",
	"views_home.tmpl":           "views/home.templ",
	"views_bad_request.tmpl":    "views/bad_request.templ",
	"views_internal_error.tmpl": "views/internal_error.templ",
	"views_not_found.tmpl":      "views/not_found.templ",

	// View Components
	"views_components_head.tmpl":   "views/components/head.templ",
	"views_components_toasts.tmpl": "views/components/toasts.templ",
}

var slotScopeTemplates = map[string]TmplTarget{
	"controllers": "controllers_controller.tmpl",
	"cmd/app":     "cmd_app_main.tmpl",
	"models":      "models_model.tmpl",
	"routes":      "router_routes_routes.tmpl",
}

func processTemplatedFiles(targetDir string, data extensions.TemplateData) error {
	for templateFile, targetPath := range baseTemplateMappings {
		if err := processTemplate(targetDir, string(templateFile), string(targetPath), data); err != nil {
			return fmt.Errorf("failed to process template %s: %w", templateFile, err)
		}
	}

	return nil
}

func rerenderSlotTemplates(targetDir string, data extensions.TemplateData) error {
	if data == nil {
		return fmt.Errorf("template data is nil")
	}

	slotNames := data.SlotNames()
	if len(slotNames) == 0 {
		return nil
	}

	neededScopes := make(map[string]struct{}, len(slotNames))
	for _, name := range slotNames {
		scope := slotScope(name)
		if scope == "" {
			continue
		}
		neededScopes[scope] = struct{}{}
	}

	if len(neededScopes) == 0 {
		return nil
	}

	scopes := make([]string, 0, len(neededScopes))
	for scope := range neededScopes {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)

	for _, scope := range scopes {
		tmplName, ok := slotScopeTemplates[scope]
		if !ok {
			return fmt.Errorf("no template registered for slot scope %q", scope)
		}

		targetPath, ok := baseTemplateMappings[tmplName]
		if !ok {
			return fmt.Errorf(
				"template mapping missing for scope %q (template %s)",
				scope,
				tmplName,
			)
		}

		if err := processTemplate(targetDir, string(tmplName), string(targetPath), data); err != nil {
			return fmt.Errorf("failed to render slot template %s: %w", tmplName, err)
		}
	}

	return nil
}

func slotScope(slot string) string {
	slot = strings.TrimSpace(slot)
	if slot == "" {
		return ""
	}

	if idx := strings.Index(slot, ":"); idx >= 0 {
		return slot[:idx]
	}

	return slot
}

func processTemplate(
	targetDir, templateFile, targetPath string,
	data extensions.TemplateData,
) error {
	return renderTemplate(targetDir, templateFile, targetPath, templates.Files, data)
}

func ProcessTemplateFromRecipe(
	targetDir, templateFile, targetPath string,
	data extensions.TemplateData,
) error {
	return renderTemplate(targetDir, templateFile, targetPath, extensions.Files, data)
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
		Funcs(slotFuncMap(data)).
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

func slotFuncMap(data extensions.TemplateData) template.FuncMap {
	return template.FuncMap{
		"slot": func(name string) []string {
			if data == nil {
				return nil
			}

			return data.Slot(name)
		},
		"slotJoined": func(name, sep string) string {
			if data == nil {
				return ""
			}

			return data.SlotJoined(name, sep)
		},
		"slotData": func(name string) []any {
			if data == nil {
				return nil
			}

			return data.SlotData(name)
		},
	}
}

func registerBuiltinExtensions() error {
	registerBuiltinOnce.Do(func() {
	})

	return registerBuiltinErr
}

func resolveExtensions(names []string) ([]extensions.Extension, error) {
	if len(names) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{}, len(names))
	lookup := make(map[string]extensions.Extension, len(names))
	uniqueNames := make([]string, 0, len(names))

	for _, rawName := range names {
		name := strings.TrimSpace(rawName)
		if name == "" {
			return nil, fmt.Errorf("extension name cannot be empty")
		}

		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("extension %s specified multiple times", name)
		}

		ext, ok := extensions.Get(name)
		if !ok {
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

		seen[name] = struct{}{}
		uniqueNames = append(uniqueNames, name)
		lookup[name] = ext
	}

	sort.Strings(uniqueNames)

	resolved := make([]extensions.Extension, 0, len(uniqueNames))
	for _, name := range uniqueNames {
		resolved = append(resolved, lookup[name])
	}

	return resolved, nil
}

const goVersion = "1.25.0"

const goModTemplate = `module %s

go %s

tool (
    github.com/a-h/templ/cmd/templ
    github.com/xo/usql
    github.com/sqlc-dev/sqlc/cmd/sqlc
    github.com/pressly/goose/v3/cmd/goose
    github.com/air-verse/air
)
`

func createGoMod(targetDir, moduleName string) error {
	goModPath := filepath.Join(targetDir, "go.mod")
	content := fmt.Sprintf(goModTemplate, moduleName, goVersion)

	return os.WriteFile(goModPath, []byte(content), 0o644)
}

func createSqliteDB(targetDir, projectName string) error {
	goModPath := filepath.Join(targetDir, projectName+".db")

	return os.WriteFile(goModPath, nil, 0o644)
}

func runGoModTidy(targetDir string) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = targetDir
	return cmd.Run()
}

func runGoFmt(targetDir string) error {
	cmd := exec.Command("go", "fmt", "./...")
	cmd.Dir = targetDir
	return cmd.Run()
}

func runGoRunBin(targetDir string) error {
	cmd := exec.Command("go", "build", "-o", "bin/run", "cmd/run/main.go")
	cmd.Dir = targetDir
	return cmd.Run()
}

func runGoMigrationBin(targetDir string) error {
	cmd := exec.Command("go", "build", "-o", "bin/migration", "cmd/migration/main.go")
	cmd.Dir = targetDir
	return cmd.Run()
}

func runConsoleBin(targetDir string) error {
	cmd := exec.Command("go", "build", "-o", "bin/console", "cmd/console/main.go")
	cmd.Dir = targetDir
	return cmd.Run()
}

func runTemplGenerate(targetDir string) error {
	cmd := exec.Command("go", "tool", "templ", "generate", "./views")
	cmd.Dir = targetDir
	return cmd.Run()
}

func runTemplFmt(targetDir string) error {
	cmd := exec.Command("go", "tool", "templ", "fmt", "./views")
	cmd.Dir = targetDir
	return cmd.Run()
}

func RunSqlcGenerate(targetDir string) error {
	cmd := exec.Command("go", "tool", "sqlc", "generate", "-f", "database/sqlc.yaml")
	cmd.Dir = targetDir
	return cmd.Run()
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

func SetupTailwind(targetDir string) error {
	binPath := filepath.Join(targetDir, "bin", "tailwindcli")

	if _, err := os.Stat(binPath); err == nil {
		fmt.Printf("Tailwind binary already exists at: %s\n", binPath)
		return nil
	}

	binDir := filepath.Join(targetDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	downloadURL, err := getTailwindDownloadURL()
	if err != nil {
		return err
	}

	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download Tailwind: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download Tailwind: status %d", resp.StatusCode)
	}

	out, err := os.Create(binPath)
	if err != nil {
		return fmt.Errorf("failed to create binary file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("failed to write binary: %w", err)
	}

	if err := os.Chmod(binPath, 0o755); err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}

	return nil
}

func getTailwindDownloadURL() (string, error) {
	baseURL := "https://github.com/tailwindlabs/tailwindcss/releases/latest/download"

	var arch string
	switch runtime.GOOS {
	case "darwin":
		arch = "macos-x64"
	case "linux":
		arch = "linux-x64"
	case "windows":
		arch = "windows-x64.exe"
	default:
		return "", fmt.Errorf(
			"unsupported platform: %s. Supported platforms: darwin (mac), linux, windows",
			runtime.GOOS,
		)
	}

	return fmt.Sprintf("%s/tailwindcss-%s", baseURL, arch), nil
}
