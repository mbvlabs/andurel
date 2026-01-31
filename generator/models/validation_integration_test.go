package models

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/pkg/constants"
)

func TestModelFileGeneration_ValidPrimaryKeyTypes(t *testing.T) {
	tests := []struct {
		name          string
		migrationsDir string
		tableName     string
		resourceName  string
		modulePath    string
		databaseType  string
	}{
		{
			name:          "Should accept PostgreSQL migration with TEXT primary key",
			migrationsDir: "text_pk",
			tableName:     "users",
			resourceName:  "User",
			modulePath:    "github.com/example/myapp",
			databaseType:  "postgresql",
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

			if err != nil {
				t.Fatalf("Expected no error but got: %v", err)
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
			name:          "Should succeed with valid PostgreSQL TEXT primary key",
			migrationsDir: "text_pk",
			tableName:     "users",
			resourceName:  "User",
			databaseType:  "postgresql",
			shouldFail:    false,
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

			err = generator.RefreshQueries(cat, tt.resourceName, tt.tableName, sqlPath, false)

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
