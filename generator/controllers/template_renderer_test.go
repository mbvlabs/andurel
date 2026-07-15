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

func TestResourceControllerPayloadAssignmentsAreShared(t *testing.T) {
	controller := &GeneratedController{
		ResourceName:            "Widget",
		ModelName:               "Widget",
		PluralName:              "widgets",
		ModelPluralName:         "widgets",
		PluralResourceName:      "Widgets",
		ModelPluralResourceName: "Widgets",
		ReceiverName:            "w",
		ModulePath:              "example.com/app",
		Type:                    ResourceController,
		DatabaseType:            "postgresql",
		IDType:                  "uuid.UUID",
		IDGoFieldName:           "ID",
		HasPrimaryKey:           true,
		Actions:                 []string{"create", "update"},
		Fields: []GeneratedField{
			{Name: "ID", GoType: "uuid.UUID", GoFormType: "string", IsSystemField: true},
			{Name: "OwnerID", GoType: "*uuid.UUID", GoFormType: "string", CamelCase: "ownerID", IsPointer: true},
			{Name: "ExternalID", GoType: "uuid.UUID", GoFormType: "string", CamelCase: "externalID"},
			{Name: "PublishedOn", GoType: "time.Time", GoFormType: "time.Time", CamelCase: "publishedOn"},
			{Name: "ExpiresOn", GoType: "*time.Time", GoFormType: "time.Time", CamelCase: "expiresOn", IsPointer: true},
			{Name: "ReviewedOn", GoType: "sql.NullTime", GoFormType: "time.Time", CamelCase: "reviewedOn"},
			{Name: "ArchivedOn", GoType: "bun.NullTime", GoFormType: "time.Time", CamelCase: "archivedOn"},
			{Name: "Title", GoType: "sql.NullString", GoFormType: "string", CamelCase: "title"},
			{Name: "Summary", GoType: "bun.NullString", GoFormType: "string", CamelCase: "summary"},
			{Name: "Payload", GoType: "[]byte", GoFormType: "string", CamelCase: "payload"},
			{Name: "Metadata", GoType: "json.RawMessage", GoFormType: "string", CamelCase: "metadata"},
			{Name: "Enabled", GoType: "bool", GoFormType: "bool", CamelCase: "enabled"},
			{Name: "Count", GoType: "int64", GoFormType: "int64", CamelCase: "count"},
			{Name: "Subtitle", GoType: "*string", GoFormType: "*string", CamelCase: "subtitle", IsPointer: true},
		},
	}

	renderer := NewTemplateRenderer()
	regular, err := renderer.RenderControllerFile(controller, "")
	if err != nil {
		t.Fatalf("render regular controller: %v", err)
	}
	vue, err := renderer.RenderControllerFile(controller, "vue")
	if err != nil {
		t.Fatalf("render Vue Inertia controller: %v", err)
	}
	react, err := renderer.RenderControllerFile(controller, "react")
	if err != nil {
		t.Fatalf("render React Inertia controller: %v", err)
	}

	if vue != react {
		t.Fatal("React and Vue should use the same backend Inertia controller template")
	}

	for _, marker := range []string{
		"data := models.CreateWidgetData{",
		"data := models.UpdateWidgetData{",
	} {
		regularAssignments := controllerDataLiteral(t, regular, marker)
		inertiaAssignments := controllerDataLiteral(t, vue, marker)
		if regularAssignments != inertiaAssignments {
			t.Fatalf("assignments for %q differ\nregular:\n%s\ninertia:\n%s", marker, regularAssignments, inertiaAssignments)
		}
	}

	for name, rendered := range map[string]string{
		"regular": regular,
		"vue":     vue,
		"react":   react,
	} {
		t.Run(name, func(t *testing.T) {
			if count := strings.Count(rendered, `Metadata:    json.RawMessage("{}"),`); count != 2 {
				t.Fatalf("RawMessage placeholder count = %d, want 2\n%s", count, rendered)
			}
			for _, invalid := range []string{
				"Metadata:    payload.Metadata,",
				"json.RawMessage{}",
			} {
				if strings.Contains(rendered, invalid) {
					t.Fatalf("rendered controller contains invalid RawMessage assignment %q\n%s", invalid, rendered)
				}
			}
			if _, err := parser.ParseFile(token.NewFileSet(), name+"_controller.go", rendered, parser.ParseComments); err != nil {
				t.Fatalf("rendered controller does not parse: %v\n%s", err, rendered)
			}
		})
	}
}

func TestResourceControllerRawMessageImportIsActionAware(t *testing.T) {
	controller := &GeneratedController{
		ResourceName:            "Widget",
		ModelName:               "Widget",
		PluralName:              "widgets",
		ModelPluralName:         "widgets",
		PluralResourceName:      "Widgets",
		ModelPluralResourceName: "Widgets",
		ReceiverName:            "w",
		ModulePath:              "example.com/app",
		Type:                    ResourceController,
		IDType:                  "string",
		IDGoFieldName:           "ID",
		HasPrimaryKey:           true,
		Fields: []GeneratedField{
			{Name: "Metadata", GoType: "json.RawMessage", GoFormType: "string", CamelCase: "metadata"},
		},
	}

	renderer := NewTemplateRenderer()
	for _, inertia := range []string{"", "vue", "react", "svelte"} {
		controller.Actions = []string{"create"}
		rendered, err := renderer.RenderControllerFile(controller, inertia)
		if err != nil {
			t.Fatalf("render create controller for %q: %v", inertia, err)
		}
		if !strings.Contains(rendered, `"encoding/json"`) {
			t.Fatalf("create controller for %q is missing encoding/json", inertia)
		}

		controller.Actions = []string{"index"}
		rendered, err = renderer.RenderControllerFile(controller, inertia)
		if err != nil {
			t.Fatalf("render read-only controller for %q: %v", inertia, err)
		}
		if strings.Contains(rendered, `"encoding/json"`) {
			t.Fatalf("read-only controller for %q unexpectedly imports encoding/json", inertia)
		}
	}
}

func controllerDataLiteral(t *testing.T, rendered, marker string) string {
	t.Helper()

	start := strings.Index(rendered, marker)
	if start == -1 {
		t.Fatalf("rendered controller is missing %q", marker)
	}
	remaining := rendered[start:]
	end := strings.Index(remaining, "\n\t}")
	if end == -1 {
		t.Fatalf("rendered controller has no end for %q", marker)
	}

	return remaining[:end+3]
}
