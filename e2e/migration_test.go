package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mbvlabs/andurel/e2e/internal"
)

func TestMigrationWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E migration test in short mode")
	}

	binary := buildAndurelBinary(t)

	testCases := []struct {
		name     string
		database string
		critical bool
	}{
		{
			name:     "postgresql",
			database: "postgresql",
			critical: true,
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

			err := project.Scaffold("-c", "vanilla")
			internal.AssertCommandSucceeds(t, err, "scaffold")

			t.Run("create_and_use_migration", func(t *testing.T) {
				testCreateAndUseMigration(t, project)
			})
		})
	}
}

func testCreateAndUseMigration(t *testing.T, project *internal.Project) {
	t.Helper()

	migrationName := "000200_create_products"
	migrationDir := filepath.Join(project.Dir, "database", "migrations")

	upSQL := `CREATE TABLE IF NOT EXISTS products (
	id UUID PRIMARY KEY,
	name VARCHAR(255) NOT NULL,
	description TEXT,
	price DECIMAL(10, 2) NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);`

	downSQL := `DROP TABLE IF EXISTS products;`

	gooseMigration := "-- +goose Up\n" + upSQL + "\n\n-- +goose Down\n" + downSQL + "\n"

	migrationFile := filepath.Join(migrationDir, migrationName+".sql")

	err := os.WriteFile(migrationFile, []byte(gooseMigration), 0644)
	if err != nil {
		t.Fatalf("Failed to create migration file: %v", err)
	}

	internal.AssertFileExists(t, project, "database/migrations/"+migrationName+".sql")

	err = project.Generate("generate", "model", "Product")
	internal.AssertCommandSucceeds(t, err, "generate model from migration")

	internal.AssertFileExists(t, project, "models/product.go")
	internal.AssertFileExists(t, project, "database/queries/products.sql")
}
