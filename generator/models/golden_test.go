package models

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"mbvlabs/andurel/generator/templates"
)

var update = flag.Bool("update", false, "update golden files")

func TestGenerator_GoldenFiles(t *testing.T) {
	tests := []struct {
		name          string
		migrationsDir string
		tableName     string
		resourceName  string
		modulePath    string
	}{
		{
			name:          "simple_user_table",
			migrationsDir: "simple_user_table",
			tableName:     "users",
			resourceName:  "User",
			modulePath:    "github.com/example/myapp",
		},
		{
			name:          "product_table_with_decimals",
			migrationsDir: "product_table_with_decimals",
			tableName:     "products",
			resourceName:  "Product",
			modulePath:    "github.com/example/shop",
		},
		{
			name:          "complex_table",
			migrationsDir: "complex_table",
			tableName:     "comprehensive_example",
			resourceName:  "ComprehensiveExample",
			modulePath:    "github.com/example/complex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			queriesDir := filepath.Join(tempDir, "database", "queries")
			modelsDir := filepath.Join(tempDir, "models")

			err := os.MkdirAll(queriesDir, 0o755)
			if err != nil {
				t.Fatalf("Failed to create queries directory: %v", err)
			}

			err = os.MkdirAll(modelsDir, 0o755)
			if err != nil {
				t.Fatalf("Failed to create models directory: %v", err)
			}

			originalWd, _ := os.Getwd()

			oldWd, _ := os.Getwd()
			defer os.Chdir(oldWd)
			os.Chdir(tempDir)

			migrationsDir := filepath.Join(originalWd, "testdata", "migrations", tt.migrationsDir)

			generator := NewGenerator("postgresql")

			cat, err := generator.buildCatalogFromTableMigrations(
				tt.tableName,
				[]string{migrationsDir},
			)
			if err != nil {
				t.Fatalf("Failed to build catalog from migrations: %v", err)
			}

			model, err := generator.Build(cat, Config{
				TableName:    tt.tableName,
				ResourceName: tt.resourceName,
				PackageName:  "models",
				DatabaseType: "postgresql",
				ModulePath:   tt.modulePath,
			})
			if err != nil {
				t.Fatalf("Failed to build model: %v", err)
			}

			templateContent, err := templates.Files.ReadFile("model.tmpl")
			if err != nil {
				t.Fatalf("Failed to read model template: %v", err)
			}

			modelContent, err := generator.GenerateModelFile(model, string(templateContent))
			if err != nil {
				t.Fatalf("Failed to generate model content: %v", err)
			}

			table, err := cat.GetTable("", tt.tableName)
			if err != nil {
				t.Fatalf("Failed to get table from catalog: %v", err)
			}

			sqlContent, err := generator.GenerateSQLContent(tt.resourceName, tt.tableName, table)
			if err != nil {
				t.Fatalf("Failed to generate SQL content: %v", err)
			}

			modelPath := filepath.Join("models", strings.ToLower(tt.resourceName)+".go")
			sqlPath := filepath.Join("database", "queries", tt.tableName+".sql")

			err = os.WriteFile(modelPath, []byte(modelContent), 0o644)
			if err != nil {
				t.Fatalf("Failed to write model file: %v", err)
			}

			err = formatGoFile(modelPath)
			if err != nil {
				t.Fatalf("Failed to format model file: %v", err)
			}

			err = os.WriteFile(sqlPath, []byte(sqlContent), 0o644)
			if err != nil {
				t.Fatalf("Failed to write SQL file: %v", err)
			}

			t.Run("model_file", func(t *testing.T) {
				compareWithGolden(t, tt.name+"_model.go", modelPath, *update, originalWd)
			})

			t.Run("sql_file", func(t *testing.T) {
				compareWithGolden(t, tt.name+"_queries.sql", sqlPath, *update, originalWd)
			})
		})
	}
}

func compareWithGolden(
	t *testing.T,
	goldenFile, actualFile string,
	update bool,
	originalWd string,
) {
	actualContent, err := os.ReadFile(actualFile)
	if err != nil {
		t.Fatalf("Failed to read actual file %s: %v", actualFile, err)
	}

	goldenPath := filepath.Join(originalWd, "testdata", goldenFile)

	if update {
		err = os.MkdirAll(filepath.Dir(goldenPath), 0o755)
		if err != nil {
			t.Fatalf("Failed to create testdata directory: %v", err)
		}

		err = os.WriteFile(goldenPath, actualContent, 0o644)
		if err != nil {
			t.Fatalf("Failed to update golden file %s: %v", goldenPath, err)
		}
		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	expectedContent, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("Failed to read golden file %s: %v (run with -update to create)", goldenPath, err)
	}

	actualStr := strings.TrimSpace(string(actualContent))
	expectedStr := strings.TrimSpace(string(expectedContent))

	if actualStr != expectedStr {
		t.Errorf("Generated content doesn't match golden file %s\n\nActual:\n%s\n\nExpected:\n%s",
			goldenPath, actualStr, expectedStr)
	}
}

func formatGoFile(filePath string) error {
	cmd := exec.Command("go", "fmt", filePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run go fmt on %s: %w", filePath, err)
	}
	return nil
}
