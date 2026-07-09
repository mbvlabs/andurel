package views

import (
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
)

func TestBuildViewField_StringConverter(t *testing.T) {
	tests := []struct {
		name                    string
		columnName              string
		dataType                string
		isNullable              bool
		expectedGoType          string
		expectedStringConverter string
		expectedInputType       string
	}{
		{
			name:                    "string type needs no conversion",
			columnName:              "name",
			dataType:                "text",
			isNullable:              false,
			expectedGoType:          "string",
			expectedStringConverter: "",
			expectedInputType:       "text",
		},
		{
			name:                    "int32 uses fmt.Sprintf with %d",
			columnName:              "count",
			dataType:                "integer",
			isNullable:              false,
			expectedGoType:          "int32",
			expectedStringConverter: `fmt.Sprintf("%d", %s)`,
			expectedInputType:       "number",
		},
		{
			name:                    "int64 uses fmt.Sprintf with %d",
			columnName:              "big_count",
			dataType:                "bigint",
			isNullable:              false,
			expectedGoType:          "int64",
			expectedStringConverter: `fmt.Sprintf("%d", %s)`,
			expectedInputType:       "number",
		},
		{
			name:                    "int16 uses fmt.Sprintf with %d",
			columnName:              "small_count",
			dataType:                "smallint",
			isNullable:              false,
			expectedGoType:          "int16",
			expectedStringConverter: `fmt.Sprintf("%d", %s)`,
			expectedInputType:       "number",
		},
		{
			name:                    "float32 uses fmt.Sprintf with %f",
			columnName:              "rate",
			dataType:                "real",
			isNullable:              false,
			expectedGoType:          "float32",
			expectedStringConverter: `fmt.Sprintf("%f", %s)`,
			expectedInputType:       "number",
		},
		{
			name:                    "float64 uses fmt.Sprintf with %f",
			columnName:              "price",
			dataType:                "double precision",
			isNullable:              false,
			expectedGoType:          "float64",
			expectedStringConverter: `fmt.Sprintf("%f", %s)`,
			expectedInputType:       "number",
		},
		{
			name:                    "bool uses fmt.Sprintf with %t",
			columnName:              "is_active",
			dataType:                "boolean",
			isNullable:              false,
			expectedGoType:          "bool",
			expectedStringConverter: `fmt.Sprintf("%t", %s)`,
			expectedInputType:       "checkbox",
		},
		{
			name:                    "time.Time uses String method",
			columnName:              "created_at",
			dataType:                "timestamp",
			isNullable:              false,
			expectedGoType:          "time.Time",
			expectedStringConverter: "%s.String()",
			expectedInputType:       "date",
		},
		{
			name:                    "uuid.UUID uses String method",
			columnName:              "user_id",
			dataType:                "uuid",
			isNullable:              false,
			expectedGoType:          "uuid.UUID",
			expectedStringConverter: "%s.String()",
			expectedInputType:       "text",
		},
		{
			name:                    "[]byte uses string conversion",
			columnName:              "data",
			dataType:                "bytea",
			isNullable:              false,
			expectedGoType:          "[]byte",
			expectedStringConverter: "string(%s)",
			expectedInputType:       "text",
		},
		{
			name:                    "[]int32 array uses fmt.Sprintf with %v",
			columnName:              "scores",
			dataType:                "integer[]",
			isNullable:              false,
			expectedGoType:          "[]int32",
			expectedStringConverter: `fmt.Sprintf("%v", %s)`,
			expectedInputType:       "text",
		},
		{
			name:                    "[]string array uses strings.Join",
			columnName:              "tags",
			dataType:                "text[]",
			isNullable:              false,
			expectedGoType:          "[]string",
			expectedStringConverter: `strings.Join(%s, ", ")`,
			expectedInputType:       "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := NewGenerator("postgresql")

			col := &catalog.Column{
				Name:       tt.columnName,
				DataType:   tt.dataType,
				IsNullable: tt.isNullable,
			}

			field, err := generator.buildViewField(col)
			if err != nil {
				t.Fatalf("buildViewField returned error: %v", err)
			}

			if field.GoType != tt.expectedGoType {
				t.Errorf("GoType = %q, want %q", field.GoType, tt.expectedGoType)
			}

			if field.StringConverter != tt.expectedStringConverter {
				t.Errorf("StringConverter = %q, want %q", field.StringConverter, tt.expectedStringConverter)
			}

			if field.InputType != tt.expectedInputType {
				t.Errorf("InputType = %q, want %q", field.InputType, tt.expectedInputType)
			}
		})
	}
}

func TestBuildViewField_UnknownTypeHasConverter(t *testing.T) {
	generator := NewGenerator("postgresql")

	// Create a column with an unknown type that will fall through to default
	col := &catalog.Column{
		Name:       "unknown_field",
		DataType:   "some_unknown_type",
		IsNullable: false,
	}

	field, err := generator.buildViewField(col)
	if err != nil {
		t.Fatalf("buildViewField returned error: %v", err)
	}

	// Default case should use fmt.Sprintf("%v", %s) for safety
	expectedConverter := `fmt.Sprintf("%v", %s)`
	if field.StringConverter != expectedConverter {
		t.Errorf("Unknown type StringConverter = %q, want %q", field.StringConverter, expectedConverter)
	}
}

func TestViewDataLoopAssignment(t *testing.T) {
	t.Run("plain loop opens a templ control block", func(t *testing.T) {
		got := viewDataLoopAssignment("", "Article", "article", false)
		if got != "{" {
			t.Fatalf("viewDataLoopAssignment() = %q, want %q", got, "{")
		}
	})

	t.Run("dto loop wraps assignment in templ go code delimiters", func(t *testing.T) {
		got := viewDataLoopAssignment("", "Article", "article", true)
		want := "{\n\t\t\t\t\t\t\t\t\t{{ articleData := newArticleData(article) }}"
		if got != want {
			t.Fatalf("viewDataLoopAssignment() = %q, want %q", got, want)
		}
	})

	t.Run("dto loop with namespace prefix", func(t *testing.T) {
		got := viewDataLoopAssignment("Admin", "Article", "article", true)
		want := "{\n\t\t\t\t\t\t\t\t\t{{ adminArticleData := newAdminArticleData(article) }}"
		if got != want {
			t.Fatalf("viewDataLoopAssignment() = %q, want %q", got, want)
		}
	})
}

func TestGenerateViewFile_ContainsRequiredImports(t *testing.T) {
	generator := NewGenerator("postgresql")

	view := &GeneratedView{
		ResourceName: "Article",
		PluralName:   "articles",
		ModulePath:   "github.com/example/myapp",
		Fields: []ViewField{
			{
				Name:            "Tags",
				GoType:          "[]string",
				DisplayName:     "Tags",
				InputType:       "text",
				StringConverter: `strings.Join(%s, ", ")`,
				DBName:          "tags",
				CamelCase:       "tags",
			},
			{
				Name:            "Scores",
				GoType:          "[]int32",
				DisplayName:     "Scores",
				InputType:       "text",
				StringConverter: `fmt.Sprintf("%v", %s)`,
				DBName:          "scores",
				CamelCase:       "scores",
			},
		},
	}

	content, err := generator.GenerateViewFile(view, false, "css_components_")
	if err != nil {
		t.Fatalf("GenerateViewFile returned error: %v", err)
	}

	// Check that required imports are present
	if !strings.Contains(content, `"fmt"`) {
		t.Error("Generated view should import fmt package")
	}

	if !strings.Contains(content, `"strings"`) {
		t.Error("Generated view should import strings package")
	}
}

func TestGenerateViewFile_ArrayFieldsUseConverters(t *testing.T) {
	generator := NewGenerator("postgresql")

	view := &GeneratedView{
		ResourceName: "Article",
		PluralName:   "articles",
		ModulePath:   "github.com/example/myapp",
		Fields: []ViewField{
			{
				Name:            "Tags",
				GoType:          "[]string",
				DisplayName:     "Tags",
				InputType:       "text",
				StringConverter: `strings.Join(%s, ", ")`,
				DBName:          "tags",
				CamelCase:       "tags",
			},
		},
	}

	content, err := generator.GenerateViewFile(view, false, "css_components_")
	if err != nil {
		t.Fatalf("GenerateViewFile returned error: %v", err)
	}

	// The generated content should contain the strings.Join conversion
	if !strings.Contains(content, "strings.Join(article.Tags") {
		t.Errorf("Generated view should use strings.Join for []string field, got:\n%s", content)
	}
}

func TestGenerateViewFile_IntArrayFieldsUseConverters(t *testing.T) {
	generator := NewGenerator("postgresql")

	view := &GeneratedView{
		ResourceName: "Article",
		PluralName:   "articles",
		ModulePath:   "github.com/example/myapp",
		Fields: []ViewField{
			{
				Name:            "Scores",
				GoType:          "[]int32",
				DisplayName:     "Scores",
				InputType:       "text",
				StringConverter: `fmt.Sprintf("%v", %s)`,
				DBName:          "scores",
				CamelCase:       "scores",
			},
		},
	}

	content, err := generator.GenerateViewFile(view, false, "css_components_")
	if err != nil {
		t.Fatalf("GenerateViewFile returned error: %v", err)
	}

	// The generated content should contain the fmt.Sprintf conversion
	if !strings.Contains(content, "fmt.Sprintf") && !strings.Contains(content, "article.Scores") {
		t.Errorf("Generated view should use fmt.Sprintf for []int32 field, got:\n%s", content)
	}
}

func TestGenerateInertiaViewFiles_ReactResourceTypesAndInputs(t *testing.T) {
	generator := NewGenerator("postgresql")
	view := &GeneratedView{
		ResourceName: "Widget",
		PluralName:   "widgets",
		ModulePath:   "github.com/example/myapp",
		IDType:       "uuid.UUID",
		IDFieldName:  "ID",
		Fields: []ViewField{
			{Name: "Name", GoFormType: "string", DisplayName: "Name", InputType: "text", CamelCase: "name"},
			{Name: "Quantity", GoFormType: "int32", DisplayName: "Quantity", InputType: "number", CamelCase: "quantity"},
			{Name: "Active", GoFormType: "bool", DisplayName: "Active", InputType: "checkbox", CamelCase: "active"},
			{Name: "PublishedOn", GoFormType: "time.Time", DisplayName: "Published On", InputType: "date", CamelCase: "publishedOn"},
		},
	}

	files, err := generator.GenerateInertiaViewFiles(view, "inertia_react_", ".tsx")
	if err != nil {
		t.Fatalf("GenerateInertiaViewFiles returned error: %v", err)
	}

	index := files["Index.tsx"]
	for _, want := range []string{
		"import { Link } from '@inertiajs/react'",
		"import { routes } from '@/routes'",
		"type Item = {",
		"ID: RouteID",
		"Quantity: number",
		"Active: boolean",
		"{item.Active ? 'Yes' : 'No'}",
	} {
		if !strings.Contains(index, want) {
			t.Fatalf("Index.tsx missing %q:\n%s", want, index)
		}
	}
	for _, unwanted := range []string{"useForm", "FormEvent", "Record<string, any>", "<>"} {
		if strings.Contains(index, unwanted) {
			t.Fatalf("Index.tsx contains %q:\n%s", unwanted, index)
		}
	}

	create := files["Create.tsx"]
	for _, want := range []string{
		"import { Link, useForm } from '@inertiajs/react'",
		"type CreateForm = {",
		"quantity: number",
		"active: boolean",
		"quantity: 0,",
		"active: false,",
		"function submit(event: SubmitEvent)",
		"form.setData('quantity', Number(event.currentTarget.value))",
	} {
		if !strings.Contains(create, want) {
			t.Fatalf("Create.tsx missing %q:\n%s", want, create)
		}
	}
	for _, unwanted := range []string{"type Item", "FormEvent", "Record<string, any>", "<>"} {
		if strings.Contains(create, unwanted) {
			t.Fatalf("Create.tsx contains %q:\n%s", unwanted, create)
		}
	}

	edit := files["Edit.tsx"]
	for _, want := range []string{
		"type EditForm = {",
		"quantity: Number(item.Quantity ?? 0),",
		"publishedOn: String(item.PublishedOn ?? '').slice(0, 10),",
		"form.setData('quantity', Number(event.currentTarget.value))",
	} {
		if !strings.Contains(edit, want) {
			t.Fatalf("Edit.tsx missing %q:\n%s", want, edit)
		}
	}
	if strings.Contains(edit, "FormEvent") || strings.Contains(edit, "<>") {
		t.Fatalf("Edit.tsx contains deprecated event type or root fragment:\n%s", edit)
	}
}
