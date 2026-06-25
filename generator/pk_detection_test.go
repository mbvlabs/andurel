package generator

import (
	"testing"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
)

func TestDetectPrimaryKey(t *testing.T) {
	tests := []struct {
		name      string
		tableName string
		cat       *catalog.Catalog
		want      PrimaryKeyInfo
	}{
		{
			name:      "named id primary key",
			tableName: "users",
			cat: catalogWithTable(t, "users",
				catalog.NewColumn("id", "uuid").SetPrimaryKey(),
				catalog.NewColumn("email", "text").SetNotNull(),
			),
			want: PrimaryKeyInfo{
				ColumnName:  "id",
				GoFieldName: "ID",
				DataType:    "uuid",
				GoType:      "uuid.UUID",
				Found:       true,
				IsNamedID:   true,
			},
		},
		{
			name:      "alternate primary key",
			tableName: "orders",
			cat: catalogWithTable(t, "orders",
				catalog.NewColumn("order_id", "uuid").SetPrimaryKey(),
				catalog.NewColumn("total_cents", "integer").SetNotNull(),
			),
			want: PrimaryKeyInfo{
				ColumnName:  "order_id",
				GoFieldName: "OrderID",
				DataType:    "uuid",
				GoType:      "uuid.UUID",
				Found:       true,
			},
		},
		{
			name:      "serial primary key",
			tableName: "widgets",
			cat: catalogWithTable(t, "widgets",
				catalog.NewColumn("id", "serial").SetPrimaryKey(),
				catalog.NewColumn("name", "text").SetNotNull(),
			),
			want: PrimaryKeyInfo{
				ColumnName:      "id",
				GoFieldName:     "ID",
				DataType:        "serial",
				GoType:          "int32",
				IsAutoIncrement: true,
				Found:           true,
				IsNamedID:       true,
			},
		},
		{
			name:      "no primary key",
			tableName: "audit_log",
			cat: catalogWithTable(t, "audit_log",
				catalog.NewColumn("event_id", "uuid").SetNotNull(),
				catalog.NewColumn("action", "text").SetNotNull(),
			),
			want: PrimaryKeyInfo{},
		},
		{
			name:      "missing table",
			tableName: "missing",
			cat:       catalog.NewCatalog("public"),
			want:      PrimaryKeyInfo{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectPrimaryKey(tt.cat, tt.tableName)
			if got != tt.want {
				t.Fatalf("DetectPrimaryKey() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func catalogWithTable(t *testing.T, tableName string, columns ...*catalog.Column) *catalog.Catalog {
	t.Helper()

	cat := catalog.NewCatalog("public")
	table := catalog.NewTable("public", tableName)

	for _, column := range columns {
		if err := table.AddColumn(column); err != nil {
			t.Fatalf("failed to add column %s: %v", column.Name, err)
		}
	}

	if err := cat.AddTable("public", table); err != nil {
		t.Fatalf("failed to add table %s: %v", tableName, err)
	}

	return cat
}
