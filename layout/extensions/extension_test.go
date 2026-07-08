package extensions

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/layout/blueprint"
)

type testExtension struct {
	name string
	deps []string
}

func (e testExtension) Name() string {
	return e.name
}

func (e testExtension) Apply(ctx *Context) error {
	return nil
}

func (e testExtension) Dependencies() []string {
	return e.deps
}

type testTemplateData struct {
	moduleName string
	inertia    string
	bp         *blueprint.Blueprint
}

func (d *testTemplateData) DatabaseDialect() string {
	return "postgresql"
}

func (d *testTemplateData) GetModuleName() string {
	return d.moduleName
}

func (d *testTemplateData) GetInertia() string {
	return d.inertia
}

func (d *testTemplateData) Builder() *blueprint.Builder {
	if d.bp == nil {
		d.bp = blueprint.New()
	}
	return blueprint.NewBuilder(d.bp)
}

func (d *testTemplateData) SetBlueprint(bp *blueprint.Blueprint) {
	d.bp = bp
}

func TestRegisterGetAndNames(t *testing.T) {
	if err := Register(nil); err == nil || !strings.Contains(err.Error(), "cannot be nil") {
		t.Fatalf("expected nil extension error, got %v", err)
	}
	if err := Register(testExtension{}); err == nil || !strings.Contains(err.Error(), "non-empty name") {
		t.Fatalf("expected empty extension name error, got %v", err)
	}

	ext := testExtension{name: "test-register-extension"}
	if err := Register(ext); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := Register(testExtension{name: "test-register-extension", deps: []string{"ignored"}}); err != nil {
		t.Fatalf("duplicate Register failed: %v", err)
	}

	got, ok := Get("test-register-extension")
	if !ok || got.Name() != ext.Name() || len(got.Dependencies()) != 0 {
		t.Fatalf("expected first registered extension to remain, got %+v, %v", got, ok)
	}

	names := Names()
	if !slices.IsSorted(names) {
		t.Fatalf("expected sorted extension names, got %v", names)
	}
	if !slices.Contains(names, "test-register-extension") {
		t.Fatalf("expected registered extension name in %v", names)
	}
}

func TestContextBuilder(t *testing.T) {
	if builder := (*Context)(nil).Builder(); builder != nil {
		t.Fatalf("expected nil builder for nil context, got %+v", builder)
	}
	if builder := (&Context{}).Builder(); builder != nil {
		t.Fatalf("expected nil builder without data, got %+v", builder)
	}

	data := &testTemplateData{}
	builder := (&Context{Data: data}).Builder()
	if builder == nil || builder.Blueprint() == nil {
		t.Fatalf("expected builder with blueprint, got %+v", builder)
	}
}

func TestDockerApply(t *testing.T) {
	for _, ctx := range []*Context{nil, &Context{}} {
		if err := (Docker{}).Apply(ctx); err == nil {
			t.Fatal("expected nil context or data error")
		}
	}

	calls := map[string]string{}
	ctx := &Context{
		Data: &testTemplateData{},
		ProcessTemplate: func(templateFile, targetPath string, data TemplateData) error {
			calls[templateFile] = targetPath
			return nil
		},
	}

	if err := (Docker{}).Apply(ctx); err != nil {
		t.Fatalf("Docker Apply failed: %v", err)
	}
	if calls["templates/docker/Dockerfile.tmpl"] != "Dockerfile" {
		t.Fatalf("expected Dockerfile render call, got %v", calls)
	}
	if calls["templates/docker/dockerignore.tmpl"] != ".dockerignore" {
		t.Fatalf("expected dockerignore render call, got %v", calls)
	}
	if deps := (Docker{}).Dependencies(); deps != nil {
		t.Fatalf("expected no dependencies, got %v", deps)
	}
}

func TestAwsSesApply(t *testing.T) {
	data := &testTemplateData{}
	var rendered []string
	ctx := &Context{
		Data: data,
		ProcessTemplate: func(templateFile, targetPath string, tmplData TemplateData) error {
			rendered = append(rendered, templateFile+"=>"+targetPath)
			return nil
		},
	}

	if err := (AwsSes{}).Apply(ctx); err != nil {
		t.Fatalf("AwsSes Apply failed: %v", err)
	}

	bp := data.bp
	if bp == nil {
		t.Fatal("expected blueprint contributions")
	}
	if len(bp.Config.Fields) != 1 || bp.Config.Fields[0].Name != "AwsSes" {
		t.Fatalf("expected AwsSes config field, got %+v", bp.Config.Fields)
	}
	if len(bp.Config.EnvVars) != 4 {
		t.Fatalf("expected AWS SES env vars, got %+v", bp.Config.EnvVars)
	}
	if len(bp.Main.ServiceProvides) != 1 || !strings.Contains(bp.Main.ServiceProvides[0], "NewAwsSes") {
		t.Fatalf("expected AWS SES service provider, got %+v", bp.Main.ServiceProvides)
	}
	for _, want := range []string{
		"templates/aws-ses/clients_email_aws_ses.tmpl=>clients/email/aws_ses.go",
		"templates/aws-ses/config_aws_ses.tmpl=>config/aws_ses.go",
	} {
		if !slices.Contains(rendered, want) {
			t.Fatalf("expected render call %q in %v", want, rendered)
		}
	}
}

func TestCssComponentsApply(t *testing.T) {
	var rendered []string
	ctx := &Context{
		Data: &testTemplateData{},
		ProcessTemplate: func(templateFile, targetPath string, data TemplateData) error {
			rendered = append(rendered, templateFile+"=>"+targetPath)
			return nil
		},
	}

	if err := (CssComponents{}).Apply(ctx); err != nil {
		t.Fatalf("CssComponents Apply failed: %v", err)
	}
	for _, want := range []string{
		"templates/css-components/css_components.tmpl=>css/components.css",
		"templates/css-components/views_examples_buttons.tmpl=>views/examples/buttons.html",
		"templates/css-components/views_components_toast.tmpl=>views/components/toast.templ",
	} {
		if !slices.Contains(rendered, want) {
			t.Fatalf("expected render call %q in %v", want, rendered)
		}
	}
}

func TestExtensionRenderTemplateErrors(t *testing.T) {
	expectedErr := errors.New("render failed")
	ctx := &Context{
		Data: &testTemplateData{},
		ProcessTemplate: func(templateFile, targetPath string, data TemplateData) error {
			return expectedErr
		},
	}

	if err := (Docker{}).Apply(ctx); !errors.Is(err, expectedErr) {
		t.Fatalf("expected Docker render error, got %v", err)
	}
	if err := (AwsSes{}).Apply(ctx); !errors.Is(err, expectedErr) {
		t.Fatalf("expected AWS SES render error, got %v", err)
	}
	if err := (CssComponents{}).Apply(ctx); !errors.Is(err, expectedErr) {
		t.Fatalf("expected CSS components render error, got %v", err)
	}
}
