package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/internal/types"
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

type modelSetupContext struct {
	ModulePath   string
	RootDir      string
	ModelPath    string
	SQLPath      string
	ResourceName string
	TableName    string
	PluralName   string
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

func (m *ModelManager) setupModelContext(resourceName, tableName string) (*modelSetupContext, error) {
	modulePath := m.projectManager.GetModulePath()

	if err := m.validator.ValidateResourceName(resourceName); err != nil {
		return nil, err
	}

	if err := m.validator.ValidateAll(resourceName, tableName, modulePath); err != nil {
		return nil, err
	}

	rootDir, err := m.fileManager.FindGoModRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to find go.mod root: %w", err)
	}

	if err := types.ValidateSQLCConfig(rootDir); err != nil {
		return nil, fmt.Errorf("SQLC configuration validation failed: %w", err)
	}

	pluralName := naming.DeriveTableName(resourceName)

	var modelFileName strings.Builder
	modelFileName.Grow(len(resourceName) + 3)
	modelFileName.WriteString(naming.ToSnakeCase(resourceName))
	modelFileName.WriteString(".go")
	modelPath := filepath.Join(m.config.Paths.Models, modelFileName.String())

	var sqlFileName strings.Builder
	sqlFileName.Grow(len(tableName) + 4)
	sqlFileName.WriteString(tableName)
	sqlFileName.WriteString(".sql")
	sqlPath := filepath.Join(m.config.Paths.Queries, sqlFileName.String())

	return &modelSetupContext{
		ModulePath:   modulePath,
		RootDir:      rootDir,
		ModelPath:    modelPath,
		SQLPath:      sqlPath,
		ResourceName: resourceName,
		TableName:    tableName,
		PluralName:   pluralName,
	}, nil
}

func (m *ModelManager) GenerateModel(resourceName string) error {
	tableName := naming.DeriveTableName(resourceName)

	ctx, err := m.setupModelContext(resourceName, tableName)
	if err != nil {
		return err
	}

	if err := m.fileManager.ValidateFileNotExists(ctx.ModelPath); err != nil {
		return err
	}
	if err := m.fileManager.ValidateFileNotExists(ctx.SQLPath); err != nil {
		return err
	}

	cat, err := m.migrationManager.BuildCatalogFromMigrations(ctx.TableName, m.config)
	if err != nil {
		return err
	}

	if err := m.modelGenerator.GenerateModel(cat, ctx.ResourceName, ctx.TableName, ctx.ModelPath, ctx.SQLPath, ctx.ModulePath); err != nil {
		return fmt.Errorf("failed to generate model: %w", err)
	}

	if err := m.fileManager.RunSQLCGenerate(); err != nil {
		return fmt.Errorf("failed to run sqlc generate: %w", err)
	}

	constructorFileName := fmt.Sprintf("%s_constructors.go", naming.ToSnakeCase(ctx.ResourceName))
	constructorPath := filepath.Join(
		filepath.Join(m.config.Paths.Models, "internal", "db"),
		constructorFileName,
	)
	if err := m.modelGenerator.GenerateConstructors(cat, ctx.ResourceName, ctx.TableName, constructorPath, ctx.ModulePath); err != nil {
		return fmt.Errorf("failed to generate constructor functions: %w", err)
	}

	fmt.Printf(
		"Successfully generated complete model for %s with database functions\n",
		ctx.ResourceName,
	)
	return nil
}

func (m *ModelManager) RefreshModel(resourceName, tableName string) error {
	ctx, err := m.setupModelContext(resourceName, tableName)
	if err != nil {
		return err
	}

	if _, err := os.Stat(ctx.ModelPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"model file %s does not exist. Use 'generate model' to create it first",
			ctx.ModelPath,
		)
	}
	if _, err := os.Stat(ctx.SQLPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"SQL file %s does not exist. Use 'generate model' to create it first",
			ctx.SQLPath,
		)
	}

	cat, err := m.migrationManager.BuildCatalogFromMigrations(ctx.TableName, m.config)
	if err != nil {
		return err
	}

	if err := m.modelGenerator.RefreshModel(cat, ctx.ResourceName, ctx.PluralName, ctx.ModelPath, ctx.SQLPath, ctx.ModulePath); err != nil {
		return fmt.Errorf("failed to refresh model: %w", err)
	}

	if err := m.fileManager.RunSQLCGenerate(); err != nil {
		return fmt.Errorf("failed to run sqlc generate: %w", err)
	}

	fmt.Printf(
		"Successfully refreshed model %s with updated database schema while preserving custom code\n",
		ctx.ResourceName,
	)
	return nil
}

func (m *ModelManager) RefreshQueries(resourceName, tableName string) error {
	ctx, err := m.setupModelContext(resourceName, tableName)
	if err != nil {
		return err
	}

	if _, err := os.Stat(ctx.ModelPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"model file %s does not exist. Use 'generate model' to create it first",
			ctx.ModelPath,
		)
	}
	if _, err := os.Stat(ctx.SQLPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"SQL file %s does not exist. Use 'generate model' to create it first",
			ctx.SQLPath,
		)
	}

	cat, err := m.migrationManager.BuildCatalogFromMigrations(ctx.TableName, m.config)
	if err != nil {
		return err
	}

	if err := m.modelGenerator.RefreshQueries(cat, ctx.ResourceName, ctx.PluralName, ctx.SQLPath); err != nil {
		return fmt.Errorf("failed to refresh queries: %w", err)
	}

	if err := m.fileManager.RunSQLCGenerate(); err != nil {
		return fmt.Errorf("failed to run sqlc generate: %w", err)
	}

	fmt.Printf(
		"Successfully refreshed SQL queries for %s while preserving custom model functions\n",
		ctx.ResourceName,
	)
	return nil
}

func (m *ModelManager) RefreshConstructors(resourceName, tableName string) error {
	ctx, err := m.setupModelContext(resourceName, tableName)
	if err != nil {
		return err
	}

	if _, err := os.Stat(ctx.ModelPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"model file %s does not exist. Use 'generate model' to create it first",
			ctx.ModelPath,
		)
	}
	if _, err := os.Stat(ctx.SQLPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"SQL file %s does not exist. Use 'generate model' to create it first",
			ctx.SQLPath,
		)
	}

	cat, err := m.migrationManager.BuildCatalogFromMigrations(ctx.TableName, m.config)
	if err != nil {
		return err
	}

	if err := m.modelGenerator.RefreshQueries(cat, ctx.ResourceName, ctx.TableName, ctx.SQLPath); err != nil {
		return fmt.Errorf("failed to refresh queries: %w", err)
	}

	constructorFileName := fmt.Sprintf("%s_constructors.go", naming.ToSnakeCase(ctx.ResourceName))
	constructorPath := filepath.Join(
		filepath.Join(m.config.Paths.Models, "internal", "db"),
		constructorFileName,
	)
	if err := m.modelGenerator.RefreshConstructors(cat, ctx.ResourceName, ctx.TableName, constructorPath, ctx.ModulePath); err != nil {
		return fmt.Errorf("failed to refresh constructor functions: %w", err)
	}

	if err := m.fileManager.RunSQLCGenerate(); err != nil {
		return fmt.Errorf("failed to run sqlc generate: %w", err)
	}

	fmt.Printf(
		"Successfully refreshed SQL queries and constructor functions for %s - schema changes are now compiler-enforced\n",
		ctx.ResourceName,
	)
	return nil
}
