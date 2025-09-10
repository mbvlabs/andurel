package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jinzhu/inflection"
	"github.com/mbvlabs/andurel/generator/controllers"
	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/models"
	"github.com/mbvlabs/andurel/generator/views"
)

type Coordinator struct {
	fileManager         *files.Manager
	modelGenerator      *models.Generator
	controllerGenerator *controllers.Generator
	viewGenerator       *views.Generator
	projectManager      *ProjectManager
	migrationManager    *MigrationManager
	validator           *InputValidator
	config              *AppConfig
}

func NewCoordinator() (Coordinator, error) {
	projectManager, err := NewProjectManager()
	if err != nil {
		return Coordinator{}, fmt.Errorf("failed to create project manager: %w", err)
	}

	config := NewDefaultAppConfig()

	c := Coordinator{
		fileManager:         files.NewManager(),
		modelGenerator:      models.NewGenerator(config.Database.Type),
		controllerGenerator: controllers.NewGenerator(config.Database.Type),
		viewGenerator:       views.NewGenerator(config.Database.Type),
		projectManager:      projectManager,
		migrationManager:    NewMigrationManager(),
		validator:           NewInputValidator(),
		config:              config,
	}

	return c, nil
}

func (c *Coordinator) GenerateModel(resourceName, tableName string) error {
	modulePath, err := c.projectManager.GetModulePath()
	if err != nil {
		return err
	}

	if err := c.validator.ValidateAll(resourceName, tableName, modulePath); err != nil {
		return err
	}
	rootDir, err := c.fileManager.FindGoModRoot()
	if err != nil {
		return fmt.Errorf("failed to find go.mod root: %w", err)
	}

	if err := c.projectManager.ValidateSQLCConfig(rootDir); err != nil {
		return fmt.Errorf("SQLC configuration validation failed: %w", err)
	}

	pluralName := inflection.Plural(strings.ToLower(resourceName))

	var modelFileName strings.Builder
	modelFileName.Grow(len(resourceName) + 3) // +3 for ".go"
	modelFileName.WriteString(strings.ToLower(resourceName))
	modelFileName.WriteString(".go")
	modelPath := filepath.Join(c.config.Paths.Models, modelFileName.String())

	var sqlFileName strings.Builder
	sqlFileName.Grow(len(pluralName) + 4) // +4 for ".sql"
	sqlFileName.WriteString(pluralName)
	sqlFileName.WriteString(".sql")
	sqlPath := filepath.Join(c.config.Paths.Queries, sqlFileName.String())

	if err := c.fileManager.ValidateFileNotExists(modelPath); err != nil {
		return err
	}
	if err := c.fileManager.ValidateFileNotExists(sqlPath); err != nil {
		return err
	}

	cat, err := c.migrationManager.BuildCatalogFromMigrations(tableName)
	if err != nil {
		return err
	}

	if err := c.modelGenerator.GenerateModel(cat, resourceName, pluralName, modelPath, sqlPath, modulePath); err != nil {
		return fmt.Errorf("failed to generate model: %w", err)
	}

	if err := c.fileManager.RunSQLCGenerate(); err != nil {
		return fmt.Errorf("failed to run sqlc generate: %w", err)
	}

	fmt.Printf(
		"Successfully generated complete model for %s with database functions\n",
		resourceName,
	)
	return nil
}

func (c *Coordinator) GenerateController(resourceName, tableName string) error {
	modulePath, err := c.projectManager.GetModulePath()
	if err != nil {
		return err
	}

	if err := c.validator.ValidateAll(resourceName, tableName, modulePath); err != nil {
		return err
	}

	var modelFileName strings.Builder
	modelFileName.Grow(len(resourceName) + 3) // +3 for ".go"
	modelFileName.WriteString(strings.ToLower(resourceName))
	modelFileName.WriteString(".go")
	modelPath := filepath.Join(c.config.Paths.Models, modelFileName.String())
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"model file %s does not exist. Generate model first",
			modelPath,
		)
	}

	cat, err := c.migrationManager.BuildCatalogFromMigrations(tableName)
	if err != nil {
		return err
	}

	if err := c.controllerGenerator.GenerateController(cat, resourceName, controllers.ResourceController, modulePath); err != nil {
		return fmt.Errorf("failed to generate controller: %w", err)
	}

	if err := c.viewGenerator.GenerateView(cat, resourceName, modulePath); err != nil {
		return fmt.Errorf("failed to generate view: %w", err)
	}

	fmt.Printf("Successfully generated resource controller and view for %s\n", resourceName)
	return nil
}

func (c *Coordinator) GenerateView(resourceName, tableName string) error {
	modulePath, err := c.projectManager.GetModulePath()
	if err != nil {
		return err
	}

	if err := c.validator.ValidateAll(resourceName, tableName, modulePath); err != nil {
		return err
	}

	var modelFileName strings.Builder
	modelFileName.Grow(len(resourceName) + 3) // +3 for ".go"
	modelFileName.WriteString(strings.ToLower(resourceName))
	modelFileName.WriteString(".go")
	modelPath := filepath.Join(c.config.Paths.Models, modelFileName.String())
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"model file %s does not exist. Generate model first",
			modelPath,
		)
	}

	cat, err := c.migrationManager.BuildCatalogFromMigrations(tableName)
	if err != nil {
		return err
	}

	if err := c.viewGenerator.GenerateView(cat, resourceName, modulePath); err != nil {
		return fmt.Errorf("failed to generate view: %w", err)
	}

	fmt.Printf("Successfully generated resource view for %s\n", resourceName)

	return nil
}

func (c *Coordinator) GenerateControllerFromModel(resourceName string) error {
	tableName, err := c.inferTableNameFromModel(resourceName)
	if err != nil {
		return err
	}
	return c.GenerateController(resourceName, tableName)
}

func (c *Coordinator) GenerateViewFromModel(resourceName string) error {
	tableName, err := c.inferTableNameFromModel(resourceName)
	if err != nil {
		return err
	}
	return c.GenerateView(resourceName, tableName)
}

func (c *Coordinator) inferTableNameFromModel(resourceName string) (string, error) {
	if err := c.validator.ValidateResourceName(resourceName); err != nil {
		return "", fmt.Errorf("resource name validation failed: %w", err)
	}

	var modelFileName strings.Builder
	modelFileName.Grow(len(resourceName) + 3) // +3 for ".go"
	modelFileName.WriteString(strings.ToLower(resourceName))
	modelFileName.WriteString(".go")
	modelPath := filepath.Join(c.config.Paths.Models, modelFileName.String())
	
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return "", fmt.Errorf(
			"model file %s does not exist. Generate model first",
			modelPath,
		)
	}

	// For now, we'll infer the table name using inflection like we do in model generation
	// In the future, this could be enhanced to read the actual table name from the model file
	pluralName := inflection.Plural(strings.ToLower(resourceName))
	
	return pluralName, nil
}
