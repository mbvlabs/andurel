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
	benchmarkInDirectory(b, tmpDir)

	// Create minimal required structure
	benchmarkMkdirAll(b, "database/migrations")
	benchmarkMkdirAll(b, "router/routes")
	benchmarkWriteFile(b, "go.mod", "module test\n\ngo 1.21\n")

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
	benchmarkInDirectory(b, tmpDir)

	// Create minimal required structure
	benchmarkMkdirAll(b, "database/migrations")
	benchmarkMkdirAll(b, "router/routes")
	benchmarkWriteFile(b, "go.mod", "module test\n\ngo 1.21\n")

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
				if err := validator.ValidateResourceName(tc.resourceName); err != nil {
					b.Fatal(err)
				}
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
				if err := validator.ValidateTableName(tc.tableName); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkFindGoModRoot benchmarks finding go.mod root
func BenchmarkFindGoModRoot(b *testing.B) {
	tmpDir := b.TempDir()

	// Create nested directory structure with go.mod at root
	nestedPath := filepath.Join(tmpDir, "a", "b", "c", "d")
	benchmarkMkdirAll(b, nestedPath)
	benchmarkWriteFile(b, filepath.Join(tmpDir, "go.mod"), "module test\n")
	benchmarkInDirectory(b, nestedPath)

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
	benchmarkInDirectory(b, tmpDir)

	benchmarkWriteFile(b, "go.mod", "module github.com/example/test\n\ngo 1.21\n")

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
	benchmarkInDirectory(b, tmpDir)

	// Create migration directory and files
	migrationDir := "internal/database/migrations"
	benchmarkMkdirAll(b, migrationDir)

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
	benchmarkWriteFile(b, filepath.Join(migrationDir, "001_create_users.sql"), migrationContent)
	benchmarkWriteFile(b, "go.mod", "module test\n\ngo 1.21\n")

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
		if err := validator.ValidateResourceName("User"); err != nil {
			b.Fatal(err)
		}
		if err := validator.ValidateTableName("users"); err != nil {
			b.Fatal(err)
		}
		if err := validator.ValidateModulePath("github.com/example/test"); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkInDirectory(b *testing.B, directory string) {
	b.Helper()
	original, err := os.Getwd()
	if err != nil {
		b.Fatal(err)
	}
	if err := os.Chdir(directory); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := os.Chdir(original); err != nil {
			b.Errorf("restore working directory: %v", err)
		}
	})
}

func benchmarkMkdirAll(b *testing.B, path string) {
	b.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		b.Fatal(err)
	}
}

func benchmarkWriteFile(b *testing.B, path, content string) {
	b.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		b.Fatal(err)
	}
}
