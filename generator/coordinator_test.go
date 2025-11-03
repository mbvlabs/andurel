package generator

import (
	"os"
	"testing"

	"github.com/mbvlabs/andurel/pkg/cache"
)

func setupTestProject(t *testing.T) (string, func()) {
	t.Helper()
	cache.ClearFileSystemCache()

	tmpDir := t.TempDir()

	goModContent := "module test\n\ngo 1.21\n"
	if err := os.WriteFile(tmpDir+"/go.mod", []byte(goModContent), 0o644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	dbDir := tmpDir + "/database"
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatalf("Failed to create database directory: %v", err)
	}

	sqlcContent := `version: "2"
sql:
  - engine: postgresql
    schema: migrations`
	if err := os.WriteFile(dbDir+"/sqlc.yaml", []byte(sqlcContent), 0o644); err != nil {
		t.Fatalf("Failed to write sqlc.yaml: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	cleanup := func() {
		os.Chdir(originalDir)
		cache.ClearFileSystemCache()
	}

	return tmpDir, cleanup
}

func TestNewCoordinator(t *testing.T) {
	_, cleanup := setupTestProject(t)
	defer cleanup()

	coord, err := NewCoordinator()
	if err != nil {
		t.Fatalf("NewCoordinator() failed: %v", err)
	}

	if coord.ModelManager == nil {
		t.Error("Expected modelManager to be initialized")
	}

	if coord.ControllerManager == nil {
		t.Error("Expected controllerManager to be initialized")
	}

	if coord.ViewManager == nil {
		t.Error("Expected viewManager to be initialized")
	}

	if coord.projectManager == nil {
		t.Error("Expected projectManager to be initialized")
	}
}

func TestCoordinator_ManagersAreProperlyInitialized(t *testing.T) {
	_, cleanup := setupTestProject(t)
	defer cleanup()

	coord, err := NewCoordinator()
	if err != nil {
		t.Fatalf("NewCoordinator() failed: %v", err)
	}

	// Verify that managers have non-nil dependencies
	// This is a structural test to ensure proper initialization

	t.Run("ModelManager has dependencies", func(t *testing.T) {
		if coord.ModelManager.validator == nil {
			t.Error("ModelManager validator should not be nil")
		}
		if coord.ModelManager.fileManager == nil {
			t.Error("ModelManager fileManager should not be nil")
		}
		if coord.ModelManager.modelGenerator == nil {
			t.Error("ModelManager modelGenerator should not be nil")
		}
		if coord.ModelManager.projectManager == nil {
			t.Error("ModelManager projectManager should not be nil")
		}
		if coord.ModelManager.migrationManager == nil {
			t.Error("ModelManager migrationManager should not be nil")
		}
		if coord.ModelManager.config == nil {
			t.Error("ModelManager config should not be nil")
		}
	})

	t.Run("ControllerManager has dependencies", func(t *testing.T) {
		if coord.ControllerManager.validator == nil {
			t.Error("ControllerManager validator should not be nil")
		}
		if coord.ControllerManager.projectManager == nil {
			t.Error("ControllerManager projectManager should not be nil")
		}
		if coord.ControllerManager.migrationManager == nil {
			t.Error("ControllerManager migrationManager should not be nil")
		}
		if coord.ControllerManager.config == nil {
			t.Error("ControllerManager config should not be nil")
		}
	})

	t.Run("ViewManager has dependencies", func(t *testing.T) {
		if coord.ViewManager.validator == nil {
			t.Error("ViewManager validator should not be nil")
		}
		if coord.ViewManager.projectManager == nil {
			t.Error("ViewManager projectManager should not be nil")
		}
		if coord.ViewManager.migrationManager == nil {
			t.Error("ViewManager migrationManager should not be nil")
		}
		if coord.ViewManager.viewGenerator == nil {
			t.Error("ViewManager viewGenerator should not be nil")
		}
		if coord.ViewManager.config == nil {
			t.Error("ViewManager config should not be nil")
		}
	})
}

func TestCoordinator_GenerateModelValidation(t *testing.T) {
	_, cleanup := setupTestProject(t)
	defer cleanup()

	coord, err := NewCoordinator()
	if err != nil {
		t.Fatalf("NewCoordinator() failed: %v", err)
	}

	// Test that validation errors are properly returned via ModelManager
	err = coord.ModelManager.GenerateModel("")
	if err == nil {
		t.Error("Expected validation error for empty resource name")
	}

	err = coord.ModelManager.GenerateModel("invalid-name")
	if err == nil {
		t.Error("Expected validation error for invalid resource name")
	}
}

func TestCoordinator_GenerateControllerValidation(t *testing.T) {
	_, cleanup := setupTestProject(t)
	defer cleanup()

	coord, err := NewCoordinator()
	if err != nil {
		t.Fatalf("NewCoordinator() failed: %v", err)
	}

	// Test that validation errors are properly returned
	err = coord.GenerateController("", "", false)
	if err == nil {
		t.Error("Expected validation error for empty resource name")
	}

	err = coord.GenerateController("invalid-name", "table", false)
	if err == nil {
		t.Error("Expected validation error for invalid resource name")
	}
}

func TestCoordinator_GenerateViewValidation(t *testing.T) {
	_, cleanup := setupTestProject(t)
	defer cleanup()

	coord, err := NewCoordinator()
	if err != nil {
		t.Fatalf("NewCoordinator() failed: %v", err)
	}

	// Test that validation errors are properly returned via ViewManager
	err = coord.ViewManager.GenerateView("", "")
	if err == nil {
		t.Error("Expected validation error for empty resource name")
	}

	err = coord.ViewManager.GenerateView("invalid-name", "table")
	if err == nil {
		t.Error("Expected validation error for invalid resource name")
	}
}

func TestCoordinator_RefreshModelValidation(t *testing.T) {
	_, cleanup := setupTestProject(t)
	defer cleanup()

	coord, err := NewCoordinator()
	if err != nil {
		t.Fatalf("NewCoordinator() failed: %v", err)
	}

	// Test that validation errors are properly returned
	err = coord.ModelManager.RefreshModel("", "")
	if err == nil {
		t.Error("Expected validation error for empty resource name")
	}

	err = coord.ModelManager.RefreshModel("invalid-name", "table")
	if err == nil {
		t.Error("Expected validation error for invalid resource name")
	}
}

func TestCoordinator_RefreshQueriesValidation(t *testing.T) {
	_, cleanup := setupTestProject(t)
	defer cleanup()

	coord, err := NewCoordinator()
	if err != nil {
		t.Fatalf("NewCoordinator() failed: %v", err)
	}

	// Test that validation errors are properly returned
	err = coord.ModelManager.RefreshQueries("", "")
	if err == nil {
		t.Error("Expected validation error for empty resource name")
	}

	err = coord.ModelManager.RefreshQueries("invalid-name", "table")
	if err == nil {
		t.Error("Expected validation error for invalid resource name")
	}
}

func TestCoordinator_RefreshConstructorsValidation(t *testing.T) {
	_, cleanup := setupTestProject(t)
	defer cleanup()

	coord, err := NewCoordinator()
	if err != nil {
		t.Fatalf("NewCoordinator() failed: %v", err)
	}

	// Test that validation errors are properly returned
	err = coord.ModelManager.RefreshConstructors("", "")
	if err == nil {
		t.Error("Expected validation error for empty resource name")
	}

	err = coord.ModelManager.RefreshConstructors("invalid-name", "table")
	if err == nil {
		t.Error("Expected validation error for invalid resource name")
	}
}

func TestCoordinator_GenerateControllerFromModelValidation(t *testing.T) {
	_, cleanup := setupTestProject(t)
	defer cleanup()

	coord, err := NewCoordinator()
	if err != nil {
		t.Fatalf("NewCoordinator() failed: %v", err)
	}

	// Test that validation errors are properly returned
	err = coord.GenerateControllerFromModel("", false)
	if err == nil {
		t.Error("Expected validation error for empty resource name")
	}

	err = coord.GenerateControllerFromModel("invalid-name", false)
	if err == nil {
		t.Error("Expected validation error for invalid resource name")
	}
}

func TestCoordinator_GenerateViewFromModelValidation(t *testing.T) {
	_, cleanup := setupTestProject(t)
	defer cleanup()

	coord, err := NewCoordinator()
	if err != nil {
		t.Fatalf("NewCoordinator() failed: %v", err)
	}

	// Test that validation errors are properly returned via ViewManager
	err = coord.ViewManager.GenerateViewFromModel("", false)
	if err == nil {
		t.Error("Expected validation error for empty resource name")
	}

	err = coord.ViewManager.GenerateViewFromModel("invalid-name", false)
	if err == nil {
		t.Error("Expected validation error for invalid resource name")
	}
}
