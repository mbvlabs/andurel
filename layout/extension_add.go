package layout

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/mbvlabs/andurel/layout/cmds"
	"github.com/mbvlabs/andurel/layout/extensions"
)

// LoadProjectContext reconstructs TemplateData and AndurelLock from an existing
// project on disk. It reads the lock file for scaffold configuration, parses
// go.mod for the module path and Go version, and reads secrets from .env.example
// (or .env as fallback) so they are preserved when blueprint templates are
// re-rendered.
//
// The blueprint is rebuilt from scratch by calling initializeBaseBlueprint and
// then re-applying all existing extensions with a no-op ProcessTemplate. This
// restores the full blueprint state (config fields, env vars, imports, etc.)
// without overwriting any existing extension-generated files.
func LoadProjectContext(rootDir string) (*TemplateData, *AndurelLock, error) {
	lock, err := ReadLockFile(rootDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read lock file: %w", err)
	}

	if lock.ScaffoldConfig == nil {
		return nil, nil, fmt.Errorf("cannot reconstruct project settings: scaffoldConfig missing from andurel.lock")
	}

	moduleName, goVer, err := parseGoMod(rootDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse go.mod: %w", err)
	}

	secrets := readSecrets(rootDir)

	diMode := "uberfx"

	td := &TemplateData{
		AppName:              lock.ScaffoldConfig.ProjectName,
		ProjectName:          lock.ScaffoldConfig.ProjectName,
		ModuleName:           moduleName,
		Database:             lock.ScaffoldConfig.Database,
		CSSFramework:         lock.ScaffoldConfig.CSSFramework,
		GoVersion:            goVer,
		SessionKey:           secrets["SESSION_KEY"],
		SessionEncryptionKey: secrets["SESSION_ENCRYPTION_KEY"],
		TokenSigningKey:      secrets["TOKEN_SIGNING_KEY"],
		Pepper:               secrets["PEPPER"],
		Extensions:           lock.ExtensionNames(),
		RunToolVersion:       GetRunToolVersion(),
		FrameworkVersion:     lock.Version,
		DIMode:               diMode,
		Inertia:              lock.ScaffoldConfig.Inertia,
	}

	bp := initializeBaseBlueprint(moduleName, diMode, td.Inertia)
	td.SetBlueprint(bp)

	if err := registerBuiltinExtensions(); err != nil {
		return nil, nil, fmt.Errorf("failed to register builtin extensions: %w", err)
	}

	existingNames := lock.ExtensionNames()
	if len(existingNames) > 0 {
		resolved, err := resolveExtensions(existingNames)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to resolve existing extensions: %w", err)
		}

		nextMigrationTime := time.Now()
		for _, ext := range resolved {
			ctx := extensions.Context{
				TargetDir: rootDir,
				Data:      td,
				DIMode:    diMode,
				Inertia:   td.Inertia,
				ProcessTemplate: func(templateFile, targetPath string, data extensions.TemplateData) error {
					return nil
				},
				AddPostStep:       func(fn func(targetDir string) error) {},
				NextMigrationTime: &nextMigrationTime,
			}

			if err := ext.Apply(&ctx); err != nil {
				return nil, nil, fmt.Errorf("failed to re-apply extension %s: %w", ext.Name(), err)
			}

			nextMigrationTime = nextMigrationTime.Add(10 * time.Second)
		}
	}

	return td, lock, nil
}

// ApplyExtension adds an extension to an existing project. It:
//  1. Reconstructs the project context (TemplateData + lock) from disk.
//  2. Re-applies all existing extensions to rebuild the full blueprint state.
//  3. Applies the new extension (generates its code files + mutates the blueprint).
//  4. Re-renders all blueprint-consuming templates (config.go, .env.example,
//     main.go, etc.) with the updated blueprint.
//  5. Runs post-steps and code generation tools (goose fix, templ generate,
//     go mod tidy).
//  6. Updates and writes andurel.lock.
//
// Returns the names of all newly applied extensions (the requested extension
// plus any unsatisfied dependencies).
func ApplyExtension(rootDir, extensionName string) ([]string, error) {
	fmt.Print("Loading project context...\n")
	td, lock, err := LoadProjectContext(rootDir)
	if err != nil {
		return nil, err
	}

	if _, exists := lock.Extensions[extensionName]; exists {
		return nil, fmt.Errorf("extension '%s' is already applied to this project", extensionName)
	}

	if err := registerBuiltinExtensions(); err != nil {
		return nil, fmt.Errorf("failed to register builtin extensions: %w", err)
	}

	if _, ok := extensions.Get(extensionName); !ok {
		available := strings.Join(extensions.Names(), ", ")
		return nil, fmt.Errorf("unknown extension '%s', available extensions: %s", extensionName, available)
	}

	existingNames := lock.ExtensionNames()
	allNames := append(append([]string{}, existingNames...), extensionName)

	resolved, err := resolveExtensions(allNames)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve extensions: %w", err)
	}

	existingSet := make(map[string]bool, len(existingNames))
	for _, name := range existingNames {
		existingSet[name] = true
	}

	var newExtensions []string
	allExtensionNames := make([]string, len(resolved))
	for i, ext := range resolved {
		name := ext.Name()
		allExtensionNames[i] = name
		if !existingSet[name] {
			newExtensions = append(newExtensions, name)
		}
	}
	td.Extensions = allExtensionNames

	fmt.Print("Applying extensions...\n")
	nextMigrationTime := time.Now()

	type postStep struct {
		extensionName string
		fn            func(targetDir string) error
	}

	var postSteps []postStep

	for _, ext := range resolved {
		currentExt := ext
		isNew := !existingSet[currentExt.Name()]
		fmt.Printf(" - %s\n", currentExt.Name())

		extCtx := extensions.Context{
			TargetDir:         rootDir,
			Data:              td,
			DIMode:            td.DIMode,
			Inertia:           td.Inertia,
			NextMigrationTime: &nextMigrationTime,
		}

		if isNew {
			extCtx.ProcessTemplate = func(templateFile, targetPath string, data extensions.TemplateData) error {
				if data == nil {
					data = td
				}
				return renderTemplate(rootDir, templateFile, targetPath, extensions.Files, data)
			}
			extCtx.AddPostStep = func(fn func(targetDir string) error) {
				if fn != nil {
					postSteps = append(postSteps, postStep{
						extensionName: currentExt.Name(),
						fn:            fn,
					})
				}
			}
		} else {
			extCtx.ProcessTemplate = func(templateFile, targetPath string, data extensions.TemplateData) error {
				return nil
			}
			extCtx.AddPostStep = func(fn func(targetDir string) error) {}
		}

		if err := currentExt.Apply(&extCtx); err != nil {
			return nil, fmt.Errorf("failed to apply extension %s: %w", currentExt.Name(), err)
		}

		nextMigrationTime = nextMigrationTime.Add(10 * time.Second)
	}

	fmt.Print("Re-rendering managed files...\n")
	if err := rerenderBlueprintTemplates(rootDir, td); err != nil {
		return nil, fmt.Errorf("failed to re-render blueprint templates: %w", err)
	}

	for _, step := range postSteps {
		if err := step.fn(rootDir); err != nil {
			return nil, fmt.Errorf("extension %s post-step failed: %w", step.extensionName, err)
		}
	}

	fmt.Print("Fixing migration timestamps...\n")
	if err := cmds.RunGooseFix(rootDir); err != nil {
		slog.Error(
			"failed to run goose fix",
			"error",
			err,
			"fix",
			"run 'andurel tool sync' then 'goose -dir database/migrations fix' after sync",
		)
	}

	fmt.Print("Running templ generate...\n")
	if err := cmds.RunTemplGenerate(rootDir); err != nil {
		slog.Error(
			"failed to run templ generate",
			"error",
			err,
			"fix",
			"run 'andurel template generate' after sync",
		)
	}

	fmt.Print("Running go mod tidy...\n")
	if err := cmds.RunGoModTidy(rootDir); err != nil {
		slog.Error(
			"failed to run go mod tidy",
			"error",
			err,
			"fix",
			"run 'go mod tidy' after sync",
		)
	}

	if lock.Extensions == nil {
		lock.Extensions = make(map[string]*Extension)
	}

	now := time.Now().Format(time.RFC3339)
	for _, name := range newExtensions {
		lock.AddExtension(name, now)
	}

	if err := lock.WriteLockFile(rootDir); err != nil {
		return nil, fmt.Errorf("failed to write lock file: %w", err)
	}

	return newExtensions, nil
}

// parseGoMod reads go.mod and extracts the module path and Go version.
func parseGoMod(rootDir string) (module, goVer string, err error) {
	goModPath := filepath.Join(rootDir, "go.mod")

	file, err := os.Open(goModPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to open go.mod: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				module = fields[1]
			}
		}
		if strings.HasPrefix(line, "go ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				goVer = fields[1]
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", "", fmt.Errorf("failed to read go.mod: %w", err)
	}

	if module == "" {
		return "", "", fmt.Errorf("module declaration not found in go.mod")
	}

	if goVer == "" {
		goVer = goVersion
	}

	return module, goVer, nil
}

// readSecrets reads secret values from .env.example, falling back to .env if
// .env.example is not found. This preserves existing secrets when the env.tmpl
// template is re-rendered during extension application.
func readSecrets(rootDir string) map[string]string {
	secrets := make(map[string]string)

	for _, name := range []string{".env.example", ".env"} {
		envPath := filepath.Join(rootDir, name)
		envMap, err := godotenv.Read(envPath)
		if err != nil {
			continue
		}
		for _, key := range []string{
			"SESSION_KEY",
			"SESSION_ENCRYPTION_KEY",
			"TOKEN_SIGNING_KEY",
			"PEPPER",
		} {
			if val, ok := envMap[key]; ok {
				if _, exists := secrets[key]; !exists {
					secrets[key] = val
				}
			}
		}
	}

	return secrets
}
