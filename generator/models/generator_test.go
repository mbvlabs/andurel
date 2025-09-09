package models

import (
	"mbvlabs/andurel/generator/internal/catalog"
	"mbvlabs/andurel/generator/types"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestGenerator_Build(t *testing.T) {
	currentDir, _ := os.Getwd()
	migrationsDir := filepath.Join(currentDir, "testdata", "migrations", "simple_user_table")

	generator := NewGenerator("postgresql")

	cat, err := generator.buildCatalogFromTableMigrations("users", []string{migrationsDir})
	if err != nil {
		t.Fatalf("Failed to build catalog from migrations: %v", err)
	}

	config := Config{
		TableName:    "users",
		ResourceName: "User",
		PackageName:  "models",
		DatabaseType: "postgresql",
		ModulePath:   "test/module",
	}

	model, err := generator.Build(cat, config)
	if err != nil {
		t.Fatalf("Failed to build model: %v", err)
	}

	if model.Name != "User" {
		t.Errorf("Expected model name 'User', got '%s'", model.Name)
	}

	if model.Package != "models" {
		t.Errorf("Expected package 'models', got '%s'", model.Package)
	}

	if model.TableName != "users" {
		t.Errorf("Expected table name 'users', got '%s'", model.TableName)
	}

	expectedFields := map[string]FieldExpectation{
		"ID": {
			Type:     "int32",
			SQLCType: "int32",
		},
		"Email": {
			Type:     "string",
			SQLCType: "string",
		},
		"Name": {
			Type:     "string",
			SQLCType: "string",
		},
		"Age": {
			Type:     "int32",
			SQLCType: "sql.NullInt32",
		},
		"IsActive": {
			Type:     "bool",
			SQLCType: "sql.NullBool",
		},
		"CreatedAt": {
			Type:     "time.Time",
			SQLCType: "pgtype.Timestamptz",
		},
		"UpdatedAt": {
			Type:     "time.Time",
			SQLCType: "pgtype.Timestamptz",
		},
	}

	if len(model.Fields) != len(expectedFields) {
		t.Errorf("Expected %d fields, got %d", len(expectedFields), len(model.Fields))
	}

	for _, field := range model.Fields {
		expected, exists := expectedFields[field.Name]
		if !exists {
			t.Errorf("Unexpected field: %s", field.Name)
			continue
		}

		if field.Type != expected.Type {
			t.Errorf("Field %s: expected type %s, got %s", field.Name, expected.Type, field.Type)
		}

		if field.SQLCType != expected.SQLCType {
			t.Errorf(
				"Field %s: expected SQLCType %s, got %s",
				field.Name,
				expected.SQLCType,
				field.SQLCType,
			)
		}

		if field.ConversionFromDB == "" {
			t.Errorf("Field %s: missing ConversionFromDB", field.Name)
		}

		// created_at and updated_at don't need ConversionToDB since they use now() in SQL
		if field.ConversionToDB == "" && field.Name != "CreatedAt" && field.Name != "UpdatedAt" {
			t.Errorf("Field %s: missing ConversionToDB", field.Name)
		}

		if field.ZeroCheck == "" {
			t.Errorf("Field %s: missing ZeroCheck", field.Name)
		}

		// Test specific conversion patterns
		switch field.Name {
		case "Age":
			expectedFromDB := "row.Age.Int32"
			if field.ConversionFromDB != expectedFromDB {
				t.Errorf("Field %s: expected ConversionFromDB %s, got %s", 
					field.Name, expectedFromDB, field.ConversionFromDB)
			}
			expectedToDB := "sql.NullInt32{Int32: data.Age, Valid: true}"
			if field.ConversionToDB != expectedToDB {
				t.Errorf("Field %s: expected ConversionToDB %s, got %s", 
					field.Name, expectedToDB, field.ConversionToDB)
			}
		case "IsActive":
			expectedFromDB := "row.IsActive.Bool"
			if field.ConversionFromDB != expectedFromDB {
				t.Errorf("Field %s: expected ConversionFromDB %s, got %s", 
					field.Name, expectedFromDB, field.ConversionFromDB)
			}
			expectedToDB := "sql.NullBool{Bool: data.IsActive, Valid: true}"
			if field.ConversionToDB != expectedToDB {
				t.Errorf("Field %s: expected ConversionToDB %s, got %s", 
					field.Name, expectedToDB, field.ConversionToDB)
			}
		case "CreatedAt":
			expectedFromDB := "row.CreatedAt.Time"
			if field.ConversionFromDB != expectedFromDB {
				t.Errorf("Field %s: expected ConversionFromDB %s, got %s", 
					field.Name, expectedFromDB, field.ConversionFromDB)
			}
			// ConversionToDB should be empty since we use now() in SQL
			if field.ConversionToDB != "" {
				t.Errorf("Field %s: expected empty ConversionToDB, got %s", 
					field.Name, field.ConversionToDB)
			}
		case "UpdatedAt":
			expectedFromDB := "row.UpdatedAt.Time"
			if field.ConversionFromDB != expectedFromDB {
				t.Errorf("Field %s: expected ConversionFromDB %s, got %s", 
					field.Name, expectedFromDB, field.ConversionFromDB)
			}
			// ConversionToDB should be empty since we use now() in SQL
			if field.ConversionToDB != "" {
				t.Errorf("Field %s: expected empty ConversionToDB, got %s", 
					field.Name, field.ConversionToDB)
			}
			// ConversionToDBForUpdate should be empty since we use now() in SQL
			if field.ConversionToDBForUpdate != "" {
				t.Errorf("Field %s: expected empty ConversionToDBForUpdate, got %s", 
					field.Name, field.ConversionToDBForUpdate)
			}
		case "Email", "Name":
			// String fields should have direct conversions (non-nullable)
			expectedFromDB := "row." + field.Name
			if field.ConversionFromDB != expectedFromDB {
				t.Errorf("Field %s: expected ConversionFromDB %s, got %s", 
					field.Name, expectedFromDB, field.ConversionFromDB)
			}
			expectedToDB := "data." + field.Name
			if field.ConversionToDB != expectedToDB {
				t.Errorf("Field %s: expected ConversionToDB %s, got %s", 
					field.Name, expectedToDB, field.ConversionToDB)
			}
		}
	}

	expectedImports := []string{
		"database/sql",
		"github.com/jackc/pgx/v5/pgtype",
		"time",
	}

	for _, expectedImport := range expectedImports {
		found := slices.Contains(model.Imports, expectedImport)
		if !found {
			t.Errorf("Missing expected import: %s", expectedImport)
		}
	}
}

func TestGenerator_CustomTypes(t *testing.T) {
	currentDir, _ := os.Getwd()
	migrationsDir := filepath.Join(
		currentDir,
		"testdata",
		"migrations",
		"product_table_with_decimals",
	)

	generator := NewGenerator("postgresql")

	cat, err := generator.buildCatalogFromTableMigrations("products", []string{migrationsDir})
	if err != nil {
		t.Fatalf("Failed to build catalog from migrations: %v", err)
	}

	config := Config{
		TableName:    "products",
		ResourceName: "Product",
		PackageName:  "models",
		DatabaseType: "postgresql",
		CustomTypes: []types.TypeOverride{
			{
				DatabaseType: "decimal",
				GoType:       "shopspring.Decimal",
				Package:      "github.com/shopspring/decimal",
				Nullable:     false,
			},
		},
	}

	model, err := generator.Build(cat, config)
	if err != nil {
		t.Fatalf("Failed to build model: %v", err)
	}

	var priceField *GeneratedField
	for i, field := range model.Fields {
		if field.Name == "Price" {
			priceField = &model.Fields[i]
			break
		}
	}

	if priceField == nil {
		t.Fatal("Price field not found")
	}

	if priceField.Type != "shopspring.Decimal" {
		t.Errorf("Expected custom type shopspring.Decimal, got %s", priceField.Type)
	}

	found := slices.Contains(model.Imports, "github.com/shopspring/decimal")

	if !found {
		t.Error("Custom package not included in imports")
	}
}

func TestGenerator_MigrationBasedFlow(t *testing.T) {
	currentDir, _ := os.Getwd()
	migrationsDir := filepath.Join(currentDir, "testdata", "migrations", "posts_multi_migration")

	tempDir := t.TempDir()
	modelsDir := filepath.Join(tempDir, "models")
	queriesDir := filepath.Join(tempDir, "database", "queries")
	err := os.MkdirAll(modelsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create models directory: %v", err)
	}
	err = os.MkdirAll(queriesDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create queries directory: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tempDir)

	generator := NewGenerator("postgresql")

	modelPath := filepath.Join("models", "post.go")
	sqlPath := filepath.Join("database", "queries", "posts.sql")

	err = generator.GenerateModelFromMigrations(
		"posts",
		"Post",
		[]string{migrationsDir},
		modelPath,
		sqlPath,
		"test/module",
	)
	if err != nil {
		t.Fatalf("Failed to generate model from migrations: %v", err)
	}

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		t.Errorf("Model file was not created: %s", modelPath)
	}

	if _, err := os.Stat(sqlPath); os.IsNotExist(err) {
		t.Errorf("SQL file was not created: %s", sqlPath)
	}

	cat, err := generator.buildCatalogFromTableMigrations("posts", []string{migrationsDir})
	if err != nil {
		t.Fatalf("Failed to build catalog from migrations: %v", err)
	}

	table, err := cat.GetTable("", "posts")
	if err != nil {
		t.Fatalf("Failed to get posts table: %v", err)
	}

	expectedColumns := []string{"id", "title", "content", "created_at", "author_id", "published_at"}
	if len(table.Columns) != len(expectedColumns) {
		t.Errorf("Expected %d columns, got %d", len(expectedColumns), len(table.Columns))
	}

	for _, expectedCol := range expectedColumns {
		found := false
		for _, col := range table.Columns {
			if col.Name == expectedCol {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected column %s not found in final table", expectedCol)
		}
	}

	var titleCol *catalog.Column
	for _, col := range table.Columns {
		if col.Name == "title" {
			titleCol = col
			break
		}
	}
	if titleCol == nil {
		t.Error("Title column not found")
	}
}

type FieldExpectation struct {
	Type     string
	SQLCType string
}
