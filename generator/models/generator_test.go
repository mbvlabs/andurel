package models

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/pkg/constants"
)

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

	// Verify conversion from DB extracts bytes
	if !strings.Contains(settingsField.ConversionFromDB, ".Bytes") {
		t.Errorf("Settings ConversionFromDB = %s, expected to contain '.Bytes'", settingsField.ConversionFromDB)
	}

	if !strings.Contains(metadataField.ConversionFromDB, ".Bytes") {
		t.Errorf("Metadata ConversionFromDB = %s, expected to contain '.Bytes'", metadataField.ConversionFromDB)
	}

	// Verify conversion to DB wraps bytes in pgtype struct
	if !strings.Contains(settingsField.ConversionToDB, "pgtype.JSONB") {
		t.Errorf("Settings ConversionToDB = %s, expected to contain 'pgtype.JSONB'", settingsField.ConversionToDB)
	}

	if !strings.Contains(metadataField.ConversionToDB, "pgtype.JSON") {
		t.Errorf("Metadata ConversionToDB = %s, expected to contain 'pgtype.JSON'", metadataField.ConversionToDB)
	}
}
