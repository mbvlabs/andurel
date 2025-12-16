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
