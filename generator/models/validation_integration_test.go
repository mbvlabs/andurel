package models

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/pkg/constants"
)

func TestModelFileGeneration_InvalidPrimaryKeyTypes(t *testing.T) {
	tests := []struct {
		name             string
		migrationsDir    string
		tableName        string
		resourceName     string
		modulePath       string
		databaseType     string
		expectedErrorMsg string
	}{
		{
			name:             "Should reject PostgreSQL migration with TEXT primary key",
			migrationsDir:    "invalid_pg_primary_key",
			tableName:        "users",
			resourceName:     "User",
			modulePath:       "github.com/example/myapp",
			databaseType:     "postgresql",
			expectedErrorMsg: "primary keys must use 'uuid'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			originalWd, _ := os.Getwd()

			oldWd, _ := os.Getwd()
			defer os.Chdir(oldWd)
			os.Chdir(tempDir)

			migrationsDir := filepath.Join(originalWd, "testdata", "migrations", tt.migrationsDir)

			generator := NewGenerator(tt.databaseType)

			_, err := generator.buildCatalogFromTableMigrations(
				tt.tableName,
				[]string{migrationsDir},
			)

			if err == nil {
				t.Fatal("Expected error due to invalid primary key type, but got none")
			}

			if !strings.Contains(err.Error(), tt.expectedErrorMsg) {
				t.Errorf(
					"Expected error message to contain '%s', but got: %s",
					tt.expectedErrorMsg,
					err.Error(),
				)
			}

			if !strings.Contains(err.Error(), "001_users_text_pk.sql") {
				t.Errorf(
					"Expected error message to contain migration file name, but got: %s",
					err.Error(),
				)
			}
		})
	}
}

func TestRefreshQueries__ValidatesIDColumns(t *testing.T) {
	tests := []struct {
		name             string
		migrationsDir    string
		tableName        string
		resourceName     string
		databaseType     string
		expectedErrorMsg string
		shouldFail       bool
	}{
		{
			name:          "Should succeed with valid PostgreSQL UUID primary key",
			migrationsDir: "simple_user_table",
			tableName:     "users",
			resourceName:  "User",
			databaseType:  "postgresql",
			shouldFail:    false,
		},
		{
			name:             "Should fail with invalid PostgreSQL TEXT primary key",
			migrationsDir:    "invalid_pg_primary_key",
			tableName:        "users",
			resourceName:     "User",
			databaseType:     "postgresql",
			expectedErrorMsg: "primary keys must use 'uuid'",
			shouldFail:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			queriesDir := filepath.Join(tempDir, "database", "queries")

			err := os.MkdirAll(queriesDir, constants.DirPermissionDefault)
			if err != nil {
				t.Fatalf("Failed to create queries directory: %v", err)
			}

			originalWd, _ := os.Getwd()
			oldWd, _ := os.Getwd()
			defer os.Chdir(oldWd)
			os.Chdir(tempDir)

			migrationsDir := filepath.Join(originalWd, "testdata", "migrations", tt.migrationsDir)
			sqlPath := filepath.Join(queriesDir, tt.tableName+".sql")

			// Create a dummy SQL file for the test
			err = os.WriteFile(sqlPath, []byte("-- dummy queries"), constants.FilePermissionPrivate)
			if err != nil {
				t.Fatalf("Failed to create SQL file: %v", err)
			}

			generator := NewGenerator(tt.databaseType)

			cat, err := generator.buildCatalogFromTableMigrations(
				tt.tableName,
				[]string{migrationsDir},
			)
			if err != nil && !tt.shouldFail {
				t.Fatalf("Failed to build catalog from migrations: %v", err)
			}
			if err != nil && tt.shouldFail {
				// Expected failure during catalog building due to ID validation
				if !strings.Contains(err.Error(), tt.expectedErrorMsg) {
					t.Errorf(
						"Expected error message to contain '%s', but got: %s",
						tt.expectedErrorMsg,
						err.Error(),
					)
				}
				return
			}

			err = generator.RefreshQueries(cat, tt.resourceName, tt.tableName, sqlPath)

			if tt.shouldFail {
				if err == nil {
					t.Fatal("Expected error due to invalid primary key type, but got none")
				}
				if !strings.Contains(err.Error(), tt.expectedErrorMsg) {
					t.Errorf(
						"Expected error message to contain '%s', but got: %s",
						tt.expectedErrorMsg,
						err.Error(),
					)
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestConstructorImports__OnlyNecessaryImports(t *testing.T) {
	tests := []struct {
		name              string
		migrationsDir     string
		tableName         string
		resourceName      string
		modulePath        string
		expectedImports   []string
		unexpectedImports []string
	}{
		{
			name:          "PostgreSQL user model should only import uuid and pgtype",
			migrationsDir: "simple_user_table",
			tableName:     "users",
			resourceName:  "User",
			modulePath:    "github.com/example/myapp",
			expectedImports: []string{
				`"github.com/google/uuid"`,
				`"github.com/jackc/pgx/v5/pgtype"`,
			},
			unexpectedImports: []string{
				`"time"`,
				`"database/sql"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			internalDBDir := filepath.Join(tempDir, "models", "internal", "db")

			err := os.MkdirAll(internalDBDir, constants.DirPermissionDefault)
			if err != nil {
				t.Fatalf("Failed to create internal/db directory: %v", err)
			}

			originalWd, _ := os.Getwd()
			oldWd, _ := os.Getwd()
			defer os.Chdir(oldWd)
			os.Chdir(tempDir)

			migrationsDir := filepath.Join(
				originalWd,
				"testdata",
				"migrations",
				tt.migrationsDir,
			)

			generator := NewGenerator("postgresql")

			cat, err := generator.buildCatalogFromTableMigrations(
				tt.tableName,
				[]string{migrationsDir},
			)
			if err != nil {
				t.Fatalf("Failed to build catalog: %v", err)
			}

			constructorFileName := fmt.Sprintf(
				"%s_constructors.go",
				strings.ToLower(tt.resourceName),
			)
			constructorPath := filepath.Join(internalDBDir, constructorFileName)

			err = generator.GenerateConstructors(
				cat,
				tt.resourceName,
				tt.tableName,
				constructorPath,
				tt.modulePath,
			)
			if err != nil {
				t.Fatalf("Failed to generate constructors: %v", err)
			}

			constructorContent, err := os.ReadFile(constructorPath)
			if err != nil {
				t.Fatalf("Failed to read constructor file: %v", err)
			}

			constructorStr := string(constructorContent)

			// Verify expected imports are present
			for _, expectedImport := range tt.expectedImports {
				if !strings.Contains(constructorStr, expectedImport) {
					t.Errorf("Constructor file should contain import %s", expectedImport)
				}
			}

			// Verify unexpected imports are not present
			for _, unexpectedImport := range tt.unexpectedImports {
				if strings.Contains(constructorStr, unexpectedImport) {
					t.Errorf(
						"Constructor file should NOT contain unused import %s",
						unexpectedImport,
					)
				}
			}
		})
	}
}

func formatGoFile(filePath string) error {
	cmd := exec.Command("go", "fmt", filePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run go fmt on %s: %w", filePath, err)
	}
	return nil
}
