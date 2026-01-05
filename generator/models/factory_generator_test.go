package models

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
)

func TestGenerator_BuildFactory(t *testing.T) {
	// Create catalog with a test table
	cat := catalog.NewCatalog("public")
	table := &catalog.Table{
		Name: "products",
		Columns: []*catalog.Column{
			{Name: "id", DataType: "uuid", IsPrimaryKey: true, IsNullable: false},
			{Name: "created_at", DataType: "timestamptz", IsNullable: false},
			{Name: "updated_at", DataType: "timestamptz", IsNullable: false},
			{Name: "name", DataType: "varchar", IsNullable: false},
			{Name: "description", DataType: "text", IsNullable: true},
			{Name: "price", DataType: "integer", IsNullable: false},
			{Name: "in_stock", DataType: "boolean", IsNullable: false},
			{Name: "category_id", DataType: "uuid", IsNullable: true, ForeignKey: &catalog.ForeignKey{ReferencedTable: "categories", ReferencedColumn: "id"}},
		},
	}
	if err := cat.AddTable("", table); err != nil {
		t.Fatalf("Failed to add table: %v", err)
	}

	gen := NewGenerator("postgresql")
	config := Config{
		TableName:    "products",
		ResourceName: "Product",
		PackageName:  "models",
		DatabaseType: "postgresql",
		ModulePath:   "github.com/test/myapp",
	}

	// Build the model first
	genModel, err := gen.Build(cat, config)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Build factory metadata
	genFactory, err := gen.BuildFactory(cat, config, genModel)
	if err != nil {
		t.Fatalf("BuildFactory failed: %v", err)
	}

	// Verify factory structure
	if genFactory.ModelName != "Product" {
		t.Errorf("ModelName = %s, want Product", genFactory.ModelName)
	}

	if genFactory.Package != "factories" {
		t.Errorf("Package = %s, want factories", genFactory.Package)
	}

	if genFactory.ModulePath != "github.com/test/myapp" {
		t.Errorf("ModulePath = %s, want github.com/test/myapp", genFactory.ModulePath)
	}

	// Verify fields
	if len(genFactory.Fields) != len(table.Columns) {
		t.Errorf("Fields count = %d, want %d", len(genFactory.Fields), len(table.Columns))
	}

	// Check for specific fields
	var nameField, priceField, categoryIDField *FactoryField
	for i := range genFactory.Fields {
		field := &genFactory.Fields[i]
		switch field.Name {
		case "Name":
			nameField = field
		case "Price":
			priceField = field
		case "CategoryID":
			categoryIDField = field
		}
	}

	if nameField == nil {
		t.Error("Name field not found")
	} else {
		if !strings.Contains(nameField.DefaultValue, "faker") {
			t.Errorf("Name field default value should use faker, got: %s", nameField.DefaultValue)
		}
		if nameField.IsAutoManaged {
			t.Error("Name field should not be auto-managed")
		}
	}

	if priceField == nil {
		t.Error("Price field not found")
	} else {
		if !strings.Contains(priceField.DefaultValue, "randomInt") {
			t.Errorf("Price field should use randomInt, got: %s", priceField.DefaultValue)
		}
	}

	if categoryIDField == nil {
		t.Error("CategoryID field not found")
	} else {
		if !categoryIDField.IsFK {
			t.Error("CategoryID should be identified as foreign key")
		}
	}
}

func TestGenerator_GenerateFactoryFile(t *testing.T) {
	gen := NewGenerator("postgresql")

	factory := &GeneratedFactory{
		ModelName:  "Product",
		Package:    "factories",
		ModulePath: "github.com/test/myapp",
		Fields: []FactoryField{
			{
				Name:          "ID",
				Type:          "uuid.UUID",
				IsID:          true,
				IsAutoManaged: true,
				DefaultValue:  "uuid.New()",
			},
			{
				Name:          "Name",
				Type:          "string",
				DefaultValue:  "faker.Word()",
				OptionName:    "WithProductName",
				IsAutoManaged: false,
			},
			{
				Name:          "Price",
				Type:          "int32",
				DefaultValue:  "faker.RandomInt(100, 10000)",
				OptionName:    "WithProductPrice",
				IsAutoManaged: false,
			},
		},
		StandardImports:   []string{"context", "fmt", "time"},
		ExternalImports:   []string{"github.com/go-faker/faker/v4", "github.com/google/uuid"},
		HasCreateFunction: true,
	}

	// Create template
	templateContent := `package {{.Package}}

import (
	"context"
{{- range .StandardImports}}
	"{{.}}"
{{- end}}

	"{{.ModulePath}}/models"
{{- range .ExternalImports}}
	"{{.}}"
{{- end}}
)

type {{.ModelName}}Factory struct {
	models.{{.ModelName}}
}

func Build{{.ModelName}}(opts ...{{.ModelName}}Option) models.{{.ModelName}} {
{{- range .Fields}}
{{- if not .IsAutoManaged}}
	// Field: {{.Name}}
{{- end}}
{{- end}}
	return models.{{.ModelName}}{}
}
`

	content, err := gen.GenerateFactoryFile(factory, templateContent)
	if err != nil {
		t.Fatalf("GenerateFactoryFile failed: %v", err)
	}

	if !strings.Contains(content, "package factories") {
		t.Error("Generated content should contain package declaration")
	}

	if !strings.Contains(content, "type ProductFactory struct") {
		t.Error("Generated content should contain factory struct")
	}

	if !strings.Contains(content, "func BuildProduct") {
		t.Error("Generated content should contain Build function")
	}

	if !strings.Contains(content, "Field: Name") {
		t.Error("Generated content should contain Name field comment")
	}

	if !strings.Contains(content, "Field: Price") {
		t.Error("Generated content should contain Price field comment")
	}

	// ID should not appear in the field comments since it's auto-managed
	if strings.Contains(content, "Field: ID") {
		t.Error("Generated content should NOT contain ID field comment (auto-managed)")
	}
}

func TestGenerator_WriteFactoryFile(t *testing.T) {
	tmpDir := t.TempDir()

	gen := NewGenerator("postgresql")
	factory := &GeneratedFactory{
		ModelName:         "Product",
		Package:           "factories",
		ModulePath:        "github.com/test/myapp",
		Fields:            []FactoryField{},
		StandardImports:   []string{"context", "fmt"},
		ExternalImports:   []string{"github.com/go-faker/faker/v4"},
		HasCreateFunction: true,
	}

	err := gen.WriteFactoryFile(factory, tmpDir)
	if err != nil {
		t.Fatalf("WriteFactoryFile failed: %v", err)
	}

	// Verify file was created
	factoryPath := filepath.Join(tmpDir, "models", "factories", "product.go")
	if _, err := os.Stat(factoryPath); os.IsNotExist(err) {
		t.Errorf("Factory file was not created at %s", factoryPath)
	}

	// Read and verify content
	content, err := os.ReadFile(factoryPath)
	if err != nil {
		t.Fatalf("Failed to read factory file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "package factories") {
		t.Error("Factory file should contain package declaration")
	}

	if !strings.Contains(contentStr, "type ProductFactory") {
		t.Error("Factory file should contain ProductFactory struct")
	}
}
