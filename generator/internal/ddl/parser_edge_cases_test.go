package ddl

import (
	"slices"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
)

func TestCreateTableParserEdgeCases(t *testing.T) {
	parser := NewDDLParser()
	stmt, err := parser.Parse(`CREATE TABLE IF NOT EXISTS tenant.orders (
		id UUID,
		user_id UUID REFERENCES users(id),
		status VARCHAR(32) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'ready')),
		phase TEXT NOT NULL,
		total NUMERIC(12,2) DEFAULT 0,
		tags TEXT[],
		metadata JSONB DEFAULT '{"source":"web,api"}',
		PRIMARY KEY (id),
		CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id),
		CONSTRAINT orders_phase_check CHECK (phase IN ('queued', 'running', 'finished')),
		CHECK (total >= 0),
		UNIQUE (status, user_id)
	)`, "001_orders.sql", "postgresql")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	create, ok := stmt.(*CreateTableStatement)
	if !ok {
		t.Fatalf("expected CreateTableStatement, got %T", stmt)
	}
	if !create.IfNotExists || create.SchemaName != "tenant" || create.TableName != "orders" {
		t.Fatalf("unexpected table metadata: %#v", create)
	}
	if len(create.Columns) != 7 {
		t.Fatalf("expected 7 columns, got %d", len(create.Columns))
	}

	byName := map[string]*catalog.Column{}
	for _, col := range create.Columns {
		byName[col.Name] = col
	}
	if !byName["id"].IsPrimaryKey || byName["id"].IsNullable {
		t.Fatalf("id should be a non-null primary key: %#v", byName["id"])
	}
	if byName["user_id"].ForeignKey == nil || byName["user_id"].ForeignKey.ReferencedTable != "users" {
		t.Fatalf("user_id should have users foreign key: %#v", byName["user_id"])
	}
	if byName["status"].Length == nil || *byName["status"].Length != 32 || byName["status"].IsNullable {
		t.Fatalf("status should be varchar(32) not null: %#v", byName["status"])
	}
	if got := byName["status"].AllowedValues; !slices.Equal(got, []string{"pending", "ready"}) {
		t.Fatalf("status allowed values = %#v", got)
	}
	if got := byName["phase"].AllowedValues; !slices.Equal(got, []string{"queued", "running", "finished"}) {
		t.Fatalf("phase allowed values = %#v", got)
	}
	if byName["total"].Precision == nil || *byName["total"].Precision != 12 || byName["total"].Scale == nil || *byName["total"].Scale != 2 {
		t.Fatalf("total should be numeric(12,2): %#v", byName["total"])
	}
	if byName["metadata"].DefaultVal == nil || !strings.Contains(*byName["metadata"].DefaultVal, "web,api") {
		t.Fatalf("metadata default with comma should be preserved: %#v", byName["metadata"])
	}
}

func TestCreateTableParserInvalidPrimaryKeyTypeReturnsValidationError(t *testing.T) {
	_, err := NewDDLParser().Parse(`CREATE TABLE files (
		id BYTEA PRIMARY KEY
	)`, "001_files.sql", "postgresql")
	if err == nil {
		t.Fatal("expected invalid primary key type error")
	}
	if !strings.Contains(err.Error(), "unsupported primary key type") {
		t.Fatalf("expected primary key validation error, got %v", err)
	}
}

func TestAlterTableParserOperationsAndVisitorApplication(t *testing.T) {
	parser := NewDDLParser()
	cat := catalog.NewCatalog("public")
	visitor := NewCatalogVisitor(cat, "001_create_users.sql", "postgresql")

	createStmt, err := parser.Parse(`CREATE TABLE users (
		id UUID PRIMARY KEY,
		email TEXT NOT NULL DEFAULT 'old@example.com'
	)`, "001_create_users.sql", "postgresql")
	if err != nil {
		t.Fatalf("parse create table: %v", err)
	}
	if err := createStmt.Accept(visitor); err != nil {
		t.Fatalf("visit create table: %v", err)
	}

	alterStmt, err := parser.Parse(`ALTER TABLE users
		ADD COLUMN IF NOT EXISTS display_name VARCHAR(80) DEFAULT 'anonymous',
		ALTER COLUMN email TYPE VARCHAR(320),
		ALTER COLUMN email DROP DEFAULT,
		ALTER COLUMN email SET NOT NULL,
		RENAME COLUMN display_name TO name`, "002_update_users.sql", "postgresql")
	if err != nil {
		t.Fatalf("parse alter table: %v", err)
	}
	alter, ok := alterStmt.(*AlterTableStatement)
	if !ok {
		t.Fatalf("expected AlterTableStatement, got %T", alterStmt)
	}
	if alter.AlterOperation != "MULTIPLE_OPERATIONS" || len(alter.Operations) != 5 {
		t.Fatalf("unexpected multiple alter operations: %#v", alter)
	}
	if err := alter.Accept(visitor); err != nil {
		t.Fatalf("visit alter table: %v", err)
	}

	table, err := cat.GetTable("", "users")
	if err != nil {
		t.Fatalf("GetTable users: %v", err)
	}
	name, err := table.GetColumn("name")
	if err != nil {
		t.Fatalf("renamed name column missing: %v", err)
	}
	if name.DataType != "varchar" || name.Length == nil || *name.Length != 80 {
		t.Fatalf("name should keep parsed varchar(80), got %#v", name)
	}
	email, err := table.GetColumn("email")
	if err != nil {
		t.Fatalf("email column missing: %v", err)
	}
	if email.DataType != "varchar" || email.Length == nil || *email.Length != 320 {
		t.Fatalf("email should be altered to varchar(320), got %#v", email)
	}
	if email.DefaultVal != nil {
		t.Fatalf("email default should be dropped, got %q", *email.DefaultVal)
	}
	if email.IsNullable {
		t.Fatal("email should remain not null")
	}
	if email.CreatedBy != "001_create_users.sql" {
		t.Fatalf("email CreatedBy = %q, want original migration file", email.CreatedBy)
	}
}

func TestAlterTableParserSingleOperationVariants(t *testing.T) {
	tests := []struct {
		name      string
		sql       string
		operation string
		check     func(*testing.T, *AlterTableStatement)
	}{
		{
			name:      "drop column",
			sql:       "ALTER TABLE users DROP COLUMN legacy_name",
			operation: "DROP_COLUMN",
			check: func(t *testing.T, stmt *AlterTableStatement) {
				if stmt.ColumnName != "legacy_name" {
					t.Fatalf("ColumnName = %q", stmt.ColumnName)
				}
			},
		},
		{
			name:      "rename table",
			sql:       "ALTER TABLE users RENAME TO accounts",
			operation: "RENAME_TABLE",
			check: func(t *testing.T, stmt *AlterTableStatement) {
				if stmt.NewTableName != "accounts" {
					t.Fatalf("NewTableName = %q", stmt.NewTableName)
				}
			},
		},
		{
			name:      "add constraint",
			sql:       "ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email)",
			operation: "ADD_CONSTRAINT",
		},
	}

	parser := NewDDLParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := parser.Parse(tt.sql, "003_alter_users.sql", "postgresql")
			if err != nil {
				t.Fatalf("Parse returned error: %v", err)
			}
			alter := stmt.(*AlterTableStatement)
			if alter.AlterOperation != tt.operation {
				t.Fatalf("AlterOperation = %q, want %q", alter.AlterOperation, tt.operation)
			}
			if tt.check != nil {
				tt.check(t, alter)
			}
		})
	}

	if _, err := parser.Parse(
		"ALTER TABLE users ENABLE ROW LEVEL SECURITY",
		"003_alter_users.sql",
		"postgresql",
	); err == nil || !strings.Contains(err.Error(), "unsupported schema-changing DDL") {
		t.Fatalf("expected actionable unsupported operation error, got %v", err)
	}
}

func TestSimpleParsersAndUnknownStatement(t *testing.T) {
	parser := NewDDLParser()

	cases := []struct {
		sql      string
		wantTyp  StatementType
		wantName string
	}{
		{"DROP TABLE IF EXISTS tenant.users", DropTable, "users"},
		{"CREATE INDEX idx_users_email ON users(email)", CreateIndex, ""},
		{"DROP INDEX idx_users_email", DropIndex, ""},
		{"CREATE SCHEMA IF NOT EXISTS tenant", CreateSchema, "tenant"},
		{"DROP SCHEMA IF EXISTS tenant", DropSchema, "tenant"},
		{"CREATE TYPE tenant.status AS ENUM ('active', 'disabled')", CreateEnum, "status"},
		{"DROP TYPE IF EXISTS tenant.status", DropEnum, "status"},
		{"VACUUM users", Unknown, ""},
	}

	for _, tt := range cases {
		t.Run(tt.sql, func(t *testing.T) {
			stmt, err := parser.Parse(tt.sql, "004_misc.sql", "postgresql")
			if err != nil {
				t.Fatalf("Parse returned error: %v", err)
			}
			if stmt.GetType() != tt.wantTyp {
				t.Fatalf("GetType = %v, want %v", stmt.GetType(), tt.wantTyp)
			}
			switch s := stmt.(type) {
			case *DropTableStatement:
				if s.TableName != tt.wantName || s.SchemaName != "tenant" || !s.IfExists {
					t.Fatalf("unexpected drop table statement: %#v", s)
				}
			case *CreateSchemaStatement:
				if s.SchemaName != tt.wantName {
					t.Fatalf("SchemaName = %q, want %q", s.SchemaName, tt.wantName)
				}
			case *DropSchemaStatement:
				if s.SchemaName != tt.wantName {
					t.Fatalf("SchemaName = %q, want %q", s.SchemaName, tt.wantName)
				}
			case *CreateEnumStatement:
				if s.EnumName != tt.wantName || s.SchemaName != "tenant" {
					t.Fatalf("unexpected create enum statement: %#v", s)
				}
				if s.EnumDef == nil || !slices.Equal(s.EnumDef.Values, []string{"active", "disabled"}) {
					t.Fatalf("unexpected enum values: %#v", s.EnumDef)
				}
			case *DropEnumStatement:
				if s.EnumName != tt.wantName || s.SchemaName != "tenant" {
					t.Fatalf("unexpected drop enum statement: %#v", s)
				}
			}
		})
	}
}
