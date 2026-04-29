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

	bunModel := func(entityName, table string) string {
		return "package models\n\ntype " + entityName + "Entity struct {\n\tbun.BaseModel `bun:\"table:" + table + "\"`\n\tID string\n}\n"
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
			name:         "model with conventional table - uses derived name",
			resourceName: "Product",
			modelContent: bunModel("Product", "products"),
			createModel:  true,
			wantTable:    "products",
		},
		{
			name:         "model with non-conventional table - uses bun tag",
			resourceName: "StudentFeedback",
			modelContent: bunModel("StudentFeedback", "student_feedback"),
			createModel:  true,
			wantTable:    "student_feedback",
		},
		{
			name:         "compound name with non-conventional table",
			resourceName: "UserRole",
			modelContent: bunModel("UserRole", "user_role"),
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

type StudentFeedbackEntity struct {
	bun.BaseModel ` + "`" + `bun:"table:student_feedback"` + "`" + `
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
		t.Errorf("ResolveTableName() = %v, want %v (bun tag should override derived 'student_feedbacks')", got, want)
	}

	derivedName := "student_feedbacks"
	if got == derivedName {
		t.Errorf("ResolveTableName() returned derived name %v instead of bun tag value %v", derivedName, want)
	}
}
