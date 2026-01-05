package models

import (
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/generator/templates"
)

func TestFactoryGeneration_MultipleForeignKeys(t *testing.T) {
	tests := []struct {
		name           string
		tableName      string
		resourceName   string
		columns        []*catalog.Column
		wantFKCount    int
		wantSigParts   []string // parts that should appear in CreateXXXs signature
		unwantSigParts []string // parts that should NOT appear
	}{
		{
			name:         "no foreign keys",
			tableName:    "posts",
			resourceName: "Post",
			columns: []*catalog.Column{
				{Name: "id", DataType: "uuid", IsPrimaryKey: true, IsNullable: false},
				{Name: "created_at", DataType: "timestamptz", IsNullable: false},
				{Name: "updated_at", DataType: "timestamptz", IsNullable: false},
				{Name: "title", DataType: "varchar", IsNullable: false},
			},
			wantFKCount: 0,
			wantSigParts: []string{
				"func CreatePosts(ctx context.Context, exec storage.Executor, count int, opts ...PostOption)",
			},
			unwantSigParts: []string{"map[", "uuid.UUID, count"},
		},
		{
			name:         "single foreign key",
			tableName:    "posts",
			resourceName: "Post",
			columns: []*catalog.Column{
				{Name: "id", DataType: "uuid", IsPrimaryKey: true, IsNullable: false},
				{Name: "created_at", DataType: "timestamptz", IsNullable: false},
				{Name: "updated_at", DataType: "timestamptz", IsNullable: false},
				{Name: "title", DataType: "varchar", IsNullable: false},
				{Name: "author_id", DataType: "uuid", IsNullable: false, ForeignKey: &catalog.ForeignKey{ReferencedTable: "users", ReferencedColumn: "id"}},
			},
			wantFKCount: 1,
			wantSigParts: []string{
				"func CreatePosts(ctx context.Context, exec storage.Executor, authorID uuid.UUID, count int, opts ...PostOption)",
				"CreatePost(ctx, exec, authorID, opts...)",
			},
			unwantSigParts: []string{"counts map["},
		},
		{
			name:         "multiple foreign keys",
			tableName:    "student_comments",
			resourceName: "StudentComment",
			columns: []*catalog.Column{
				{Name: "id", DataType: "uuid", IsPrimaryKey: true, IsNullable: false},
				{Name: "created_at", DataType: "timestamptz", IsNullable: false},
				{Name: "updated_at", DataType: "timestamptz", IsNullable: false},
				{Name: "comment", DataType: "text", IsNullable: false},
				{Name: "student_id", DataType: "uuid", IsNullable: false, ForeignKey: &catalog.ForeignKey{ReferencedTable: "students", ReferencedColumn: "id"}},
				{Name: "teacher_id", DataType: "uuid", IsNullable: false, ForeignKey: &catalog.ForeignKey{ReferencedTable: "teachers", ReferencedColumn: "id"}},
			},
			wantFKCount: 2,
			wantSigParts: []string{
				"func CreateStudentComments(ctx context.Context, exec storage.Executor, studentID uuid.UUID, teacherID uuid.UUID, count int, opts ...StudentCommentOption)",
				"CreateStudentComment(ctx, exec, studentID, teacherID, opts...)",
			},
			unwantSigParts: []string{"counts map[", "for studentID, count := range"},
		},
		{
			name:         "three foreign keys",
			tableName:    "assignments",
			resourceName: "Assignment",
			columns: []*catalog.Column{
				{Name: "id", DataType: "uuid", IsPrimaryKey: true, IsNullable: false},
				{Name: "created_at", DataType: "timestamptz", IsNullable: false},
				{Name: "updated_at", DataType: "timestamptz", IsNullable: false},
				{Name: "student_id", DataType: "uuid", IsNullable: false, ForeignKey: &catalog.ForeignKey{ReferencedTable: "students", ReferencedColumn: "id"}},
				{Name: "teacher_id", DataType: "uuid", IsNullable: false, ForeignKey: &catalog.ForeignKey{ReferencedTable: "teachers", ReferencedColumn: "id"}},
				{Name: "course_id", DataType: "uuid", IsNullable: false, ForeignKey: &catalog.ForeignKey{ReferencedTable: "courses", ReferencedColumn: "id"}},
			},
			wantFKCount: 3,
			wantSigParts: []string{
				"func CreateAssignments(ctx context.Context, exec storage.Executor, studentID uuid.UUID, teacherID uuid.UUID, courseID uuid.UUID, count int, opts ...AssignmentOption)",
				"CreateAssignment(ctx, exec, studentID, teacherID, courseID, opts...)",
			},
			unwantSigParts: []string{"counts map["},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			cat := catalog.NewCatalog("public")
			table := &catalog.Table{
				Name:    tt.tableName,
				Columns: tt.columns,
			}
			if err := cat.AddTable("", table); err != nil {
				t.Fatalf("Failed to add table: %v", err)
			}

			gen := NewGenerator("postgresql")
			config := Config{
				TableName:    tt.tableName,
				ResourceName: tt.resourceName,
				PackageName:  "models",
				DatabaseType: "postgresql",
				ModulePath:   "github.com/test/myapp",
			}

			// Build model
			genModel, err := gen.Build(cat, config)
			if err != nil {
				t.Fatalf("Build failed: %v", err)
			}

			// Build factory
			genFactory, err := gen.BuildFactory(cat, config, genModel)
			if err != nil {
				t.Fatalf("BuildFactory failed: %v", err)
			}

			// Verify FK count
			if len(genFactory.ForeignKeyFields) != tt.wantFKCount {
				t.Errorf("ForeignKeyFields count = %d, want %d", len(genFactory.ForeignKeyFields), tt.wantFKCount)
			}

			if genFactory.HasForeignKeys != (tt.wantFKCount > 0) {
				t.Errorf("HasForeignKeys = %v, want %v", genFactory.HasForeignKeys, tt.wantFKCount > 0)
			}

			// Generate factory file to verify signature
			templateContent, err := templates.Files.ReadFile("factory.tmpl")
			if err != nil {
				t.Fatalf("Failed to read template: %v", err)
			}

			content, err := gen.GenerateFactoryFile(genFactory, string(templateContent))
			if err != nil {
				t.Fatalf("GenerateFactoryFile failed: %v", err)
			}

			// Verify wanted signature parts are present
			for _, part := range tt.wantSigParts {
				if !strings.Contains(content, part) {
					t.Errorf("Generated content missing required part:\n  %q\n\nGenerated content:\n%s", part, content)
				}
			}

			// Verify unwanted signature parts are absent
			for _, part := range tt.unwantSigParts {
				if strings.Contains(content, part) {
					t.Errorf("Generated content contains unwanted part:\n  %q\n\nGenerated content:\n%s", part, content)
				}
			}
		})
	}
}
