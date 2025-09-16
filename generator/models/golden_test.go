package models

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mbvlabs/andurel/pkg/constants"
	"github.com/sebdah/goldie/v2"
)

func TestCompleteModelGeneration__GoldenFile(t *testing.T) {
	tests := []struct {
		name         string
		referenceDir string
		tableName    string
		resourceName string
		modulePath   string
		databaseType string
	}{
		{
			name:         "Should generate complete model files from user table migration",
			referenceDir: "user",
			tableName:    "users",
			resourceName: "User",
			modulePath:   "bob-new",
			databaseType: "postgresql",
		},
		{
			name:         "Should generate complete model files from user and team table migration",
			referenceDir: "user_team_relation",
			tableName:    "users",
			resourceName: "User",
			modulePath:   "bob-new",
			databaseType: "postgresql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			// Prepare workspace dirs
			modelsDir := filepath.Join(tempDir, "models")
			internalDBDir := filepath.Join(modelsDir, "internal", "db")

			if err := os.MkdirAll(internalDBDir, constants.DirPermissionDefault); err != nil {
				t.Fatalf("Failed to create internal/db directory: %v", err)
			}

			// Ensure module path matches goldens (import uses "bob-new")
			if err := os.WriteFile(
				filepath.Join(tempDir, "go.mod"),
				[]byte("module "+tt.modulePath+"\n\ngo 1.22.0\n"),
				0o644,
			); err != nil {
				t.Fatalf("Failed to write go.mod: %v", err)
			}

			// Resolve golden inputs and outputs
			originalWd, _ := os.Getwd()
			goldenBase := filepath.Join(originalWd, "testdata", tt.referenceDir)
			goldenModelDir := filepath.Join(goldenBase, "model")
			goldenBobDir := filepath.Join(goldenModelDir, "internal", "db")

			// Seed bob-generated file into temp workspace (simulating bob output)
			bobGoldenPath := filepath.Join(goldenBobDir, "user.bob.go")
			bobContent, err := os.ReadFile(bobGoldenPath)
			if err != nil {
				t.Fatalf("Failed to read bob golden file: %v", err)
			}
			bobOutPath := filepath.Join(internalDBDir, "user.bob.go")
			if err := os.WriteFile(bobOutPath, bobContent, 0o644); err != nil {
				t.Fatalf("Failed to write bob file: %v", err)
			}

			// Chdir into workspace for generator relative paths
			oldWd, _ := os.Getwd()
			defer os.Chdir(oldWd)
			os.Chdir(tempDir)

			// Use the models generator to create the wrapper model from bob-generated struct
			gen := NewGenerator(tt.databaseType)
			modelPath := filepath.Join("models", "user.go")
			if err := gen.GenerateModelFromBob(tt.resourceName, tt.tableName, modelPath, tt.modulePath); err != nil {
				t.Fatalf("Failed to generate model from bob: %v", err)
			}

			// Read generated outputs
			generatedModelContent, err := os.ReadFile(modelPath)
			if err != nil {
				t.Fatalf("Failed to read generated model file: %v", err)
			}
			generatedBobContent, err := os.ReadFile(bobOutPath)
			if err != nil {
				t.Fatalf("Failed to read generated bob file: %v", err)
			}

			if err := formatGoFile(modelPath); err != nil {
				t.Fatalf("Failed to fmt generated bob file: %v", err)
			}

			// Assert model wrapper file against golden
			gModel := goldie.New(
				t,
				goldie.WithFixtureDir(goldenModelDir),
				goldie.WithNameSuffix(".go"),
			)
			gModel.Assert(t, "user", generatedModelContent)

			// Assert bob-generated file content against golden for completeness
			gBob := goldie.New(
				t,
				goldie.WithFixtureDir(goldenBobDir),
				goldie.WithNameSuffix(".bob.go"),
			)
			gBob.Assert(t, "user", generatedBobContent)
		})
	}
}

func formatGoFile(filePath string) error {
	cmd := exec.Command("go", "fmt", filePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run go fmt on %s: %w", filePath, err)
	}

	return nil
}
