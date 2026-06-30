package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/pkg/cache"
	"github.com/mbvlabs/andurel/pkg/naming"
	"github.com/sebdah/goldie/v2"
)

func TestScaffoldGenerationGoldens(t *testing.T) {
	g := goldie.New(t, goldie.WithFixtureDir(scaffoldGenerationGoldenDir(t)))

	scenarios := []struct {
		name             string
		resourceName     string
		tableName        string
		migrations       string
		skipFactory      bool
		primaryKeyColumn string
		diMode           string
		cssFramework     string
		extensions       []string
		inertia          string
	}{
		{
			name:         "full_crud_manual_tailwind",
			resourceName: "Widget",
			migrations:   "controller_view_generation",
			diMode:       "manual",
			cssFramework: "tailwind",
		},
		{
			name:         "full_crud_uberfx_tailwind",
			resourceName: "Widget",
			migrations:   "controller_view_generation",
			diMode:       "uberfx",
			cssFramework: "tailwind",
		},
		{
			name:         "full_crud_manual_css_components",
			resourceName: "Widget",
			migrations:   "controller_view_generation",
			diMode:       "manual",
			cssFramework: "tailwind",
			extensions:   []string{"css-components"},
		},
		{
			name:         "skip_factory",
			resourceName: "Widget",
			migrations:   "controller_view_generation",
			skipFactory:  true,
			diMode:       "manual",
			cssFramework: "tailwind",
		},
		{
			name:         "table_name_override",
			resourceName: "FeedbackEntry",
			tableName:    "student_feedback",
			migrations:   "scaffold_generation_student_feedback",
			skipFactory:  true,
			diMode:       "manual",
			cssFramework: "tailwind",
		},
		{
			name:         "irregular_plural",
			resourceName: "Company",
			migrations:   "scaffold_generation_companies",
			diMode:       "manual",
			cssFramework: "tailwind",
		},
		{
			name:         "array_fields",
			resourceName: "Document",
			migrations:   "scaffold_generation_documents",
			skipFactory:  true,
			diMode:       "manual",
			cssFramework: "tailwind",
		},
		{
			name:             "custom_primary_key",
			resourceName:     "Warehouse",
			migrations:       "scaffold_generation_warehouses",
			skipFactory:      true,
			primaryKeyColumn: "slug",
			diMode:           "manual",
			cssFramework:     "tailwind",
		},
		{
			name:         "vue",
			resourceName: "Project",
			migrations:   "scaffold_generation_projects",
			skipFactory:  true,
			diMode:       "manual",
			cssFramework: "tailwind",
			inertia:      "vue",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			gen := setupScaffoldGoldenProject(
				t,
				scenario.migrations,
				scenario.diMode,
				scenario.cssFramework,
				scenario.extensions,
				scenario.inertia,
			)

			if err := gen.GenerateScaffold(
				scenario.resourceName,
				"",
				scenario.tableName,
				scenario.skipFactory,
				scenario.primaryKeyColumn,
				scenario.inertia,
			); err != nil {
				t.Fatalf("failed to generate scaffold: %v", err)
			}

			assertScaffoldArtifacts(t, g, scenario.name, scenario.resourceName, scenario.tableName, scenario.skipFactory, scenario.diMode, scenario.inertia)
		})
	}
}

func TestScaffoldGenerationGoldensInertiaProjectDefaultsToTempl(t *testing.T) {
	gen := setupScaffoldGoldenProject(
		t,
		"scaffold_generation_projects",
		"manual",
		"tailwind",
		nil,
		"vue",
	)

	if err := gen.GenerateScaffold("Project", "", "", true, "", ""); err != nil {
		t.Fatalf("failed to generate scaffold: %v", err)
	}

	assertGeneratedFileContains(t, "views/projects_resource.templ", "type ProjectIndex struct")
	assertControllerViewGoldenFileMissing(t, filepath.Join("resources", "js", "Pages", "Project", "Index.vue"))
	assertGeneratedFileContains(t, "controllers/projects.go", "testapp/internal/hypermedia")
	assertGeneratedFileNotContains(t, "controllers/projects.go", "testapp/internal/inertia")
}

func TestScaffoldGenerationNamespaced(t *testing.T) {
	gen := setupScaffoldGoldenProject(
		t,
		"controller_view_generation",
		"manual",
		"tailwind",
		nil,
		"",
	)

	if err := gen.GenerateScaffold("Widget", "admin", "", false, "", ""); err != nil {
		t.Fatalf("failed to generate namespaced scaffold: %v", err)
	}

	assertGeneratedFileContains(t, filepath.Join("models", "widget.go"), "type WidgetEntity struct")
	assertGeneratedFileContains(t, filepath.Join("controllers", "admin", "widgets.go"), "package admin")
	assertGeneratedFileContains(t, filepath.Join("controllers", "admin", "widgets.go"), "views.AdminWidgetIndex")
	assertGeneratedFileContains(t, filepath.Join("router", "routes", "admin_widgets.go"), `"admin.widgets.index"`)
	assertGeneratedFileContains(t, filepath.Join("router", "connect_admin_widgets_routes.go"), `controllers "testapp/controllers/admin"`)
	assertGeneratedFileContains(t, filepath.Join("views", "admin_widgets_resource.templ"), "type AdminWidgetIndex struct")
}

func setupScaffoldGoldenProject(t *testing.T, migrationsFixture, diMode, cssFramework string, extensions []string, inertia string) Generator {
	t.Helper()

	cache.ClearFileSystemCache()
	t.Cleanup(cache.ClearFileSystemCache)

	projectDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Fatalf("failed to restore working directory: %v", err)
		}
	})

	writeControllerViewFixtureFile(t, projectDir, "go.mod", "module testapp\n\ngo 1.26\n")
	writeControllerViewFixtureFile(t, projectDir, "models/model.go", modelNamespaceFixture)
	writeControllerViewFixtureFile(t, projectDir, "bin/templ", "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(filepath.Join(projectDir, "bin", "templ"), 0o755); err != nil {
		t.Fatalf("failed to chmod fake templ binary: %v", err)
	}

	writeControllerViewFixtureFile(t, projectDir, "cmd/app/main.go", manualMainFixture)
	writeControllerViewFixtureFile(t, projectDir, "controllers/controller.go", fxControllerModuleFixture)

	lock := layout.NewAndurelLock("test")
	lock.DatabaseConfig = &layout.DatabaseConfig{NullType: "sql.Null"}
	lock.ScaffoldConfig = &layout.ScaffoldConfig{
		ProjectName:  "testapp",
		Database:     "postgresql",
		CSSFramework: cssFramework,
		DIMode:       diMode,
		Inertia:      inertia,
	}
	for _, ext := range extensions {
		lock.AddExtension(ext, "test-applied-at")
	}
	if err := lock.WriteLockFile(projectDir); err != nil {
		t.Fatalf("failed to write andurel.lock: %v", err)
	}

	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("failed to enter temp project: %v", err)
	}

	gen, err := New()
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}
	gen.coordinator.config.Database.MigrationDirs = []string{
		scaffoldGenerationFixtureDir(t, migrationsFixture),
	}

	return gen
}

func assertScaffoldArtifacts(t *testing.T, g *goldie.Goldie, fixtureDir, resourceName, tableName string, skipFactory bool, diMode string, inertia string) {
	t.Helper()

	if tableName == "" {
		tableName = naming.DeriveTableName(resourceName)
	}
	modelFile := naming.ToSnakeCase(resourceName) + ".go"

	paths := []string{
		"models/model.go",
		filepath.Join("models", modelFile),
		filepath.Join("controllers", tableName+".go"),
		filepath.Join("router", "routes", tableName+".go"),
	}

	if skipFactory {
		paths = append(paths, "!"+filepath.Join("models", "factories", modelFile))
	} else {
		paths = append(paths, filepath.Join("models", "factories", modelFile))
	}

	if diMode == "manual" {
		paths = append(paths, filepath.Join("router", "connect_"+tableName+"_routes.go"), filepath.Join("cmd", "app", "main.go"))
	} else {
		paths = append(paths, filepath.Join("controllers", "controller.go"))
	}

	if inertia == "vue" {
		paths = append(paths,
			"!"+filepath.Join("views", tableName+"_resource.templ"),
			filepath.Join("resources", "js", "Pages", resourceName, "Index.vue"),
			filepath.Join("resources", "js", "Pages", resourceName, "Show.vue"),
			filepath.Join("resources", "js", "Pages", resourceName, "Create.vue"),
			filepath.Join("resources", "js", "Pages", resourceName, "Edit.vue"),
		)
	} else {
		paths = append(paths, filepath.Join("views", tableName+"_resource.templ"))
	}

	for _, path := range paths {
		if path[0] == '!' {
			assertControllerViewGoldenFileMissing(t, path[1:])
			continue
		}

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read generated artifact %s: %v", path, err)
		}
		g.Assert(t, filepath.Join(fixtureDir, path), content)
	}
}

func scaffoldGenerationFixtureDir(t *testing.T, name string) string {
	t.Helper()

	return filepath.Join(generatorPackageDir(t), "testdata", "migrations", name)
}

func scaffoldGenerationGoldenDir(t *testing.T) string {
	t.Helper()

	return filepath.Join(generatorPackageDir(t), "testdata", "golden", "scaffold")
}
