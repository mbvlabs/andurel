package ddl

import (
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
)

func TestCatalogVisitorCreateAlterRenameAndDrop(t *testing.T) {
	cat := catalog.NewCatalog("public")
	visitor := NewCatalogVisitor(cat, "001_schema.sql", "postgresql")
	create := &CreateTableStatement{
		SchemaName:  "tenant",
		TableName:   "accounts",
		Columns:     []*catalog.Column{catalog.NewColumn("id", "uuid").SetPrimaryKey(), catalog.NewColumn("name", "text")},
		IfNotExists: true,
	}
	if err := visitor.VisitCreateTable(create); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := visitor.VisitCreateTable(create); err != nil {
		t.Fatalf("if-not-exists create should be a no-op: %v", err)
	}
	table, err := cat.GetTable("tenant", "accounts")
	if err != nil || table.CreatedBy != "001_schema.sql" {
		t.Fatalf("created table = %#v, %v", table, err)
	}

	if err := visitor.VisitAlterTable(&AlterTableStatement{
		SchemaName:     "tenant",
		TableName:      "accounts",
		AlterOperation: "ADD_COLUMN",
		ColumnDef:      catalog.NewColumn("score", "integer"),
	}); err != nil {
		t.Fatalf("add column: %v", err)
	}
	if err := visitor.VisitAlterTable(&AlterTableStatement{
		SchemaName:     "tenant",
		TableName:      "accounts",
		AlterOperation: "ALTER_COLUMN",
		ColumnName:     "score",
		ColumnChanges: map[string]any{
			"type":         "numeric(12,2)",
			"nullable":     false,
			"default":      "10",
			"drop_default": false,
			"ignored":      "value",
		},
	}); err != nil {
		t.Fatalf("alter column: %v", err)
	}
	score, err := table.GetColumn("score")
	if err != nil || score.DataType != "numeric" || score.Precision == nil || *score.Precision != 12 || score.Scale == nil || *score.Scale != 2 || score.IsNullable || score.DefaultVal == nil {
		t.Fatalf("altered score = %#v, %v", score, err)
	}
	if score.ModifiedBy != "001_schema.sql" {
		t.Fatalf("modified-by migration = %q", score.ModifiedBy)
	}

	if err := visitor.VisitAlterTable(&AlterTableStatement{
		SchemaName:     "tenant",
		TableName:      "accounts",
		AlterOperation: "RENAME_COLUMN",
		ColumnName:     "score",
		NewColumnName:  "rating",
	}); err != nil {
		t.Fatalf("rename column: %v", err)
	}
	if err := visitor.VisitAlterTable(&AlterTableStatement{
		SchemaName:     "tenant",
		TableName:      "accounts",
		AlterOperation: "DROP_COLUMN",
		ColumnName:     "rating",
	}); err != nil {
		t.Fatalf("drop column: %v", err)
	}
	if err := visitor.VisitAlterTable(&AlterTableStatement{
		SchemaName:     "tenant",
		TableName:      "accounts",
		AlterOperation: "RENAME_TABLE",
		NewTableName:   "customers",
	}); err != nil {
		t.Fatalf("rename table: %v", err)
	}
	if _, err := cat.GetTable("tenant", "accounts"); err == nil {
		t.Fatal("old table remained after rename")
	}
	if err := visitor.VisitDropTable(&DropTableStatement{SchemaName: "tenant", TableName: "customers"}); err != nil {
		t.Fatalf("drop table: %v", err)
	}
}

func TestCatalogVisitorErrorsConstraintsAndStubOperations(t *testing.T) {
	cat := catalog.NewCatalog("public")
	visitor := NewCatalogVisitor(cat, "002_errors.sql", "postgresql")
	duplicate := catalog.NewColumn("id", "uuid")
	if err := visitor.VisitCreateTable(&CreateTableStatement{TableName: "duplicates", Columns: []*catalog.Column{duplicate, duplicate}}); err == nil || !strings.Contains(err.Error(), "failed to add column") {
		t.Fatalf("expected duplicate column error, got %v", err)
	}
	if err := visitor.VisitCreateTable(&CreateTableStatement{TableName: "users", Columns: []*catalog.Column{catalog.NewColumn("id", "uuid")}}); err != nil {
		t.Fatalf("create users: %v", err)
	}

	for _, stmt := range []*AlterTableStatement{
		{TableName: "missing", AlterOperation: "DROP_COLUMN", ColumnName: "name"},
		{TableName: "users", AlterOperation: "ALTER_COLUMN", ColumnName: "missing", ColumnChanges: map[string]any{"type": "text"}},
		{TableName: "users", AlterOperation: "DROP_CONSTRAINT", Raw: "ALTER TABLE users DROP CONSTRAINT users_pkey"},
		{TableName: "users", AlterOperation: "ADD_CONSTRAINT", Raw: "ALTER TABLE users ADD PRIMARY KEY (id)"},
		{TableName: "users", AlterOperation: "UNSUPPORTED", Raw: "ALTER TABLE users ENABLE TRIGGER ALL"},
		{TableName: "users", AlterOperation: "MULTIPLE_OPERATIONS", Operations: []string{"NOT VALID SQL"}},
	} {
		if err := visitor.VisitAlterTable(stmt); err == nil {
			t.Fatalf("expected alter error for %#v", stmt)
		}
	}

	if err := visitor.VisitAlterTable(&AlterTableStatement{TableName: "users", AlterOperation: "ADD_CONSTRAINT", Raw: "ALTER TABLE users ADD UNIQUE (id)"}); err != nil {
		t.Fatalf("non-primary unique constraint should be ignored: %v", err)
	}
	if err := visitor.VisitDropTable(&DropTableStatement{TableName: "missing"}); err == nil {
		t.Fatal("expected missing drop table error")
	}

	for name, err := range map[string]error{
		"create index":  visitor.VisitCreateIndex(&CreateIndexStatement{}),
		"drop index":    visitor.VisitDropIndex(&DropIndexStatement{}),
		"create schema": visitor.VisitCreateSchema(&CreateSchemaStatement{}),
		"drop schema":   visitor.VisitDropSchema(&DropSchemaStatement{}),
		"create enum":   visitor.VisitCreateEnum(&CreateEnumStatement{}),
		"drop enum":     visitor.VisitDropEnum(&DropEnumStatement{}),
	} {
		if err != nil {
			t.Fatalf("%s stub returned error: %v", name, err)
		}
	}
}
