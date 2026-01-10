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

	// Verify model file exists and has correct content
	internal.AssertFileExists(t, project, "models/product.go")
	modelContent, err := os.ReadFile(filepath.Join(project.Dir, "models/product.go"))
	if err != nil {
		t.Fatalf("Failed to read model file: %v", err)
	}

	modelStr := string(modelContent)
	modelRequiredElements := []string{
		"package models",
		// Struct definition
		"type Product struct",
		"Name",
		"Price",
		"CreatedAt",
		"UpdatedAt",
		// CRUD functions
		"func FindProduct(",
		"func CreateProduct(",
		"func UpdateProduct(",
		"func DestroyProduct(",
		// Query functions
		"func AllProducts(",
		"func PaginateProducts(",
		"func UpsertProduct(",
		// Data structs
		"type CreateProductData struct",
		"type UpdateProductData struct",
		"type PaginatedProducts struct",
	}

	for _, element := range modelRequiredElements {
		if !strings.Contains(modelStr, element) {
			t.Errorf("Model file should contain %q", element)
		}
	}

	// Verify queries file exists and has correct content
	internal.AssertFileExists(t, project, "database/queries/products.sql")
	queriesContent, err := os.ReadFile(filepath.Join(project.Dir, "database/queries/products.sql"))
	if err != nil {
		t.Fatalf("Failed to read queries file: %v", err)
	}

	queriesStr := string(queriesContent)
	queriesRequiredElements := []string{
		"-- name: QueryProductByID :one",
		"-- name: QueryProducts :many",
		"-- name: InsertProduct :one",
		"-- name: UpdateProduct :one",
		"-- name: DeleteProduct :exec",
		"from products",
	}

	for _, element := range queriesRequiredElements {
		if !strings.Contains(queriesStr, element) {
			t.Errorf("Queries file should contain %q", element)
		}
	}

	// Verify factory file exists by default
	internal.AssertFileExists(t, project, "models/factories/product.go")
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

	// Verify controller file exists and has correct content
	internal.AssertFileExists(t, project, "controllers/orders.go")
	controllerContent, err := os.ReadFile(filepath.Join(project.Dir, "controllers/orders.go"))
	if err != nil {
		t.Fatalf("Failed to read controller file: %v", err)
	}

	controllerStr := string(controllerContent)
	controllerRequiredElements := []string{
		"package controllers",
		"type Orders struct",
		"func NewOrders",
		"func (r Orders) Index",
		"func (r Orders) Show",
		"func (r Orders) New",
		"func (r Orders) Create",
		"func (r Orders) Edit",
		"func (r Orders) Update",
		"func (r Orders) Destroy",
	}

	for _, element := range controllerRequiredElements {
		if !strings.Contains(controllerStr, element) {
			t.Errorf("Controller file should contain %q", element)
		}
	}

	// Verify view file exists and has correct content (--with-views was passed)
	internal.AssertFileExists(t, project, "views/orders_resource.templ")
	viewContent, err := os.ReadFile(filepath.Join(project.Dir, "views/orders_resource.templ"))
	if err != nil {
		t.Fatalf("Failed to read view file: %v", err)
	}

	viewStr := string(viewContent)
	viewRequiredElements := []string{
		"package views",
		// Template functions
		"templ OrderIndex(",
		"templ OrderShow(",
		"templ OrderNew(",
		"templ OrderEdit(",
		// Model references
		"models.Order",
		"[]models.Order",
		// Form elements
		"OrderForm",
		// Field references
		"CustomerName",
		"Total",
	}

	for _, element := range viewRequiredElements {
		if !strings.Contains(viewStr, element) {
			t.Errorf("View file should contain %q", element)
		}
	}

	// Verify routes file exists and has correct content
	internal.AssertFileExists(t, project, "router/routes/orders.go")
	routesContent, err := os.ReadFile(filepath.Join(project.Dir, "router/routes/orders.go"))
	if err != nil {
		t.Fatalf("Failed to read routes file: %v", err)
	}

	routesStr := string(routesContent)
	routesRequiredElements := []string{
		"package routes",
		"func OrderRoutes(",
		// Route definitions
		"OrderIndex",
		"OrderShow",
		"OrderNew",
		"OrderCreate",
		"OrderEdit",
		"OrderUpdate",
		"OrderDestroy",
		// Path patterns
		"/orders",
		"/orders/{id}",
		"/orders/new",
		"/orders/{id}/edit",
	}

	for _, element := range routesRequiredElements {
		if !strings.Contains(routesStr, element) {
			t.Errorf("Routes file should contain %q", element)
		}
	}
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

	// Verify view file exists and has correct content
	internal.AssertFileExists(t, project, "views/categories_resource.templ")
	viewContent, err := os.ReadFile(filepath.Join(project.Dir, "views/categories_resource.templ"))
	if err != nil {
		t.Fatalf("Failed to read view file: %v", err)
	}

	viewStr := string(viewContent)
	viewRequiredElements := []string{
		"package views",
		// Template functions
		"templ CategoryIndex(",
		"templ CategoryShow(",
		"templ CategoryNew(",
		"templ CategoryEdit(",
		// Model references
		"models.Category",
		"[]models.Category",
		// Form elements
		"CategoryForm",
		// Field references
		"Name",
		"Description",
	}

	for _, element := range viewRequiredElements {
		if !strings.Contains(viewStr, element) {
			t.Errorf("View file should contain %q", element)
		}
	}

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

	// Verify model file exists and has correct content
	internal.AssertFileExists(t, project, "models/item.go")
	modelContent, err := os.ReadFile(filepath.Join(project.Dir, "models/item.go"))
	if err != nil {
		t.Fatalf("Failed to read model file: %v", err)
	}

	modelStr := string(modelContent)
	modelRequiredElements := []string{
		"package models",
		// Struct definition
		"type Item struct",
		"Name",
		"Quantity",
		// CRUD functions
		"func FindItem(",
		"func CreateItem(",
		"func UpdateItem(",
		"func DestroyItem(",
		// Query functions
		"func AllItems(",
		"func PaginateItems(",
	}

	for _, element := range modelRequiredElements {
		if !strings.Contains(modelStr, element) {
			t.Errorf("Model file should contain %q", element)
		}
	}

	// Verify controller file exists and has correct content
	internal.AssertFileExists(t, project, "controllers/items.go")
	controllerContent, err := os.ReadFile(filepath.Join(project.Dir, "controllers/items.go"))
	if err != nil {
		t.Fatalf("Failed to read controller file: %v", err)
	}

	controllerStr := string(controllerContent)
	controllerRequiredElements := []string{
		"package controllers",
		"type Items struct",
		"func NewItems",
	}

	for _, element := range controllerRequiredElements {
		if !strings.Contains(controllerStr, element) {
			t.Errorf("Controller file should contain %q", element)
		}
	}

	// Verify view file exists and has correct content
	internal.AssertFileExists(t, project, "views/items_resource.templ")
	viewContent, err := os.ReadFile(filepath.Join(project.Dir, "views/items_resource.templ"))
	if err != nil {
		t.Fatalf("Failed to read view file: %v", err)
	}

	viewStr := string(viewContent)
	viewRequiredElements := []string{
		"package views",
		"ItemIndex",
		"ItemShow",
		"models.Item",
	}

	for _, element := range viewRequiredElements {
		if !strings.Contains(viewStr, element) {
			t.Errorf("View file should contain %q", element)
		}
	}
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

	internal.AssertFileExists(t, project, "models/student_feedback.go")
	internal.AssertFileExists(t, project, "controllers/student_feedback.go")
	internal.AssertFileExists(t, project, "views/student_feedback_resource.templ")

	modelContent, err := os.ReadFile(filepath.Join(project.Dir, "models/student_feedback.go"))
	if err != nil {
		t.Fatalf("Failed to read model file: %v", err)
	}
	if !strings.Contains(
		string(modelContent),
		"STUDENTFEEDBACK_MODEL_TABLE_NAME: student_feedback",
	) {
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
