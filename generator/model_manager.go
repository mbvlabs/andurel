package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/models"
	"github.com/mbvlabs/andurel/pkg/naming"
)

type ModelManager struct {
	validator        *InputValidator
	fileManager      files.Manager
	modelGenerator   *models.Generator
	projectManager   *ProjectManager
	migrationManager *MigrationManager
	config           *UnifiedConfig
}

func NewModelManager(
	validator *InputValidator,
	fileManager files.Manager,
	modelGenerator *models.Generator,
	projectManager *ProjectManager,
	migrationManager *MigrationManager,
	config *UnifiedConfig,
) *ModelManager {
	return &ModelManager{
		validator:        validator,
		fileManager:      fileManager,
		modelGenerator:   modelGenerator,
		projectManager:   projectManager,
		migrationManager: migrationManager,
		config:           config,
	}
}

func (m *ModelManager) GenerateModel(resourceName string) error {
	modulePath, err := m.projectManager.GetModulePath()
	if err != nil {
		return err
	}

	if err := m.validator.ValidateResourceName(resourceName); err != nil {
		return err
	}

	tableName := naming.DeriveTableName(resourceName)

	if err := m.validator.ValidateAll(resourceName, tableName, modulePath); err != nil {
		return err
	}

	rootDir, err := m.fileManager.FindGoModRoot()
	if err != nil {
		return fmt.Errorf("failed to find go.mod root: %w", err)
	}

	if err := m.projectManager.ValidateSQLCConfig(rootDir); err != nil {
		return fmt.Errorf("SQLC configuration validation failed: %w", err)
	}

	var modelFileName strings.Builder
	modelFileName.Grow(len(resourceName) + 3) // +3 for ".go"
	modelFileName.WriteString(naming.ToSnakeCase(resourceName))
	modelFileName.WriteString(".go")
	modelPath := filepath.Join(m.config.Paths.Models, modelFileName.String())

	var sqlFileName strings.Builder
	sqlFileName.Grow(len(tableName) + 4) // +4 for ".sql"
	sqlFileName.WriteString(tableName)
	sqlFileName.WriteString(".sql")
	sqlPath := filepath.Join(m.config.Paths.Queries, sqlFileName.String())

	if err := m.fileManager.ValidateFileNotExists(modelPath); err != nil {
		return err
	}
	if err := m.fileManager.ValidateFileNotExists(sqlPath); err != nil {
		return err
	}

	cat, err := m.migrationManager.BuildCatalogFromMigrations(tableName)
	if err != nil {
		return err
	}

	if err := m.modelGenerator.GenerateModel(cat, resourceName, tableName, modelPath, sqlPath, modulePath); err != nil {
		return fmt.Errorf("failed to generate model: %w", err)
	}

	if err := m.fileManager.RunSQLCGenerate(); err != nil {
		return fmt.Errorf("failed to run sqlc generate: %w", err)
	}

	constructorFileName := fmt.Sprintf("%s_constructors.go", naming.ToSnakeCase(resourceName))
	constructorPath := filepath.Join(
		filepath.Join(m.config.Paths.Models, "internal", "db"),
		constructorFileName,
	)
	if err := m.modelGenerator.GenerateConstructors(cat, resourceName, tableName, constructorPath, modulePath); err != nil {
		return fmt.Errorf("failed to generate constructor functions: %w", err)
	}

	fmt.Printf(
		"Successfully generated complete model for %s with database functions\n",
		resourceName,
	)
	return nil
}

func (m *ModelManager) RefreshModel(resourceName, tableName string) error {
	modulePath, err := m.projectManager.GetModulePath()
	if err != nil {
		return err
	}

	if err := m.validator.ValidateResourceName(resourceName); err != nil {
		return err
	}

	if err := m.validator.ValidateAll(resourceName, tableName, modulePath); err != nil {
		return err
	}

	rootDir, err := m.fileManager.FindGoModRoot()
	if err != nil {
		return fmt.Errorf("failed to find go.mod root: %w", err)
	}

	if err := m.projectManager.ValidateSQLCConfig(rootDir); err != nil {
		return fmt.Errorf("SQLC configuration validation failed: %w", err)
	}

	pluralName := naming.DeriveTableName(resourceName)

	var modelFileName strings.Builder
	modelFileName.Grow(len(resourceName) + 3) // +3 for ".go"
	modelFileName.WriteString(naming.ToSnakeCase(resourceName))
	modelFileName.WriteString(".go")
	modelPath := filepath.Join(m.config.Paths.Models, modelFileName.String())

	var sqlFileName strings.Builder
	sqlFileName.Grow(len(pluralName) + 4) // +4 for ".sql"
	sqlFileName.WriteString(pluralName)
	sqlFileName.WriteString(".sql")
	sqlPath := filepath.Join(m.config.Paths.Queries, sqlFileName.String())

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"model file %s does not exist. Use 'generate model' to create it first",
			modelPath,
		)
	}
	if _, err := os.Stat(sqlPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"SQL file %s does not exist. Use 'generate model' to create it first",
			sqlPath,
		)
	}

	cat, err := m.migrationManager.BuildCatalogFromMigrations(tableName)
	if err != nil {
		return err
	}

	if err := m.modelGenerator.RefreshModel(cat, resourceName, pluralName, modelPath, sqlPath, modulePath); err != nil {
		return fmt.Errorf("failed to refresh model: %w", err)
	}

	if err := m.fileManager.RunSQLCGenerate(); err != nil {
		return fmt.Errorf("failed to run sqlc generate: %w", err)
	}

	fmt.Printf(
		"Successfully refreshed model %s with updated database schema while preserving custom code\n",
		resourceName,
	)
	return nil
}

func (m *ModelManager) RefreshQueries(resourceName, tableName string) error {
	modulePath, err := m.projectManager.GetModulePath()
	if err != nil {
		return err
	}

	if err := m.validator.ValidateResourceName(resourceName); err != nil {
		return err
	}

	if err := m.validator.ValidateAll(resourceName, tableName, modulePath); err != nil {
		return err
	}

	rootDir, err := m.fileManager.FindGoModRoot()
	if err != nil {
		return fmt.Errorf("failed to find go.mod root: %w", err)
	}

	if err := m.projectManager.ValidateSQLCConfig(rootDir); err != nil {
		return fmt.Errorf("SQLC configuration validation failed: %w", err)
	}

	pluralName := naming.DeriveTableName(resourceName)

	var modelFileName strings.Builder
	modelFileName.Grow(len(resourceName) + 3) // +3 for ".go"
	modelFileName.WriteString(naming.ToSnakeCase(resourceName))
	modelFileName.WriteString(".go")
	modelPath := filepath.Join(m.config.Paths.Models, modelFileName.String())

	var sqlFileName strings.Builder
	sqlFileName.Grow(len(pluralName) + 4) // +4 for ".sql"
	sqlFileName.WriteString(pluralName)
	sqlFileName.WriteString(".sql")
	sqlPath := filepath.Join(m.config.Paths.Queries, sqlFileName.String())

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"model file %s does not exist. Use 'generate model' to create it first",
			modelPath,
		)
	}
	if _, err := os.Stat(sqlPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"SQL file %s does not exist. Use 'generate model' to create it first",
			sqlPath,
		)
	}

	cat, err := m.migrationManager.BuildCatalogFromMigrations(tableName)
	if err != nil {
		return err
	}

	if err := m.modelGenerator.RefreshQueries(cat, resourceName, pluralName, sqlPath); err != nil {
		return fmt.Errorf("failed to refresh queries: %w", err)
	}

	if err := m.fileManager.RunSQLCGenerate(); err != nil {
		return fmt.Errorf("failed to run sqlc generate: %w", err)
	}

	fmt.Printf(
		"Successfully refreshed SQL queries for %s while preserving custom model functions\n",
		resourceName,
	)
	return nil
}

func (m *ModelManager) RefreshConstructors(resourceName, tableName string) error {
	modulePath, err := m.projectManager.GetModulePath()
	if err != nil {
		return err
	}

	if err := m.validator.ValidateResourceName(resourceName); err != nil {
		return err
	}

	if err := m.validator.ValidateAll(resourceName, tableName, modulePath); err != nil {
		return err
	}

	rootDir, err := m.fileManager.FindGoModRoot()
	if err != nil {
		return fmt.Errorf("failed to find go.mod root: %w", err)
	}

	if err := m.projectManager.ValidateSQLCConfig(rootDir); err != nil {
		return fmt.Errorf("SQLC configuration validation failed: %w", err)
	}

	var modelFileName strings.Builder
	modelFileName.Grow(len(resourceName) + 3) // +3 for ".go"
	modelFileName.WriteString(naming.ToSnakeCase(resourceName))
	modelFileName.WriteString(".go")
	modelPath := filepath.Join(m.config.Paths.Models, modelFileName.String())

	var sqlFileName strings.Builder
	sqlFileName.Grow(len(tableName) + 4) // +4 for ".sql"
	sqlFileName.WriteString(tableName)
	sqlFileName.WriteString(".sql")
	sqlPath := filepath.Join(m.config.Paths.Queries, sqlFileName.String())

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"model file %s does not exist. Use 'generate model' to create it first",
			modelPath,
		)
	}
	if _, err := os.Stat(sqlPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"SQL file %s does not exist. Use 'generate model' to create it first",
			sqlPath,
		)
	}

	cat, err := m.migrationManager.BuildCatalogFromMigrations(tableName)
	if err != nil {
		return err
	}

	if err := m.modelGenerator.RefreshQueries(cat, resourceName, tableName, sqlPath); err != nil {
		return fmt.Errorf("failed to refresh queries: %w", err)
	}

	constructorFileName := fmt.Sprintf("%s_constructors.go", naming.ToSnakeCase(resourceName))
	constructorPath := filepath.Join(
		filepath.Join(m.config.Paths.Models, "internal", "db"),
		constructorFileName,
	)
	if err := m.modelGenerator.RefreshConstructors(cat, resourceName, tableName, constructorPath, modulePath); err != nil {
		return fmt.Errorf("failed to refresh constructor functions: %w", err)
	}

	if err := m.fileManager.RunSQLCGenerate(); err != nil {
		return fmt.Errorf("failed to run sqlc generate: %w", err)
	}

	fmt.Printf(
		"Successfully refreshed SQL queries and constructor functions for %s - schema changes are now compiler-enforced\n",
		resourceName,
	)
	return nil
}
