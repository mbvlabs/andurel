package models

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/pkg/naming"
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
		"Email:string":             "faker.Email()",
		"Name:string":              "faker.Name()",
		"Age:int32":                "randomInt(1, 1000, 100)",
		"Enabled:bool":             "randomBool()",
		"CreatedAt:time.Time":      "time.Time{}",
		"ID:uuid.UUID":             "uuid.UUID{}",
		"Metadata:json.RawMessage": `json.RawMessage("{}")`,
		"Payload:[]byte":           "[]byte{}",
		"Maybe:sql.NullString":     "sql.NullString{}",
		"MaybeBool:sql.NullBool":   "sql.NullBool{}",
		"MaybeInt:sql.NullInt64":   "sql.NullInt64{}",
		"ArchivedAt:sql.NullTime":  "sql.NullTime{}",
		"Maybe:bun.NullInt64":      "bun.NullInt64{}",
		"PublishedAt:bun.NullTime": "bun.NullTime{}",
		"Optional:*string":         "nil",
		"Custom:Money":             "*new(Money)",
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

func TestGenerateModelUpsertRequiresExplicitPrimaryKey(t *testing.T) {
	tests := []struct {
		name       string
		resource   string
		tableName  string
		primaryKey *catalog.Column
		idType     string
		receiver   string
	}{
		{
			name:       "uuid primary key",
			resource:   "Product",
			tableName:  "products",
			primaryKey: catalog.NewColumn("id", "uuid").SetPrimaryKey(),
			idType:     "uuid.UUID",
			receiver:   "p",
		},
		{
			name:       "serial primary key",
			resource:   "Event",
			tableName:  "events",
			primaryKey: catalog.NewColumn("id", "bigserial").SetPrimaryKey(),
			idType:     "int64",
			receiver:   "e",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			cat := catalog.NewCatalog("public")
			table := tableWithColumns(t, tt.tableName,
				tt.primaryKey,
				catalog.NewColumn("name", "text").SetNotNull(),
			)
			if err := cat.AddTable("public", table); err != nil {
				t.Fatalf("add table: %v", err)
			}

			modelPath := filepath.Join(root, strings.ToLower(tt.resource)+".go")
			if err := NewGenerator("postgresql").GenerateModel(cat, tt.resource, tt.tableName, modelPath, "example.com/app", "", "sql.Null", "id", false); err != nil {
				t.Fatalf("generate model: %v", err)
			}
			content, err := os.ReadFile(modelPath)
			if err != nil {
				t.Fatalf("read generated model: %v", err)
			}
			generated := string(content)
			signature := "func (" + tt.receiver + " " + strings.ToLower(tt.resource) + ") Upsert(ctx context.Context, db storage.Executor, id " + tt.idType + ", data Create" + tt.resource + "Data)"
			upsertStart := strings.Index(generated, signature)
			if upsertStart < 0 {
				t.Fatalf("generated model missing explicit-ID Upsert signature %q:\n%s", signature, generated)
			}
			upsert := generated[upsertStart:]
			if !strings.Contains(upsert, "ID: id,") {
				t.Fatalf("generated Upsert does not assign the supplied primary key:\n%s", upsert)
			}
			if strings.Contains(upsert, "uuid.New()") {
				t.Fatalf("generated Upsert replaces the supplied primary key:\n%s", upsert)
			}
		})
	}
}

func TestGenerateModelPaginationPluralizesAcronymResourcesWithTableOverrides(t *testing.T) {
	tests := map[string]string{
		"ServerSSHCredential": "ServerSSHCredentials",
		"WireGuardPeer":       "WireGuardPeers",
		"WireGuardPeerStatus": "WireGuardPeerStatuses",
	}
	for resourceName, pluralName := range tests {
		t.Run(resourceName, func(t *testing.T) {
			root := t.TempDir()
			tableName := "legacy_" + naming.ToSnakeCase(resourceName)
			cat := catalog.NewCatalog("public")
			table := tableWithColumns(t, tableName,
				catalog.NewColumn("id", "uuid").SetPrimaryKey(),
				catalog.NewColumn("name", "text").SetNotNull(),
			)
			if err := cat.AddTable("public", table); err != nil {
				t.Fatalf("add table: %v", err)
			}

			modelPath := filepath.Join(root, naming.ToSnakeCase(resourceName)+".go")
			if err := NewGenerator("postgresql").GenerateModel(cat, resourceName, naming.DeriveTableName(resourceName), modelPath, "example.com/app", tableName, "sql.Null", "id", false); err != nil {
				t.Fatalf("generate model: %v", err)
			}
			content, err := os.ReadFile(modelPath)
			if err != nil {
				t.Fatalf("read generated model: %v", err)
			}
			generated := string(content)
			for _, want := range []string{
				"type Paginated" + pluralName + " struct",
				pluralName + " []" + resourceName + "Entity",
				") (Paginated" + pluralName + ", error)",
			} {
				if !strings.Contains(generated, want) {
					t.Fatalf("generated pagination missing %q:\n%s", want, generated)
				}
			}
		})
	}
}

func TestGenerateModelCRUDUsesRepositoryNotFoundAndTimestampSemantics(t *testing.T) {
	root := t.TempDir()
	cat := catalog.NewCatalog("public")
	table := tableWithColumns(t, "products",
		catalog.NewColumn("id", "uuid").SetPrimaryKey(),
		catalog.NewColumn("name", "text").SetNotNull(),
		catalog.NewColumn("created_at", "timestamptz").SetNotNull(),
		catalog.NewColumn("updated_at", "timestamptz").SetNotNull(),
	)
	if err := cat.AddTable("public", table); err != nil {
		t.Fatalf("add table: %v", err)
	}

	modelPath := filepath.Join(root, "product.go")
	if err := NewGenerator("postgresql").GenerateModel(cat, "Product", "products", modelPath, "example.com/app", "", "sql.Null", "id", false); err != nil {
		t.Fatalf("generate model: %v", err)
	}
	content, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatalf("read generated model: %v", err)
	}
	generated := string(content)
	if count := strings.Count(generated, "if errors.Is(err, sql.ErrNoRows) {"); count != 3 {
		t.Fatalf("expected Find, Update, and Destroy not-found translation, got %d:\n%s", count, generated)
	}
	if count := strings.Count(generated, "return ProductEntity{}, ErrNotFound"); count != 2 {
		t.Fatalf("expected Find and Update to return ErrNotFound, got %d:\n%s", count, generated)
	}
	destroyStart := strings.Index(generated, "func (p product) Destroy(")
	allStart := strings.Index(generated, "func (p product) All(")
	if destroyStart < 0 || allStart <= destroyStart {
		t.Fatalf("could not isolate generated Destroy method:\n%s", generated)
	}
	destroy := generated[destroyStart:allStart]
	if !strings.Contains(destroy, `Returning("*")`) || !strings.Contains(destroy, "Scan(ctx)") {
		t.Fatalf("Destroy does not use DELETE RETURNING:\n%s", destroy)
	}
	if !strings.Contains(destroy, "return ErrNotFound") {
		t.Fatalf("Destroy does not translate a missing row:\n%s", destroy)
	}

	updateDataStart := strings.Index(generated, "type UpdateProductData struct {")
	updateMethodStart := strings.Index(generated, "func (p product) Update(")
	if updateDataStart < 0 || updateMethodStart <= updateDataStart {
		t.Fatalf("could not isolate generated UpdateProductData:\n%s", generated)
	}
	updateData := generated[updateDataStart:updateMethodStart]
	if strings.Contains(updateData, "UpdatedAt") {
		t.Fatalf("UpdateProductData exposes ignored UpdatedAt input:\n%s", updateData)
	}
	if !strings.Contains(generated[updateMethodStart:], "UpdatedAt: time.Now(),") {
		t.Fatalf("Update does not manage UpdatedAt internally:\n%s", generated[updateMethodStart:])
	}
}

func TestGenerateModelModesPersistAndRestrictGeneratedOperations(t *testing.T) {
	tests := []struct {
		mode    ModelMode
		present []string
		absent  []string
	}{
		{
			mode:    ModelModeCRUD,
			present: []string{" Find(", " Create(", " Update(", " Destroy(", " All(", " Paginate(", " Upsert("},
		},
		{
			mode:    ModelModeReadOnly,
			present: []string{" Find(", " All(", " Paginate("},
			absent:  []string{" Create(", " Update(", " Destroy(", " Upsert(", "type CreateProductData", "type UpdateProductData"},
		},
		{
			mode:    ModelModeCreateOnly,
			present: []string{" Create(", "type CreateProductData"},
			absent:  []string{" Find(", " Update(", " Destroy(", " All(", " Paginate(", " Upsert(", "type UpdateProductData"},
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			root := t.TempDir()
			cat := catalog.NewCatalog("public")
			table := tableWithColumns(t, "products",
				catalog.NewColumn("id", "uuid").SetPrimaryKey(),
				catalog.NewColumn("name", "text").SetNotNull(),
			)
			if err := cat.AddTable("public", table); err != nil {
				t.Fatalf("add table: %v", err)
			}

			modelPath := filepath.Join(root, "product.go")
			if err := NewGenerator("postgresql").GenerateModelWithMode(cat, "Product", "products", modelPath, "example.com/app", "", "sql.Null", "id", false, tt.mode); err != nil {
				t.Fatalf("generate model: %v", err)
			}
			content, err := os.ReadFile(modelPath)
			if err != nil {
				t.Fatalf("read generated model: %v", err)
			}
			generated := string(content)
			if !strings.Contains(generated, "// andurel:model-mode "+string(tt.mode)) {
				t.Fatalf("generated model does not persist mode %q:\n%s", tt.mode, generated)
			}
			if strings.Contains(generated, "func (e *ProductEntity) Validate() error") {
				t.Fatalf("generated model contains an empty Validate method:\n%s", generated)
			}
			for _, want := range tt.present {
				if !strings.Contains(generated, want) {
					t.Fatalf("%s model missing %q:\n%s", tt.mode, want, generated)
				}
			}
			for _, unwanted := range tt.absent {
				if strings.Contains(generated, unwanted) {
					t.Fatalf("%s model contains %q:\n%s", tt.mode, unwanted, generated)
				}
			}
		})
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

func TestBuildFactoryUsesSuppliedModelAndFieldNames(t *testing.T) {
	tests := []struct {
		modelName string
		tableName string
		fieldName string
		want      string
	}{
		{modelName: "Application", tableName: "applications", fieldName: "Name", want: "WithApplicationName"},
		{modelName: "Credential", tableName: "credentials", fieldName: "Provider", want: "WithCredentialProvider"},
		{modelName: "EnvironmentHealthCheck", tableName: "environment_health_checks", fieldName: "Url", want: "WithEnvironmentHealthCheckUrl"},
	}
	for _, tt := range tests {
		t.Run(tt.modelName, func(t *testing.T) {
			model := &GeneratedModel{
				Name:       tt.modelName,
				EntityName: tt.modelName + "Entity",
				Fields: []GeneratedField{
					{Name: tt.fieldName, Type: "string"},
				},
			}
			factory, err := NewGenerator("postgresql").BuildFactory(nil, Config{TableName: tt.tableName}, model)
			if err != nil {
				t.Fatalf("build factory: %v", err)
			}
			if got := factory.Fields[0].OptionName; got != tt.want {
				t.Fatalf("option name = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildFactoryUsesSchemaAwareSemanticDefaults(t *testing.T) {
	model := &GeneratedModel{
		Name:       "Endpoint",
		EntityName: "EndpointEntity",
		Fields: []GeneratedField{
			{Name: "Url", Type: "string"},
			{Name: "Cidr", Type: "string"},
			{Name: "Status", Type: "string", AllowedValues: []string{"pending", "ready"}},
		},
	}
	factory, err := NewGenerator("postgresql").BuildFactory(nil, Config{TableName: "endpoints"}, model)
	if err != nil {
		t.Fatalf("build factory: %v", err)
	}
	want := map[string]string{
		"Url":    "faker.URL()",
		"Cidr":   `"10.0.0.0/24"`,
		"Status": `"pending"`,
	}
	for _, field := range factory.Fields {
		if expected := want[field.Name]; expected != "" && field.DefaultValue != expected {
			t.Fatalf("%s default = %q, want %q", field.Name, field.DefaultValue, expected)
		}
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
	if !model.HasPrimaryKey || model.IDFieldName != "tenant_id" || model.IDGoFieldName != "TenantId" || model.IDType != "uuid.UUID" {
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
