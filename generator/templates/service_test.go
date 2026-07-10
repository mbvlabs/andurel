package templates

import (
	"strings"
	"testing"
	"testing/fstest"
	"text/template"
)

func TestTemplateBuilderAndRender(t *testing.T) {
	service := NewTemplateService()

	builder := NewTemplateBuilder(service).
		WithResource("Product", "Products", "example.com/app", "model", "postgresql", []string{"Name"}).
		WithDatabase("postgresql", "Pool", "pgx").
		WithProject("example.com/app", "app").
		WithCustom("receiver", "products")

	if builder.data.Resource.Name != "Product" {
		t.Fatalf("resource data not set: %#v", builder.data.Resource)
	}
	if builder.data.Database.Method != "Pool" {
		t.Fatalf("database data not set: %#v", builder.data.Database)
	}
	if builder.data.Project.ModulePath != "example.com/app" {
		t.Fatalf("project data not set: %#v", builder.data.Project)
	}
	if builder.data.Custom["receiver"] != "products" {
		t.Fatalf("custom data not set: %#v", builder.data.Custom)
	}
	if _, err := builder.Render("missing.tmpl"); err == nil ||
		!strings.Contains(err.Error(), "operation: get template") {
		t.Fatalf("expected builder render error, got %v", err)
	}
}

func TestTemplateServiceRenderTemplate(t *testing.T) {
	service := NewTemplateService()

	data := map[string]string{
		"ReceiverName":       "products",
		"PluralResourceName": "Products",
		"MethodName":         "Publish",
	}
	rendered, err := service.RenderTemplate("action_method.tmpl", data)
	if err != nil {
		t.Fatalf("RenderTemplate: %v", err)
	}
	if !strings.Contains(rendered, "func (products Products) Publish") {
		t.Fatalf("unexpected rendered method:\n%s", rendered)
	}

	if _, err := service.RenderTemplate("missing.tmpl", data); err == nil ||
		!strings.Contains(err.Error(), "operation: get template") {
		t.Fatalf("expected missing template error, got %v", err)
	}
}

func TestTemplateServiceRenderTemplateWithCustomFunctions(t *testing.T) {
	service := NewTemplateService()

	data := map[string]string{
		"ReceiverName":       "products",
		"PluralResourceName": "Products",
		"MethodName":         "Publish",
	}
	rendered, err := service.RenderTemplateWithCustomFunctions(
		"action_method.tmpl",
		data,
		template.FuncMap{"unused": func() string { return "ok" }},
	)
	if err != nil {
		t.Fatalf("RenderTemplateWithCustomFunctions: %v", err)
	}
	if !strings.Contains(rendered, "Publish") {
		t.Fatalf("unexpected rendered method:\n%s", rendered)
	}

	if _, err := service.RenderTemplateWithCustomFunctions("missing.tmpl", data, nil); err == nil ||
		!strings.Contains(err.Error(), "operation: get template") {
		t.Fatalf("expected missing template error, got %v", err)
	}
}

func TestTemplateServiceRenderTemplateWithPartials(t *testing.T) {
	service := NewTemplateService()
	templateFiles := fstest.MapFS{
		"primary.tmpl": &fstest.MapFile{Data: []byte(`{{template "Assignment" .}}`)},
		"partial.tmpl": &fstest.MapFile{Data: []byte(`{{define "Assignment"}}payload.{{.}}{{end}}`)},
	}

	rendered, err := service.renderTemplateWithCustomFunctionsAndPartials(
		templateFiles,
		"primary.tmpl",
		[]string{"partial.tmpl"},
		"Name",
		nil,
	)
	if err != nil {
		t.Fatalf("renderTemplateWithCustomFunctionsAndPartials: %v", err)
	}
	if rendered != "payload.Name" {
		t.Fatalf("rendered = %q, want %q", rendered, "payload.Name")
	}
}

func TestTemplateServiceRenderTemplateWithPartialsErrors(t *testing.T) {
	service := NewTemplateService()

	t.Run("missing partial", func(t *testing.T) {
		templateFiles := fstest.MapFS{
			"primary.tmpl": &fstest.MapFile{Data: []byte(`primary`)},
		}
		_, err := service.renderTemplateWithCustomFunctionsAndPartials(
			templateFiles,
			"primary.tmpl",
			[]string{"missing.tmpl"},
			nil,
			nil,
		)
		if err == nil || !strings.Contains(err.Error(), "operation: get partial") ||
			!strings.Contains(err.Error(), "template_name: missing.tmpl") {
			t.Fatalf("expected contextual missing partial error, got %v", err)
		}
	})

	t.Run("invalid partial", func(t *testing.T) {
		templateFiles := fstest.MapFS{
			"primary.tmpl": &fstest.MapFile{Data: []byte(`primary`)},
			"invalid.tmpl": &fstest.MapFile{Data: []byte(`{{define "broken"}}`)},
		}
		_, err := service.renderTemplateWithCustomFunctionsAndPartials(
			templateFiles,
			"primary.tmpl",
			[]string{"invalid.tmpl"},
			nil,
			nil,
		)
		if err == nil || !strings.Contains(err.Error(), "operation: parse partial") ||
			!strings.Contains(err.Error(), "template_name: invalid.tmpl") {
			t.Fatalf("expected contextual invalid partial error, got %v", err)
		}
	})
}

func TestTemplateCacheAndGlobalHelpers(t *testing.T) {
	ClearCache()
	funcs := getDefaultTemplateFunctions()

	first, err := GetCachedTemplate("action_method.tmpl", funcs)
	if err != nil {
		t.Fatalf("GetCachedTemplate first: %v", err)
	}
	second, err := GetCachedTemplate("action_method.tmpl", funcs)
	if err != nil {
		t.Fatalf("GetCachedTemplate second: %v", err)
	}
	if first != second {
		t.Fatal("expected cached template pointer to be reused")
	}

	if GetGlobalTemplateService() == nil {
		t.Fatal("expected global template service")
	}
	if NewTemplateBuilderUsingGlobal() == nil {
		t.Fatal("expected global template builder")
	}
	if rendered, err := RenderTemplateUsingGlobal("action_method.tmpl", map[string]string{
		"ReceiverName":       "jobs",
		"PluralResourceName": "Jobs",
		"MethodName":         "Run",
	}); err != nil || !strings.Contains(rendered, "Run") {
		t.Fatalf("RenderTemplateUsingGlobal = %q, %v", rendered, err)
	}
}

func TestTemplateFunctions(t *testing.T) {
	funcs := getDefaultTemplateFunctions()
	data := &TemplateData{Database: DatabaseData{Type: "postgresql", Method: "Pool"}}

	if got := funcs["DatabaseType"].(func(any) string)(data); got != "postgresql" {
		t.Fatalf("DatabaseType = %q", got)
	}
	if got := funcs["DatabaseType"].(func(any) string)("bad"); got != "" {
		t.Fatalf("DatabaseType for non-template data = %q", got)
	}
	if got := funcs["DatabaseMethod"].(func(any) string)(data); got != "Pool" {
		t.Fatalf("DatabaseMethod = %q", got)
	}
	if got := funcs["DatabaseMethod"].(func(any) string)(&TemplateData{}); got != "Conn" {
		t.Fatalf("DatabaseMethod default = %q", got)
	}
	if got := funcs["uuidParam"].(func(string, string) string)("id", "postgresql"); got != "id" {
		t.Fatalf("uuidParam = %q", got)
	}
	if got := toCamelCase("product_reviews"); got != "productReviews" {
		t.Fatalf("toCamelCase = %q", got)
	}
	if got := toCamelCase(""); got != "" {
		t.Fatalf("toCamelCase empty = %q", got)
	}
	if got := toLowerCamelCase("Product"); got != "product" {
		t.Fatalf("toLowerCamelCase = %q", got)
	}
	if got := toLowerCamelCase(""); got != "" {
		t.Fatalf("toLowerCamelCase empty = %q", got)
	}
}
