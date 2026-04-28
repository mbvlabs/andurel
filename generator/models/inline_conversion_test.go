package models

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/generator/templates"
)

func TestBunModelGeneration(t *testing.T) {
	originalWd, _ := os.Getwd()
	migrationsDir := filepath.Join(originalWd, "testdata", "migrations", "simple_user_table")

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
		ModulePath:   "github.com/example/myapp",
	})
	if err != nil {
		t.Fatalf("Failed to build model: %v", err)
	}

	// Verify model structure
	if model.EntityName != "UserEntity" {
		t.Errorf("EntityName = %s, want UserEntity", model.EntityName)
	}
	if model.NamespaceVar != "User" {
		t.Errorf("NamespaceVar = %s, want User", model.NamespaceVar)
	}

	// Generate model file
	templateContent, err := templates.Files.ReadFile("model.tmpl")
	if err != nil {
		t.Fatalf("Failed to read model template: %v", err)
	}

	modelContent, err := generator.GenerateModelFile(model, string(templateContent))
	if err != nil {
		t.Fatalf("Failed to generate model file: %v", err)
	}

	// Verify bun patterns in generated code
	if !strings.Contains(modelContent, "bun.BaseModel") {
		t.Error("Generated code should contain bun.BaseModel")
	}
	if !strings.Contains(modelContent, "bun:\"") {
		t.Error("Generated code should contain bun tags")
	}
	if !strings.Contains(modelContent, "func (") {
		t.Error("Generated code should contain methods")
	}

	// Verify field types
	for _, field := range model.Fields {
		if field.Name == "Email" || field.Name == "Name" {
			if field.Type != "string" {
				t.Errorf("Field %s should be string, got %s", field.Name, field.Type)
			}
		}
		if field.Name == "Age" {
			if field.Type != "int32" {
				t.Errorf("Field %s should be int32, got %s", field.Name, field.Type)
			}
		}
		if field.Name == "IsActive" {
			if field.Type != "bool" {
				t.Errorf("Field %s should be bool, got %s", field.Name, field.Type)
			}
		}
	}
}

func TestBunNamespaceMethods(t *testing.T) {
	originalWd, _ := os.Getwd()
	migrationsDir := filepath.Join(originalWd, "testdata", "migrations", "simple_user_table")

	generator := NewGenerator("postgresql")

	cat, err := generator.BuildCatalogFromMigrations("users", []string{migrationsDir})
	if err != nil {
		t.Fatalf("Failed to build catalog: %v", err)
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

	// Verify namespace method names
	expectedMethods := []string{"Find", "Create", "Update", "Destroy", "All", "Paginate", "Upsert"}

	// Generate and check code
	templateContent, err := templates.Files.ReadFile("model.tmpl")
	if err != nil {
		t.Fatalf("Failed to read template: %v", err)
	}

	modelContent, err := generator.GenerateModelFile(model, string(templateContent))
	if err != nil {
		t.Fatalf("Failed to generate model: %v", err)
	}

	for _, method := range expectedMethods {
		expectedMethod := "func (" + model.ReceiverName + " *" + model.NamespaceType + ") " + method
		if !strings.Contains(modelContent, expectedMethod) {
			t.Errorf("Generated code should contain method: %s", expectedMethod)
		}
	}
}

func TestBunFieldTags(t *testing.T) {
	originalWd, _ := os.Getwd()
	migrationsDir := filepath.Join(originalWd, "testdata", "migrations", "simple_user_table")

	generator := NewGenerator("postgresql")

	cat, err := generator.BuildCatalogFromMigrations("users", []string{migrationsDir})
	if err != nil {
		t.Fatalf("Failed to build catalog: %v", err)
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

	// Check bun tags
	for _, field := range model.Fields {
		if field.BunTag == "" {
			t.Errorf("Field %s should have a bun tag", field.Name)
		}
		if field.Name == "ID" && field.IsPrimaryKey {
			if !strings.Contains(field.BunTag, "pk") {
				t.Errorf("ID field should have 'pk' in bun tag")
			}
		}
	}
}
