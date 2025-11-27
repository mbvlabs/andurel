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
		{
			name:     "sqlite",
			database: "sqlite",
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

			err := project.Scaffold("-d", tc.database, "-c", "vanilla")
			internal.AssertCommandSucceeds(t, err, "scaffold")

			t.Run("create_and_use_migration", func(t *testing.T) {
				testCreateAndUseMigration(t, project)
			})
		})
	}
}

func testCreateAndUseMigration(t *testing.T, project *internal.Project) {
	t.Helper()

	migrationName := "000200_create_users"
	migrationDir := filepath.Join(project.Dir, "database", "migrations")

	var upSQL, downSQL string

	if project.Database == "postgresql" {
		upSQL = `CREATE TABLE IF NOT EXISTS users (
	id UUID PRIMARY KEY,
	email VARCHAR(255) NOT NULL UNIQUE,
	name VARCHAR(255) NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);`
	} else {
		upSQL = `CREATE TABLE IF NOT EXISTS users (
	id TEXT PRIMARY KEY,
	email TEXT NOT NULL UNIQUE,
	name TEXT NOT NULL,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);`
	}

	downSQL = `DROP TABLE IF EXISTS users;`

	gooseMigration := "-- +goose Up\n" + upSQL + "\n\n-- +goose Down\n" + downSQL + "\n"

	migrationFile := filepath.Join(migrationDir, migrationName+".sql")

	err := os.WriteFile(migrationFile, []byte(gooseMigration), 0644)
	if err != nil {
		t.Fatalf("Failed to create migration file: %v", err)
	}

	internal.AssertFileExists(t, project, "database/migrations/"+migrationName+".sql")

	err = project.Generate("generate", "model", "User")
	internal.AssertCommandSucceeds(t, err, "generate model from migration")

	internal.AssertFileExists(t, project, "models/user.go")
	internal.AssertFileExists(t, project, "database/queries/users.sql")
}
