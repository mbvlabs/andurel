package controllers

import (
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
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
		{"bun.NullString", true},
		{"bun.NullBool", true},
		{"bun.NullInt32", true},
		{"bun.NullInt64", true},
		{"bun.NullFloat64", true},
		{"bun.NullTime", true},
	}
	for _, tt := range tests {
		t.Run(tt.goType, func(t *testing.T) {
			if got := isNullableType(tt.goType); got != tt.want {
				t.Errorf("isNullableType(%q) = %v, want %v", tt.goType, got, tt.want)
			}
		})
	}
}

func TestResolveControllerBaseType(t *testing.T) {
	tests := []struct {
		goType string
		want   string
	}{
		{"sql.NullString", "string"},
		{"bun.NullString", "string"},
		{"sql.NullBool", "bool"},
		{"bun.NullBool", "bool"},
		{"sql.NullInt16", "int16"},
		{"sql.NullInt32", "int32"},
		{"bun.NullInt32", "int32"},
		{"sql.NullInt64", "int64"},
		{"bun.NullInt64", "int64"},
		{"sql.NullFloat64", "float64"},
		{"bun.NullFloat64", "float64"},
		{"sql.NullTime", "time.Time"},
		{"bun.NullTime", "time.Time"},
		{"*string", "string"},
		{"*int32", "int32"},
		{"*time.Time", "time.Time"},
		{"string", "string"},
		{"time.Time", "time.Time"},
		{"int32", "int32"},
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
	strategies := []struct {
		name      string
		nullType  string
		goType    string
		formType  string
		isPointer bool
	}{
		{"pointer", "pointer", "*time.Time", "time.Time", true},
		{"sql.Null", "sql.Null", "sql.NullTime", "time.Time", true},
		{"bun.Null", "bun.Null", "bun.NullTime", "time.Time", true},
	}

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			gen := NewGenerator("postgresql")
			gen.SetNullType(s.nullType)

			col := &catalog.Column{
				Name:       "started_at",
				DataType:   "timestamp",
				IsNullable: true,
			}

			field, err := gen.buildField(col)
			if err != nil {
				t.Fatalf("buildField failed: %v", err)
			}

			if field.GoType != s.goType {
				t.Errorf("GoType = %q, want %q", field.GoType, s.goType)
			}
			if field.GoFormType != s.formType {
				t.Errorf("GoFormType = %q, want %q", field.GoFormType, s.formType)
			}
			if field.IsPointer != s.isPointer {
				t.Errorf("IsPointer = %v, want %v", field.IsPointer, s.isPointer)
			}
			if field.IsSystemField {
				t.Error("IsSystemField should be false for user-defined column")
			}
		})
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

	if field.GoType != "time.Time" {
		t.Errorf("GoType = %q, want %q", field.GoType, "time.Time")
	}
	if field.GoFormType != "time.Time" {
		t.Errorf("GoFormType = %q, want %q", field.GoFormType, "time.Time")
	}
	if field.IsPointer {
		t.Error("IsPointer should be false for non-nullable column")
	}
}

func TestBuildField_NullableString(t *testing.T) {
	strategies := []struct {
		name      string
		nullType  string
		goType    string
		formType  string
		isPointer bool
	}{
		{"pointer", "pointer", "*string", "*string", true},
		{"sql.Null", "sql.Null", "sql.NullString", "string", true},
		{"bun.Null", "bun.Null", "bun.NullString", "string", true},
	}

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			gen := NewGenerator("postgresql")
			gen.SetNullType(s.nullType)

			col := &catalog.Column{
				Name:       "description",
				DataType:   "varchar",
				IsNullable: true,
			}

			field, err := gen.buildField(col)
			if err != nil {
				t.Fatalf("buildField failed: %v", err)
			}

			if field.GoType != s.goType {
				t.Errorf("GoType = %q, want %q", field.GoType, s.goType)
			}
			if field.GoFormType != s.formType {
				t.Errorf("GoFormType = %q, want %q", field.GoFormType, s.formType)
			}
			if field.IsPointer != s.isPointer {
				t.Errorf("IsPointer = %v, want %v", field.IsPointer, s.isPointer)
			}
		})
	}
}

func TestBuildField_NullableInt32(t *testing.T) {
	strategies := []struct {
		name      string
		nullType  string
		goType    string
		formType  string
		isPointer bool
	}{
		{"pointer", "pointer", "*int32", "int32", true},
		{"sql.Null", "sql.Null", "sql.NullInt32", "int32", true},
		{"bun.Null", "bun.Null", "bun.NullInt32", "int32", true},
	}

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			gen := NewGenerator("postgresql")
			gen.SetNullType(s.nullType)

			col := &catalog.Column{
				Name:       "quantity",
				DataType:   "integer",
				IsNullable: true,
			}

			field, err := gen.buildField(col)
			if err != nil {
				t.Fatalf("buildField failed: %v", err)
			}

			if field.GoType != s.goType {
				t.Errorf("GoType = %q, want %q", field.GoType, s.goType)
			}
			if field.GoFormType != s.formType {
				t.Errorf("GoFormType = %q, want %q", field.GoFormType, s.formType)
			}
			if field.IsPointer != s.isPointer {
				t.Errorf("IsPointer = %v, want %v", field.IsPointer, s.isPointer)
			}
		})
	}
}

func TestBuildField_NullableBool(t *testing.T) {
	strategies := []struct {
		name      string
		nullType  string
		goType    string
		formType  string
		isPointer bool
	}{
		{"pointer", "pointer", "*bool", "bool", true},
		{"sql.Null", "sql.Null", "sql.NullBool", "bool", true},
		{"bun.Null", "bun.Null", "bun.NullBool", "bool", true},
	}

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			gen := NewGenerator("postgresql")
			gen.SetNullType(s.nullType)

			col := &catalog.Column{
				Name:       "published",
				DataType:   "boolean",
				IsNullable: true,
			}

			field, err := gen.buildField(col)
			if err != nil {
				t.Fatalf("buildField failed: %v", err)
			}

			if field.GoType != s.goType {
				t.Errorf("GoType = %q, want %q", field.GoType, s.goType)
			}
			if field.GoFormType != s.formType {
				t.Errorf("GoFormType = %q, want %q", field.GoFormType, s.formType)
			}
			if field.IsPointer != s.isPointer {
				t.Errorf("IsPointer = %v, want %v", field.IsPointer, s.isPointer)
			}
		})
	}
}

func TestBuildField_NullableFloat64(t *testing.T) {
	strategies := []struct {
		name      string
		nullType  string
		goType    string
		formType  string
		isPointer bool
	}{
		{"pointer", "pointer", "*float64", "float64", true},
		{"sql.Null", "sql.Null", "sql.NullFloat64", "float64", true},
		{"bun.Null", "bun.Null", "bun.NullFloat64", "float64", true},
	}

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			gen := NewGenerator("postgresql")
			gen.SetNullType(s.nullType)

			col := &catalog.Column{
				Name:       "price",
				DataType:   "double precision",
				IsNullable: true,
			}

			field, err := gen.buildField(col)
			if err != nil {
				t.Fatalf("buildField failed: %v", err)
			}

			if field.GoType != s.goType {
				t.Errorf("GoType = %q, want %q", field.GoType, s.goType)
			}
			if field.GoFormType != s.formType {
				t.Errorf("GoFormType = %q, want %q", field.GoFormType, s.formType)
			}
			if field.IsPointer != s.isPointer {
				t.Errorf("IsPointer = %v, want %v", field.IsPointer, s.isPointer)
			}
		})
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

	if gen.typeMapper.NullType != "sql.Null" {
		t.Errorf("default NullType = %q, want %q", gen.typeMapper.NullType, "sql.Null")
	}

	gen.SetNullType("bun.Null")
	if gen.typeMapper.NullType != "bun.Null" {
		t.Errorf("after SetNullType NullType = %q, want %q", gen.typeMapper.NullType, "bun.Null")
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
			field:     GeneratedField{Name: "Published", GoType: "bun.NullBool"},
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
			{Name: "ID", GoType: "uuid.UUID", GoFormType: "string", IsSystemField: true},
			{Name: "Name", GoType: "sql.NullString", GoFormType: "string", CamelCase: "name"},
			{Name: "Published", GoType: "bun.NullBool", GoFormType: "bool", CamelCase: "published"},
			{Name: "Metadata", GoType: "json.RawMessage", GoFormType: "string", CamelCase: "metadata"},
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
		"Name: entity.Name.String,",
		"Published: entity.Published.Bool,",
		"Metadata: string(entity.Metadata),",
		`"items": newWidgetDataList(widgetsList.Widgets),`,
		`"item": newWidgetData(widget),`,
		`Metadata:    json.RawMessage("{}"),`,
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(rendered, snippet) {
			t.Fatalf("rendered controller missing %q\n\n%s", snippet, rendered)
		}
	}
}
