package models

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/pkg/constants"
)

func TestConstructorConversions__ProperlyHandlesNullableColumns(t *testing.T) {
	tests := []struct {
		name                  string
		migrationsDir         string
		tableName             string
		resourceName          string
		modulePath            string
		databaseType          string
		expectedCreateParams  []string
		expectedUpdateParams  []string
		unexpectedCreateCode  []string
		unexpectedUpdateCode  []string
	}{
		{
			name:          "PostgreSQL should properly convert nullable and non-nullable columns",
			migrationsDir: "simple_user_table",
			tableName:     "users",
			resourceName:  "User",
			modulePath:    "github.com/example/myapp",
			databaseType:  "postgresql",
			expectedCreateParams: []string{
				"params := db.CreateInsertUserParams(",
				"data.Email,",
				"data.Name,",
				"pgtype.Int4{Int32: data.Age, Valid: true}",
				"pgtype.Bool{Bool: data.IsActive, Valid: true}",
			},
			expectedUpdateParams: []string{
				"params := db.CreateUpdateUserParams(",
				"data.ID,",
				"data.Email,",
				"data.Name,",
				"pgtype.Int4{Int32: data.Age, Valid: true}",
				"pgtype.Bool{Bool: data.IsActive, Valid: true}",
			},
			unexpectedCreateCode: []string{
				"CreateInsertUserParams()",
			},
			unexpectedUpdateCode: []string{
				"CreateUpdateUserParams()",
			},
		},
		{
			name:          "SQLite should properly convert nullable and non-nullable columns",
			migrationsDir: "sqlite_user_table",
			tableName:     "users",
			resourceName:  "User",
			modulePath:    "github.com/example/myapp",
			databaseType:  "sqlite",
			expectedCreateParams: []string{
				"params := db.CreateInsertUserParams(",
				"data.Email,",
				"sql.NullTime{Time: data.EmailVerifiedAt, Valid: true}",
				"data.Password,",
				"data.IsAdmin,",
			},
			expectedUpdateParams: []string{
				"params := db.CreateUpdateUserParams(",
				"data.ID.String(),",
				"data.Email,",
				"sql.NullTime{Time: data.EmailVerifiedAt, Valid: true}",
				"data.Password,",
				"data.IsAdmin,",
			},
			unexpectedCreateCode: []string{
				"CreateInsertUserParams()",
			},
			unexpectedUpdateCode: []string{
				"CreateUpdateUserParams()",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			modelsDir := filepath.Join(tempDir, "models")

			err := os.MkdirAll(modelsDir, constants.DirPermissionDefault)
			if err != nil {
				t.Fatalf("Failed to create models directory: %v", err)
			}

			originalWd, _ := os.Getwd()
			oldWd, _ := os.Getwd()
			defer os.Chdir(oldWd)
			os.Chdir(tempDir)

			migrationsDir := filepath.Join(
				originalWd,
				"testdata",
				"migrations",
				tt.migrationsDir,
			)

			generator := NewGenerator(tt.databaseType)

			cat, err := generator.buildCatalogFromTableMigrations(
				tt.tableName,
				[]string{migrationsDir},
			)
			if err != nil {
				t.Fatalf("Failed to build catalog: %v", err)
			}

			modelFileName := fmt.Sprintf("%s.go", strings.ToLower(tt.resourceName))
			modelPath := filepath.Join(modelsDir, modelFileName)

			queriesDir := filepath.Join(tempDir, "database", "queries")
			err = os.MkdirAll(queriesDir, constants.DirPermissionDefault)
			if err != nil {
				t.Fatalf("Failed to create queries directory: %v", err)
			}

			sqlFileName := fmt.Sprintf("%s.sql", strings.ToLower(tt.tableName))
			sqlPath := filepath.Join(queriesDir, sqlFileName)

			err = generator.GenerateModel(
				cat,
				tt.resourceName,
				tt.tableName,
				modelPath,
				sqlPath,
				tt.modulePath,
				"",
			)
			if err != nil {
				t.Fatalf("Failed to generate model: %v", err)
			}

			modelContent, err := os.ReadFile(modelPath)
			if err != nil {
				t.Fatalf("Failed to read model file: %v", err)
			}

			modelStr := string(modelContent)

			for _, expectedParam := range tt.expectedCreateParams {
				if !strings.Contains(modelStr, expectedParam) {
					t.Errorf(
						"Model file should contain Create constructor parameter: %s\nGenerated content:\n%s",
						expectedParam,
						modelStr,
					)
				}
			}

			for _, expectedParam := range tt.expectedUpdateParams {
				if !strings.Contains(modelStr, expectedParam) {
					t.Errorf(
						"Model file should contain Update constructor parameter: %s\nGenerated content:\n%s",
						expectedParam,
						modelStr,
					)
				}
			}

			for _, unexpectedCode := range tt.unexpectedCreateCode {
				if strings.Contains(modelStr, unexpectedCode) {
					t.Errorf(
						"Model file should NOT contain empty Create constructor call: %s",
						unexpectedCode,
					)
				}
			}

			for _, unexpectedCode := range tt.unexpectedUpdateCode {
				if strings.Contains(modelStr, unexpectedCode) {
					t.Errorf(
						"Model file should NOT contain empty Update constructor call: %s",
						unexpectedCode,
					)
				}
			}
		})
	}
}

func TestConstructorConversions__FieldsExcludedCorrectly(t *testing.T) {
	tests := []struct {
		name                string
		migrationsDir       string
		tableName           string
		resourceName        string
		modulePath          string
		databaseType        string
		unexpectedInCreate  []string
		unexpectedInUpdate  []string
	}{
		{
			name:          "Create should exclude ID, CreatedAt, UpdatedAt",
			migrationsDir: "simple_user_table",
			tableName:     "users",
			resourceName:  "User",
			modulePath:    "github.com/example/myapp",
			databaseType:  "postgresql",
			unexpectedInCreate: []string{
				"data.ID",
				"data.CreatedAt",
				"data.UpdatedAt",
			},
			unexpectedInUpdate: []string{
				"data.CreatedAt",
				"data.UpdatedAt",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			modelsDir := filepath.Join(tempDir, "models")

			err := os.MkdirAll(modelsDir, constants.DirPermissionDefault)
			if err != nil {
				t.Fatalf("Failed to create models directory: %v", err)
			}

			originalWd, _ := os.Getwd()
			oldWd, _ := os.Getwd()
			defer os.Chdir(oldWd)
			os.Chdir(tempDir)

			migrationsDir := filepath.Join(
				originalWd,
				"testdata",
				"migrations",
				tt.migrationsDir,
			)

			generator := NewGenerator(tt.databaseType)

			cat, err := generator.buildCatalogFromTableMigrations(
				tt.tableName,
				[]string{migrationsDir},
			)
			if err != nil {
				t.Fatalf("Failed to build catalog: %v", err)
			}

			modelFileName := fmt.Sprintf("%s.go", strings.ToLower(tt.resourceName))
			modelPath := filepath.Join(modelsDir, modelFileName)

			queriesDir := filepath.Join(tempDir, "database", "queries")
			err = os.MkdirAll(queriesDir, constants.DirPermissionDefault)
			if err != nil {
				t.Fatalf("Failed to create queries directory: %v", err)
			}

			sqlFileName := fmt.Sprintf("%s.sql", strings.ToLower(tt.tableName))
			sqlPath := filepath.Join(queriesDir, sqlFileName)

			err = generator.GenerateModel(
				cat,
				tt.resourceName,
				tt.tableName,
				modelPath,
				sqlPath,
				tt.modulePath,
				"",
			)
			if err != nil {
				t.Fatalf("Failed to generate model: %v", err)
			}

			modelContent, err := os.ReadFile(modelPath)
			if err != nil {
				t.Fatalf("Failed to read model file: %v", err)
			}

			modelStr := string(modelContent)

			createStart := strings.Index(modelStr, "func CreateUser(")
			createEnd := strings.Index(modelStr[createStart:], "func UpdateUser(")
			createFunc := modelStr[createStart : createStart+createEnd]

			for _, unexpected := range tt.unexpectedInCreate {
				paramsStart := strings.Index(createFunc, "params := db.CreateInsertUserParams(")
				paramsEnd := strings.Index(createFunc[paramsStart:], ")")
				paramsSection := createFunc[paramsStart : paramsStart+paramsEnd]

				if strings.Contains(paramsSection, unexpected) {
					t.Errorf(
						"Create constructor params should NOT contain: %s\nParams section:\n%s",
						unexpected,
						paramsSection,
					)
				}
			}

			updateStart := strings.Index(modelStr, "func UpdateUser(")
			updateEnd := strings.Index(modelStr[updateStart:], "func DestroyUser(")
			updateFunc := modelStr[updateStart : updateStart+updateEnd]

			for _, unexpected := range tt.unexpectedInUpdate {
				paramsStart := strings.Index(updateFunc, "params := db.CreateUpdateUserParams(")
				paramsEnd := strings.Index(updateFunc[paramsStart:], ")")
				paramsSection := updateFunc[paramsStart : paramsStart+paramsEnd]

				if strings.Contains(paramsSection, unexpected) {
					t.Errorf(
						"Update constructor params should NOT contain: %s\nParams section:\n%s",
						unexpected,
						paramsSection,
					)
				}
			}
		})
	}
}
