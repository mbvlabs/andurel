package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/internal/catalog"
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

func (m *ModelManager) setupModelContext(resourceName, tableName string, tableNameOverridden bool) (*modelSetupContext, error) {
	modulePath := m.projectManager.GetModulePath()

	if err := m.validator.ValidateResourceName(resourceName); err != nil {
		return nil, err
	}

	if tableNameOverridden {
		if err := m.validator.ValidateModulePath(modulePath); err != nil {
			return nil, fmt.Errorf("module path validation failed: %w", err)
		}
	} else {
		if err := m.validator.ValidateAll(resourceName, tableName, modulePath); err != nil {
			return nil, err
		}
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

func (m *ModelManager) GenerateModel(resourceName string, tableNameOverride string, skipFactory bool) error {
	tableName := tableNameOverride
	if tableName == "" {
		tableName = naming.DeriveTableName(resourceName)
	}

	if tableNameOverride != "" {
		if err := m.validator.ValidateTableNameOverride(resourceName, tableNameOverride); err != nil {
			return err
		}
	}

	ctx, err := m.setupModelContext(resourceName, tableName, tableNameOverride != "")
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

	if err := m.modelGenerator.GenerateModel(cat, ctx.ResourceName, ctx.TableName, ctx.ModelPath, ctx.SQLPath, ctx.ModulePath, tableNameOverride); err != nil {
		return fmt.Errorf("failed to generate model: %w", err)
	}

	if err := m.fileManager.RunSQLCGenerate(); err != nil {
		return fmt.Errorf("failed to run sqlc generate: %w", err)
	}

	// Generate factory (unless skipped)
	if !skipFactory {
		if err := m.generateFactory(cat, ctx); err != nil {
			// Log the error but don't fail the entire generation
			fmt.Printf("Warning: failed to generate factory: %v\n", err)
		} else {
			fmt.Printf("✓ Generated factory: models/factories/%s.go\n", strings.ToLower(ctx.ResourceName))
		}
	}

	fmt.Printf(
		"Successfully generated complete model for %s with database functions\n",
		ctx.ResourceName,
	)
	return nil
}

// generateFactory creates a factory file for the model
func (m *ModelManager) generateFactory(cat *catalog.Catalog, ctx *modelSetupContext) error {
	// Build the model first
	genModel, err := m.modelGenerator.Build(cat, models.Config{
		TableName:    ctx.TableName,
		ResourceName: ctx.ResourceName,
		PackageName:  "models",
		DatabaseType: m.config.Database.Type,
		ModulePath:   ctx.ModulePath,
	})
	if err != nil {
		return fmt.Errorf("failed to build model for factory: %w", err)
	}

	// Build factory metadata
	genFactory, err := m.modelGenerator.BuildFactory(cat, models.Config{
		TableName:    ctx.TableName,
		ResourceName: ctx.ResourceName,
		PackageName:  "factories",
		DatabaseType: m.config.Database.Type,
		ModulePath:   ctx.ModulePath,
	}, genModel)
	if err != nil {
		return fmt.Errorf("failed to build factory: %w", err)
	}

	// Get root directory
	rootDir, err := m.fileManager.FindGoModRoot()
	if err != nil {
		return fmt.Errorf("failed to find go.mod root: %w", err)
	}

	// Write factory file
	if err := m.modelGenerator.WriteFactoryFile(genFactory, rootDir); err != nil {
		return fmt.Errorf("failed to write factory: %w", err)
	}

	return nil
}

func (m *ModelManager) RefreshQueries(resourceName, tableName string) error {
	modelPath := BuildModelPath(m.config.Paths.Models, resourceName)

	tableNameOverridden := false
	if overriddenTableName, found := ExtractTableNameOverride(modelPath, resourceName); found {
		tableName = overriddenTableName
		tableNameOverridden = true
	}

	ctx, err := m.setupModelContext(resourceName, tableName, tableNameOverridden)
	if err != nil {
		return err
	}

	if _, err := os.Stat(ctx.ModelPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"model file %s does not exist. Generate model first",
			ctx.ModelPath,
		)
	}
	if _, err := os.Stat(ctx.SQLPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"SQL file %s does not exist. Generate model first",
			ctx.SQLPath,
		)
	}

	cat, err := m.migrationManager.BuildCatalogFromMigrations(ctx.TableName, m.config)
	if err != nil {
		return err
	}

	if err := m.modelGenerator.RefreshQueries(cat, ctx.ResourceName, ctx.PluralName, ctx.SQLPath, tableNameOverridden); err != nil {
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

type queriesSetupContext struct {
	ModulePath   string
	RootDir      string
	SQLPath      string
	ResourceName string
	TableName    string
	PluralName   string
}

func (m *ModelManager) setupQueriesContext(tableName string) (*queriesSetupContext, error) {
	modulePath := m.projectManager.GetModulePath()

	if err := m.validator.ValidateTableName(tableName); err != nil {
		return nil, err
	}

	if err := m.validator.ValidateModulePath(modulePath); err != nil {
		return nil, fmt.Errorf("module path validation failed: %w", err)
	}

	rootDir, err := m.fileManager.FindGoModRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to find go.mod root: %w", err)
	}

	if err := types.ValidateSQLCConfig(rootDir); err != nil {
		return nil, fmt.Errorf("SQLC configuration validation failed: %w", err)
	}

	// Derive resource name from table name
	resourceName := naming.DeriveResourceName(tableName)

	var sqlFileName strings.Builder
	sqlFileName.Grow(len(tableName) + 4)
	sqlFileName.WriteString(tableName)
	sqlFileName.WriteString(".sql")
	sqlPath := filepath.Join(m.config.Paths.Queries, sqlFileName.String())

	return &queriesSetupContext{
		ModulePath:   modulePath,
		RootDir:      rootDir,
		SQLPath:      sqlPath,
		ResourceName: resourceName,
		TableName:    tableName,
		PluralName:   tableName, // Table name is already the plural form
	}, nil
}

func (m *ModelManager) checkExistingModel(resourceName string) {
	modelFileName := naming.ToSnakeCase(resourceName) + ".go"
	modelPath := filepath.Join(m.config.Paths.Models, modelFileName)

	if _, err := os.Stat(modelPath); err == nil {
		fmt.Printf(
			"Warning: Model file %s already exists for this resource.\n",
			modelPath,
		)
	}
}

func (m *ModelManager) GenerateQueriesOnly(tableName string) error {
	ctx, err := m.setupQueriesContext(tableName)
	if err != nil {
		return err
	}

	m.checkExistingModel(ctx.ResourceName)

	if err := m.fileManager.ValidateFileNotExists(ctx.SQLPath); err != nil {
		return err
	}

	cat, err := m.migrationManager.BuildCatalogFromMigrations(ctx.TableName, m.config)
	if err != nil {
		return err
	}

	table, err := cat.GetTable("", ctx.TableName)
	if err != nil {
		return fmt.Errorf(`table '%s' not found in catalog: %w

Make sure the table exists in your migrations (database/migrations/).
Run 'andurel queries generate %s' after creating the migration.`,
			ctx.TableName, err, ctx.TableName)
	}

	if err := m.modelGenerator.GenerateQueriesOnlyFile(ctx.ResourceName, ctx.TableName, table, ctx.SQLPath, true); err != nil {
		return fmt.Errorf("failed to generate SQL file: %w", err)
	}

	if err := m.fileManager.RunSQLCGenerate(); err != nil {
		return fmt.Errorf("failed to run sqlc generate: %w", err)
	}

	fmt.Printf(
		"Successfully generated SQL queries for %s (table: %s)\n",
		ctx.ResourceName,
		ctx.TableName,
	)
	return nil
}

func (m *ModelManager) RefreshQueriesOnly(tableName string) error {
	ctx, err := m.setupQueriesContext(tableName)
	if err != nil {
		return err
	}

	if _, err := os.Stat(ctx.SQLPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"SQL file %s does not exist. Use 'andurel queries generate %s' to create it first",
			ctx.SQLPath,
			ctx.TableName,
		)
	}

	cat, err := m.migrationManager.BuildCatalogFromMigrations(ctx.TableName, m.config)
	if err != nil {
		return err
	}

	if err := m.modelGenerator.RefreshQueriesOnly(cat, ctx.ResourceName, ctx.TableName, ctx.SQLPath, true); err != nil {
		return fmt.Errorf("failed to refresh queries: %w", err)
	}

	if err := m.fileManager.RunSQLCGenerate(); err != nil {
		return fmt.Errorf("failed to run sqlc generate: %w", err)
	}

	fmt.Printf(
		"Successfully refreshed SQL queries for %s (table: %s)\n",
		ctx.ResourceName,
		ctx.TableName,
	)
	return nil
}
