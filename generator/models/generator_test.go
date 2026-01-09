package models

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/pkg/constants"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func TestGetSimpleGoType(t *testing.T) {
	tests := []struct {
		name         string
		goType       string
		sqlcType     string
		expectedType string
	}{
		// JSONB and JSON types
		{
			name:         "pgtype.JSONB should return []byte",
			goType:       "[]byte",
			sqlcType:     "pgtype.JSONB",
			expectedType: "[]byte",
		},
		{
			name:         "pgtype.JSON should return []byte",
			goType:       "[]byte",
			sqlcType:     "pgtype.JSON",
			expectedType: "[]byte",
		},
		// Integer types
		{
			name:         "pgtype.Int4 should return int32",
			goType:       "int32",
			sqlcType:     "pgtype.Int4",
			expectedType: "int32",
		},
		{
			name:         "pgtype.Int8 should return int64",
			goType:       "int64",
			sqlcType:     "pgtype.Int8",
			expectedType: "int64",
		},
		{
			name:         "pgtype.Int2 should return int16",
			goType:       "int16",
			sqlcType:     "pgtype.Int2",
			expectedType: "int16",
		},
		// Float types
		{
			name:         "pgtype.Float4 should return float32",
			goType:       "float32",
			sqlcType:     "pgtype.Float4",
			expectedType: "float32",
		},
		{
			name:         "pgtype.Float8 should return float64",
			goType:       "float64",
			sqlcType:     "pgtype.Float8",
			expectedType: "float64",
		},
		// Boolean type
		{
			name:         "pgtype.Bool should return bool",
			goType:       "bool",
			sqlcType:     "pgtype.Bool",
			expectedType: "bool",
		},
		// String type
		{
			name:         "pgtype.Text should return string",
			goType:       "string",
			sqlcType:     "pgtype.Text",
			expectedType: "string",
		},
		// Time types
		{
			name:         "pgtype.Timestamp should return time.Time",
			goType:       "time.Time",
			sqlcType:     "pgtype.Timestamp",
			expectedType: "time.Time",
		},
		{
			name:         "pgtype.Timestamptz should return time.Time",
			goType:       "time.Time",
			sqlcType:     "pgtype.Timestamptz",
			expectedType: "time.Time",
		},
		{
			name:         "pgtype.Date should return time.Time",
			goType:       "time.Time",
			sqlcType:     "pgtype.Date",
			expectedType: "time.Time",
		},
		{
			name:         "pgtype.Time should return time.Time",
			goType:       "time.Time",
			sqlcType:     "pgtype.Time",
			expectedType: "time.Time",
		},
		// sql.Null* types
		{
			name:         "sql.NullString should return string",
			goType:       "string",
			sqlcType:     "sql.NullString",
			expectedType: "string",
		},
		{
			name:         "sql.NullInt64 should return int64",
			goType:       "int64",
			sqlcType:     "sql.NullInt64",
			expectedType: "int64",
		},
		{
			name:         "sql.NullFloat64 should return float64",
			goType:       "float64",
			sqlcType:     "sql.NullFloat64",
			expectedType: "float64",
		},
		{
			name:         "sql.NullBool should return bool",
			goType:       "bool",
			sqlcType:     "sql.NullBool",
			expectedType: "bool",
		},
		{
			name:         "sql.NullTime should return time.Time",
			goType:       "time.Time",
			sqlcType:     "sql.NullTime",
			expectedType: "time.Time",
		},
		// Simple types that should pass through
		{
			name:         "string should pass through",
			goType:       "string",
			sqlcType:     "string",
			expectedType: "string",
		},
		{
			name:         "int32 should pass through",
			goType:       "int32",
			sqlcType:     "int32",
			expectedType: "int32",
		},
		{
			name:         "[]byte should pass through",
			goType:       "[]byte",
			sqlcType:     "[]byte",
			expectedType: "[]byte",
		},
		{
			name:         "uuid.UUID should pass through",
			goType:       "uuid.UUID",
			sqlcType:     "uuid.UUID",
			expectedType: "uuid.UUID",
		},
	}

	generator := NewGenerator("postgresql")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.getSimpleGoType(tt.goType, tt.sqlcType)
			if result != tt.expectedType {
				t.Errorf("getSimpleGoType(%s, %s) = %s, want %s",
					tt.goType, tt.sqlcType, result, tt.expectedType)
			}
		})
	}
}

// TestJSONBFieldGeneration verifies that JSONB fields are correctly generated
// as []byte in model structs, not as pgtype.JSONB
func TestJSONBFieldGeneration(t *testing.T) {
	tempDir := t.TempDir()
	queriesDir := filepath.Join(tempDir, "database", "queries")
	modelsDir := filepath.Join(tempDir, "models")

	err := os.MkdirAll(queriesDir, constants.DirPermissionDefault)
	if err != nil {
		t.Fatalf("Failed to create queries directory: %v", err)
	}

	err = os.MkdirAll(modelsDir, constants.DirPermissionDefault)
	if err != nil {
		t.Fatalf("Failed to create models directory: %v", err)
	}

	originalWd, _ := os.Getwd()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tempDir)

	migrationsDir := filepath.Join(originalWd, "testdata", "migrations", "jsonb_test")

	generator := NewGenerator("postgresql")

	cat, err := generator.buildCatalogFromTableMigrations(
		"configs",
		[]string{migrationsDir},
	)
	if err != nil {
		t.Fatalf("Failed to build catalog from migrations: %v", err)
	}

	// Build model to ensure it works
	model, err := generator.Build(cat, Config{
		TableName:    "configs",
		ResourceName: "Config",
		PackageName:  "models",
		DatabaseType: "postgresql",
		ModulePath:   "github.com/example/test",
	})
	if err != nil {
		t.Fatalf("Failed to build model from catalog: %v", err)
	}

	// Verify model has fields
	if len(model.Fields) == 0 {
		t.Fatal("Expected model to have fields")
	}

	// Find the JSONB fields and verify they use []byte type
	var settingsField, metadataField *GeneratedField
	for i := range model.Fields {
		if model.Fields[i].Name == "Settings" {
			settingsField = &model.Fields[i]
		}
		if model.Fields[i].Name == "Metadata" {
			metadataField = &model.Fields[i]
		}
	}

	if settingsField == nil {
		t.Fatal("Expected to find 'Settings' field (from 'settings' jsonb column)")
	}

	if metadataField == nil {
		t.Fatal("Expected to find 'Metadata' field (from 'metadata' json column)")
	}

	// Verify JSONB field uses []byte, not pgtype.JSONB
	if settingsField.Type != "[]byte" {
		t.Errorf("Settings field Type = %s, want []byte (not pgtype.JSONB)", settingsField.Type)
	}

	// Verify JSONB field has correct SQLCType for conversions
	if settingsField.SQLCType != "pgtype.JSONB" {
		t.Errorf("Settings field SQLCType = %s, want pgtype.JSONB", settingsField.SQLCType)
	}

	// Verify JSON field uses []byte, not pgtype.JSON
	if metadataField.Type != "[]byte" {
		t.Errorf("Metadata field Type = %s, want []byte (not pgtype.JSON)", metadataField.Type)
	}

	// Verify JSON field has correct SQLCType for conversions
	if metadataField.SQLCType != "pgtype.JSON" {
		t.Errorf("Metadata field SQLCType = %s, want pgtype.JSON", metadataField.SQLCType)
	}

	// Verify conversion from DB - in pgx v5, JSONB/JSON are []byte type aliases, so no .Bytes needed
	if settingsField.ConversionFromDB != "row.Settings" {
		t.Errorf("Settings ConversionFromDB = %s, expected 'row.Settings'", settingsField.ConversionFromDB)
	}

	if metadataField.ConversionFromDB != "row.Metadata" {
		t.Errorf("Metadata ConversionFromDB = %s, expected 'row.Metadata'", metadataField.ConversionFromDB)
	}

	// Verify conversion to DB - in pgx v5, []byte is passed directly without wrapping
	if settingsField.ConversionToDB != "data.Settings" {
		t.Errorf("Settings ConversionToDB = %s, expected 'data.Settings'", settingsField.ConversionToDB)
	}

	if metadataField.ConversionToDB != "data.Metadata" {
		t.Errorf("Metadata ConversionToDB = %s, expected 'data.Metadata'", metadataField.ConversionToDB)
	}
}

// TestGeneratedModelUsesResourceName verifies that generated models use the
// exact resource name provided by the user (e.g., "Category") rather than
// attempting to derive it from the table name (e.g., "categories").
func TestGeneratedModelUsesResourceName(t *testing.T) {
	tests := []struct {
		name         string
		tableName    string
		resourceName string
		migration    string
	}{
		{
			name:         "categories_to_category",
			tableName:    "categories",
			resourceName: "Category",
			migration: `-- +goose Up
CREATE TABLE categories (
    id uuid PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) NOT NULL
);

-- +goose Down
DROP TABLE categories;`,
		},
		{
			name:         "queries_to_query",
			tableName:    "queries",
			resourceName: "Query",
			migration: `-- +goose Up
CREATE TABLE queries (
    id uuid PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    sql_text TEXT NOT NULL,
    status VARCHAR(50) NOT NULL
);

-- +goose Down
DROP TABLE queries;`,
		},
		{
			name:         "factories_to_factory",
			tableName:    "factories",
			resourceName: "Factory",
			migration: `-- +goose Up
CREATE TABLE factories (
    id uuid PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    name VARCHAR(200) NOT NULL,
    location VARCHAR(200)
);

-- +goose Down
DROP TABLE factories;`,
		},
		{
			name:         "stories_to_story",
			tableName:    "stories",
			resourceName: "Story",
			migration: `-- +goose Up
CREATE TABLE stories (
    id uuid PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    title VARCHAR(255) NOT NULL,
    content TEXT
);

-- +goose Down
DROP TABLE stories;`,
		},
		{
			name:         "companies_to_company",
			tableName:    "companies",
			resourceName: "Company",
			migration: `-- +goose Up
CREATE TABLE companies (
    id uuid PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    name VARCHAR(200) NOT NULL,
    industry VARCHAR(100)
);

-- +goose Down
DROP TABLE companies;`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			migrationsDir := filepath.Join(tempDir, "database", "migrations")

			err := os.MkdirAll(migrationsDir, constants.DirPermissionDefault)
			if err != nil {
				t.Fatalf("Failed to create migrations directory: %v", err)
			}

			migrationFile := filepath.Join(migrationsDir, "001_create_"+tt.tableName+".sql")
			err = os.WriteFile(migrationFile, []byte(tt.migration), constants.FilePermissionPrivate)
			if err != nil {
				t.Fatalf("Failed to write migration file: %v", err)
			}

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
				ModulePath:   "github.com/example/test",
			})
			if err != nil {
				t.Fatalf("Failed to build model from catalog: %v", err)
			}

			if model.Name != tt.resourceName {
				t.Errorf("Model name = %s, want %s", model.Name, tt.resourceName)
			}

			templateContent, err := os.ReadFile(filepath.Join("..", "..", "generator", "templates", "model.tmpl"))
			if err != nil {
				t.Fatalf("Failed to read model template: %v", err)
			}

			generatedCode, err := generator.GenerateModelFile(model, string(templateContent))
			if err != nil {
				t.Fatalf("Failed to generate model file: %v", err)
			}

			// Golden file testing
			goldenFile := filepath.Join("testdata", "golden", tt.name+".golden")

			// Update golden file if -update flag is set
			if *updateGolden {
				err := os.MkdirAll(filepath.Dir(goldenFile), constants.DirPermissionDefault)
				if err != nil {
					t.Fatalf("Failed to create golden directory: %v", err)
				}
				err = os.WriteFile(goldenFile, []byte(generatedCode), constants.FilePermissionPrivate)
				if err != nil {
					t.Fatalf("Failed to write golden file: %v", err)
				}
			}

			// Read golden file
			expectedCode, err := os.ReadFile(goldenFile)
			if err != nil {
				t.Fatalf("Failed to read golden file %s: %v\nRun 'go test -update' to create it", goldenFile, err)
			}

			if string(expectedCode) != generatedCode {
				t.Errorf("Generated code differs from golden file.\nExpected:\n%s\n\nGot:\n%s", string(expectedCode), generatedCode)
			}

			// Also verify critical patterns
			expectedType := "db." + tt.resourceName
			if !strings.Contains(generatedCode, expectedType) {
				t.Errorf("Generated code should contain '%s'", expectedType)
			}

			// Verify it doesn't use naive singularization
			naiveSingular := "db." + strings.TrimSuffix(tt.tableName, "s")
			naiveSingular = strings.ToUpper(naiveSingular[:3]) + naiveSingular[3:]
			if naiveSingular != expectedType && strings.Contains(generatedCode, naiveSingular) {
				t.Errorf("Generated code should NOT contain naive singularization '%s'", naiveSingular)
			}
		})
	}
}
