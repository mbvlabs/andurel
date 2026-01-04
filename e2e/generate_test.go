package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/e2e/internal"
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
		tc := tc
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

	internal.AssertFileExists(t, project, "models/product.go")
	internal.AssertFileExists(t, project, "database/queries/products.sql")
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

	internal.AssertFileExists(t, project, "controllers/orders.go")
	internal.AssertFileExists(t, project, "views/orders_resource.templ")
	internal.AssertFileExists(t, project, "router/routes/orders.go")
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

	internal.AssertFileExists(t, project, "views/categories_resource.templ")
}

func testGenerateResource(t *testing.T, project *internal.Project) {
	t.Helper()

	createMigration(t, project, "000103_create_items", "items", []string{
		"name VARCHAR(255) NOT NULL",
		"quantity INTEGER",
	})

	err := project.Generate("generate", "resource", "Item")
	internal.AssertCommandSucceeds(t, err, "generate resource")

	internal.AssertFileExists(t, project, "models/item.go")
	internal.AssertFileExists(t, project, "controllers/items.go")
	internal.AssertFileExists(t, project, "views/items_resource.templ")
}

func testGenerateResourceWithTableNameOverride(t *testing.T, project *internal.Project) {
	t.Helper()

	createMigration(t, project, "000104_create_student_feedback", "student_feedback", []string{
		"student_id VARCHAR(255) NOT NULL",
		"feedback TEXT NOT NULL",
		"rating INTEGER",
	})

	err := project.Generate("generate", "resource", "StudentFeedback", "--table-name=student_feedback")
	internal.AssertCommandSucceeds(t, err, "generate resource with table-name override")

	internal.AssertFileExists(t, project, "models/student_feedback.go")
	internal.AssertFileExists(t, project, "controllers/student_feedback.go")
	internal.AssertFileExists(t, project, "views/student_feedback_resource.templ")

	modelContent, err := os.ReadFile(filepath.Join(project.Dir, "models/student_feedback.go"))
	if err != nil {
		t.Fatalf("Failed to read model file: %v", err)
	}
	if !strings.Contains(string(modelContent), "STUDENTFEEDBACK_MODEL_TABLE_NAME: student_feedback") {
		t.Errorf("Model file should contain table name override marker")
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

	// Verify model files exist
	internal.AssertFileExists(t, project, "models/book.go")
	internal.AssertFileExists(t, project, "database/queries/books.sql")

	// Verify factory file exists (default behavior)
	internal.AssertFileExists(t, project, "models/factories/book.go")

	// Verify factory content
	factoryContent, err := os.ReadFile(filepath.Join(project.Dir, "models/factories/book.go"))
	if err != nil {
		t.Fatalf("Failed to read factory file: %v", err)
	}

	factoryStr := string(factoryContent)

	// Check for required factory elements
	requiredElements := []string{
		"package factories",
		"type BookFactory struct",
		"type BookOption func(*BookFactory)",
		"func BuildBook(opts ...BookOption) models.Book",
		"func CreateBook(ctx context.Context, exec storage.Executor, opts ...BookOption) (models.Book, error)",
		"func CreateBooks(ctx context.Context, exec storage.Executor, count int, opts ...BookOption) ([]models.Book, error)",
		// Field-specific option functions
		"func WithBooksTitle(value string) BookOption",
		"func WithBooksAuthor(value string) BookOption",
	}

	for _, element := range requiredElements {
		if !strings.Contains(factoryStr, element) {
			t.Errorf("Factory file should contain %q", element)
		}
	}

	// Verify factory uses faker for defaults
	fakerElements := []string{
		"github.com/go-faker/faker/v4",
	}

	for _, element := range fakerElements {
		if !strings.Contains(factoryStr, element) {
			t.Errorf("Factory file should contain faker import: %q", element)
		}
	}
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

	// Verify model files exist
	internal.AssertFileExists(t, project, "models/article.go")
	internal.AssertFileExists(t, project, "database/queries/articles.sql")

	// Verify factory file does NOT exist
	if project.FileExists("models/factories/article.go") {
		t.Error("Factory file should NOT exist when using --skip-factory flag")
	}
}

func createMigration(t *testing.T, project *internal.Project, migrationName, tableName string, columns []string) {
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

	err := os.WriteFile(migrationFile, []byte(gooseMigration), 0644)
	if err != nil {
		t.Fatalf("Failed to create migration file: %v", err)
	}
}
