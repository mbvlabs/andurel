package models

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
)

func TestBuildUUIDImports(t *testing.T) {
	tests := []struct {
		name     string
		table    *catalog.Table
		wantUUID bool
	}{
		{
			name: "no primary key without uuid fields",
			table: tableWithColumns(t, "audit_logs",
				catalog.NewColumn("action", "text").SetNotNull(),
				catalog.NewColumn("occurred_at", "timestamp").SetNotNull(),
			),
			wantUUID: false,
		},
		{
			name: "no primary key with uuid field",
			table: tableWithColumns(t, "audit_logs",
				catalog.NewColumn("event_id", "uuid").SetNotNull(),
				catalog.NewColumn("action", "text").SetNotNull(),
			),
			wantUUID: true,
		},
		{
			name: "uuid primary key",
			table: tableWithColumns(t, "audit_logs",
				catalog.NewColumn("id", "uuid").SetPrimaryKey(),
				catalog.NewColumn("action", "text").SetNotNull(),
			),
			wantUUID: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat := catalog.NewCatalog("public")
			if err := cat.AddTable("public", tt.table); err != nil {
				t.Fatalf("failed to add table: %v", err)
			}

			model, err := NewGenerator("postgresql").Build(cat, Config{
				TableName:         tt.table.Name,
				ResourceName:      "AuditLog",
				PackageName:       "models",
				DatabaseType:      "postgresql",
				ModulePath:        "github.com/example/shop",
				GenerateWithoutPK: !tableHasPrimaryKey(tt.table),
			})
			if err != nil {
				t.Fatalf("Build() returned error: %v", err)
			}

			if got := hasImport(model.ExternalImports, "github.com/google/uuid"); got != tt.wantUUID {
				t.Fatalf("uuid import = %v, want %v; imports: %v", got, tt.wantUUID, model.ExternalImports)
			}
		})
	}
}

func tableWithColumns(t *testing.T, name string, columns ...*catalog.Column) *catalog.Table {
	t.Helper()

	table := catalog.NewTable("public", name)
	for _, column := range columns {
		if err := table.AddColumn(column); err != nil {
			t.Fatalf("failed to add column %s: %v", column.Name, err)
		}
	}
	return table
}

func tableHasPrimaryKey(table *catalog.Table) bool {
	for _, column := range table.Columns {
		if column.IsPrimaryKey {
			return true
		}
	}
	return false
}

func hasImport(imports []string, target string) bool {
	return slices.Contains(imports, target)
}

func TestGeneratorFactoryDefaultsAndZeroValues(t *testing.T) {
	g := NewGenerator("postgresql")

	defaults := map[string]string{
		"Email:string":             "faker.Word()",
		"Name:string":              "faker.Word()",
		"Age:int32":                "randomInt(1, 1000, 100)",
		"Enabled:bool":             "randomBool()",
		"CreatedAt:time.Time":      "time.Time{}",
		"ID:uuid.UUID":             "uuid.UUID{}",
		"Metadata:json.RawMessage": "json.RawMessage{}",
		"Payload:[]byte":           "[]byte{}",
		"Maybe:sql.NullString":     "sql.NullString{String: faker.Word(), Valid: true}",
		"Maybe:bun.NullInt64":      "bun.NullInt64{Int64: randomInt64(1, 1000, 100), Valid: true}",
		"Custom:Money":             "Money{}",
	}
	for key, want := range defaults {
		parts := strings.Split(key, ":")
		if got := g.determineFactoryDefault(parts[0], parts[1]); got != want {
			t.Fatalf("determineFactoryDefault(%s) = %q, want %q", key, got, want)
		}
	}

	stringDefaults := map[string]string{
		"email":       "faker.Email()",
		"full_name":   "faker.Name()",
		"phoneNumber": "faker.Phonenumber()",
		"avatar_url":  "faker.URL()",
		"description": "faker.Sentence()",
		"title":       "faker.Word()",
		"address":     "faker.GetRealAddress().Address",
		"city":        "faker.GetRealAddress().City",
		"country":     "faker.GetRealAddress().Country",
		"zipcode":     "faker.GetRealAddress().PostalCode",
		"theme_color": "faker.GetRandomColor()",
		"misc":        "faker.Word()",
	}
	for name, want := range stringDefaults {
		if got := g.stringFactoryDefault(name); got != want {
			t.Fatalf("stringFactoryDefault(%q) = %q, want %q", name, got, want)
		}
	}

	intDefaults := map[string]string{
		"price_cents": "faker.RandomInt(100, 10000)",
		"quantity":    "faker.RandomInt(1, 100)",
		"age":         "faker.RandomInt(18, 80)",
		"rank":        "faker.RandomInt(1, 1000)",
	}
	for name, want := range intDefaults {
		if got := g.intFactoryDefault(name); got != want {
			t.Fatalf("intFactoryDefault(%q) = %q, want %q", name, got, want)
		}
	}

	zeros := map[string]string{
		"string":          `""`,
		"int64":           "0",
		"float64":         "0",
		"bool":            "false",
		"time.Time":       "time.Time{}",
		"uuid.UUID":       "uuid.UUID{}",
		"json.RawMessage": "nil",
		"[]byte":          "nil",
		"[]string":        "nil",
		"sql.NullString":  "sql.NullString{}",
		"bun.NullTime":    "bun.NullTime{}",
		"Money":           "Money{}",
	}
	for typ, want := range zeros {
		if got := g.getFactoryGoZero(typ); got != want {
			t.Fatalf("getFactoryGoZero(%q) = %q, want %q", typ, got, want)
		}
	}
}

func TestGeneratorTemplateRenderingAndImports(t *testing.T) {
	g := NewGenerator("postgresql")
	model := &GeneratedModel{Name: "Product", PluralName: "Products", Fields: []GeneratedField{{Name: "Sku", BunTag: "sku,notnull"}}}
	content, err := g.GenerateModelFile(model, `{{lower .Name}} {{Plural .Name}} {{range .Fields}}{{columnName .BunTag}}{{end}}`)
	if err != nil {
		t.Fatalf("GenerateModelFile: %v", err)
	}
	if content != "product Products sku" {
		t.Fatalf("model content = %q", content)
	}
	if _, err := g.GenerateModelFile(model, "{{"); err == nil || !strings.Contains(err.Error(), "failed to parse") {
		t.Fatalf("expected model parse error, got %v", err)
	}
	if _, err := g.GenerateModelFile(model, "{{.Missing.Field}}"); err == nil || !strings.Contains(err.Error(), "failed to execute") {
		t.Fatalf("expected model execute error, got %v", err)
	}

	factory := &GeneratedFactory{ModelName: "Product"}
	factoryContent, err := g.GenerateFactoryFile(factory, `{{toLower .ModelName}}`)
	if err != nil {
		t.Fatalf("GenerateFactoryFile: %v", err)
	}
	if factoryContent != "product" {
		t.Fatalf("factory content = %q", factoryContent)
	}
	if _, err := g.GenerateFactoryFile(factory, "{{"); err == nil || !strings.Contains(err.Error(), "failed to parse") {
		t.Fatalf("expected factory parse error, got %v", err)
	}
	if _, err := g.GenerateFactoryFile(factory, "{{.Missing.Field}}"); err == nil || !strings.Contains(err.Error(), "failed to execute") {
		t.Fatalf("expected factory execute error, got %v", err)
	}

	std, ext := groupAndSortImports(map[string]bool{
		"time":                   true,
		"context":                true,
		"github.com/google/uuid": true,
		"example.com/app":        true,
	})
	if !slices.Equal(std, []string{"context", "time"}) {
		t.Fatalf("std imports = %#v", std)
	}
	if !slices.Equal(ext, []string{"example.com/app", "github.com/google/uuid"}) {
		t.Fatalf("external imports = %#v", ext)
	}
}

func TestBuildFactoryMetadata(t *testing.T) {
	g := NewGenerator("postgresql")
	genModel := &GeneratedModel{
		Name:          "Order",
		EntityName:    "OrderEntity",
		NamespaceVar:  "Order",
		IDType:        "int64",
		IDGoFieldName: "OrderID",
		HasCreatedAt:  true,
		HasUpdatedAt:  true,
		Fields: []GeneratedField{
			{Name: "OrderID", Type: "int64", IsPrimaryKey: true},
			{Name: "CustomerID", Type: "uuid.UUID", IsForeignKey: true},
			{Name: "CreatedAt", Type: "time.Time"},
		},
	}

	factory, err := g.BuildFactory(catalog.NewCatalog("public"), Config{TableName: "orders", ModulePath: "example.com/app"}, genModel)
	if err != nil {
		t.Fatalf("BuildFactory: %v", err)
	}
	if factory.ModelName != "Order" || factory.IDType != "int64" || factory.IDGoFieldName != "OrderID" {
		t.Fatalf("unexpected factory identity: %#v", factory)
	}
	if !factory.HasForeignKeys || len(factory.ForeignKeyFields) != 1 {
		t.Fatalf("expected FK metadata, got %#v", factory.ForeignKeyFields)
	}
	if !slices.Contains(factory.StandardImports, "time") {
		t.Fatalf("expected time import, got %#v", factory.StandardImports)
	}
	if slices.Contains(factory.ExternalImports, "github.com/google/uuid") {
		t.Fatalf("int64 ID should not add uuid ID import: %#v", factory.ExternalImports)
	}
}

func TestBuildModelPrimaryKeyOverridesAndImports(t *testing.T) {
	table := tableWithColumns(t, "memberships",
		catalog.NewColumn("tenant_id", "uuid").SetPrimaryKey(),
		catalog.NewColumn("owner_id", "uuid").SetForeignKey("users", "id"),
		catalog.NewColumn("metadata", "jsonb"),
		catalog.NewColumn("created_at", "timestamp").SetNotNull(),
		catalog.NewColumn("updated_at", "timestamp").SetNotNull(),
	)
	cat := catalog.NewCatalog("public")
	if err := cat.AddTable("public", table); err != nil {
		t.Fatalf("add table: %v", err)
	}
	g := NewGenerator("postgresql")
	model, err := g.Build(cat, Config{
		TableName:        "memberships",
		ResourceName:     "Membership",
		PackageName:      "models",
		ModulePath:       "example.com/app",
		PrimaryKeyColumn: "tenant_id",
		NullType:         "pointer",
	})
	if err != nil {
		t.Fatalf("build model: %v", err)
	}
	if !model.HasPrimaryKey || model.IDFieldName != "tenant_id" || model.IDGoFieldName != "TenantID" || model.IDType != "uuid.UUID" {
		t.Fatalf("primary key override was not applied: %#v", model)
	}
	if !model.HasCreatedAt || !model.HasUpdatedAt {
		t.Fatalf("timestamps were not detected: %#v", model)
	}
	for _, want := range []string{"encoding/json", "github.com/google/uuid", "example.com/app/internal/storage", "example.com/app/internal/validation"} {
		if !slices.Contains(model.Imports, want) {
			t.Fatalf("model imports missing %q: %#v", want, model.Imports)
		}
	}
	if findColumn(table, "missing") != nil {
		t.Fatal("missing column lookup returned a value")
	}

	withoutPK, err := g.Build(cat, Config{TableName: "memberships", ResourceName: "Membership", GenerateWithoutPK: true})
	if err != nil {
		t.Fatalf("build without primary key: %v", err)
	}
	if withoutPK.HasPrimaryKey {
		t.Fatalf("GenerateWithoutPK selected a primary key: %#v", withoutPK)
	}
	if _, err := g.Build(cat, Config{TableName: "missing", ResourceName: "Missing"}); err == nil {
		t.Fatal("expected missing table error")
	}
}

func TestGenerateModelAndFactoryFiles(t *testing.T) {
	root := t.TempDir()
	cat := catalog.NewCatalog("public")
	table := tableWithColumns(t, "products",
		catalog.NewColumn("id", "uuid").SetPrimaryKey(),
		catalog.NewColumn("name", "text").SetNotNull(),
	)
	if err := cat.AddTable("public", table); err != nil {
		t.Fatalf("add products table: %v", err)
	}
	g := NewGenerator("postgresql")
	modelPath := filepath.Join(root, "product.go")
	if err := g.GenerateModel(cat, "Product", "products", modelPath, "example.com/app", "", "sql.Null", "id", false); err != nil {
		t.Fatalf("generate model: %v", err)
	}
	modelContent, err := os.ReadFile(modelPath)
	if err != nil || !strings.Contains(string(modelContent), "type ProductEntity struct") {
		t.Fatalf("generated model = %v\n%s", err, modelContent)
	}

	model, err := g.Build(cat, Config{TableName: "products", ResourceName: "Product", ModulePath: "example.com/app"})
	if err != nil {
		t.Fatalf("build factory model: %v", err)
	}
	factory, err := g.BuildFactory(cat, Config{TableName: "products", ModulePath: "example.com/app"}, model)
	if err != nil {
		t.Fatalf("build factory: %v", err)
	}
	if err := g.WriteFactoryFile(factory, root); err != nil {
		t.Fatalf("write factory: %v", err)
	}
	factoryPath := filepath.Join(root, "models", "factories", "product.go")
	factoryContent, err := os.ReadFile(factoryPath)
	if err != nil || !strings.Contains(string(factoryContent), "func BuildProduct") {
		t.Fatalf("generated factory = %v\n%s", err, factoryContent)
	}
}

func TestBuildCatalogFromMigrations(t *testing.T) {
	directory := t.TempDir()
	migration := `-- +goose Up
CREATE TABLE widgets (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL
);
-- +goose Down
DROP TABLE widgets;
`
	if err := os.WriteFile(filepath.Join(directory, "20260713120000_create_widgets.sql"), []byte(migration), 0o600); err != nil {
		t.Fatalf("write migration: %v", err)
	}
	cat, err := NewGenerator("postgresql").BuildCatalogFromMigrations("widgets", []string{directory})
	if err != nil {
		t.Fatalf("build catalog from migrations: %v", err)
	}
	if _, err := cat.GetTable("public", "widgets"); err != nil {
		t.Fatalf("widgets table missing from catalog: %v", err)
	}
	if _, err := NewGenerator("postgresql").BuildCatalogFromMigrations("widgets", []string{filepath.Join(directory, "missing")}); err == nil {
		t.Fatal("expected migration discovery error")
	}
}
