package controllers

import (
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/generator/internal/catalog"
)

func TestIsNullableType(t *testing.T) {
	tests := []struct {
		goType string
		want   bool
	}{
		{"*string", true},
		{"*time.Time", true},
		{"*int32", true},
		{"string", false},
		{"time.Time", false},
		{"int32", false},
		{"sql.NullString", true},
		{"sql.NullBool", true},
		{"sql.NullInt16", true},
		{"sql.NullInt32", true},
		{"sql.NullInt64", true},
		{"sql.NullFloat64", true},
		{"sql.NullTime", true},
		{"pgtype.Text", true},
		{"pgtype.Timestamp", true},
		{"pgtype.UUID", false},
	}
	for _, tt := range tests {
		t.Run(tt.goType, func(t *testing.T) {
			if got := isNullableType(tt.goType); got != tt.want {
				t.Errorf("isNullableType(%q) = %v, want %v", tt.goType, got, tt.want)
			}
		})
	}
}

func TestBuildDisambiguatesUnchangedPluralModelService(t *testing.T) {
	cat := catalog.NewCatalog("public")
	table := catalog.NewTable("public", "equipment")
	if err := table.AddColumn(catalog.NewColumn("id", "uuid").SetPrimaryKey()); err != nil {
		t.Fatalf("add column: %v", err)
	}
	if err := cat.AddTable("public", table); err != nil {
		t.Fatalf("add table: %v", err)
	}

	controller, err := NewGenerator("postgresql").Build(cat, Config{
		ResourceName:    "Equipment",
		ModelName:       "Equipment",
		PluralName:      "equipment",
		ModelPluralName: "equipment",
		TableName:       "equipment",
		ModulePath:      "example.com/app",
		ControllerType:  ResourceController,
		Actions:         []string{"index"},
	})
	if err != nil {
		t.Fatalf("Build() returned error: %v", err)
	}
	if controller.ModelServiceName != "EquipmentService" {
		t.Fatalf("model service name = %q", controller.ModelServiceName)
	}
	if controller.ModelCollectionName != "Equipment" {
		t.Fatalf("model collection name = %q", controller.ModelCollectionName)
	}

	rendered, err := NewTemplateRenderer().RenderControllerFile(controller, "")
	if err != nil {
		t.Fatalf("RenderControllerFile() returned error: %v", err)
	}
	for _, want := range []string{
		"equipmentService models.EquipmentService",
		"equipmentList.Equipment",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered controller missing %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "equipmentList.EquipmentService") {
		t.Fatalf("rendered controller uses service name as collection field:\n%s", rendered)
	}
}

func TestResolveControllerBaseType(t *testing.T) {
	tests := []struct {
		goType string
		want   string
	}{
		{"sql.NullString", "string"},
		{"sql.NullString", "string"},
		{"sql.NullBool", "bool"},
		{"sql.NullBool", "bool"},
		{"sql.NullInt16", "int16"},
		{"sql.NullInt32", "int32"},
		{"sql.NullInt32", "int32"},
		{"sql.NullInt64", "int64"},
		{"sql.NullInt64", "int64"},
		{"sql.NullFloat64", "float64"},
		{"sql.NullFloat64", "float64"},
		{"sql.NullTime", "time.Time"},
		{"sql.NullTime", "time.Time"},
		{"*string", "string"},
		{"*int32", "int32"},
		{"*time.Time", "time.Time"},
		{"string", "string"},
		{"time.Time", "time.Time"},
		{"int32", "int32"},
		{"pgtype.Text", "string"},
		{"pgtype.Bool", "bool"},
		{"pgtype.Int4", "int32"},
		{"pgtype.Timestamp", "time.Time"},
		{"pgtype.UUID", "uuid.UUID"},
	}
	for _, tt := range tests {
		t.Run(tt.goType, func(t *testing.T) {
			if got := resolveControllerBaseType(tt.goType); got != tt.want {
				t.Errorf("resolveControllerBaseType(%q) = %q, want %q", tt.goType, got, tt.want)
			}
		})
	}
}

func TestBuildField_NullableTimestamp(t *testing.T) {
	gen := NewGenerator("postgresql")
	col := &catalog.Column{
		Name:       "started_at",
		DataType:   "timestamp",
		IsNullable: true,
	}

	field, err := gen.buildField(col)
	if err != nil {
		t.Fatalf("buildField failed: %v", err)
	}

	if field.GoType != "pgtype.Timestamp" {
		t.Errorf("GoType = %q, want %q", field.GoType, "pgtype.Timestamp")
	}
	if field.GoFormType != "time.Time" {
		t.Errorf("GoFormType = %q, want %q", field.GoFormType, "time.Time")
	}
	if !field.IsPointer {
		t.Error("IsPointer should be true for pgtype timestamp")
	}
	if field.IsSystemField {
		t.Error("IsSystemField should be false for user-defined column")
	}
}

func TestBuildField_NonNullableTimestamp(t *testing.T) {
	col := &catalog.Column{
		Name:       "created_at",
		DataType:   "timestamp",
		IsNullable: false,
	}

	gen := NewGenerator("postgresql")
	field, err := gen.buildField(col)
	if err != nil {
		t.Fatalf("buildField failed: %v", err)
	}

	if field.GoType != "pgtype.Timestamp" {
		t.Errorf("GoType = %q, want %q", field.GoType, "pgtype.Timestamp")
	}
	if field.GoFormType != "time.Time" {
		t.Errorf("GoFormType = %q, want %q", field.GoFormType, "time.Time")
	}
	if !field.IsPointer {
		t.Error("IsPointer should be true because pgtype.Timestamp carries Valid")
	}
}

func TestBuildField_NullableString(t *testing.T) {
	gen := NewGenerator("postgresql")
	col := &catalog.Column{
		Name:       "description",
		DataType:   "varchar",
		IsNullable: true,
	}

	field, err := gen.buildField(col)
	if err != nil {
		t.Fatalf("buildField failed: %v", err)
	}

	if field.GoType != "pgtype.Text" {
		t.Errorf("GoType = %q, want %q", field.GoType, "pgtype.Text")
	}
	if field.GoFormType != "string" {
		t.Errorf("GoFormType = %q, want %q", field.GoFormType, "string")
	}
	if !field.IsPointer {
		t.Error("IsPointer should be true for pgtype.Text")
	}
}

func TestBuildField_NullableInt32(t *testing.T) {
	gen := NewGenerator("postgresql")
	col := &catalog.Column{
		Name:       "quantity",
		DataType:   "integer",
		IsNullable: true,
	}

	field, err := gen.buildField(col)
	if err != nil {
		t.Fatalf("buildField failed: %v", err)
	}

	if field.GoType != "pgtype.Int4" {
		t.Errorf("GoType = %q, want %q", field.GoType, "pgtype.Int4")
	}
	if field.GoFormType != "int32" {
		t.Errorf("GoFormType = %q, want %q", field.GoFormType, "int32")
	}
	if !field.IsPointer {
		t.Error("IsPointer should be true for pgtype.Int4")
	}
}

func TestBuildField_NullableBool(t *testing.T) {
	gen := NewGenerator("postgresql")
	col := &catalog.Column{
		Name:       "published",
		DataType:   "boolean",
		IsNullable: true,
	}

	field, err := gen.buildField(col)
	if err != nil {
		t.Fatalf("buildField failed: %v", err)
	}

	if field.GoType != "pgtype.Bool" {
		t.Errorf("GoType = %q, want %q", field.GoType, "pgtype.Bool")
	}
	if field.GoFormType != "bool" {
		t.Errorf("GoFormType = %q, want %q", field.GoFormType, "bool")
	}
	if !field.IsPointer {
		t.Error("IsPointer should be true for pgtype.Bool")
	}
}

func TestBuildField_NullableFloat64(t *testing.T) {
	gen := NewGenerator("postgresql")
	col := &catalog.Column{
		Name:       "price",
		DataType:   "double precision",
		IsNullable: true,
	}

	field, err := gen.buildField(col)
	if err != nil {
		t.Fatalf("buildField failed: %v", err)
	}

	if field.GoType != "pgtype.Float8" {
		t.Errorf("GoType = %q, want %q", field.GoType, "pgtype.Float8")
	}
	if field.GoFormType != "float64" {
		t.Errorf("GoFormType = %q, want %q", field.GoFormType, "float64")
	}
	if !field.IsPointer {
		t.Error("IsPointer should be true for pgtype.Float8")
	}
}

func TestBuildField_SystemFields(t *testing.T) {
	gen := NewGenerator("postgresql")

	systemFields := []string{"id", "created_at", "updated_at"}

	for _, name := range systemFields {
		t.Run(name, func(t *testing.T) {
			col := &catalog.Column{
				Name:         name,
				DataType:     "uuid",
				IsNullable:   false,
				IsPrimaryKey: name == "id",
			}

			field, err := gen.buildField(col)
			if err != nil {
				t.Fatalf("buildField failed: %v", err)
			}

			if !field.IsSystemField {
				t.Errorf("IsSystemField should be true for %q", name)
			}
		})
	}
}

func TestSetNullType(t *testing.T) {
	gen := NewGenerator("postgresql")

	if gen.typeMapper.NullType != "pgtype.Null" {
		t.Errorf("default NullType = %q, want %q", gen.typeMapper.NullType, "pgtype.Null")
	}

	gen.SetNullType("pointer")
	if gen.typeMapper.NullType != "pointer" {
		t.Errorf("after SetNullType NullType = %q, want %q", gen.typeMapper.NullType, "pointer")
	}
}

func TestInertiaDataTypeAndValue(t *testing.T) {
	tests := []struct {
		name      string
		field     GeneratedField
		wantType  string
		wantValue string
	}{
		{
			name:      "sql null string",
			field:     GeneratedField{Name: "Name", GoType: "sql.NullString"},
			wantType:  "string",
			wantValue: "entity.Name.String",
		},
		{
			name:      "bun null bool",
			field:     GeneratedField{Name: "Published", GoType: "sql.NullBool"},
			wantType:  "bool",
			wantValue: "entity.Published.Bool",
		},
		{
			name:      "raw message",
			field:     GeneratedField{Name: "Metadata", GoType: "json.RawMessage"},
			wantType:  "string",
			wantValue: "string(entity.Metadata)",
		},
		{
			name:      "pointer string",
			field:     GeneratedField{Name: "Subtitle", GoType: "*string"},
			wantType:  "string",
			wantValue: `func() string { if entity.Subtitle == nil { return "" }; return *entity.Subtitle }()`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inertiaDataType(tt.field); got != tt.wantType {
				t.Errorf("inertiaDataType() = %q, want %q", got, tt.wantType)
			}

			source := "entity." + tt.field.Name
			if got := inertiaDataValue(tt.field, source); got != tt.wantValue {
				t.Errorf("inertiaDataValue() = %q, want %q", got, tt.wantValue)
			}
		})
	}
}

func TestRenderInertiaControllerUsesDataStructAndRawMessagePlaceholder(t *testing.T) {
	controller := &GeneratedController{
		ResourceName:       "Widget",
		PluralName:         "widgets",
		PluralResourceName: "Widgets",
		ReceiverName:       "w",
		Package:            "controllers",
		ModulePath:         "testapp",
		Type:               ResourceController,
		IDType:             "uuid.UUID",
		IDGoFieldName:      "ID",
		HasPrimaryKey:      true,
		Fields: []GeneratedField{
			{
				Name:          "ID",
				GoType:        "uuid.UUID",
				GoFormType:    "string",
				CamelCase:     "id",
				IsSystemField: true,
			},
			{Name: "Name", GoType: "sql.NullString", GoFormType: "string", CamelCase: "name"},
			{Name: "Published", GoType: "sql.NullBool", GoFormType: "bool", CamelCase: "published"},
			{
				Name:       "Metadata",
				GoType:     "json.RawMessage",
				GoFormType: "string",
				CamelCase:  "metadata",
			},
			{Name: "CreatedAt", GoType: "time.Time", GoFormType: "time.Time", IsSystemField: true},
		},
	}

	rendered, err := NewTemplateRenderer().RenderControllerFile(controller, "vue")
	if err != nil {
		t.Fatalf("RenderControllerFile failed: %v", err)
	}

	expectedSnippets := []string{
		`"encoding/json"`,
		"type WidgetData struct {",
		"Name string",
		"Published bool",
		"Metadata string",
		`ID uuid.UUID ` + "`json:\"id\"`",
		`Name string ` + "`json:\"name\"`",
		"Name: entity.Name.String,",
		"Published: entity.Published.Bool,",
		"Metadata: string(entity.Metadata),",
		"inertia.FromStruct(WidgetIndexProps{",
		"Items: newWidgetDataList(widgetsList.Widgets),",
		"inertia.FromStruct(WidgetItemProps{",
		"Item: newWidgetData(widget),",
		`Metadata:    json.RawMessage("{}"),`,
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(rendered, snippet) {
			t.Fatalf("rendered controller missing %q\n\n%s", snippet, rendered)
		}
	}
}

func TestRenderInertiaControllerEmitsPgxPgtypeForSystemTimestamps(t *testing.T) {
	controller := &GeneratedController{
		ResourceName:       "Widget",
		PluralName:         "widgets",
		PluralResourceName: "Widgets",
		ReceiverName:       "w",
		Package:            "controllers",
		ModulePath:         "testapp",
		Type:               ResourceController,
		IDType:             "uuid.UUID",
		IDGoFieldName:      "ID",
		HasPrimaryKey:      true,
		Fields: []GeneratedField{
			{
				Name:          "ID",
				GoType:        "uuid.UUID",
				GoFormType:    "string",
				CamelCase:     "id",
				IsSystemField: true,
			},
			{Name: "Name", GoType: "string", GoFormType: "string", CamelCase: "name"},
			{
				Name:          "CreatedAt",
				GoType:        "pgtype.Timestamptz",
				GoFormType:    "time.Time",
				CamelCase:     "createdAt",
				IsSystemField: true,
			},
			{
				Name:          "UpdatedAt",
				GoType:        "pgtype.Timestamptz",
				GoFormType:    "time.Time",
				CamelCase:     "updatedAt",
				IsSystemField: true,
			},
		},
	}

	rendered, err := NewTemplateRenderer().RenderControllerFile(controller, "vue")
	if err != nil {
		t.Fatalf("RenderControllerFile failed: %v", err)
	}

	if !strings.Contains(rendered, `"github.com/jackc/pgx/v5/pgtype"`) {
		t.Fatalf("expected pgx/v5/pgtype import for system timestamps\n\n%s", rendered)
	}
	if strings.Contains(rendered, `"github.com/jackc/pgtype"`) {
		t.Fatalf("must not emit standalone jackc/pgtype import\n\n%s", rendered)
	}
	if !strings.Contains(rendered, "CreatedAt pgtype.Timestamptz") {
		t.Fatalf("expected CreatedAt pgtype.Timestamptz on WidgetData\n\n%s", rendered)
	}
}
