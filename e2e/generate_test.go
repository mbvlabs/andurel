package e2e

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/e2e/internal"
	"github.com/mbvlabs/andurel/pkg/constants"
)

var updateGenerateGolden = flag.Bool(
	"update-generate-golden",
	false,
	"update generate command golden files",
)

func TestGenerateCommands(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E generate test in short mode")
	}

	binary := buildAndurelBinary(t)

	testCases := []struct {
		name     string
		database string
		css      string
		critical bool
	}{
		{
			name:     "postgresql-tailwind",
			database: "postgresql",
			css:      "tailwind",
			critical: true,
		},
		{
			name:     "postgresql-vanilla",
			database: "postgresql",
			css:      "vanilla",
			critical: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if isCriticalOnly() && !tc.critical {
				t.Skip("Skipping non-critical test in critical-only mode")
			}

			t.Parallel()

			project := internal.NewProjectWithDatabase(t, binary, tc.database)

			err := project.Scaffold("-c", tc.css)
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

			t.Run("generate_resource_with_table_name_override", func(t *testing.T) {
				testGenerateResourceWithTableNameOverride(t, project)
			})

			t.Run("generate_model_with_factory", func(t *testing.T) {
				testGenerateModelWithFactory(t, project)
			})

			t.Run("generate_model_skip_factory", func(t *testing.T) {
				testGenerateModelSkipFactory(t, project)
			})
		})
	}
}

func testGenerateModel(t *testing.T, project *internal.Project) {
	t.Helper()

	createMigration(t, project, "000100_create_products", "products", []string{
		"name VARCHAR(255) NOT NULL",
		"price DECIMAL(10,2)",
	})

	err := project.Generate("generate", "model", "Product")
	internal.AssertCommandSucceeds(t, err, "generate model")

	// Verify model file exists and compare against golden file
	internal.AssertFileExists(t, project, "models/product.go")
	modelContent, err := os.ReadFile(filepath.Join(project.Dir, "models/product.go"))
	if err != nil {
		t.Fatalf("Failed to read model file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "product_model.golden"),
		string(modelContent),
	)

	// Verify queries file exists and compare against golden file
	internal.AssertFileExists(t, project, "database/queries/products.sql")
	queriesContent, err := os.ReadFile(filepath.Join(project.Dir, "database/queries/products.sql"))
	if err != nil {
		t.Fatalf("Failed to read queries file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "product_queries.golden"),
		string(queriesContent),
	)

	// Verify factory file exists and compare against golden file
	internal.AssertFileExists(t, project, "models/factories/product.go")
	factoryContent, err := os.ReadFile(filepath.Join(project.Dir, "models/factories/product.go"))
	if err != nil {
		t.Fatalf("Failed to read factory file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "product_factory.golden"),
		string(factoryContent),
	)
}

func testGenerateController(t *testing.T, project *internal.Project) {
	t.Helper()

	createMigration(t, project, "000101_create_orders", "orders", []string{
		"customer_name VARCHAR(255) NOT NULL",
		"total DECIMAL(10,2)",
	})

	err := project.Generate("generate", "model", "Order")
	internal.AssertCommandSucceeds(t, err, "generate model")

	err = project.Generate("generate", "controller", "Order", "--with-views")
	internal.AssertCommandSucceeds(t, err, "generate controller")

	// Verify controller file exists and compare against golden file
	internal.AssertFileExists(t, project, "controllers/orders.go")
	controllerContent, err := os.ReadFile(filepath.Join(project.Dir, "controllers/orders.go"))
	if err != nil {
		t.Fatalf("Failed to read controller file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "order_controller.golden"),
		string(controllerContent),
	)

	// Verify view file exists and compare against golden file (--with-views was passed)
	internal.AssertFileExists(t, project, "views/orders_resource.templ")
	viewContent, err := os.ReadFile(filepath.Join(project.Dir, "views/orders_resource.templ"))
	if err != nil {
		t.Fatalf("Failed to read view file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "order_view.golden"),
		string(viewContent),
	)

	// Verify routes file exists and compare against golden file
	internal.AssertFileExists(t, project, "router/routes/orders.go")
	routesContent, err := os.ReadFile(filepath.Join(project.Dir, "router/routes/orders.go"))
	if err != nil {
		t.Fatalf("Failed to read routes file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "order_routes.golden"),
		string(routesContent),
	)
}

func testGenerateView(t *testing.T, project *internal.Project) {
	t.Helper()

	createMigration(t, project, "000102_create_categories", "categories", []string{
		"name VARCHAR(255) NOT NULL",
		"description TEXT",
	})

	err := project.Generate("generate", "model", "Category")
	internal.AssertCommandSucceeds(t, err, "generate model")

	err = project.Generate("generate", "view", "Category")
	internal.AssertCommandSucceeds(t, err, "generate view")

	// Verify view file exists and compare against golden file
	internal.AssertFileExists(t, project, "views/categories_resource.templ")
	viewContent, err := os.ReadFile(filepath.Join(project.Dir, "views/categories_resource.templ"))
	if err != nil {
		t.Fatalf("Failed to read view file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "category_view.golden"),
		string(viewContent),
	)

	// Controller file should NOT exist when only generating views
	if project.FileExists("controllers/categories.go") {
		t.Error("Controller file should NOT exist when only generating views")
	}
}

func testGenerateResource(t *testing.T, project *internal.Project) {
	t.Helper()

	createMigration(t, project, "000103_create_items", "items", []string{
		"name VARCHAR(255) NOT NULL",
		"quantity INTEGER",
	})

	err := project.Generate("generate", "resource", "Item")
	internal.AssertCommandSucceeds(t, err, "generate resource")

	// Verify model file exists and compare against golden file
	internal.AssertFileExists(t, project, "models/item.go")
	modelContent, err := os.ReadFile(filepath.Join(project.Dir, "models/item.go"))
	if err != nil {
		t.Fatalf("Failed to read model file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "item_model.golden"),
		string(modelContent),
	)

	// Verify controller file exists and compare against golden file
	internal.AssertFileExists(t, project, "controllers/items.go")
	controllerContent, err := os.ReadFile(filepath.Join(project.Dir, "controllers/items.go"))
	if err != nil {
		t.Fatalf("Failed to read controller file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "item_controller.golden"),
		string(controllerContent),
	)

	// Verify view file exists and compare against golden file
	internal.AssertFileExists(t, project, "views/items_resource.templ")
	viewContent, err := os.ReadFile(filepath.Join(project.Dir, "views/items_resource.templ"))
	if err != nil {
		t.Fatalf("Failed to read view file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "item_view.golden"),
		string(viewContent),
	)
}

func testGenerateResourceWithTableNameOverride(t *testing.T, project *internal.Project) {
	t.Helper()

	createMigration(t, project, "000104_create_student_feedback", "student_feedback", []string{
		"student_id VARCHAR(255) NOT NULL",
		"feedback TEXT NOT NULL",
		"rating INTEGER",
	})

	err := project.Generate(
		"generate",
		"resource",
		"StudentFeedback",
		"--table-name=student_feedback",
	)
	internal.AssertCommandSucceeds(t, err, "generate resource with table-name override")

	// Verify model file exists and compare against golden file
	internal.AssertFileExists(t, project, "models/student_feedback.go")
	modelContent, err := os.ReadFile(filepath.Join(project.Dir, "models/student_feedback.go"))
	if err != nil {
		t.Fatalf("Failed to read model file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "student_feedback_model.golden"),
		string(modelContent),
	)

	// Verify controller file exists and compare against golden file
	internal.AssertFileExists(t, project, "controllers/student_feedback.go")
	controllerContent, err := os.ReadFile(filepath.Join(project.Dir, "controllers/student_feedback.go"))
	if err != nil {
		t.Fatalf("Failed to read controller file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "student_feedback_controller.golden"),
		string(controllerContent),
	)

	// Verify view file exists and compare against golden file
	internal.AssertFileExists(t, project, "views/student_feedback_resource.templ")
	viewContent, err := os.ReadFile(filepath.Join(project.Dir, "views/student_feedback_resource.templ"))
	if err != nil {
		t.Fatalf("Failed to read view file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "student_feedback_view.golden"),
		string(viewContent),
	)
}

func testGenerateModelWithFactory(t *testing.T, project *internal.Project) {
	t.Helper()

	createMigration(t, project, "000105_create_books", "books", []string{
		"title VARCHAR(255) NOT NULL",
		"author VARCHAR(255) NOT NULL",
		"price DECIMAL(10,2)",
		"pages INTEGER",
	})

	err := project.Generate("generate", "model", "Book")
	internal.AssertCommandSucceeds(t, err, "generate model")

	// Verify model file exists and compare against golden file
	internal.AssertFileExists(t, project, "models/book.go")
	modelContent, err := os.ReadFile(filepath.Join(project.Dir, "models/book.go"))
	if err != nil {
		t.Fatalf("Failed to read model file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "book_model.golden"),
		string(modelContent),
	)

	// Verify queries file exists and compare against golden file
	internal.AssertFileExists(t, project, "database/queries/books.sql")
	queriesContent, err := os.ReadFile(filepath.Join(project.Dir, "database/queries/books.sql"))
	if err != nil {
		t.Fatalf("Failed to read queries file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "book_queries.golden"),
		string(queriesContent),
	)

	// Verify factory file exists and compare against golden file (default behavior)
	internal.AssertFileExists(t, project, "models/factories/book.go")
	factoryContent, err := os.ReadFile(filepath.Join(project.Dir, "models/factories/book.go"))
	if err != nil {
		t.Fatalf("Failed to read factory file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "book_factory.golden"),
		string(factoryContent),
	)
}

func testGenerateModelSkipFactory(t *testing.T, project *internal.Project) {
	t.Helper()

	createMigration(t, project, "000106_create_articles", "articles", []string{
		"title VARCHAR(255) NOT NULL",
		"content TEXT",
		"published BOOLEAN DEFAULT false",
	})

	err := project.Generate("generate", "model", "Article", "--skip-factory")
	internal.AssertCommandSucceeds(t, err, "generate model with --skip-factory")

	// Verify model file exists and compare against golden file
	internal.AssertFileExists(t, project, "models/article.go")
	modelContent, err := os.ReadFile(filepath.Join(project.Dir, "models/article.go"))
	if err != nil {
		t.Fatalf("Failed to read model file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "article_model.golden"),
		string(modelContent),
	)

	// Verify queries file exists and compare against golden file
	internal.AssertFileExists(t, project, "database/queries/articles.sql")
	queriesContent, err := os.ReadFile(filepath.Join(project.Dir, "database/queries/articles.sql"))
	if err != nil {
		t.Fatalf("Failed to read queries file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "article_queries.golden"),
		string(queriesContent),
	)

	// Verify factory file does NOT exist
	if project.FileExists("models/factories/article.go") {
		t.Error("Factory file should NOT exist when using --skip-factory flag")
	}
}

func createMigration(
	t *testing.T,
	project *internal.Project,
	migrationName, tableName string,
	columns []string,
) {
	t.Helper()

	migrationDir := filepath.Join(project.Dir, "database", "migrations")

	var columnDefs []string

	idColumn := "id UUID PRIMARY KEY"
	timestampType := "TIMESTAMP WITH TIME ZONE"
	now := "NOW()"

	columnDefs = append(columnDefs, "\t"+idColumn)
	for _, col := range columns {
		columnDefs = append(columnDefs, "\t"+col)
	}
	columnDefs = append(columnDefs, "\tcreated_at "+timestampType+" DEFAULT "+now)
	columnDefs = append(columnDefs, "\tupdated_at "+timestampType+" DEFAULT "+now)

	upSQL := "CREATE TABLE IF NOT EXISTS " + tableName + " (\n" +
		strings.Join(columnDefs, ",\n") +
		"\n);"

	downSQL := "DROP TABLE IF EXISTS " + tableName + ";"

	gooseMigration := "-- +goose Up\n" + upSQL + "\n\n-- +goose Down\n" + downSQL + "\n"

	migrationFile := filepath.Join(migrationDir, migrationName+".sql")

	err := os.WriteFile(migrationFile, []byte(gooseMigration), 0o644)
	if err != nil {
		t.Fatalf("Failed to create migration file: %v", err)
	}
}

func compareOrUpdateGenerateGolden(t *testing.T, goldenPath, actual string) {
	t.Helper()

	fullGoldenPath := filepath.Join(".", goldenPath)

	if *updateGenerateGolden {
		err := os.MkdirAll(filepath.Dir(fullGoldenPath), constants.DirPermissionDefault)
		if err != nil {
			t.Fatalf("Failed to create golden directory: %v", err)
		}
		err = os.WriteFile(fullGoldenPath, []byte(actual), constants.FilePermissionPrivate)
		if err != nil {
			t.Fatalf("Failed to write golden file: %v", err)
		}
		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	expected, err := os.ReadFile(fullGoldenPath)
	if err != nil {
		t.Fatalf(
			"Failed to read golden file %s: %v\nRun 'go test -update-generate-golden' to create it",
			goldenPath,
			err,
		)
	}

	if string(expected) != actual {
		t.Errorf(
			"Generated code differs from golden file %s.\n\nRun 'go test -update-generate-golden' to update golden files if changes are expected.",
			goldenPath,
		)
	}
}
