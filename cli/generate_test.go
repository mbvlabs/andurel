package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sebdah/goldie/v2"
	"github.com/spf13/cobra"
)

func TestGenerateCommands(t *testing.T) {
	rootCmd := NewRootCommand("test", "test-date")

	tests := []struct {
		name string
		args []string
	}{
		{"generate help", []string{"generate", "--help"}},
		{"model help", []string{"generate", "model", "--help"}},
		{"controller help", []string{"generate", "controller", "--help"}},
		{"resource help", []string{"generate", "resource", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()
			if err != nil {
				t.Errorf("Command %v failed: %v", tt.args, err)
			}
		})
	}
}

func TestGenerateCommandStructure(t *testing.T) {
	rootCmd := NewRootCommand("test", "test-date")

	var generateCmd *cobra.Command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "generate" {
			generateCmd = cmd
			break
		}
	}

	if generateCmd == nil {
		t.Fatal("generate command not found")
	}

	expectedCommands := []string{"model", "controller", "view", "resource"}
	foundCommands := make(map[string]bool)

	for _, cmd := range generateCmd.Commands() {
		cmdName := strings.Fields(cmd.Use)[0]
		foundCommands[cmdName] = true
	}

	for _, expectedCmd := range expectedCommands {
		if !foundCommands[expectedCmd] {
			t.Errorf(
				"Expected command '%s' not found. Available commands: %v",
				expectedCmd,
				getCommandNames(generateCmd.Commands()),
			)
		}
	}
}

func getCommandNames(commands []*cobra.Command) []string {
	var names []string
	for _, cmd := range commands {
		cmdName := strings.Fields(cmd.Use)[0]
		names = append(names, cmdName)
	}
	return names
}

func TestProjectScaffolding__GoldenFile(t *testing.T) {
	tests := []struct {
		name           string
		projectName    string
		repoFlag       string
		expectedModule string
	}{
		{
			name:           "Should_scaffold_project_with_simple_name",
			projectName:    "testapp",
			repoFlag:       "",
			expectedModule: "testapp",
		},
		{
			name:           "Should_scaffold_project_with_github_repo",
			projectName:    "myapp",
			repoFlag:       "github.com/testuser",
			expectedModule: "github.com/testuser/myapp",
		},
		{
			name:           "Should_scaffold_project_with_simple_repo",
			projectName:    "webapp",
			repoFlag:       "myorg",
			expectedModule: "myorg/webapp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			projectDir := filepath.Join(tempDir, tt.projectName)

			originalWd, _ := os.Getwd()

			rootCmd := NewRootCommand("test", "test-date")

			args := []string{"new", tt.projectName}
			if tt.repoFlag != "" {
				args = append(args, "--repo", tt.repoFlag)
			}

			rootCmd.SetArgs(args)

			oldWd, _ := os.Getwd()
			defer os.Chdir(oldWd)
			os.Chdir(tempDir)

			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("Project scaffolding failed: %v", err)
			}

			scaffoldOutput := captureScaffoldedProject(t, projectDir)

			fixtureDir := filepath.Join(originalWd, "testdata")
			g := goldie.New(t, goldie.WithFixtureDir(fixtureDir), goldie.WithNameSuffix(".txt"))

			g.Assert(t, tt.name, []byte(scaffoldOutput))
		})
	}
}

func captureScaffoldedProject(t *testing.T, projectDir string) string {
	var output strings.Builder
	var allFiles []string

	err := filepath.WalkDir(projectDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			relPath, err := filepath.Rel(projectDir, path)
			if err != nil {
				return err
			}

			if strings.Contains(relPath, ".git/") ||
				strings.HasSuffix(relPath, ".mod.sum") ||
				strings.HasSuffix(relPath, "go.sum") {
				return nil
			}

			allFiles = append(allFiles, relPath)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Failed to walk project directory: %v", err)
	}

	sort.Strings(allFiles)

	output.WriteString("=== PROJECT STRUCTURE ===\n")
	for _, file := range allFiles {
		output.WriteString(file + "\n")
	}
	output.WriteString("\n")

	for _, file := range allFiles {
		filePath := filepath.Join(projectDir, file)

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read file %s: %v", file, err)
		}

		contentStr := string(content)
		if strings.HasSuffix(file, ".env.example") {
			contentStr = normalizeEnvSecrets(contentStr)
		}

		output.WriteString(fmt.Sprintf("=== %s ===\n", file))
		output.WriteString(contentStr)
		output.WriteString("\n\n")
	}

	return output.String()
}

func normalizeEnvSecrets(content string) string {
	content = replaceEnvValue(content, "PASSWORD_SALT=", "test_password_salt_value")
	content = replaceEnvValue(content, "SESSION_KEY=", "test_session_key_value")
	content = replaceEnvValue(
		content,
		"SESSION_ENCRYPTION_KEY=",
		"test_session_encryption_key_value",
	)
	content = replaceEnvValue(content, "TOKEN_SIGNING_KEY=", "test_token_signing_key_value")
	return content
}

func replaceEnvValue(content, prefix, testValue string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) {
			lines[i] = prefix + testValue
		}
	}
	return strings.Join(lines, "\n")
}

// TODO: Re-enable controller generation golden file tests once templ dependency issues are resolved
// func TestControllerGeneration__GoldenFile(t *testing.T) {
// 	tests := []struct {
// 		name        string
// 		args        []string
// 		resourceName string
// 		tableName   string
// 		withViews   bool
// 	}{
// 		{
// 			name:        "Should_generate_controller_without_views_by_default",
// 			args:        []string{"generate", "controller", "Product", "products"},
// 			resourceName: "Product",
// 			tableName:   "products", 
// 			withViews:   false,
// 		},
// 		{
// 			name:        "Should_generate_controller_with_views_when_flag_provided",
// 			args:        []string{"generate", "controller", "Product", "products", "--with-views"},
// 			resourceName: "Product",
// 			tableName:   "products",
// 			withViews:   true,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			tempDir := t.TempDir()
			
// 			originalWd, _ := os.Getwd()
			
// 			// Set up project structure with pre-generated model
// 			if err := setupTestProjectWithModel(t, tempDir, originalWd, tt.resourceName, tt.tableName); err != nil {
// 				t.Fatalf("Failed to set up test project: %v", err)
// 			}

// 			// Change to temp directory
// 			oldWd, _ := os.Getwd()
// 			defer os.Chdir(oldWd)
// 			os.Chdir(tempDir)

// 			// Generate the controller
// 			rootCmd := NewRootCommand("test", "test-date")
// 			rootCmd.SetArgs(tt.args)
			
// 			if err := rootCmd.Execute(); err != nil {
// 				t.Fatalf("Controller generation failed: %v", err)
// 			}

// 			// Capture the generated files
// 			generatedOutput := captureControllerGeneration(t, tempDir, tt.resourceName, tt.tableName, tt.withViews)

// 			// Assert against golden file
// 			fixtureDir := filepath.Join(originalWd, "testdata")
// 			g := goldie.New(t, goldie.WithFixtureDir(fixtureDir), goldie.WithNameSuffix(".txt"))
			
// 			g.Assert(t, tt.name, []byte(generatedOutput))
// 		})
// 	}
// }

func setupTestProjectWithModel(t *testing.T, tempDir, originalWd string, resourceName, tableName string) error {
	// Copy product table migration to database/migrations (default location)
	srcMigration := filepath.Join(filepath.Dir(originalWd), "generator", "controllers", "testdata", "migrations", "product_table", "001_create_products.sql")
	
	content, err := os.ReadFile(srcMigration)
	if err != nil {
		return fmt.Errorf("failed to read migration file %s: %w", srcMigration, err)
	}

	// Create required directories
	dirs := []string{"controllers", "models", "models/internal/db", "views", "router/routes", "database/migrations", "database/queries"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tempDir, dir), 0755); err != nil {
			return err
		}
	}
	
	// Write migration to database/migrations
	dstMigration := filepath.Join(tempDir, "database", "migrations", "001_create_products.sql")
	if err := os.WriteFile(dstMigration, content, 0644); err != nil {
		return err
	}

	// Create go.mod
	goMod := `module github.com/example/shop

go 1.21
`
	if err := os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return err
	}

	// Create sqlc.yaml in database directory
	sqlcConfig := `version: "2"
sql:
  - schema: migrations
    queries: queries
    engine: postgresql
    gen:
      go:
        package: db
        out: ../models/internal/db
        output_db_file_name: db.go
        output_models_file_name: entities.go
        emit_methods_with_db_argument: true
        sql_package: pgx/v5
        overrides:
          - db_type: uuid
            go_type: github.com/google/uuid.UUID
`
	if err := os.WriteFile(filepath.Join(tempDir, "database", "sqlc.yaml"), []byte(sqlcConfig), 0644); err != nil {
		return err
	}

	// Copy base controller file
	srcController := filepath.Join(filepath.Dir(originalWd), "generator", "controllers", "testdata", "base_controller.go")
	dstController := filepath.Join(tempDir, "controllers", "controller.go")
	
	controllerContent, err := os.ReadFile(srcController)
	if err != nil {
		return err
	}
	
	if err := os.WriteFile(dstController, controllerContent, 0644); err != nil {
		return err
	}

	// Copy base routes file
	srcRoutes := filepath.Join(filepath.Dir(originalWd), "generator", "controllers", "testdata", "base_routes.go")
	dstRoutes := filepath.Join(tempDir, "router", "routes", "routes.go")
	
	routesContent, err := os.ReadFile(srcRoutes)
	if err != nil {
		return err
	}
	
	if err := os.WriteFile(dstRoutes, routesContent, 0644); err != nil {
		return err
	}

	// Create a pre-generated model file to satisfy the dependency
	modelContent := `package models

import (
	"context"
	"github.com/google/uuid"
	"github.com/example/shop/models/internal/db"
)

type CreateProductPayload struct {
	Name        string
	Price       float64
	Description string
	CategoryId  int32
	InStock     bool
	Metadata    string
}

type UpdateProductPayload struct {
	ID          uuid.UUID
	Name        string
	Price       float64
	Description string
	CategoryId  int32
	InStock     bool
	Metadata    string
}

func CreateProduct(ctx context.Context, pool interface{}, payload CreateProductPayload) (*db.Product, error) {
	// Mock implementation
	return &db.Product{}, nil
}

func UpdateProduct(ctx context.Context, pool interface{}, payload UpdateProductPayload) (*db.Product, error) {
	// Mock implementation  
	return &db.Product{}, nil
}

func DestroyProduct(ctx context.Context, pool interface{}, id uuid.UUID) error {
	// Mock implementation
	return nil
}

func FindProduct(ctx context.Context, pool interface{}, id uuid.UUID) (*db.Product, error) {
	// Mock implementation
	return &db.Product{}, nil
}

func PaginateProducts(ctx context.Context, pool interface{}, page, perPage int64) (*db.ProductPaginationResult, error) {
	// Mock implementation
	return &db.ProductPaginationResult{}, nil
}
`

	modelFile := filepath.Join(tempDir, "models", strings.ToLower(resourceName)+".go")
	if err := os.WriteFile(modelFile, []byte(modelContent), 0644); err != nil {
		return err
	}

	// Create mock db types
	dbContent := `package db

import "github.com/google/uuid"

type Product struct {
	ID          uuid.UUID
	Name        string
	Price       float64
	Description string
	CategoryId  int32
	InStock     bool
	Metadata    string
}

type ProductPaginationResult struct {
	Products []Product
}
`

	dbFile := filepath.Join(tempDir, "models", "internal", "db", "entities.go")
	if err := os.WriteFile(dbFile, []byte(dbContent), 0644); err != nil {
		return err
	}

	return nil
}

func setupTestProject(t *testing.T, tempDir, originalWd string) error {
	// Copy product table migration to database/migrations (default location)
	srcMigration := filepath.Join(filepath.Dir(originalWd), "generator", "controllers", "testdata", "migrations", "product_table", "001_create_products.sql")
	
	content, err := os.ReadFile(srcMigration)
	if err != nil {
		return fmt.Errorf("failed to read migration file %s: %w", srcMigration, err)
	}

	// Create required directories
	dirs := []string{"controllers", "models", "models/internal/db", "views", "router/routes", "database/migrations", "database/queries"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tempDir, dir), 0755); err != nil {
			return err
		}
	}
	
	// Write migration to database/migrations
	dstMigration := filepath.Join(tempDir, "database", "migrations", "001_create_products.sql")
	if err := os.WriteFile(dstMigration, content, 0644); err != nil {
		return err
	}

	// Create go.mod
	goMod := `module github.com/example/shop

go 1.21
`
	if err := os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return err
	}

	// Create sqlc.yaml in database directory
	sqlcConfig := `version: "2"
sql:
  - schema: migrations
    queries: queries
    engine: postgresql
    gen:
      go:
        package: db
        out: ../models/internal/db
        output_db_file_name: db.go
        output_models_file_name: entities.go
        emit_methods_with_db_argument: true
        sql_package: pgx/v5
        overrides:
          - db_type: uuid
            go_type: github.com/google/uuid.UUID
`
	if err := os.WriteFile(filepath.Join(tempDir, "database", "sqlc.yaml"), []byte(sqlcConfig), 0644); err != nil {
		return err
	}

	// Copy base controller file
	srcController := filepath.Join(filepath.Dir(originalWd), "generator", "controllers", "testdata", "base_controller.go")
	dstController := filepath.Join(tempDir, "controllers", "controller.go")
	
	controllerContent, err := os.ReadFile(srcController)
	if err != nil {
		return err
	}
	
	if err := os.WriteFile(dstController, controllerContent, 0644); err != nil {
		return err
	}

	// Copy base routes file
	srcRoutes := filepath.Join(filepath.Dir(originalWd), "generator", "controllers", "testdata", "base_routes.go")
	dstRoutes := filepath.Join(tempDir, "router", "routes", "routes.go")
	
	routesContent, err := os.ReadFile(srcRoutes)
	if err != nil {
		return err
	}
	
	if err := os.WriteFile(dstRoutes, routesContent, 0644); err != nil {
		return err
	}

	return nil
}

func captureControllerGeneration(t *testing.T, tempDir, resourceName, tableName string, withViews bool) string {
	var output strings.Builder
	var relevantFiles []string

	// Files we want to capture
	expectedFiles := []string{
		filepath.Join("controllers", strings.ToLower(tableName)+".go"),
		filepath.Join("controllers", "controller.go"),  // Updated registration
		filepath.Join("router", "routes", strings.ToLower(tableName)+".go"), // Generated routes
		filepath.Join("router", "routes", "routes.go"),  // Updated routes registration
	}

	// If views were generated, include them
	if withViews {
		expectedFiles = append(expectedFiles, 
			filepath.Join("views", strings.ToLower(tableName)+"_resource.templ"),
			filepath.Join("views", strings.ToLower(tableName)+"_resource_templ.go"),
		)
	}

	// Check which files exist and add them
	for _, file := range expectedFiles {
		fullPath := filepath.Join(tempDir, file)
		if _, err := os.Stat(fullPath); err == nil {
			relevantFiles = append(relevantFiles, file)
		}
	}

	sort.Strings(relevantFiles)

	// Build output
	output.WriteString("=== CONTROLLER GENERATION OUTPUT ===\n")
	for _, file := range relevantFiles {
		output.WriteString(file + "\n")
	}
	output.WriteString("\n")

	for _, file := range relevantFiles {
		filePath := filepath.Join(tempDir, file)
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Logf("Warning: Failed to read file %s: %v", file, err)
			continue
		}

		output.WriteString(fmt.Sprintf("=== %s ===\n", file))
		output.WriteString(string(content))
		output.WriteString("\n\n")
	}

	return output.String()
}
