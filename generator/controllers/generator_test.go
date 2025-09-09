package controllers

import (
	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/generator/internal/ddl"
	"strings"
	"testing"
)

func TestGenerateResourceController(t *testing.T) {
	cat := catalog.NewCatalog("public")

	createTableSQL := `
		CREATE TABLE users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email VARCHAR(255) NOT NULL UNIQUE,
			name VARCHAR(255) NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`

	if err := ddl.ApplyDDL(cat, createTableSQL, "test_migration.sql"); err != nil {
		t.Fatalf("Failed to apply DDL: %v", err)
	}

	generator := NewGenerator("postgresql")

	config := Config{
		ResourceName:   "User",
		PluralName:     "users",
		PackageName:    "controllers",
		ModulePath:     "example.com/myapp",
		ControllerType: ResourceController,
	}

	controller, err := generator.Build(cat, config)
	if err != nil {
		t.Fatalf("Failed to build controller: %v", err)
	}

	// Test controller properties
	if controller.ResourceName != "User" {
		t.Errorf("Expected ResourceName 'User', got '%s'", controller.ResourceName)
	}

	if controller.PluralName != "users" {
		t.Errorf("Expected PluralName 'users', got '%s'", controller.PluralName)
	}

	if controller.Type != ResourceController {
		t.Errorf("Expected ControllerType ResourceController, got %v", controller.Type)
	}

	// Test that fields were generated
	if len(controller.Fields) == 0 {
		t.Error("Expected fields to be generated for resource controller")
	}

	// Test specific fields
	expectedFields := map[string]bool{
		"ID":        true,
		"Email":     true,
		"Name":      true,
		"CreatedAt": true,
		"UpdatedAt": true,
	}

	for _, field := range controller.Fields {
		if !expectedFields[field.Name] {
			t.Errorf("Unexpected field '%s'", field.Name)
		}
		delete(expectedFields, field.Name)
	}

	for field := range expectedFields {
		t.Errorf("Missing expected field '%s'", field)
	}
}

func TestGenerateNormalController(t *testing.T) {
	cat := catalog.NewCatalog("public")
	generator := NewGenerator("postgresql")

	config := Config{
		ResourceName:   "Dashboard",
		PluralName:     "dashboards",
		PackageName:    "controllers",
		ModulePath:     "example.com/myapp",
		ControllerType: NormalController,
	}

	controller, err := generator.Build(cat, config)
	if err != nil {
		t.Fatalf("Failed to build controller: %v", err)
	}

	// Test controller properties
	if controller.ResourceName != "Dashboard" {
		t.Errorf("Expected ResourceName 'Dashboard', got '%s'", controller.ResourceName)
	}

	if controller.Type != NormalController {
		t.Errorf("Expected ControllerType NormalController, got %v", controller.Type)
	}

	// Normal controllers should not have fields
	if len(controller.Fields) != 0 {
		t.Errorf("Expected no fields for normal controller, got %d", len(controller.Fields))
	}
}

func TestRenderResourceControllerFile(t *testing.T) {
	cat := catalog.NewCatalog("public")

	createTableSQL := `
		CREATE TABLE products (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255) NOT NULL,
			price DECIMAL(10,2) NOT NULL,
			description TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`

	if err := ddl.ApplyDDL(cat, createTableSQL, "test_migration.sql"); err != nil {
		t.Fatalf("Failed to apply DDL: %v", err)
	}

	generator := NewGenerator("postgresql")

	config := Config{
		ResourceName:   "Product",
		PluralName:     "products",
		PackageName:    "controllers",
		ModulePath:     "example.com/myapp",
		ControllerType: ResourceController,
	}

	controller, err := generator.Build(cat, config)
	if err != nil {
		t.Fatalf("Failed to build controller: %v", err)
	}

	content, err := generator.RenderControllerFile(controller)
	if err != nil {
		t.Fatalf("Failed to render controller file: %v", err)
	}

	// Test that content contains expected elements
	expectedStrings := []string{
		"package controllers",
		"type Products struct",
		"func (r Products) Index(c echo.Context) error",
		"func (r Products) Show(c echo.Context) error",
		"func (r Products) New(c echo.Context) error",
		"func (r Products) Create(c echo.Context) error",
		"func (r Products) Edit(c echo.Context) error",
		"func (r Products) Update(c echo.Context) error",
		"func (r Products) Destroy(c echo.Context) error",
		"CreateProductFormPayload",
		"UpdateProductFormPayload",
		"example.com/myapp",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(content, expected) {
			t.Errorf("Expected content to contain '%s'", expected)
		}
	}
}

func TestRenderNormalControllerFile(t *testing.T) {
	cat := catalog.NewCatalog("public")
	generator := NewGenerator("postgresql")

	config := Config{
		ResourceName:   "Dashboard",
		PluralName:     "dashboards",
		PackageName:    "controllers",
		ModulePath:     "example.com/myapp",
		ControllerType: NormalController,
	}

	controller, err := generator.Build(cat, config)
	if err != nil {
		t.Fatalf("Failed to build controller: %v", err)
	}

	content, err := generator.RenderControllerFile(controller)
	if err != nil {
		t.Fatalf("Failed to render controller file: %v", err)
	}

	// Test that content contains expected elements for normal controller
	expectedStrings := []string{
		"package controllers",
		"type Dashboards struct",
		"example.com/myapp",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(content, expected) {
			t.Errorf("Expected content to contain '%s'", expected)
		}
	}

	// Normal controller should not contain CRUD methods
	unexpectedStrings := []string{
		"CreateDashboardFormPayload",
		"UpdateDashboardFormPayload",
	}

	for _, unexpected := range unexpectedStrings {
		if strings.Contains(content, unexpected) {
			t.Errorf("Expected content NOT to contain '%s'", unexpected)
		}
	}
}

func TestFieldTypes(t *testing.T) {
	cat := catalog.NewCatalog("public")

	createTableSQL := `
		CREATE TABLE test_types (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			title VARCHAR(255) NOT NULL,
			count INTEGER NOT NULL,
			price DECIMAL(10,2) NOT NULL,
			is_active BOOLEAN NOT NULL,
			birth_date DATE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`

	if err := ddl.ApplyDDL(cat, createTableSQL, "test_migration.sql"); err != nil {
		t.Fatalf("Failed to apply DDL: %v", err)
	}

	generator := NewGenerator("postgresql")

	config := Config{
		ResourceName:   "TestType",
		PluralName:     "test_types",
		PackageName:    "controllers",
		ModulePath:     "example.com/myapp",
		ControllerType: ResourceController,
	}

	controller, err := generator.Build(cat, config)
	if err != nil {
		t.Fatalf("Failed to build controller: %v", err)
	}

	fieldTypes := make(map[string]string)
	formTypes := make(map[string]string)

	for _, field := range controller.Fields {
		fieldTypes[field.Name] = field.GoType
		formTypes[field.Name] = field.GoFormType
	}

	// Test field type mapping
	expectedTypes := map[string]string{
		"Title":     "string",
		"Count":     "int32",
		"Price":     "float64", // Decimal maps to float64
		"IsActive":  "bool",
		"BirthDate": "time.Time",
	}

	for field, expectedType := range expectedTypes {
		if actualType, exists := fieldTypes[field]; !exists {
			t.Errorf("Field '%s' not found", field)
		} else if actualType != expectedType {
			t.Errorf("Field '%s': expected type '%s', got '%s'", field, expectedType, actualType)
		}
	}

	// Test form type mapping
	expectedFormTypes := map[string]string{
		"Title":     "string",
		"Count":     "int32",
		"Price":     "float64",
		"IsActive":  "bool",
		"BirthDate": "time.Time",
	}

	for field, expectedFormType := range expectedFormTypes {
		if actualFormType, exists := formTypes[field]; !exists {
			t.Errorf("Field '%s' not found", field)
		} else if actualFormType != expectedFormType {
			t.Errorf("Field '%s': expected form type '%s', got '%s'", field, expectedFormType, actualFormType)
		}
	}
}
