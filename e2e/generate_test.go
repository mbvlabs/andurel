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

			project := internal.NewProjectWithDatabase(t, binary, getSharedBinDir(), tc.database)

			err := project.Scaffold("-c", tc.css)
			internal.AssertCommandSucceeds(t, err, "scaffold")

			t.Run("generate_model", func(t *testing.T) {
				testGenerateModel(t, project)
			})

			t.Run("generate_model_without_timestamps", func(t *testing.T) {
				testGenerateModelWithoutTimestamps(t, project)
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

			t.Run("generate_model_with_table_name", func(t *testing.T) {
				testGenerateModelWithTableName(t, project)
			})

			t.Run("generate_controller_without_views", func(t *testing.T) {
				testGenerateControllerWithoutViews(t, project)
			})

			t.Run("generate_view_with_controller", func(t *testing.T) {
				testGenerateViewWithController(t, project)
			})

			t.Run("generate_queries", func(t *testing.T) {
				testGenerateQueries(t, project)
			})

			t.Run("generate_queries_with_refresh", func(t *testing.T) {
				testGenerateQueriesWithRefresh(t, project)
			})

			t.Run("generate_model_with_array_types", func(t *testing.T) {
				testGenerateModelWithArrayTypes(t, project)
			})

			t.Run("generate_view_with_array_types", func(t *testing.T) {
				testGenerateViewWithArrayTypes(t, project)
			})

			t.Run("generate_fragment", func(t *testing.T) {
				testGenerateFragment(t, project)
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

func testGenerateModelWithoutTimestamps(t *testing.T, project *internal.Project) {
	t.Helper()

	createMigrationRaw(t, project, "000113_create_server_provision_steps", "server_provision_steps", []string{
		"started_at TIMESTAMP WITH TIME ZONE NOT NULL",
		"completed_at TIMESTAMP WITH TIME ZONE",
		"server_id UUID NOT NULL",
	})

	err := project.Generate("generate", "model", "ServerProvisionStep", "--skip-factory")
	internal.AssertCommandSucceeds(t, err, "generate model without timestamps")

	internal.AssertFileExists(t, project, "models/server_provision_step.go")

	internal.AssertFileExists(t, project, "database/queries/server_provision_steps.sql")
	queriesContent, err := os.ReadFile(filepath.Join(project.Dir, "database/queries/server_provision_steps.sql"))
	if err != nil {
		t.Fatalf("Failed to read queries file: %v", err)
	}

	queriesStr := string(queriesContent)
	if strings.Contains(queriesStr, "order by created_at desc") {
		t.Error("Expected pagination ordering to avoid created_at when column is missing")
	}
	if !strings.Contains(queriesStr, "order by id desc") {
		t.Error("Expected pagination ordering to fall back to id desc")
	}
	if strings.Contains(queriesStr, "now()") {
		t.Error("Expected no now() placeholders without created_at/updated_at columns")
	}
}

func testGenerateController(t *testing.T, project *internal.Project) {
	t.Helper()

	createMigration(t, project, "000101_create_orders", "orders", []string{
		"account_id UUID NOT NULL",
		"customer_name VARCHAR(255) NOT NULL",
		"total DECIMAL(10,2)",
		"signature BYTEA",
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
		"warehouse_id UUID",
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

	// Verify main.go uses camelCase variable name for snake_case table name
	mainContent, err := os.ReadFile(filepath.Join(project.Dir, "cmd", "app", "main.go"))
	if err != nil {
		t.Fatalf("Failed to read cmd/app/main.go: %v", err)
	}

	mainContentStr := string(mainContent)
	requiredPatterns := []string{
		"studentFeedback := controllers.NewStudentFeedback(db)",
		"RegisterStudentFeedbackRoutes(studentFeedback)",
	}
	for _, pattern := range requiredPatterns {
		if !strings.Contains(mainContentStr, pattern) {
			t.Errorf("cmd/app/main.go should contain %q", pattern)
		}
	}
	if strings.Contains(mainContentStr, "student_feedback") {
		t.Error("cmd/app/main.go should not contain snake_case variable names for controller registration")
	}
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

func testGenerateModelWithTableName(t *testing.T, project *internal.Project) {
	t.Helper()

	// Create a table with a name that doesn't follow standard pluralization
	// e.g., "person" model but "people_data" table
	createMigration(t, project, "000107_create_people_data", "people_data", []string{
		"first_name VARCHAR(255) NOT NULL",
		"last_name VARCHAR(255) NOT NULL",
		"email VARCHAR(255)",
	})

	err := project.Generate("generate", "model", "Person", "--table-name=people_data")
	internal.AssertCommandSucceeds(t, err, "generate model with --table-name")

	// Verify model file exists and compare against golden file
	internal.AssertFileExists(t, project, "models/person.go")
	modelContent, err := os.ReadFile(filepath.Join(project.Dir, "models/person.go"))
	if err != nil {
		t.Fatalf("Failed to read model file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "person_model.golden"),
		string(modelContent),
	)

	// Verify queries file exists with custom table name and compare against golden file
	internal.AssertFileExists(t, project, "database/queries/people_data.sql")
	queriesContent, err := os.ReadFile(filepath.Join(project.Dir, "database/queries/people_data.sql"))
	if err != nil {
		t.Fatalf("Failed to read queries file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "person_queries.golden"),
		string(queriesContent),
	)

	// Verify factory file exists and compare against golden file
	internal.AssertFileExists(t, project, "models/factories/person.go")
	factoryContent, err := os.ReadFile(filepath.Join(project.Dir, "models/factories/person.go"))
	if err != nil {
		t.Fatalf("Failed to read factory file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "person_factory.golden"),
		string(factoryContent),
	)
}

func testGenerateControllerWithoutViews(t *testing.T, project *internal.Project) {
	t.Helper()

	createMigration(t, project, "000108_create_invoices", "invoices", []string{
		"invoice_number VARCHAR(50) NOT NULL",
		"amount DECIMAL(10,2) NOT NULL",
		"status VARCHAR(20) DEFAULT 'pending'",
		"pdf_data BYTEA",
	})

	err := project.Generate("generate", "model", "Invoice")
	internal.AssertCommandSucceeds(t, err, "generate model")

	// Generate controller WITHOUT views (default behavior)
	err = project.Generate("generate", "controller", "Invoice")
	internal.AssertCommandSucceeds(t, err, "generate controller without views")

	// Verify controller file exists and compare against golden file
	internal.AssertFileExists(t, project, "controllers/invoices.go")
	controllerContent, err := os.ReadFile(filepath.Join(project.Dir, "controllers/invoices.go"))
	if err != nil {
		t.Fatalf("Failed to read controller file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "invoice_controller.golden"),
		string(controllerContent),
	)

	// Verify routes file exists and compare against golden file
	internal.AssertFileExists(t, project, "router/routes/invoices.go")
	routesContent, err := os.ReadFile(filepath.Join(project.Dir, "router/routes/invoices.go"))
	if err != nil {
		t.Fatalf("Failed to read routes file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "invoice_routes.golden"),
		string(routesContent),
	)

	// Verify router registration file exists and compare against golden file
	internal.AssertFileExists(t, project, "router/connect_invoices_routes.go")
	routerContent, err := os.ReadFile(filepath.Join(project.Dir, "router/connect_invoices_routes.go"))
	if err != nil {
		t.Fatalf("Failed to read router registration file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "invoice_router.golden"),
		string(routerContent),
	)

	// View file should NOT exist when generating controller without views
	if project.FileExists("views/invoices_resource.templ") {
		t.Error("View file should NOT exist when generating controller without --with-views")
	}
}

func testGenerateViewWithController(t *testing.T, project *internal.Project) {
	t.Helper()

	createMigration(t, project, "000109_create_reviews", "reviews", []string{
		"title VARCHAR(255) NOT NULL",
		"body TEXT",
		"rating INTEGER NOT NULL",
	})

	err := project.Generate("generate", "model", "Review")
	internal.AssertCommandSucceeds(t, err, "generate model")

	// Generate view WITH controller
	err = project.Generate("generate", "view", "Review", "--with-controller")
	internal.AssertCommandSucceeds(t, err, "generate view with controller")

	// Verify view file exists and compare against golden file
	internal.AssertFileExists(t, project, "views/reviews_resource.templ")
	viewContent, err := os.ReadFile(filepath.Join(project.Dir, "views/reviews_resource.templ"))
	if err != nil {
		t.Fatalf("Failed to read view file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "review_view.golden"),
		string(viewContent),
	)

	// Verify controller file exists (because --with-controller was passed)
	internal.AssertFileExists(t, project, "controllers/reviews.go")
	controllerContent, err := os.ReadFile(filepath.Join(project.Dir, "controllers/reviews.go"))
	if err != nil {
		t.Fatalf("Failed to read controller file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "review_controller.golden"),
		string(controllerContent),
	)

	// Verify routes file exists
	internal.AssertFileExists(t, project, "router/routes/reviews.go")
	routesContent, err := os.ReadFile(filepath.Join(project.Dir, "router/routes/reviews.go"))
	if err != nil {
		t.Fatalf("Failed to read routes file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "review_routes.golden"),
		string(routesContent),
	)
}

func testGenerateQueries(t *testing.T, project *internal.Project) {
	t.Helper()

	// Create a junction table for testing queries-only generation (no timestamps)
	createMigrationRaw(t, project, "000110_create_user_roles", "user_roles", []string{
		"user_id UUID NOT NULL",
		"role_id UUID NOT NULL",
	})

	err := project.Generate("queries", "generate", "user_roles")
	internal.AssertCommandSucceeds(t, err, "generate queries")

	// Verify queries file exists and compare against golden file
	internal.AssertFileExists(t, project, "database/queries/user_roles.sql")
	queriesContent, err := os.ReadFile(filepath.Join(project.Dir, "database/queries/user_roles.sql"))
	if err != nil {
		t.Fatalf("Failed to read queries file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "user_role_queries.golden"),
		string(queriesContent),
	)

	// Model file should NOT exist for queries-only generation
	if project.FileExists("models/user_role.go") {
		t.Error("Model file should NOT exist for queries-only generation")
	}

	// Factory file should NOT exist for queries-only generation
	if project.FileExists("models/factories/user_role.go") {
		t.Error("Factory file should NOT exist for queries-only generation")
	}
}

func testGenerateQueriesWithRefresh(t *testing.T, project *internal.Project) {
	t.Helper()

	// Create a table for testing refresh functionality (no timestamps)
	createMigrationRaw(t, project, "000111_create_tag_assignments", "tag_assignments", []string{
		"taggable_type VARCHAR(100) NOT NULL",
		"taggable_id UUID NOT NULL",
		"tag_id UUID NOT NULL",
	})

	// First generate the queries
	err := project.Generate("queries", "generate", "tag_assignments")
	internal.AssertCommandSucceeds(t, err, "generate queries")

	// Verify queries file exists and compare against golden file
	internal.AssertFileExists(t, project, "database/queries/tag_assignments.sql")
	queriesContent, err := os.ReadFile(filepath.Join(project.Dir, "database/queries/tag_assignments.sql"))
	if err != nil {
		t.Fatalf("Failed to read queries file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "tag_assignment_queries.golden"),
		string(queriesContent),
	)

	// Model file should NOT exist for queries-only generation
	if project.FileExists("models/tag_assignment.go") {
		t.Error("Model file should NOT exist for queries-only generation")
	}

	// Now test refresh functionality
	err = project.Generate("queries", "refresh", "tag_assignments")
	internal.AssertCommandSucceeds(t, err, "queries refresh")
}

// testGenerateModelWithArrayTypes tests that PostgreSQL array types (text[], integer[])
// are correctly generated as native Go slices ([]string, []int32) instead of
// non-existent pgtype.Array types. Also tests jsonb/json types.
func testGenerateModelWithArrayTypes(t *testing.T, project *internal.Project) {
	t.Helper()

	// Create migration with array types directly using raw SQL
	migrationDir := filepath.Join(project.Dir, "database", "migrations")
	migrationContent := `-- +goose Up
CREATE TABLE IF NOT EXISTS posts (
	id UUID PRIMARY KEY,
	title VARCHAR(255) NOT NULL,
	tags TEXT[] NOT NULL,
	scores INTEGER[] NOT NULL,
	settings JSONB NOT NULL,
	metadata JSON,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS posts;
`
	migrationFile := filepath.Join(migrationDir, "000112_create_posts_with_arrays.sql")
	err := os.WriteFile(migrationFile, []byte(migrationContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create migration file: %v", err)
	}

	err = project.Generate("generate", "model", "Post", "--skip-factory")
	internal.AssertCommandSucceeds(t, err, "generate model with array types")

	// Verify model file exists and compare against golden file
	internal.AssertFileExists(t, project, "models/post.go")
	modelContent, err := os.ReadFile(filepath.Join(project.Dir, "models/post.go"))
	if err != nil {
		t.Fatalf("Failed to read model file: %v", err)
	}

	// Verify array types are generated correctly as native Go slices
	modelStr := string(modelContent)
	if strings.Contains(modelStr, "pgtype.Array") {
		t.Error("Model should NOT contain pgtype.Array - array types should use native Go slices")
	}

	// Verify []string is used for text[] column (struct field has tabs/spaces)
	if !strings.Contains(modelStr, "Tags") || !strings.Contains(modelStr, "[]string") {
		t.Error("Expected 'Tags []string' in model for text[] column")
	}

	// Verify []int32 is used for integer[] column
	if !strings.Contains(modelStr, "Scores") || !strings.Contains(modelStr, "[]int32") {
		t.Error("Expected 'Scores []int32' in model for integer[] column")
	}

	// Verify []byte is used for jsonb/json columns
	if !strings.Contains(modelStr, "Settings") || !strings.Contains(modelStr, "[]byte") {
		t.Error("Expected 'Settings []byte' in model for jsonb column")
	}
	if !strings.Contains(modelStr, "Metadata") {
		t.Error("Expected 'Metadata []byte' in model for json column")
	}

	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "post_model.golden"),
		modelStr,
	)

	// Verify queries file exists and compare against golden file
	internal.AssertFileExists(t, project, "database/queries/posts.sql")
	queriesContent, err := os.ReadFile(filepath.Join(project.Dir, "database/queries/posts.sql"))
	if err != nil {
		t.Fatalf("Failed to read queries file: %v", err)
	}
	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "post_queries.golden"),
		string(queriesContent),
	)
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

// createMigrationRaw creates a migration file without auto-adding created_at/updated_at columns.
// Use this for tables that don't have timestamps (e.g., junction tables).
func createMigrationRaw(
	t *testing.T,
	project *internal.Project,
	migrationName, tableName string,
	columns []string,
) {
	t.Helper()

	migrationDir := filepath.Join(project.Dir, "database", "migrations")

	var columnDefs []string

	idColumn := "id UUID PRIMARY KEY"

	columnDefs = append(columnDefs, "\t"+idColumn)
	for _, col := range columns {
		columnDefs = append(columnDefs, "\t"+col)
	}

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

// testGenerateViewWithArrayTypes tests that views with array types (text[], integer[])
// correctly use string converters (strings.Join, fmt.Sprintf) to display array values.
func testGenerateViewWithArrayTypes(t *testing.T, project *internal.Project) {
	t.Helper()

	// Create migration with array types
	migrationDir := filepath.Join(project.Dir, "database", "migrations")
	migrationContent := `-- +goose Up
CREATE TABLE IF NOT EXISTS documents (
	id UUID PRIMARY KEY,
	title VARCHAR(255) NOT NULL,
	tags TEXT[] NOT NULL,
	page_numbers INTEGER[] NOT NULL,
	view_count INTEGER NOT NULL,
	is_published BOOLEAN DEFAULT false,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS documents;
`
	migrationFile := filepath.Join(migrationDir, "000113_create_documents_with_arrays.sql")
	err := os.WriteFile(migrationFile, []byte(migrationContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create migration file: %v", err)
	}

	// Generate model first (required for view generation)
	err = project.Generate("generate", "model", "Document", "--skip-factory")
	internal.AssertCommandSucceeds(t, err, "generate model with array types")

	// Generate view
	err = project.Generate("generate", "view", "Document")
	internal.AssertCommandSucceeds(t, err, "generate view with array types")

	// Verify view file exists
	internal.AssertFileExists(t, project, "views/documents_resource.templ")
	viewContent, err := os.ReadFile(filepath.Join(project.Dir, "views/documents_resource.templ"))
	if err != nil {
		t.Fatalf("Failed to read view file: %v", err)
	}

	viewStr := string(viewContent)

	// Verify strings.Join is used for []string (text[]) column
	if !strings.Contains(viewStr, "strings.Join(document.Tags") {
		t.Error("View should use strings.Join for text[] column (Tags)")
	}

	// Verify fmt.Sprintf is used for []int32 (integer[]) column
	if !strings.Contains(viewStr, "fmt.Sprintf") || !strings.Contains(viewStr, "document.PageNumbers") {
		t.Error("View should use fmt.Sprintf for integer[] column (PageNumbers)")
	}

	// Verify fmt.Sprintf is used for integer column
	if !strings.Contains(viewStr, "fmt.Sprintf") || !strings.Contains(viewStr, "document.ViewCount") {
		t.Error("View should use fmt.Sprintf for integer column (ViewCount)")
	}

	// Verify fmt.Sprintf with %t is used for boolean column
	if !strings.Contains(viewStr, "fmt.Sprintf") || !strings.Contains(viewStr, "document.IsPublished") {
		t.Error("View should use fmt.Sprintf for boolean column (IsPublished)")
	}

	// Verify imports include both fmt and strings
	if !strings.Contains(viewStr, `"fmt"`) {
		t.Error("View should import fmt package")
	}
	if !strings.Contains(viewStr, `"strings"`) {
		t.Error("View should import strings package")
	}

	compareOrUpdateGenerateGolden(
		t,
		filepath.Join("testdata", "golden", "generate", "document_view.golden"),
		viewStr,
	)
}

// testGenerateFragment tests the fragment generation command which adds
// a method stub, route variable, and route registration to an existing controller.
func testGenerateFragment(t *testing.T, project *internal.Project) {
	t.Helper()

	// Create a table for the Webhook controller
	createMigration(t, project, "000114_create_webhooks", "webhooks", []string{
		"endpoint VARCHAR(255) NOT NULL",
		"secret VARCHAR(255) NOT NULL",
		"active BOOLEAN DEFAULT true",
	})

	// Generate model first
	err := project.Generate("generate", "model", "Webhook", "--skip-factory")
	internal.AssertCommandSucceeds(t, err, "generate model for webhook")

	// Generate controller (without views since we just need the controller files)
	err = project.Generate("generate", "controller", "Webhook")
	internal.AssertCommandSucceeds(t, err, "generate controller for webhook")

	// Verify controller, routes, and connect files exist before fragment generation
	internal.AssertFileExists(t, project, "controllers/webhooks.go")
	internal.AssertFileExists(t, project, "router/routes/webhooks.go")
	internal.AssertFileExists(t, project, "router/connect_webhooks_routes.go")

	// Test 1: Generate a simple fragment with default GET method
	err = project.Generate("generate", "fragment", "Webhook", "Ping", "/ping")
	internal.AssertCommandSucceeds(t, err, "generate fragment Webhook Ping")

	// Verify controller has the new method
	controllerContent, err := os.ReadFile(filepath.Join(project.Dir, "controllers/webhooks.go"))
	if err != nil {
		t.Fatalf("Failed to read controller file: %v", err)
	}
	controllerStr := string(controllerContent)
	if !strings.Contains(controllerStr, "func (w Webhooks) Ping(etx *echo.Context) error") {
		t.Error("Controller should contain Ping method")
	}

	// Verify routes file has the new route variable
	routesContent, err := os.ReadFile(filepath.Join(project.Dir, "router/routes/webhooks.go"))
	if err != nil {
		t.Fatalf("Failed to read routes file: %v", err)
	}
	routesStr := string(routesContent)
	if !strings.Contains(routesStr, "var WebhookPing = routing.NewSimpleRoute") {
		t.Error("Routes file should contain WebhookPing route variable")
	}
	if !strings.Contains(routesStr, `"/ping"`) {
		t.Error("Routes file should contain /ping path")
	}

	// Verify connect file has the new route registration
	connectContent, err := os.ReadFile(filepath.Join(project.Dir, "router/connect_webhooks_routes.go"))
	if err != nil {
		t.Fatalf("Failed to read connect file: %v", err)
	}
	connectStr := string(connectContent)
	if !strings.Contains(connectStr, "routes.WebhookPing.Path()") {
		t.Error("Connect file should contain WebhookPing route registration")
	}
	if !strings.Contains(connectStr, "webhook.Ping") {
		t.Error("Connect file should contain webhook.Ping handler")
	}
	if !strings.Contains(connectStr, "http.MethodGet") {
		t.Error("Connect file should use http.MethodGet for default method")
	}

	// Test 2: Generate a fragment with POST method and :id parameter
	err = project.Generate("generate", "fragment", "Webhook", "Verify", "/:id/verify", "--method", "POST")
	internal.AssertCommandSucceeds(t, err, "generate fragment Webhook Verify")

	// Verify controller has the Verify method
	controllerContent, err = os.ReadFile(filepath.Join(project.Dir, "controllers/webhooks.go"))
	if err != nil {
		t.Fatalf("Failed to read controller file: %v", err)
	}
	controllerStr = string(controllerContent)
	if !strings.Contains(controllerStr, "func (w Webhooks) Verify(etx *echo.Context) error") {
		t.Error("Controller should contain Verify method")
	}

	// Verify routes file has the new route variable with ID constructor
	routesContent, err = os.ReadFile(filepath.Join(project.Dir, "router/routes/webhooks.go"))
	if err != nil {
		t.Fatalf("Failed to read routes file: %v", err)
	}
	routesStr = string(routesContent)
	if !strings.Contains(routesStr, "var WebhookVerify = routing.NewRouteWithID") {
		t.Error("Routes file should contain WebhookVerify route variable with NewRouteWithID")
	}
	if !strings.Contains(routesStr, `"/:id/verify"`) {
		t.Error("Routes file should contain /:id/verify path")
	}

	// Verify connect file has the Verify route registration with POST method
	connectContent, err = os.ReadFile(filepath.Join(project.Dir, "router/connect_webhooks_routes.go"))
	if err != nil {
		t.Fatalf("Failed to read connect file: %v", err)
	}
	connectStr = string(connectContent)
	if !strings.Contains(connectStr, "routes.WebhookVerify.Path()") {
		t.Error("Connect file should contain WebhookVerify route registration")
	}
	if !strings.Contains(connectStr, "webhook.Verify") {
		t.Error("Connect file should contain webhook.Verify handler")
	}
	if !strings.Contains(connectStr, "http.MethodPost") {
		t.Error("Connect file should use http.MethodPost for POST method")
	}

	// Test 3: Verify duplicate detection - running the same fragment again should fail
	err = project.GenerateExpectError("generate", "fragment", "Webhook", "Ping", "/ping")
	if err == nil {
		t.Error("Expected error when generating duplicate fragment, but got none")
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
