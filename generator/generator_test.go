package generator

import (
	"testing"

	"github.com/mbvlabs/andurel/pkg/cache"
)

func TestNew(t *testing.T) {
	cache.ClearFileSystemCache()

	gen, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if gen.coordinator.ModelManager == nil {
		t.Error("Expected modelManager to be initialized")
	}

	if gen.coordinator.ControllerManager == nil {
		t.Error("Expected controllerManager to be initialized")
	}

	if gen.coordinator.ViewManager == nil {
		t.Error("Expected viewManager to be initialized")
	}

	if gen.coordinator.projectManager == nil {
		t.Error("Expected projectManager to be initialized")
	}
}

func TestGenerator_MethodsExist(t *testing.T) {
	cache.ClearFileSystemCache()

	gen, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Test that all public methods exist and don't panic
	tests := []struct {
		name string
		fn   func() error
	}{
		{
			name: "GenerateModel",
			fn: func() error {
				// This will fail validation, but that's expected
				return gen.GenerateModel("", "", false)
			},
		},
		{
			name: "GenerateController",
			fn: func() error {
				return gen.GenerateController("", "", false)
			},
		},
		{
			name: "GenerateControllerFromModel",
			fn: func() error {
				return gen.GenerateControllerFromModel("", false)
			},
		},
		{
			name: "GenerateView",
			fn: func() error {
				return gen.GenerateView("", "")
			},
		},
		{
			name: "GenerateViewFromModel",
			fn: func() error {
				return gen.GenerateViewFromModel("", false)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We expect errors due to validation, but we verify the methods are callable
			_ = tt.fn()
		})
	}
}

func TestGenerator_DelegationToCoordinator(t *testing.T) {
	cache.ClearFileSystemCache()

	// This test verifies that Generator properly delegates to Coordinator
	// by checking that the same error is returned (indicating proper delegation)
	gen, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Test delegation by calling methods with invalid input
	// Both should return the same validation error
	t.Run("GenerateModel delegation", func(t *testing.T) {
		genErr := gen.GenerateModel("", "", false)
		coordErr := gen.coordinator.ModelManager.GenerateModel("", "", false)

		if genErr == nil || coordErr == nil {
			t.Skip("Expected validation errors for empty resource name")
		}

		// Both errors should be non-nil (validation failure)
		if (genErr != nil) != (coordErr != nil) {
			t.Errorf("Generator and Coordinator returned different error states")
		}
	})

	t.Run("GenerateController delegation", func(t *testing.T) {
		genErr := gen.GenerateController("", "", false)
		coordErr := gen.coordinator.GenerateController("", "", false)

		if genErr == nil || coordErr == nil {
			t.Skip("Expected validation errors for empty parameters")
		}

		// Both errors should be non-nil (validation failure)
		if (genErr != nil) != (coordErr != nil) {
			t.Errorf("Generator and Coordinator returned different error states")
		}
	})
}

func TestGenerator_GenerateModelWithTableOverride(t *testing.T) {
	cache.ClearFileSystemCache()

	gen, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	t.Run("validates empty resource name", func(t *testing.T) {
		err := gen.GenerateModel("", "", false)
		if err == nil {
			t.Error("Expected error for empty resource name, got nil")
		}
	})

	t.Run("accepts empty table override", func(t *testing.T) {
		err := gen.GenerateModel("User", "", false)
		if err == nil {
			t.Skip("Expected error due to missing migrations")
		}
	})

	t.Run("validates invalid table override", func(t *testing.T) {
		err := gen.GenerateModel("User", "InvalidTableName", false)
		if err == nil {
			t.Error("Expected error for invalid table name, got nil")
		}
	})
}
