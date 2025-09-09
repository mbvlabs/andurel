package models

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"mbvlabs/andurel/generator/internal/catalog"
	"mbvlabs/andurel/generator/internal/types"
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
			SQLCType: "pgtype.Int4",
		},
		"IsActive": {
			Type:     "bool",
			SQLCType: "pgtype.Bool",
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

		// Test specific conversion patterns based on nullability
		switch field.Name {
		case "Age":
			// Nullable field - uses pgtype
			expectedFromDB := "row.Age.Int32"
			if field.ConversionFromDB != expectedFromDB {
				t.Errorf("Field %s: expected ConversionFromDB %s, got %s",
					field.Name, expectedFromDB, field.ConversionFromDB)
			}
			expectedToDB := "pgtype.Int4{Int32: data.Age, Valid: true}"
			if field.ConversionToDB != expectedToDB {
				t.Errorf("Field %s: expected ConversionToDB %s, got %s",
					field.Name, expectedToDB, field.ConversionToDB)
			}
		case "IsActive":
			// Nullable field - uses pgtype
			expectedFromDB := "row.IsActive.Bool"
			if field.ConversionFromDB != expectedFromDB {
				t.Errorf("Field %s: expected ConversionFromDB %s, got %s",
					field.Name, expectedFromDB, field.ConversionFromDB)
			}
			expectedToDB := "pgtype.Bool{Bool: data.IsActive, Valid: true}"
			if field.ConversionToDB != expectedToDB {
				t.Errorf("Field %s: expected ConversionToDB %s, got %s",
					field.Name, expectedToDB, field.ConversionToDB)
			}
		case "CreatedAt":
			// Non-nullable timestamp - uses pgtype for SQLC but time.Time for Go
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
			// Non-nullable timestamp - uses pgtype for SQLC but time.Time for Go
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
			// Non-nullable string fields - direct conversions
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
		case "ID":
			// Non-nullable int - direct conversions
			expectedFromDB := "row.ID"
			if field.ConversionFromDB != expectedFromDB {
				t.Errorf("Field %s: expected ConversionFromDB %s, got %s",
					field.Name, expectedFromDB, field.ConversionFromDB)
			}
			expectedToDB := "data.ID"
			if field.ConversionToDB != expectedToDB {
				t.Errorf("Field %s: expected ConversionToDB %s, got %s",
					field.Name, expectedToDB, field.ConversionToDB)
			}
		}
	}

	expectedImports := []string{
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
	err := os.MkdirAll(modelsDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create models directory: %v", err)
	}
	err = os.MkdirAll(queriesDir, 0o755)
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

func TestGenerator_ComplexTable(t *testing.T) {
	currentDir, _ := os.Getwd()
	migrationsDir := filepath.Join(currentDir, "testdata", "migrations", "complex_table")

	generator := NewGenerator("postgresql")

	cat, err := generator.buildCatalogFromTableMigrations("comprehensive_example", []string{migrationsDir})
	if err != nil {
		t.Fatalf("Failed to build catalog from migrations: %v", err)
	}

	config := Config{
		TableName:    "comprehensive_example",
		ResourceName: "ComprehensiveExample",
		PackageName:  "models",
		DatabaseType: "postgresql",
		ModulePath:   "test/module",
	}

	model, err := generator.Build(cat, config)
	if err != nil {
		t.Fatalf("Failed to build model: %v", err)
	}

	if model.Name != "ComprehensiveExample" {
		t.Errorf("Expected model name 'ComprehensiveExample', got '%s'", model.Name)
	}

	if model.TableName != "comprehensive_example" {
		t.Errorf("Expected table name 'comprehensive_example', got '%s'", model.TableName)
	}

	// Test that we have fields for all the types from the complex migration that are successfully parsed
	expectedFieldTypes := map[string]string{
		"ID":                "int64",
		"UuidId":            "uuid.UUID",
		"SmallInt":          "int16",
		"RegularInt":        "int32",
		"BigInt":            "int64",
		"DecimalPrecise":    "float64",
		"NumericField":      "float64",
		"RealFloat":         "float32",
		"DoubleFloat":       "float64",
		"SmallSerial":       "int16",
		"BigSerial":         "int64",
		"FixedChar":         "string",
		"VariableChar":      "string",
		"UnlimitedText":     "string",
		"TextWithDefault":   "string",
		"TextNotNull":       "string",
		"IsActive":          "bool",
		"IsVerified":        "bool",
		"NullableFlag":      "bool",
		"CreatedDate":       "time.Time",
		"BirthDate":         "time.Time",
		"ExactTime":         "time.Time",
		"TimeWithZone":      "time.Time",
		"CreatedTimestamp":  "time.Time",
		"UpdatedTimestamp":  "time.Time",
		"TimestampWithZone": "time.Time",
		"DurationInterval":  "string",
		"WorkHours":         "string",
		"FileData":          "[]byte",
		"RequiredBinary":    "[]byte",
		"IpAddress":         "string",
		"IpNetwork":         "string",
		"MacAddress":        "string",
		"Mac8Address":       "string",
		"PointLocation":     "string",
		"LineSegment":       "string",
		"RectangularBox":    "string",
		"PathData":          "string",
		"PolygonShape":      "string",
		"CircleArea":        "string",
		"JsonData":          "[]byte",
		"JsonbData":         "[]byte",
		"JsonbNotNull":      "[]byte",
		"IntegerArray":      "[]int32",
		"TextArray":         "[]string",
		"MultidimArray":     "[]int32",
		"IntRange":          "string",
		"BigintRange":       "string",
		"NumericRange":      "string",
	}

	// Verify we have the expected number of fields that can be successfully parsed
	if len(model.Fields) != 49 {
		t.Errorf("Expected 49 fields for complex table, got %d", len(model.Fields))
	}

	// Test some specific field types
	fieldsByName := make(map[string]GeneratedField)
	for _, field := range model.Fields {
		fieldsByName[field.Name] = field
	}

	for fieldName, expectedType := range expectedFieldTypes {
		field, exists := fieldsByName[fieldName]
		if !exists {
			t.Errorf("Expected field %s not found", fieldName)
			continue
		}

		if field.Type != expectedType {
			t.Errorf("Field %s: expected type %s, got %s", fieldName, expectedType, field.Type)
		}
	}

	// Verify imports include necessary packages
	expectedImports := []string{
		"github.com/jackc/pgx/v5/pgtype",
		"time",
		"github.com/google/uuid",
	}

	for _, expectedImport := range expectedImports {
		found := slices.Contains(model.Imports, expectedImport)
		if !found {
			t.Errorf("Missing expected import: %s", expectedImport)
		}
	}
}

type FieldExpectation struct {
	Type     string
	SQLCType string
}
