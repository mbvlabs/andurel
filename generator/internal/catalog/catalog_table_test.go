package catalog

import (
	"strings"
	"testing"
)

func TestCatalogSchemaTableEnumLifecycleAndErrors(t *testing.T) {
	cat := NewCatalog("public")

	if schema, err := cat.GetSchema(""); err != nil || schema.Name != "public" {
		t.Fatalf("GetSchema default = (%v, %v), want public schema", schema, err)
	}
	if _, err := cat.GetSchema("missing"); err == nil {
		t.Fatal("expected missing schema error")
	}

	if _, err := cat.CreateSchema("tenant"); err != nil {
		t.Fatalf("CreateSchema tenant returned error: %v", err)
	}
	if _, err := cat.CreateSchema("tenant"); err == nil {
		t.Fatal("expected duplicate schema error")
	}

	users := NewTable("", "users")
	if err := users.AddColumn(NewColumn("id", "uuid").SetPrimaryKey()); err != nil {
		t.Fatalf("AddColumn id returned error: %v", err)
	}
	if err := cat.AddTable("", users); err != nil {
		t.Fatalf("AddTable users returned error: %v", err)
	}
	if users.Schema != "public" {
		t.Fatalf("AddTable should set default schema, got %q", users.Schema)
	}
	if err := cat.AddTable("", NewTable("", "users")); err == nil {
		t.Fatal("expected duplicate table error")
	}
	if err := cat.AddTable("missing", NewTable("", "other")); err == nil {
		t.Fatal("expected missing schema on AddTable")
	}

	if table, err := cat.GetTable("", "users"); err != nil || table != users {
		t.Fatalf("GetTable default = (%v, %v), want users table", table, err)
	}
	if _, err := cat.GetTable("", "missing"); err == nil {
		t.Fatal("expected missing table error")
	}
	if tables, err := cat.ListTables(""); err != nil || len(tables) != 1 || tables[0].Name != "users" {
		t.Fatalf("ListTables = (%v, %v), want one users table", tables, err)
	}

	if err := cat.AddEnum("", &Enum{Name: "status", Values: []string{"active"}}); err != nil {
		t.Fatalf("AddEnum returned error: %v", err)
	}
	if err := cat.AddEnum("", &Enum{Name: "status"}); err == nil {
		t.Fatal("expected duplicate enum error")
	}
	if err := cat.AddEnum("missing", &Enum{Name: "role"}); err == nil {
		t.Fatal("expected missing schema on AddEnum")
	}

	if err := cat.RenameTable("", "missing", "accounts"); err == nil {
		t.Fatal("expected missing table on RenameTable")
	}
	if err := cat.AddTable("", NewTable("", "accounts")); err != nil {
		t.Fatalf("AddTable accounts returned error: %v", err)
	}
	if err := cat.RenameTable("", "users", "accounts"); err == nil {
		t.Fatal("expected rename conflict")
	}
	if err := cat.DropTable("", "accounts"); err != nil {
		t.Fatalf("DropTable accounts returned error: %v", err)
	}
	if err := cat.RenameTable("", "users", "members"); err != nil {
		t.Fatalf("RenameTable users to members returned error: %v", err)
	}
	if _, err := cat.GetTable("", "members"); err != nil {
		t.Fatalf("renamed table should be retrievable: %v", err)
	}
	if err := cat.DropTable("", "members"); err != nil {
		t.Fatalf("DropTable members returned error: %v", err)
	}
	if err := cat.DropTable("", "members"); err == nil {
		t.Fatal("expected missing table on DropTable")
	}
	if err := cat.DropTable("missing", "members"); err == nil {
		t.Fatal("expected missing schema on DropTable")
	}
}

func TestTableAlterationAndCloneBehavior(t *testing.T) {
	table := NewTable("public", "users").SetCreatedBy("001_create_users.sql")
	id := NewColumn("id", "uuid").SetPrimaryKey().SetCreatedBy("001_create_users.sql")
	email := NewColumn("email", "text").SetNotNull().SetUnique().SetDefault("'user@example.com'")
	email.SetForeignKey("profiles", "email")
	email.SetModifiedBy("002_update_users.sql")
	if err := table.AddColumn(id); err != nil {
		t.Fatalf("AddColumn id returned error: %v", err)
	}
	if err := table.AddColumn(email); err != nil {
		t.Fatalf("AddColumn email returned error: %v", err)
	}
	if err := table.AddColumn(NewColumn("email", "text")); err == nil {
		t.Fatal("expected duplicate column error")
	}

	pks := table.GetPrimaryKeyColumns()
	if len(pks) != 1 || pks[0].Name != "id" {
		t.Fatalf("GetPrimaryKeyColumns = %#v, want id", pks)
	}

	replacement := NewColumn("email", "varchar").SetLength(320)
	if err := table.ModifyColumn("email", replacement); err != nil {
		t.Fatalf("ModifyColumn returned error: %v", err)
	}
	modified, err := table.GetColumn("email")
	if err != nil {
		t.Fatalf("GetColumn email returned error: %v", err)
	}
	if modified.CreatedBy != email.CreatedBy {
		t.Fatalf("ModifyColumn should preserve CreatedBy, got %q want %q", modified.CreatedBy, email.CreatedBy)
	}
	if err := table.RenameColumn("email", "contact_email"); err != nil {
		t.Fatalf("RenameColumn returned error: %v", err)
	}
	if err := table.RenameColumn("missing", "other"); err == nil {
		t.Fatal("expected missing source column error")
	}
	if err := table.RenameColumn("contact_email", "id"); err == nil {
		t.Fatal("expected destination column conflict")
	}
	if err := table.DropColumn("missing"); err == nil {
		t.Fatal("expected missing column on DropColumn")
	}

	index := &Index{Name: "idx_users_email", Columns: []string{"contact_email"}, IsUnique: true, CreatedBy: "003_index.sql"}
	if err := table.AddIndex(index); err != nil {
		t.Fatalf("AddIndex returned error: %v", err)
	}
	if err := table.AddIndex(index); err == nil {
		t.Fatal("expected duplicate index error")
	}

	clone := table.Clone()
	clone.Columns[0].Name = "clone_id"
	clone.Indexes[0].Columns[0] = "clone_email"
	if table.Columns[0].Name == "clone_id" {
		t.Fatal("Clone should deep-copy columns")
	}
	if table.Indexes[0].Columns[0] == "clone_email" {
		t.Fatal("Clone should deep-copy index column slices")
	}
	if clone.CreatedBy != table.CreatedBy || clone.Indexes[0].CreatedBy != index.CreatedBy {
		t.Fatal("Clone should preserve table and index metadata")
	}

	if err := table.DropIndex("idx_users_email"); err != nil {
		t.Fatalf("DropIndex returned error: %v", err)
	}
	if err := table.DropIndex("idx_users_email"); err == nil {
		t.Fatal("expected missing index error")
	}
}

func TestCatalogAlterTableOperations(t *testing.T) {
	cat := NewCatalog("public")
	table := NewTable("", "users")
	if err := table.AddColumn(NewColumn("id", "uuid").SetPrimaryKey()); err != nil {
		t.Fatalf("AddColumn id returned error: %v", err)
	}
	if err := cat.AddTable("", table); err != nil {
		t.Fatalf("AddTable returned error: %v", err)
	}

	if err := cat.AlterTable("", "users", TableAlteration{Type: AddColumn}); err == nil {
		t.Fatal("expected missing add-column definition error")
	}
	if err := cat.AlterTable("", "users", TableAlteration{Type: AddColumn, Column: NewColumn("name", "text")}); err != nil {
		t.Fatalf("AlterTable AddColumn returned error: %v", err)
	}
	if err := cat.AlterTable("", "users", TableAlteration{Type: ModifyColumn}); err == nil {
		t.Fatal("expected missing modify-column definition error")
	}
	if err := cat.AlterTable("", "users", TableAlteration{Type: ModifyColumn, Column: NewColumn("name", "varchar").SetLength(255)}); err != nil {
		t.Fatalf("AlterTable ModifyColumn returned error: %v", err)
	}
	if err := cat.AlterTable("", "users", TableAlteration{Type: RenameColumn}); err == nil {
		t.Fatal("expected missing rename-column names error")
	}
	if err := cat.AlterTable("", "users", TableAlteration{Type: RenameColumn, OldName: "name", NewName: "display_name"}); err != nil {
		t.Fatalf("AlterTable RenameColumn returned error: %v", err)
	}
	if err := cat.AlterTable("", "users", TableAlteration{Type: AddIndex}); err == nil {
		t.Fatal("expected missing index definition error")
	}
	if err := cat.AlterTable("", "users", TableAlteration{Type: AddIndex, IndexDef: &Index{Name: "idx_users_display_name"}}); err != nil {
		t.Fatalf("AlterTable AddIndex returned error: %v", err)
	}
	if err := cat.AlterTable("", "users", TableAlteration{Type: DropIndex}); err == nil {
		t.Fatal("expected missing drop-index name error")
	}
	if err := cat.AlterTable("", "users", TableAlteration{Type: DropIndex, IndexName: "idx_users_display_name"}); err != nil {
		t.Fatalf("AlterTable DropIndex returned error: %v", err)
	}
	if err := cat.AlterTable("", "users", TableAlteration{Type: DropColumn}); err == nil {
		t.Fatal("expected missing drop-column name error")
	}
	if err := cat.AlterTable("", "users", TableAlteration{Type: DropColumn, OldName: "display_name"}); err != nil {
		t.Fatalf("AlterTable DropColumn returned error: %v", err)
	}
	if err := cat.AlterTable("", "users", TableAlteration{Type: AlterationType(999)}); err == nil || !strings.Contains(err.Error(), "unknown alteration type") {
		t.Fatalf("expected unknown alteration type error, got %v", err)
	}
}
