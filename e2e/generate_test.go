package e2e

import (
	"path/filepath"
	"testing"

	"github.com/mbvlabs/andurel/e2e/internal"
)

func TestGenerateCommands(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E generate test in short mode")
	}

	binary := buildAndurelBinary(t)

	testCases := []struct {
		name       string
		database   string
		css        string
		critical   bool
	}{
		{
			name:     "postgresql-tailwind",
			database: "postgresql",
			css:      "tailwind",
			critical: true,
		},
		{
			name:     "sqlite-vanilla",
			database: "sqlite",
			css:      "vanilla",
			critical: true,
		},
		{
			name:     "postgresql-vanilla",
			database: "postgresql",
			css:      "vanilla",
			critical: false,
		},
		{
			name:     "sqlite-tailwind",
			database: "sqlite",
			css:      "tailwind",
			critical: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if isCriticalOnly() && !tc.critical {
				t.Skip("Skipping non-critical test in critical-only mode")
			}

			t.Parallel()

			project := internal.NewProject(t, binary)

			err := project.Scaffold("-d", tc.database, "-c", tc.css)
			internal.AssertCommandSucceeds(t, err, "scaffold")

			t.Run("generate_model", func(t *testing.T) {
				testGenerateModel(t, project)
			})

			t.Run("generate_controller", func(t *testing.T) {
				testGenerateController(t, project)
			})

			t.Run("generate_view", func(t *testing.T) {
				testGenerateView(t, project)
			})

			t.Run("generate_resource", func(t *testing.T) {
				testGenerateResource(t, project)
			})
		})
	}
}

func testGenerateModel(t *testing.T, project *internal.Project) {
	t.Helper()

	createTestMigration(t, project, "create_products")

	err := project.Generate("generate", "model", "Product")
	internal.AssertCommandSucceeds(t, err, "generate model")

	internal.AssertFileExists(t, project, "models/product.go")

	internal.AssertGoVetPasses(t, project)
}

func testGenerateController(t *testing.T, project *internal.Project) {
	t.Helper()

	createTestMigration(t, project, "create_orders")

	err := project.Generate("generate", "model", "Order")
	internal.AssertCommandSucceeds(t, err, "generate model")

	err = project.Generate("generate", "controller", "Order", "--with-views")
	internal.AssertCommandSucceeds(t, err, "generate controller")

	internal.AssertFileExists(t, project, "controllers/orders_controller.go")
	internal.AssertFileExists(t, project, "views/orders/index.templ")
	internal.AssertFileExists(t, project, "views/orders/show.templ")
	internal.AssertFileExists(t, project, "views/orders/new.templ")
	internal.AssertFileExists(t, project, "views/orders/edit.templ")

	internal.AssertGoVetPasses(t, project)
}

func testGenerateView(t *testing.T, project *internal.Project) {
	t.Helper()

	createTestMigration(t, project, "create_categories")

	err := project.Generate("generate", "model", "Category")
	internal.AssertCommandSucceeds(t, err, "generate model")

	err = project.Generate("generate", "view", "Category")
	internal.AssertCommandSucceeds(t, err, "generate view")

	internal.AssertFileExists(t, project, "views/categories/index.templ")
	internal.AssertFileExists(t, project, "views/categories/show.templ")
	internal.AssertFileExists(t, project, "views/categories/new.templ")
	internal.AssertFileExists(t, project, "views/categories/edit.templ")

	internal.AssertGoVetPasses(t, project)
}

func testGenerateResource(t *testing.T, project *internal.Project) {
	t.Helper()

	createTestMigration(t, project, "create_items")

	err := project.Generate("generate", "resource", "Item")
	internal.AssertCommandSucceeds(t, err, "generate resource")

	internal.AssertFileExists(t, project, "models/item.go")
	internal.AssertFileExists(t, project, "controllers/items_controller.go")
	internal.AssertFileExists(t, project, "views/items/index.templ")
	internal.AssertFileExists(t, project, "views/items/show.templ")
	internal.AssertFileExists(t, project, "views/items/new.templ")
	internal.AssertFileExists(t, project, "views/items/edit.templ")

	internal.AssertGoVetPasses(t, project)
}

func createTestMigration(t *testing.T, project *internal.Project, name string) {
	t.Helper()

	migrationDir := filepath.Join(project.Dir, "database", "migrations")

	upSQL := `CREATE TABLE IF NOT EXISTS ` + name + ` (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);`

	downSQL := `DROP TABLE IF EXISTS ` + name + `;`

	err := internal.RunCommand(t, "sh", project.Dir, nil, "-c",
		`cd "`+migrationDir+`" && echo "`+upSQL+`" > 999999_`+name+`.up.sql`)
	internal.AssertCommandSucceeds(t, err, "create migration up")

	err = internal.RunCommand(t, "sh", project.Dir, nil, "-c",
		`cd "`+migrationDir+`" && echo "`+downSQL+`" > 999999_`+name+`.down.sql`)
	internal.AssertCommandSucceeds(t, err, "create migration down")

	err = internal.RunCommand(t, project.BinaryPath, project.Dir,
		[]string{"ANDUREL_TEST_MODE=true"}, "sqlc", "generate")
	internal.AssertCommandSucceeds(t, err, "sqlc generate")
}
