package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mbvlabs/andurel/generator/models"
)

func TestDetectPrimaryKey_NamedID(t *testing.T) {
	originalWd, _ := os.Getwd()
	migrationsDir := filepath.Join(originalWd, "models", "testdata", "migrations", "simple_user_table")

	generator := models.NewGenerator("postgresql")
	cat, err := generator.BuildCatalogFromMigrations("users", []string{migrationsDir})
	if err != nil {
		t.Fatalf("Failed to build catalog: %v", err)
	}

	info := DetectPrimaryKey(cat, "users")

	if !info.Found {
		t.Fatal("Expected PK to be found")
	}
	if !info.IsNamedID {
		t.Error("Expected IsNamedID to be true for 'id' PK")
	}
	if info.ColumnName != "id" {
		t.Errorf("Expected ColumnName 'id', got %q", info.ColumnName)
	}
	if info.GoFieldName != "ID" {
		t.Errorf("Expected GoFieldName 'ID', got %q", info.GoFieldName)
	}
	if info.GoType != "uuid.UUID" {
		t.Errorf("Expected GoType 'uuid.UUID', got %q", info.GoType)
	}
}

func TestDetectPrimaryKey_AlternateName(t *testing.T) {
	originalWd, _ := os.Getwd()
	migrationsDir := filepath.Join(originalWd, "models", "testdata", "migrations", "custom_pk")

	generator := models.NewGenerator("postgresql")
	cat, err := generator.BuildCatalogFromMigrations("orders", []string{migrationsDir})
	if err != nil {
		t.Fatalf("Failed to build catalog: %v", err)
	}

	info := DetectPrimaryKey(cat, "orders")

	if !info.Found {
		t.Fatal("Expected PK to be found")
	}
	if info.IsNamedID {
		t.Error("Expected IsNamedID to be false for 'order_id' PK")
	}
	if info.ColumnName != "order_id" {
		t.Errorf("Expected ColumnName 'order_id', got %q", info.ColumnName)
	}
	if info.GoFieldName != "OrderID" {
		t.Errorf("Expected GoFieldName 'OrderID', got %q", info.GoFieldName)
	}
}

func TestDetectPrimaryKey_NotFound(t *testing.T) {
	originalWd, _ := os.Getwd()
	migrationsDir := filepath.Join(originalWd, "models", "testdata", "migrations", "no_pk")

	generator := models.NewGenerator("postgresql")
	cat, err := generator.BuildCatalogFromMigrations("audit_log", []string{migrationsDir})
	if err != nil {
		t.Fatalf("Failed to build catalog: %v", err)
	}

	info := DetectPrimaryKey(cat, "audit_log")

	if info.Found {
		t.Fatal("Expected no PK to be found")
	}
}

func TestNopPrimaryKeyResolver(t *testing.T) {
	resolver := NopPrimaryKeyResolver{}

	info := PrimaryKeyInfo{
		ColumnName:  "user_id",
		GoFieldName: "UserID",
		GoType:      "uuid.UUID",
		Found:       true,
		IsNamedID:   false,
	}

	resolved, err := resolver.ResolveAlternatePK(info, "test_table")
	if err != nil {
		t.Fatalf("ResolveAlternatePK should not error: %v", err)
	}
	if resolved.ColumnName != info.ColumnName {
		t.Errorf("Expected ColumnName %q, got %q", info.ColumnName, resolved.ColumnName)
	}

	ok, err := resolver.ConfirmNoPK("test_table")
	if err != nil {
		t.Fatalf("ConfirmNoPK should not error: %v", err)
	}
	if !ok {
		t.Error("NopPrimaryKeyResolver should return true for ConfirmNoPK")
	}
}

// Test that NopPrimaryKeyResolver can be set on ModelManager
func TestModelManager_SetPrimaryKeyResolver(t *testing.T) {
	// Just verify the setter compiles and doesn't panic
	mm := &ModelManager{}
	mm.SetPrimaryKeyResolver(NopPrimaryKeyResolver{})
}

// Test that ControllerManager.SetPrimaryKeyResolver compiles
func TestControllerManager_SetPrimaryKeyResolver(t *testing.T) {
	cm := &ControllerManager{}
	cm.SetPrimaryKeyResolver(NopPrimaryKeyResolver{})
}
