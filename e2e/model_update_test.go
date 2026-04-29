package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/e2e/internal"
)

func TestModelUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E model update test in short mode")
	}

	binary := buildAndurelBinary(t)

	t.Run("postgresql-tailwind", func(t *testing.T) {
		t.Parallel()

		project := internal.NewProjectWithDatabase(t, binary, getSharedBinDir(), "postgresql")
		project.CSS = "tailwind"

		err := project.Scaffold("-c", "tailwind")
		internal.AssertCommandSucceeds(t, err, "scaffold")

		t.Run("preserves_custom_type_fields_on_update", func(t *testing.T) {
			testModelUpdatePreservesCustomFields(t, project)
		})
	})
}

// testModelUpdatePreservesCustomFields verifies that running `model update`
// after a migration change does not remove fields with custom types (e.g. enums)
// that were manually added to the entity struct.
func testModelUpdatePreservesCustomFields(t *testing.T, project *internal.Project) {
	t.Helper()

	// Step 1: Create initial migration and generate the model.
	createMigration(t, project, "000300_create_widgets", "widgets", []string{
		"name VARCHAR(255) NOT NULL",
		"price DECIMAL(10,2)",
	})

	err := project.Generate("model", "Widget", "create", "--skip-factory")
	internal.AssertCommandSucceeds(t, err, "generate widget model")

	internal.AssertFileExists(t, project, "models/widget.go")

	// Step 2: Inject a custom-typed field (e.g. an enum) into the entity struct.
	// This simulates a developer adding a field whose type is not in the standard
	// Go type set produced by the type mapper.
	modelPath := filepath.Join(project.Dir, "models", "widget.go")
	modelContent, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatalf("failed to read widget model: %v", err)
	}

	// Insert a custom Status field with a non-standard type right after the struct opening.
	// We also add the type definition so the file remains valid Go.
	modelStr := string(modelContent)

	// Add the enum type declaration before the struct.
	modelStr = strings.Replace(
		modelStr,
		"type WidgetEntity struct {",
		"type WidgetStatus string\n\nconst (\n\tWidgetStatusActive   WidgetStatus = \"active\"\n\tWidgetStatusInactive WidgetStatus = \"inactive\"\n)\n\ntype WidgetEntity struct {",
		1,
	)

	// Insert the Status field inside the struct, after the bun.BaseModel line.
	modelStr = strings.Replace(
		modelStr,
		"bun.BaseModel `bun:\"table:widgets\"`",
		"bun.BaseModel `bun:\"table:widgets\"`\n\tStatus        WidgetStatus `bun:\"status\"`",
		1,
	)

	if err := os.WriteFile(modelPath, []byte(modelStr), 0o600); err != nil {
		t.Fatalf("failed to write modified widget model: %v", err)
	}

	// Confirm the custom field is present before update.
	if !strings.Contains(modelStr, "Status") || !strings.Contains(modelStr, "WidgetStatus") {
		t.Fatal("custom Status field was not injected correctly")
	}

	// Step 3: Add a new migration that adds a standard column.
	migrationDir := filepath.Join(project.Dir, "database", "migrations")
	addColumnMigration := "-- +goose Up\nALTER TABLE widgets ADD COLUMN description TEXT;\n\n-- +goose Down\nALTER TABLE widgets DROP COLUMN description;\n"
	if err := os.WriteFile(
		filepath.Join(migrationDir, "000301_add_description_to_widgets.sql"),
		[]byte(addColumnMigration),
		0o644,
	); err != nil {
		t.Fatalf("failed to write alter migration: %v", err)
	}

	// Step 4: Run model update with --yes to bypass the interactive prompt.
	err = project.Generate("model", "Widget", "update", "--yes")
	internal.AssertCommandSucceeds(t, err, "model update")

	// Step 5: Verify the updated model.
	updatedContent, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatalf("failed to read updated widget model: %v", err)
	}
	updated := string(updatedContent)

	// The new standard column from the migration must be present.
	if !strings.Contains(updated, "Description") {
		t.Error("updated model should contain the new Description field from the migration")
	}

	// The custom-typed field must be preserved.
	if !strings.Contains(updated, "Status") {
		t.Error("updated model should preserve the custom Status field")
	}
	if !strings.Contains(updated, "WidgetStatus") {
		t.Error("updated model should preserve the WidgetStatus type on the Status field")
	}

	// The bun tag of the custom field must be preserved.
	if !strings.Contains(updated, `bun:"status"`) {
		t.Error("updated model should preserve the bun tag on the Status field")
	}

	// Original standard fields must still be present.
	if !strings.Contains(updated, "Name") {
		t.Error("updated model should still contain the Name field")
	}
	if !strings.Contains(updated, "Price") {
		t.Error("updated model should still contain the Price field")
	}
}
