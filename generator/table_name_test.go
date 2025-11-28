package generator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildModelPath(t *testing.T) {
	tests := []struct {
		name         string
		modelsDir    string
		resourceName string
		want         string
	}{
		{
			name:         "simple resource name",
			modelsDir:    "models",
			resourceName: "User",
			want:         filepath.Join("models", "user.go"),
		},
		{
			name:         "compound resource name",
			modelsDir:    "internal/models",
			resourceName: "StudentFeedback",
			want:         filepath.Join("internal/models", "student_feedback.go"),
		},
		{
			name:         "absolute path",
			modelsDir:    "/app/models",
			resourceName: "Order",
			want:         filepath.Join("/app/models", "order.go"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildModelPath(tt.modelsDir, tt.resourceName)
			if got != tt.want {
				t.Errorf("BuildModelPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveTableName(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "resolve_table_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	modelsDir := filepath.Join(tempDir, "models")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		t.Fatalf("Failed to create models dir: %v", err)
	}

	tests := []struct {
		name         string
		resourceName string
		modelContent string
		createModel  bool
		wantTable    string
	}{
		{
			name:         "no model file - uses derived name",
			resourceName: "User",
			createModel:  false,
			wantTable:    "users",
		},
		{
			name:         "model without override - uses derived name",
			resourceName: "Product",
			modelContent: "package models\n\ntype Product struct {\n\tID string\n}\n",
			createModel:  true,
			wantTable:    "products",
		},
		{
			name:         "model with override - uses override",
			resourceName: "StudentFeedback",
			modelContent: "package models\n// STUDENTFEEDBACK_MODEL_TABLE_NAME: student_feedback\n\ntype StudentFeedback struct {\n\tID string\n}\n",
			createModel:  true,
			wantTable:    "student_feedback",
		},
		{
			name:         "compound name with override",
			resourceName: "UserRole",
			modelContent: "package models\n// USERROLE_MODEL_TABLE_NAME: user_role\n\ntype UserRole struct {\n\tUserID string\n\tRoleID string\n}\n",
			createModel:  true,
			wantTable:    "user_role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.createModel {
				modelPath := BuildModelPath(modelsDir, tt.resourceName)
				if err := os.WriteFile(modelPath, []byte(tt.modelContent), 0o644); err != nil {
					t.Fatalf("Failed to write model file: %v", err)
				}
				defer os.Remove(modelPath)
			}

			got := ResolveTableName(modelsDir, tt.resourceName)
			if got != tt.wantTable {
				t.Errorf("ResolveTableName() = %v, want %v", got, tt.wantTable)
			}
		})
	}
}

func TestResolveTableName_OverrideTakesPrecedence(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "resolve_table_precedence")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	modelsDir := filepath.Join(tempDir, "models")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		t.Fatalf("Failed to create models dir: %v", err)
	}

	modelContent := `package models
// STUDENTFEEDBACK_MODEL_TABLE_NAME: student_feedback

type StudentFeedback struct {
	ID string
}
`
	modelPath := BuildModelPath(modelsDir, "StudentFeedback")
	if err := os.WriteFile(modelPath, []byte(modelContent), 0o644); err != nil {
		t.Fatalf("Failed to write model file: %v", err)
	}

	got := ResolveTableName(modelsDir, "StudentFeedback")
	want := "student_feedback"

	if got != want {
		t.Errorf("ResolveTableName() = %v, want %v (override should take precedence over derived 'student_feedbacks')", got, want)
	}

	derivedName := "student_feedbacks"
	if got == derivedName {
		t.Errorf("ResolveTableName() returned derived name %v instead of override %v", derivedName, want)
	}
}
