package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/generator"
	"github.com/mbvlabs/andurel/v2/layout"
)

func TestGenerateModelMapsFlagsToGenerator(t *testing.T) {
	resetCLITestSeams(t)
	fake := installFakeGenerator(t)

	result := executeCLITest(
		t,
		"generate",
		"model",
		"Widget",
		"--skip-factory",
		"--table-name",
		"inventory_widgets",
	)
	if result.err != nil {
		t.Fatalf("generate model failed: %v", result.err)
	}

	want := []modelCall{{
		name:        "Widget",
		tableName:   "inventory_widgets",
		skipFactory: true,
	}}
	if !reflect.DeepEqual(fake.modelCalls, want) {
		t.Fatalf("model calls: expected %#v, got %#v", want, fake.modelCalls)
	}
	if len(fake.modelWithPKCalls) != 0 {
		t.Fatalf("expected GenerateModelWithPK not to be called, got %#v", fake.modelWithPKCalls)
	}
}

func TestGenerateModelMapsPrimaryKeyToGenerator(t *testing.T) {
	resetCLITestSeams(t)
	fake := installFakeGenerator(t)

	result := executeCLITest(
		t,
		"generate",
		"model",
		"Warehouse",
		"--skip-factory",
		"--table-name",
		"warehouses",
		"--primary-key",
		"code",
	)
	if result.err != nil {
		t.Fatalf("generate model failed: %v", result.err)
	}

	want := []modelWithPKCall{{
		name:        "Warehouse",
		tableName:   "warehouses",
		skipFactory: true,
		primaryKey:  "code",
	}}
	if !reflect.DeepEqual(fake.modelWithPKCalls, want) {
		t.Fatalf("model with pk calls: expected %#v, got %#v", want, fake.modelWithPKCalls)
	}
	if len(fake.modelCalls) != 0 {
		t.Fatalf("expected GenerateModel not to be called, got %#v", fake.modelCalls)
	}
}

func TestGenerateModelMapsModeToGenerator(t *testing.T) {
	resetCLITestSeams(t)
	fake := installFakeGenerator(t)

	result := executeCLITest(
		t,
		"generate",
		"model",
		"AuditLog",
		"--mode",
		"read-only",
		"--primary-key",
		"event_id",
	)
	if result.err != nil {
		t.Fatalf("generate read-only model failed: %v", result.err)
	}
	want := []modelModeCall{{
		name:       "AuditLog",
		primaryKey: "event_id",
		mode:       generator.ModelModeReadOnly,
	}}
	if !reflect.DeepEqual(fake.modelModeCalls, want) {
		t.Fatalf("model mode calls: expected %#v, got %#v", want, fake.modelModeCalls)
	}
	if len(fake.modelCalls) != 0 || len(fake.modelWithPKCalls) != 0 {
		t.Fatalf(
			"mode generation used legacy calls: models=%#v with_pk=%#v",
			fake.modelCalls,
			fake.modelWithPKCalls,
		)
	}
}

func TestGenerateModelDryRunUsesGenerationPlan(t *testing.T) {
	resetCLITestSeams(t)
	fake := installFakeGenerator(t)
	fake.modelPlan = &generator.ModelGenerationPlan{}

	result := executeCLITest(
		t,
		"generate",
		"model",
		"ServerSSHCredential",
		"--dry-run",
		"--json",
		"--mode",
		"read-only",
		"--primary-key",
		"credential_id",
	)
	if result.err != nil {
		t.Fatalf("generate model dry run failed: %v", result.err)
	}
	want := []modelPlanCall{{
		name: "ServerSSHCredential",
		options: generator.ModelGenerationOptions{
			PrimaryKeyColumn: "credential_id",
			Mode:             generator.ModelModeReadOnly,
		},
	}}
	if !reflect.DeepEqual(fake.modelPlanCalls, want) {
		t.Fatalf("model plan calls: expected %#v, got %#v", want, fake.modelPlanCalls)
	}
	if len(fake.modelCalls) != 0 || len(fake.modelWithPKCalls) != 0 ||
		len(fake.modelModeCalls) != 0 {
		t.Fatalf("dry run invoked writing generation methods: %#v", fake)
	}
}

func TestGenerateModelDryRunReportsHumanSummary(t *testing.T) {
	resetCLITestSeams(t)
	fake := installFakeGenerator(t)
	fake.modelPlan = &generator.ModelGenerationPlan{Files: []generator.PlannedFile{
		{
			Path:       "models/model.go",
			Exists:     true,
			OldContent: "package models\n",
			NewContent: "package models\n\n\tNewProducts,\n",
		},
		{
			Path:       "models/product.go",
			NewContent: "package models\n",
		},
	}}

	result := executeCLITest(t, "generate", "model", "Product", "--dry-run")
	if result.err != nil {
		t.Fatalf("generate model dry run failed: %v", result.err)
	}
	for _, want := range []string{
		"Dry run: Would change 2 files for generate model",
		"  create models/product.go",
		"  update models/model.go",
	} {
		if !strings.Contains(result.stdout, want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, result.stdout)
		}
	}
}

func TestGenerateModelDryRunReturnsPlanningError(t *testing.T) {
	resetCLITestSeams(t)
	fake := installFakeGenerator(t)
	wantErr := errors.New("plan model")
	fake.modelPlanErr = wantErr

	result := executeCLITest(t, "generate", "model", "Product", "--dry-run")
	if !errors.Is(result.err, wantErr) {
		t.Fatalf("generate model dry run error = %v, want %v", result.err, wantErr)
	}
}

func TestGenerateModelUpdateMapsYesFlag(t *testing.T) {
	resetCLITestSeams(t)
	var gotName string
	var gotAutoApply bool
	runModelUpdateFunc = func(resourceName string, autoApply bool, skipFactory bool) error {
		gotName = resourceName
		gotAutoApply = autoApply
		return nil
	}

	result := executeCLITest(t, "generate", "model", "Widget", "--update", "--yes")
	if result.err != nil {
		t.Fatalf("generate model update failed: %v", result.err)
	}
	if gotName != "Widget" || !gotAutoApply {
		t.Fatalf(
			"expected update Widget autoApply=true, got name=%q autoApply=%v",
			gotName,
			gotAutoApply,
		)
	}
}

func TestGenerateModelRunsFromProjectRoot(t *testing.T) {
	resetCLITestSeams(t)

	rootDir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(rootDir, "go.mod"),
		[]byte("module example.com/app\n"),
		0o644,
	); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	nestedDir := filepath.Join(rootDir, "internal", "feature")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("create nested dir: %v", err)
	}

	chdirCLITestRoot(t, nestedDir)

	findGoModRoot = func() (string, error) {
		return rootDir, nil
	}

	fake := installFakeGenerator(t)
	var gotWD string
	fake.onGenerateModel = func() {
		gotWD, _ = os.Getwd()
	}

	var stdout, stderr bytes.Buffer
	cmd := NewRootCommand("test", "test-date")
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"generate", "model", "Widget"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("generate model failed: %v", err)
	}
	if gotWD != rootDir {
		t.Fatalf("expected generator to run in project root %q, got %q", rootDir, gotWD)
	}
}

func TestGenerateScaffoldMapsFlagsToGenerator(t *testing.T) {
	resetCLITestSeams(t)
	fake := installFakeGenerator(t)

	result := executeInertiaCLITest(
		t,
		"vue",
		"generate",
		"scaffold",
		"Project",
		"--skip-factory",
		"--table-name",
		"work_projects",
		"--primary-key",
		"slug",
	)
	if result.err != nil {
		t.Fatalf("generate scaffold failed: %v", result.err)
	}

	want := []scaffoldCall{{
		name:        "Project",
		namespace:   "",
		tableName:   "work_projects",
		skipFactory: true,
		primaryKey:  "slug",
		inertia:     "vue",
	}}
	if !reflect.DeepEqual(fake.scaffoldCalls, want) {
		t.Fatalf("scaffold calls: expected %#v, got %#v", want, fake.scaffoldCalls)
	}
}

func TestGenerateScaffoldMapsNamespaceToGenerator(t *testing.T) {
	resetCLITestSeams(t)
	fake := installFakeGenerator(t)

	result := executeCLITest(t, "generate", "scaffold", "admin/Widget")
	if result.err != nil {
		t.Fatalf("generate scaffold failed: %v", result.err)
	}

	want := []scaffoldCall{{
		name:      "Widget",
		namespace: "admin",
	}}
	if !reflect.DeepEqual(fake.scaffoldCalls, want) {
		t.Fatalf("scaffold calls: expected %#v, got %#v", want, fake.scaffoldCalls)
	}
}

func TestGenerateScaffoldAPINestsNamespaceUnderAPI(t *testing.T) {
	resetCLITestSeams(t)
	fake := installFakeGenerator(t)

	result := executeCLITest(t, "generate", "scaffold", "v1/User", "--api")
	if result.err != nil {
		t.Fatalf("generate scaffold failed: %v", result.err)
	}

	want := []scaffoldCall{{
		name:      "User",
		namespace: "api/v1",
		isAPI:     true,
	}}
	if !reflect.DeepEqual(fake.scaffoldCalls, want) {
		t.Fatalf("scaffold calls: expected %#v, got %#v", want, fake.scaffoldCalls)
	}
}

func TestGenerateScaffoldRejectsInvalidNamespaceBeforeGenerator(t *testing.T) {
	resetCLITestSeams(t)
	fake := installFakeGenerator(t)

	result := executeCLITest(t, "generate", "scaffold", "admin/reports/Widget")
	if result.err == nil {
		t.Fatalf("expected invalid namespace error")
	}
	if len(fake.scaffoldCalls) != 0 {
		t.Fatalf("expected no scaffold calls, got %#v", fake.scaffoldCalls)
	}
}

func TestGenerateControllerMapsActionsAndVue(t *testing.T) {
	resetCLITestSeams(t)
	var got controllerCall
	generateControllerWithActionsFunc = func(name, modelName string, actions []string, inertia string, isAPI bool) error {
		got = controllerCall{
			name:      name,
			modelName: modelName,
			actions:   append([]string(nil), actions...),
			inertia:   inertia,
		}
		return nil
	}

	result := executeInertiaCLITest(
		t,
		"vue",
		"generate",
		"controller",
		"Widget",
		"index",
		"export",
	)
	if result.err != nil {
		t.Fatalf("generate controller failed: %v", result.err)
	}

	want := controllerCall{
		name:      "Widget",
		modelName: "",
		actions:   []string{"index", "export"},
		inertia:   "vue",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("controller call: expected %#v, got %#v", want, got)
	}
}

func TestGenerateControllerMapsNamespaceToGenerator(t *testing.T) {
	resetCLITestSeams(t)
	fake := installFakeGenerator(t)

	result := executeCLITest(t, "generate", "controller", "admin/Widget", "index")
	if result.err != nil {
		t.Fatalf("generate controller failed: %v", result.err)
	}

	want := []controllerCall{{
		name:      "Widget",
		namespace: "admin",
		modelName: "Widget",
		actions:   []string{"index"},
	}}
	if !reflect.DeepEqual(fake.controllerCalls, want) {
		t.Fatalf("controller calls: expected %#v, got %#v", want, fake.controllerCalls)
	}
}

func TestGenerateControllerAPINestsNamespaceUnderAPI(t *testing.T) {
	resetCLITestSeams(t)
	fake := installFakeGenerator(t)

	result := executeCLITest(t, "generate", "controller", "v1/User", "create", "--api")
	if result.err != nil {
		t.Fatalf("generate controller failed: %v", result.err)
	}

	want := []controllerCall{{
		name:      "User",
		namespace: "api/v1",
		modelName: "User",
		actions:   []string{"create"},
		isAPI:     true,
	}}
	if !reflect.DeepEqual(fake.controllerCalls, want) {
		t.Fatalf("controller calls: expected %#v, got %#v", want, fake.controllerCalls)
	}
}

func TestGenerateControllerRejectsInvalidNamespaceBeforeGenerator(t *testing.T) {
	resetCLITestSeams(t)
	fake := installFakeGenerator(t)

	result := executeCLITest(t, "generate", "controller", "admin/reports/Widget", "index")
	if result.err == nil {
		t.Fatalf("expected invalid namespace error")
	}
	if len(fake.controllerCalls) != 0 {
		t.Fatalf("expected no controller calls, got %#v", fake.controllerCalls)
	}
}

func TestGenerateControllerMapsModelName(t *testing.T) {
	resetCLITestSeams(t)
	var got controllerCall
	generateControllerWithActionsFunc = func(name, modelName string, actions []string, inertia string, isAPI bool) error {
		got = controllerCall{
			name:      name,
			modelName: modelName,
			actions:   append([]string(nil), actions...),
			inertia:   inertia,
		}
		return nil
	}

	result := executeCLITest(
		t,
		"generate",
		"controller",
		"Dashboard",
		"index",
		"--model-name",
		"User",
	)
	if result.err != nil {
		t.Fatalf("generate controller failed: %v", result.err)
	}

	want := controllerCall{
		name:      "Dashboard",
		modelName: "User",
		actions:   []string{"index"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("controller call: expected %#v, got %#v", want, got)
	}
}

func TestGenerateControllerInertiaRefreshesRoutesTSForCustomActions(t *testing.T) {
	resetCLITestSeams(t)
	rootDir := t.TempDir()
	writeCLITestFile(t, rootDir, "go.mod", "module example.com/app\n\ngo 1.27.1\n")

	lock := layout.NewAndurelLock("test")
	lock.ScaffoldConfig = &layout.ScaffoldConfig{
		ProjectName: "app",
		Inertia:     "react",
	}
	if err := lock.WriteLockFile(rootDir); err != nil {
		t.Fatalf("write andurel.lock: %v", err)
	}

	findGoModRoot = func() (string, error) {
		return rootDir, nil
	}
	generateControllerWithActionsFunc = func(name, modelName string, actions []string, inertia string, isAPI bool) error {
		if name != "Widget" || modelName != "" || inertia != "react" || isAPI {
			t.Fatalf(
				"unexpected controller call: name=%q model=%q inertia=%q api=%v",
				name,
				modelName,
				inertia,
				isAPI,
			)
		}
		if !reflect.DeepEqual(actions, []string{"export"}) {
			t.Fatalf("unexpected actions: %#v", actions)
		}
		writeCLITestFile(t, rootDir, "router/routes/widgets.go", `package routes

import "example.com/app/pkg/routing"

const WidgetPrefix = "/widgets"

var WidgetExport = routing.NewSimpleRoute(
	"/export",
	"widgets.export",
	WidgetPrefix,
	routing.InertiaRoute(),
)
`)
		return nil
	}

	chdirCLITestRoot(t, rootDir)

	var stdout, stderr bytes.Buffer
	cmd := NewRootCommand("test", "test-date")
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"generate", "controller", "Widget", "export"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("generate controller failed: %v\nstderr:\n%s", err, stderr.String())
	}

	assertCLITestFileContains(
		t,
		rootDir,
		filepath.Join("resources", "js", "routes.ts"),
		"widgetExport: () => '/widgets/export'",
	)
}

func TestControllerActionClassification(t *testing.T) {
	actions := []string{"index", "show", "export", "INDEX", "archive"}

	if got := crudControllerActions(actions); !reflect.DeepEqual(got, []string{"index", "show"}) {
		t.Fatalf("crud actions: expected [index show], got %v", got)
	}
	if got := nonCRUDControllerActions(
		actions,
	); !reflect.DeepEqual(
		got,
		[]string{"export", "archive"},
	) {
		t.Fatalf("custom actions: expected [export archive], got %v", got)
	}
}

func TestGenerateViewCallsTemplGenerate(t *testing.T) {
	resetCLITestSeams(t)
	var got []string
	runTemplFunc = func(args ...string) error {
		got = append([]string(nil), args...)
		return nil
	}

	result := executeCLITest(t, "sync", "views")
	if result.err != nil {
		t.Fatalf("generate view failed: %v", result.err)
	}
	if !reflect.DeepEqual(got, []string{"generate", "-path", "./views"}) {
		t.Fatalf("templ args: expected [generate -path ./views], got %v", got)
	}
}
