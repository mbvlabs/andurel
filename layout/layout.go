// Package layout provides functionality to scaffold a new Go web application project
package layout

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/mbvlabs/andurel/layout/templates"
	"github.com/mbvlabs/andurel/pkg/constants"
)

type Element struct {
	RootDir string
	SubDirs []Element
}

type TemplateData struct {
	ProjectName          string
	ModuleName           string
	Database             string
	SessionKey           string
	SessionEncryptionKey string
	TokenSigningKey      string
	PasswordSalt         string
}

// Scaffold TODO: figure out a way to have full repo path on init, i.e. github.com/mbvlabs/andurel
// breaks because go mod tidy will look up that path and not find it
func Scaffold(targetDir, projectName, repo, database string) error {
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
	if err := processTemplatedFiles(targetDir, templateData); err != nil {
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
			return fmt.Errorf("failed to run go fmt: %w", err)
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

func processTemplatedFiles(targetDir string, data TemplateData) error {
	templateMappings := map[TmplTarget]TmplTargetPath{
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
		"cmd_app_main.tmpl": "cmd/app/main.go",
		"cmd_run_main.tmpl": "cmd/run/main.go",

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

	for templateFile, targetPath := range templateMappings {
		if err := processTemplate(targetDir, string(templateFile), string(targetPath), data); err != nil {
			return fmt.Errorf("failed to process template %s: %w", templateFile, err)
		}
	}

	return nil
}

func processTemplate(targetDir, templateFile, targetPath string, data TemplateData) error {
	content, err := templates.Files.ReadFile(templateFile)
	if err != nil {
		return fmt.Errorf("failed to read template %s: %w", templateFile, err)
	}

	contentStr := string(content)

	// if strings.HasSuffix(templateFile, "_templ.tmpl") {
	// 	contentStr = strings.ReplaceAll(contentStr, moduleName, data.ModuleName)
	// }

	tmpl, err := template.New(templateFile).Parse(contentStr)
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", templateFile, err)
	}

	fullTargetPath := filepath.Join(targetDir, targetPath)
	if err := os.MkdirAll(filepath.Dir(fullTargetPath), constants.DirPermissionDefault); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", targetPath, err)
	}

	file, err := os.Create(fullTargetPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", targetPath, err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template %s: %w", templateFile, err)
	}

	return nil
}

const goVersion = "1.25.0"

func createGoMod(targetDir, moduleName string) error {
	goModPath := filepath.Join(targetDir, "go.mod")
	goModContent := fmt.Sprintf(
		"module %s\n\ngo %s\n\ntool (\n    github.com/a-h/templ/cmd/templ\n    github.com/sqlc-dev/sqlc/cmd/sqlc\n    github.com/pressly/goose/v3/cmd/goose\n    github.com/air-verse/air\n)\n",
		moduleName,
		goVersion,
	)

	return os.WriteFile(goModPath, []byte(goModContent), 0o644)
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

// func downloadTailwindBinary(targetDir string) error {
// 	binPath := filepath.Join("bin", "tailwindcli")
//
// 	if _, err := os.Stat(binPath); err == nil {
// 		fmt.Printf("Tailwind binary found at: %s\n", binPath)
// 		return nil
// 	}
//
// 	fmt.Println("Tailwind binary not found, downloading...")
//
// 	if err := os.MkdirAll("bin", 0o755); err != nil {
// 		return fmt.Errorf("failed to create bin directory: %w", err)
// 	}
//
// 	downloadURL, err := getTailwindDownloadURL()
// 	if err != nil {
// 		return err
// 	}
//
// 	fmt.Printf("Downloading latest Tailwind CLI for %s...\n", runtime.GOOS)
//
// 	resp, err := http.Get(downloadURL)
// 	if err != nil {
// 		return fmt.Errorf("failed to download Tailwind: %w", err)
// 	}
// 	defer resp.Body.Close()
//
// 	if resp.StatusCode != http.StatusOK {
// 		return fmt.Errorf("failed to download Tailwind: status %d", resp.StatusCode)
// 	}
//
// 	out, err := os.Create(binPath)
// 	if err != nil {
// 		return fmt.Errorf("failed to create binary file: %w", err)
// 	}
// 	defer out.Close()
//
// 	if _, err := io.Copy(out, resp.Body); err != nil {
// 		return fmt.Errorf("failed to write binary: %w", err)
// 	}
//
// 	if err := os.Chmod(binPath, 0o755); err != nil {
// 		return fmt.Errorf("failed to make binary executable: %w", err)
// 	}
//
// 	fmt.Printf("Tailwind CLI downloaded to %s\n", binPath)
// 	return nil
// }
//
// func getTailwindDownloadURL() (string, error) {
// 	baseURL := "https://github.com/tailwindlabs/tailwindcss/releases/latest/download"
//
// 	var arch string
// 	switch runtime.GOOS {
// 	case "darwin":
// 		arch = "macos-x64"
// 	case "linux":
// 		arch = "linux-x64"
// 	case "windows":
// 		arch = "windows-x64.exe"
// 	default:
// 		return "", fmt.Errorf(
// 			"unsupported platform: %s. Supported platforms: darwin (mac), linux, windows",
// 			runtime.GOOS,
// 		)
// 	}
//
// 	return fmt.Sprintf("%s/tailwindcss-%s", baseURL, arch), nil
// }
