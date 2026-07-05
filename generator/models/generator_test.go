package models

import (
	"slices"
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
