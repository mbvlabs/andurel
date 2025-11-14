package e2e

import (
	"path/filepath"
	"strings"
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

			project := internal.NewProject(t, binary)

			err := project.Scaffold("-d", tc.database, "-c", "vanilla")
			internal.AssertCommandSucceeds(t, err, "scaffold")

			t.Run("create_migration", func(t *testing.T) {
				testCreateMigration(t, project)
			})

			t.Run("sqlc_generate", func(t *testing.T) {
				testSQLCGenerate(t, project)
			})

			t.Run("generate_model_after_migration", func(t *testing.T) {
				testGenerateModelAfterMigration(t, project)
			})
		})
	}
}

func testCreateMigration(t *testing.T, project *internal.Project) {
	t.Helper()

	migrationName := "create_test_table"
	migrationDir := filepath.Join(project.Dir, "database", "migrations")

	upSQL := `CREATE TABLE IF NOT EXISTS test_table (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	email TEXT,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);`

	downSQL := `DROP TABLE IF EXISTS test_table;`

	err := internal.RunCommand(t, "sh", project.Dir, nil, "-c",
		`cd "`+migrationDir+`" && echo "`+upSQL+`" > 999998_`+migrationName+`.up.sql`)
	internal.AssertCommandSucceeds(t, err, "create migration up file")

	err = internal.RunCommand(t, "sh", project.Dir, nil, "-c",
		`cd "`+migrationDir+`" && echo "`+downSQL+`" > 999998_`+migrationName+`.down.sql`)
	internal.AssertCommandSucceeds(t, err, "create migration down file")

	internal.AssertFileExists(t, project, "database/migrations/999998_"+migrationName+".up.sql")
	internal.AssertFileExists(t, project, "database/migrations/999998_"+migrationName+".down.sql")
}

func testSQLCGenerate(t *testing.T, project *internal.Project) {
	t.Helper()

	queriesFile := filepath.Join(project.Dir, "database", "queries", "test_table.sql")
	queries := `-- name: GetTestTable :one
SELECT * FROM test_table WHERE id = ?;

-- name: ListTestTables :many
SELECT * FROM test_table ORDER BY created_at DESC;

-- name: CreateTestTable :one
INSERT INTO test_table (name, email) VALUES (?, ?) RETURNING *;

-- name: UpdateTestTable :one
UPDATE test_table SET name = ?, email = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? RETURNING *;

-- name: DeleteTestTable :exec
DELETE FROM test_table WHERE id = ?;
`

	err := internal.RunCommand(t, "sh", project.Dir, nil, "-c",
		`echo "`+strings.ReplaceAll(queries, `"`, `\"`)+`" > "`+queriesFile+`"`)
	internal.AssertCommandSucceeds(t, err, "create queries file")

	err = internal.RunCommand(t, project.BinaryPath, project.Dir,
		[]string{"ANDUREL_TEST_MODE=true"}, "sqlc", "generate")
	internal.AssertCommandSucceeds(t, err, "sqlc generate")

	internal.AssertFileExists(t, project, "database/db.go")
	internal.AssertFileExists(t, project, "database/models.go")
	internal.AssertFileExists(t, project, "database/test_table.sql.go")

	internal.AssertGoVetPasses(t, project)
}

func testGenerateModelAfterMigration(t *testing.T, project *internal.Project) {
	t.Helper()

	err := project.Generate("generate", "model", "TestTable")
	internal.AssertCommandSucceeds(t, err, "generate model")

	internal.AssertFileExists(t, project, "models/test_table.go")

	internal.AssertGoVetPasses(t, project)
}
