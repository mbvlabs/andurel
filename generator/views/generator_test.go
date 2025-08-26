package views

import (
	"mbvlabs/andurel/generator/internal/catalog"
	"mbvlabs/andurel/generator/internal/ddl"
	"strings"
	"testing"
)

func TestGenerateView(t *testing.T) {
	cat := catalog.NewCatalog("public")

	createTableSQL := `
		CREATE TABLE users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email VARCHAR(255) NOT NULL UNIQUE,
			name VARCHAR(255) NOT NULL,
			age INTEGER NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`

	if err := ddl.ApplyDDL(cat, createTableSQL, "test_migration.sql"); err != nil {
		t.Fatalf("Failed to apply DDL: %v", err)
	}

	generator := NewGenerator("postgresql")

	config := Config{
		ResourceName: "User",
		PluralName:   "users",
		ModulePath:   "example.com/myapp",
	}

	view, err := generator.Build(cat, config)
	if err != nil {
		t.Fatalf("Failed to build view: %v", err)
	}

	// Test view properties
	if view.ResourceName != "User" {
		t.Errorf("Expected ResourceName 'User', got '%s'", view.ResourceName)
	}

	if view.PluralName != "users" {
		t.Errorf("Expected PluralName 'users', got '%s'", view.PluralName)
	}

	// Test that fields were generated (excluding ID)
	if len(view.Fields) == 0 {
		t.Error("Expected fields to be generated for view")
	}

	// Test specific fields (ID should be excluded)
	expectedFields := map[string]bool{
		"Email":     true,
		"Name":      true,
		"Age":       true,
		"IsActive":  true,
		"CreatedAt": true,
		"UpdatedAt": true,
	}

	actualFields := make(map[string]bool)
	for _, field := range view.Fields {
		actualFields[field.Name] = true
		if field.Name == "ID" {
			t.Error("ID field should be excluded from views")
		}
	}

	for field := range expectedFields {
		if !actualFields[field] {
			t.Errorf("Missing expected field '%s'", field)
		}
	}

	for field := range actualFields {
		if !expectedFields[field] {
			t.Errorf("Unexpected field '%s'", field)
		}
	}
}

func TestRenderViewFile(t *testing.T) {
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
		ResourceName: "Product",
		PluralName:   "products",
		ModulePath:   "example.com/myapp",
	}

	view, err := generator.Build(cat, config)
	if err != nil {
		t.Fatalf("Failed to build view: %v", err)
	}

	content, err := generator.RenderViewFile(view)
	if err != nil {
		t.Fatalf("Failed to render view file: %v", err)
	}

	// Test that content contains expected elements
	expectedStrings := []string{
		"package views",
		"templ ProductIndex(products []models.Product)",
		"templ ProductShow(product models.Product)",
		"templ ProductNew()",
		"templ ProductEdit(product models.Product)",
		"example.com/myapp/models",
		"Product Details",
		"New Product",
		"Edit Product",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(content, expected) {
			t.Errorf("Expected content to contain '%s'", expected)
		}
	}
}

func TestViewFieldTypes(t *testing.T) {
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
		ResourceName: "TestType",
		PluralName:   "test_types",
		ModulePath:   "example.com/myapp",
	}

	view, err := generator.Build(cat, config)
	if err != nil {
		t.Fatalf("Failed to build view: %v", err)
	}

	fieldTypes := make(map[string]string)
	inputTypes := make(map[string]string)
	stringConverters := make(map[string]string)

	for _, field := range view.Fields {
		fieldTypes[field.Name] = field.GoType
		inputTypes[field.Name] = field.InputType
		stringConverters[field.Name] = field.StringConverter
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

	// Test input type mapping
	expectedInputTypes := map[string]string{
		"Title":     "text",
		"Count":     "number",
		"Price":     "number",
		"IsActive":  "checkbox",
		"BirthDate": "date",
	}

	for field, expectedInputType := range expectedInputTypes {
		if actualInputType, exists := inputTypes[field]; !exists {
			t.Errorf("Field '%s' not found", field)
		} else if actualInputType != expectedInputType {
			t.Errorf("Field '%s': expected input type '%s', got '%s'", field, expectedInputType, actualInputType)
		}
	}

	// Test string converter mapping
	expectedConverters := map[string]string{
		"Title":     "",
		"Count":     "fmt.Sprintf(\"%d\", %s)",
		"Price":     "fmt.Sprintf(\"%f\", %s)",
		"IsActive":  "fmt.Sprintf(\"%t\", %s)",
		"BirthDate": "%s.String()",
	}

	for field, expectedConverter := range expectedConverters {
		if actualConverter, exists := stringConverters[field]; !exists {
			t.Errorf("Field '%s' not found", field)
		} else if actualConverter != expectedConverter {
			t.Errorf("Field '%s': expected string converter '%s', got '%s'", field, expectedConverter, actualConverter)
		}
	}
}

func TestDisplayNameFormatting(t *testing.T) {
	cat := catalog.NewCatalog("public")

	createTableSQL := `
		CREATE TABLE test_names (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			first_name VARCHAR(255) NOT NULL,
			last_name VARCHAR(255) NOT NULL,
			phone_number VARCHAR(20),
			email_address VARCHAR(255),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`

	if err := ddl.ApplyDDL(cat, createTableSQL, "test_migration.sql"); err != nil {
		t.Fatalf("Failed to apply DDL: %v", err)
	}

	generator := NewGenerator("postgresql")

	config := Config{
		ResourceName: "TestName",
		PluralName:   "test_names",
		ModulePath:   "example.com/myapp",
	}

	view, err := generator.Build(cat, config)
	if err != nil {
		t.Fatalf("Failed to build view: %v", err)
	}

	displayNames := make(map[string]string)
	fieldNames := make(map[string]string)
	camelCases := make(map[string]string)

	for _, field := range view.Fields {
		displayNames[field.DBName] = field.DisplayName
		fieldNames[field.DBName] = field.Name
		camelCases[field.DBName] = field.CamelCase
	}

	// Test display name formatting
	expectedDisplayNames := map[string]string{
		"first_name":    "First Name",
		"last_name":     "Last Name",
		"phone_number":  "Phone Number",
		"email_address": "Email Address",
	}

	for dbName, expectedDisplayName := range expectedDisplayNames {
		if actualDisplayName, exists := displayNames[dbName]; !exists {
			t.Errorf("Field with DB name '%s' not found", dbName)
		} else if actualDisplayName != expectedDisplayName {
			t.Errorf("DB name '%s': expected display name '%s', got '%s'", dbName, expectedDisplayName, actualDisplayName)
		}
	}

	// Test field name formatting (PascalCase)
	expectedFieldNames := map[string]string{
		"first_name":    "FirstName",
		"last_name":     "LastName",
		"phone_number":  "PhoneNumber",
		"email_address": "EmailAddress",
	}

	for dbName, expectedFieldName := range expectedFieldNames {
		if actualFieldName, exists := fieldNames[dbName]; !exists {
			t.Errorf("Field with DB name '%s' not found", dbName)
		} else if actualFieldName != expectedFieldName {
			t.Errorf("DB name '%s': expected field name '%s', got '%s'", dbName, expectedFieldName, actualFieldName)
		}
	}

	// Test camel case formatting
	expectedCamelCases := map[string]string{
		"first_name":    "firstName",
		"last_name":     "lastName",
		"phone_number":  "phoneNumber",
		"email_address": "emailAddress",
	}

	for dbName, expectedCamelCase := range expectedCamelCases {
		if actualCamelCase, exists := camelCases[dbName]; !exists {
			t.Errorf("Field with DB name '%s' not found", dbName)
		} else if actualCamelCase != expectedCamelCase {
			t.Errorf("DB name '%s': expected camel case '%s', got '%s'", dbName, expectedCamelCase, actualCamelCase)
		}
	}
}

func TestSystemFieldIdentification(t *testing.T) {
	cat := catalog.NewCatalog("public")

	createTableSQL := `
		CREATE TABLE test_system (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255) NOT NULL,
			custom_field VARCHAR(255),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`

	if err := ddl.ApplyDDL(cat, createTableSQL, "test_migration.sql"); err != nil {
		t.Fatalf("Failed to apply DDL: %v", err)
	}

	generator := NewGenerator("postgresql")

	config := Config{
		ResourceName: "TestSystem",
		PluralName:   "test_system",
		ModulePath:   "example.com/myapp",
	}

	view, err := generator.Build(cat, config)
	if err != nil {
		t.Fatalf("Failed to build view: %v", err)
	}

	systemFields := make(map[string]bool)
	for _, field := range view.Fields {
		systemFields[field.DBName] = field.IsSystemField
	}

	// Test system field identification
	expectedSystemFields := map[string]bool{
		"name":         false,
		"custom_field": false,
		"created_at":   true,
		"updated_at":   true,
	}

	for dbName, expectedIsSystem := range expectedSystemFields {
		if actualIsSystem, exists := systemFields[dbName]; !exists {
			t.Errorf("Field with DB name '%s' not found", dbName)
		} else if actualIsSystem != expectedIsSystem {
			t.Errorf("DB name '%s': expected IsSystemField '%v', got '%v'", dbName, expectedIsSystem, actualIsSystem)
		}
	}
}