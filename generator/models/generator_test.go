package models

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mbvlabs/andurel/generator/templates"
)

func TestBuildModel(t *testing.T) {
	originalWd, _ := os.Getwd()
	migrationsDir := filepath.Join(originalWd, "testdata", "migrations", "none_pgtype_data_types")

	generator := NewGenerator("postgresql")

	cat, err := generator.BuildCatalogFromMigrations("articles", []string{migrationsDir})
	if err != nil {
		t.Fatalf("Failed to build catalog from migrations: %v", err)
	}

	model, err := generator.Build(cat, Config{
		TableName:    "articles",
		ResourceName: "Article",
		PackageName:  "models",
		DatabaseType: "postgresql",
		ModulePath:   "github.com/example/test",
	})
	if err != nil {
		t.Fatalf("Failed to build model from catalog: %v", err)
	}

	if len(model.Fields) == 0 {
		t.Fatal("Expected model to have fields")
	}

	// Check entity name and namespace
	if model.EntityName != "ArticleEntity" {
		t.Errorf("EntityName = %s, want ArticleEntity", model.EntityName)
	}
	if model.NamespaceVar != "Article" {
		t.Errorf("NamespaceVar = %s, want Article", model.NamespaceVar)
	}
	if model.NamespaceType != "article" {
		t.Errorf("NamespaceType = %s, want article", model.NamespaceType)
	}

	// Find specific fields and verify their types
	var tagsField, scoresField *GeneratedField
	for i := range model.Fields {
		switch model.Fields[i].Name {
		case "Tags":
			tagsField = &model.Fields[i]
		case "Scores":
			scoresField = &model.Fields[i]
		}
	}

	// Test text[] -> []string
	if tagsField == nil {
		t.Fatal("Expected to find 'Tags' field (from 'tags' text[] column)")
	}
	if tagsField.Type != "[]string" {
		t.Errorf("Tags field Type = %s, want []string", tagsField.Type)
	}
	if tagsField.BunTag == "" {
		t.Error("Tags field should have a bun tag")
	}

	// Test integer[] -> []int32
	if scoresField == nil {
		t.Fatal("Expected to find 'Scores' field (from 'scores' integer[] column)")
	}
	if scoresField.Type != "[]int32" {
		t.Errorf("Scores field Type = %s, want []int32", scoresField.Type)
	}

	// Verify bun tags are generated
	hasIDField := false
	for _, field := range model.Fields {
		if field.Name == "ID" {
			hasIDField = true
			if field.BunTag == "" {
				t.Error("ID field should have a bun tag")
			}
			if !field.IsPrimaryKey {
				t.Error("ID field should be marked as primary key")
			}
		}
	}
	if !hasIDField {
		t.Error("Expected to find ID field")
	}
}

func TestBuildModelWithTimestamps(t *testing.T) {
	originalWd, _ := os.Getwd()
	migrationsDir := filepath.Join(originalWd, "testdata", "migrations", "comments_in_migration")

	generator := NewGenerator("postgresql")

	cat, err := generator.BuildCatalogFromMigrations("products", []string{migrationsDir})
	if err != nil {
		t.Fatalf("Failed to build catalog from migrations: %v", err)
	}

	model, err := generator.Build(cat, Config{
		TableName:    "products",
		ResourceName: "Product",
		PackageName:  "models",
		DatabaseType: "postgresql",
		ModulePath:   "github.com/example/test",
	})
	if err != nil {
		t.Fatalf("Failed to build model: %v", err)
	}

	// Verify all expected fields are present with correct types
	expectedFields := map[string]string{
		"ID":          "uuid.UUID",
		"CreatedAt":   "time.Time",
		"UpdatedAt":   "time.Time",
		"Name":        "string",
		"Description": "*string",
		"Price":       "float64",
		"Sku":         "string",
		"IsActive":    "*bool",
	}

	if len(model.Fields) != len(expectedFields) {
		t.Errorf("Expected %d fields, got %d", len(expectedFields), len(model.Fields))
	}

	for _, field := range model.Fields {
		expectedType, ok := expectedFields[field.Name]
		if !ok {
			t.Errorf("Unexpected field: %s", field.Name)
			continue
		}
		if field.Type != expectedType {
			t.Errorf("Field %s: expected type %s, got %s", field.Name, expectedType, field.Type)
		}
	}

	// Generate the model file and verify it uses bun patterns
	templateContent, err := templates.Files.ReadFile("model.tmpl")
	if err != nil {
		t.Fatalf("Failed to read model template: %v", err)
	}

	generatedCode, err := generator.GenerateModelFile(model, string(templateContent))
	if err != nil {
		t.Fatalf("Failed to generate model file: %v", err)
	}

	// Verify bun patterns in generated code
	if !containsAny(generatedCode, "bun.BaseModel", `bun:"`) {
		t.Error("Generated code should contain bun tags and BaseModel")
	}
	if !containsAny(generatedCode, "func (", "Find(", "Create(", "Update(", "Destroy(") {
		t.Error("Generated code should contain namespace methods")
	}
}

func TestBuildModelUsesResourceName(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			migrationsDir := filepath.Join(tempDir, "database", "migrations")

			err := os.MkdirAll(migrationsDir, 0o755)
			if err != nil {
				t.Fatalf("Failed to create migrations directory: %v", err)
			}

			migrationFile := filepath.Join(migrationsDir, "001_create_"+tt.tableName+".sql")
			err = os.WriteFile(migrationFile, []byte(tt.migration), 0o644)
			if err != nil {
				t.Fatalf("Failed to write migration file: %v", err)
			}

			generator := NewGenerator("postgresql")

			cat, err := generator.BuildCatalogFromMigrations(tt.tableName, []string{migrationsDir})
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

			if model.EntityName != tt.resourceName+"Entity" {
				t.Errorf("EntityName = %s, want %s", model.EntityName, tt.resourceName+"Entity")
			}
			if model.NamespaceVar != tt.resourceName {
				t.Errorf("NamespaceVar = %s, want %s", model.NamespaceVar, tt.resourceName)
			}
		})
	}
}

func TestBuildModelWithNullableFields(t *testing.T) {
	tempDir := t.TempDir()
	migrationsDir := filepath.Join(tempDir, "database", "migrations")

	err := os.MkdirAll(migrationsDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create migrations directory: %v", err)
	}

	migration := `-- +goose Up
CREATE TABLE users (
    id uuid PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(200),
    bio TEXT,
    age INTEGER
);

-- +goose Down
DROP TABLE users;`

	migrationFile := filepath.Join(migrationsDir, "001_create_users.sql")
	err = os.WriteFile(migrationFile, []byte(migration), 0o644)
	if err != nil {
		t.Fatalf("Failed to write migration file: %v", err)
	}

	generator := NewGenerator("postgresql")

	cat, err := generator.BuildCatalogFromMigrations("users", []string{migrationsDir})
	if err != nil {
		t.Fatalf("Failed to build catalog from migrations: %v", err)
	}

	model, err := generator.Build(cat, Config{
		TableName:    "users",
		ResourceName: "User",
		PackageName:  "models",
		DatabaseType: "postgresql",
		ModulePath:   "github.com/example/test",
	})
	if err != nil {
		t.Fatalf("Failed to build model: %v", err)
	}

	// Check nullable fields have pointer types
	for _, field := range model.Fields {
		switch field.Name {
		case "Email", "Bio", "Age":
			if !field.IsNullable {
				t.Errorf("Field %s should be nullable", field.Name)
			}
			if len(field.Type) == 0 || field.Type[0] != '*' {
				t.Errorf("Field %s should have pointer type, got %s", field.Name, field.Type)
			}
		case "Name":
			if field.IsNullable {
				t.Errorf("Field %s should not be nullable", field.Name)
			}
		}
	}
}

// containsAny checks if the string contains any of the substrings
func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) > 0 && len(sub) > 0 {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
