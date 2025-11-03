package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mbvlabs/andurel/generator/files"
)

// BenchmarkNewCoordinator benchmarks the creation of a new coordinator
func BenchmarkNewCoordinator(b *testing.B) {
	// Change to a temp directory for benchmarking
	tmpDir := b.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tmpDir)

	// Create minimal required structure
	os.MkdirAll("database/migrations", 0755)
	os.MkdirAll("router/routes", 0755)
	os.WriteFile("go.mod", []byte("module test\n\ngo 1.21\n"), 0644)
	os.WriteFile("database/sqlc.yaml", []byte("version: \"2\"\nsql:\n  - engine: \"postgresql\"\n    queries: \"database/queries\"\n    schema: \"database/migrations\"\n"), 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := NewCoordinator()
		if err != nil {
			b.Fatalf("failed to create coordinator: %v", err)
		}
	}
}

// BenchmarkConfigManagerLoad benchmarks config loading
func BenchmarkConfigManagerLoad(b *testing.B) {
	tmpDir := b.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tmpDir)

	// Create minimal required structure
	os.MkdirAll("database/migrations", 0755)
	os.MkdirAll("router/routes", 0755)
	os.WriteFile("go.mod", []byte("module test\n\ngo 1.21\n"), 0644)
	os.WriteFile("database/sqlc.yaml", []byte("version: \"2\"\nsql:\n  - engine: \"postgresql\"\n    queries: \"database/queries\"\n    schema: \"database/migrations\"\n"), 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cm := NewConfigManager()
		_, err := cm.Load()
		if err != nil {
			b.Fatalf("failed to load config: %v", err)
		}
	}
}

// BenchmarkValidateResourceName benchmarks resource name validation
func BenchmarkValidateResourceName(b *testing.B) {
	validator := NewInputValidator()

	testCases := []struct {
		name         string
		resourceName string
	}{
		{"ValidSimple", "User"},
		{"ValidComplex", "AdminUserProfile"},
		{"ValidLong", "VeryLongResourceNameWithManyWords"},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = validator.ValidateResourceName(tc.resourceName)
			}
		})
	}
}

// BenchmarkValidateTableName benchmarks table name validation
func BenchmarkValidateTableName(b *testing.B) {
	validator := NewInputValidator()

	testCases := []struct {
		name      string
		tableName string
	}{
		{"ValidSimple", "users"},
		{"ValidComplex", "admin_user_profiles"},
		{"ValidLong", "very_long_table_name_with_many_words"},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = validator.ValidateTableName(tc.tableName)
			}
		})
	}
}

// BenchmarkFindGoModRoot benchmarks finding go.mod root
func BenchmarkFindGoModRoot(b *testing.B) {
	tmpDir := b.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	// Create nested directory structure with go.mod at root
	nestedPath := filepath.Join(tmpDir, "a", "b", "c", "d")
	os.MkdirAll(nestedPath, 0755)
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\n"), 0644)
	os.Chdir(nestedPath)

	fm := files.NewUnifiedFileManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := fm.FindGoModRoot()
		if err != nil {
			b.Fatalf("failed to find go.mod root: %v", err)
		}
	}
}

// BenchmarkProjectManagerGetModulePath benchmarks getting module path
func BenchmarkProjectManagerGetModulePath(b *testing.B) {
	tmpDir := b.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tmpDir)

	os.WriteFile("go.mod", []byte("module github.com/example/test\n\ngo 1.21\n"), 0644)

	pm, err := NewProjectManager()
	if err != nil {
		b.Fatalf("failed to create project manager: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pm.GetModulePath()
	}
}

// BenchmarkMigrationManagerBuildCatalog benchmarks catalog building from migrations
func BenchmarkMigrationManagerBuildCatalog(b *testing.B) {
	tmpDir := b.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tmpDir)

	// Create migration directory and files
	migrationDir := "internal/database/migrations"
	os.MkdirAll(migrationDir, 0755)

	// Create a sample migration
	migrationContent := `-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY,
    username TEXT NOT NULL,
    email TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE users;
`
	os.WriteFile(filepath.Join(migrationDir, "001_create_users.sql"), []byte(migrationContent), 0644)
	os.WriteFile("go.mod", []byte("module test\n\ngo 1.21\n"), 0644)
	os.WriteFile("sqlc.yaml", []byte("version: \"2\"\nsql:\n  - engine: \"postgresql\"\n    queries: \"internal/database/queries\"\n    schema: \"internal/database/migrations\"\n"), 0644)

	config := &UnifiedConfig{
		Database: DatabaseConfig{
			Type:          "postgresql",
			MigrationDirs: []string{migrationDir},
		},
	}

	mm := NewMigrationManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mm.BuildCatalogFromMigrations("users", config)
		if err != nil {
			b.Fatalf("failed to build catalog: %v", err)
		}
	}
}

// BenchmarkInputValidation benchmarks input validation operations
func BenchmarkInputValidation(b *testing.B) {
	validator := NewInputValidator()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate validation operations
		_ = validator.ValidateResourceName("User")
		_ = validator.ValidateTableName("users")
		_ = validator.ValidateModulePath("github.com/example/test")
	}
}
