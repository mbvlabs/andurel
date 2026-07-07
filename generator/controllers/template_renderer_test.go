package controllers

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestRenderAPIControllerHonorsRequestedActions(t *testing.T) {
	controller := &GeneratedController{
		ResourceName:            "Work",
		ModelName:               "Work",
		PluralName:              "works",
		ModelPluralName:         "works",
		PluralResourceName:      "Works",
		ModelPluralResourceName: "Works",
		ReceiverName:            "w",
		Namespace:               "api",
		NamespacePascal:         "Api",
		ModulePath:              "example.com/app",
		Type:                    ResourceController,
		IDType:                  "uuid.UUID",
		IDGoFieldName:           "ID",
		Actions:                 []string{"create"},
		IsAPI:                   true,
	}

	rendered, err := NewTemplateRenderer().RenderControllerFile(controller, "")
	if err != nil {
		t.Fatalf("RenderControllerFile returned error: %v", err)
	}

	expectedParts := []string{
		"routes.ApiWorkCreate.Path()",
		"Handler: w.Create",
	}
	for _, part := range expectedParts {
		if !strings.Contains(rendered, part) {
			t.Fatalf("expected rendered API controller to contain %q:\n%s", part, rendered)
		}
	}

	unexpectedParts := []string{
		"routes.ApiWorkIndex.Path()",
		"routes.ApiWorkShow.Path()",
		"routes.ApiWorkUpdate.Path()",
		"routes.ApiWorkDestroy.Path()",
	}
	for _, part := range unexpectedParts {
		if strings.Contains(rendered, part) {
			t.Fatalf("expected rendered API controller not to contain %q:\n%s", part, rendered)
		}
	}
}

func TestRenderAPIControllerUsesLastNamespaceSegmentAsPackage(t *testing.T) {
	controller := &GeneratedController{
		ResourceName:            "Work",
		ModelName:               "Work",
		PluralName:              "works",
		ModelPluralName:         "works",
		PluralResourceName:      "Works",
		ModelPluralResourceName: "Works",
		ReceiverName:            "w",
		Namespace:               "api/v1",
		NamespacePascal:         "ApiV1",
		ModulePath:              "example.com/app",
		Type:                    ResourceController,
		IDType:                  "uuid.UUID",
		IDGoFieldName:           "ID",
		Actions:                 []string{"create"},
		IsAPI:                   true,
	}

	rendered, err := NewTemplateRenderer().RenderControllerFile(controller, "")
	if err != nil {
		t.Fatalf("RenderControllerFile returned error: %v", err)
	}
	if !strings.Contains(rendered, "package v1") {
		t.Fatalf("expected rendered API controller to use package v1:\n%s", rendered)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "controller.go", rendered, parser.ParseComments); err != nil {
		t.Fatalf("expected rendered API controller to parse: %v\n%s", err, rendered)
	}
}

func TestRenderNormalControllerStillHonorsRequestedActions(t *testing.T) {
	controller := &GeneratedController{
		ResourceName:            "Work",
		ModelName:               "Work",
		PluralName:              "works",
		ModelPluralName:         "works",
		PluralResourceName:      "Works",
		ModelPluralResourceName: "Works",
		ReceiverName:            "w",
		ModulePath:              "example.com/app",
		Type:                    ResourceController,
		IDType:                  "uuid.UUID",
		IDGoFieldName:           "ID",
		Actions:                 []string{"create"},
	}

	rendered, err := NewTemplateRenderer().RenderControllerFile(controller, "")
	if err != nil {
		t.Fatalf("RenderControllerFile returned error: %v", err)
	}

	expectedParts := []string{
		"routes.WorkCreate.Path()",
		"Handler: w.Create",
	}
	for _, part := range expectedParts {
		if !strings.Contains(rendered, part) {
			t.Fatalf("expected rendered controller to contain %q:\n%s", part, rendered)
		}
	}

	unexpectedParts := []string{
		"routes.WorkIndex.Path()",
		"routes.WorkShow.Path()",
		"routes.WorkUpdate.Path()",
		"routes.WorkDestroy.Path()",
	}
	for _, part := range unexpectedParts {
		if strings.Contains(rendered, part) {
			t.Fatalf("expected rendered controller not to contain %q:\n%s", part, rendered)
		}
	}
}
