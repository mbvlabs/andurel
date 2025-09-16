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

func (c *Coordinator) GenerateModel(resourceName string) error {
	modulePath, err := c.projectManager.GetModulePath()
	if err != nil {
		return err
	}

	tableName := inflection.Plural(strings.ToLower(resourceName))

	if err := c.validator.ValidateAll(resourceName, tableName, modulePath); err != nil {
		return err
	}
	rootDir, err := c.fileManager.FindGoModRoot()
	if err != nil {
		return fmt.Errorf("failed to find go.mod root: %w", err)
	}

	if err := c.projectManager.ValidateBobConfig(rootDir); err != nil {
		return fmt.Errorf("Bob configuration validation failed: %w", err)
	}

	var modelFileName strings.Builder
	modelFileName.Grow(len(resourceName) + 3) // +3 for ".go"
	modelFileName.WriteString(strings.ToLower(resourceName))
	modelFileName.WriteString(".go")
	modelPath := filepath.Join(c.config.Paths.Models, modelFileName.String())

	if err := c.fileManager.ValidateFileNotExists(modelPath); err != nil {
		return err
	}

	// Run bob first to generate database code
	if err := c.fileManager.RunBobGenerate(); err != nil {
		return fmt.Errorf("failed to run bob generate: %w", err)
	}

	// Generate model from bob generated structs
	if err := c.modelGenerator.GenerateModelFromBob(resourceName, tableName, modelPath, modulePath); err != nil {
		return fmt.Errorf("failed to generate model: %w", err)
	}

	fmt.Printf(
		"Successfully generated complete model for %s with database functions\n",
		resourceName,
	)
	return nil
}

func (c *Coordinator) GenerateController(resourceName, tableName string, withViews bool) error {
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

	controllerType := controllers.ResourceControllerNoViews
	if withViews {
		controllerType = controllers.ResourceController
	}

	if err := c.controllerGenerator.GenerateController(cat, resourceName, controllerType, modulePath); err != nil {
		return fmt.Errorf("failed to generate controller: %w", err)
	}

	if withViews {
		if err := c.viewGenerator.GenerateViewWithController(cat, resourceName, modulePath, true); err != nil {
			return fmt.Errorf("failed to generate view: %w", err)
		}
		fmt.Printf("Successfully generated resource controller %s with views\n", resourceName)
	} else {
		fmt.Printf("Successfully generated resource controller %s (no views)\n", resourceName)
	}

	return nil
}

func (c *Coordinator) GenerateControllerFromModel(resourceName string, withViews bool) error {
	modulePath, err := c.projectManager.GetModulePath()
	if err != nil {
		return err
	}

	var modelFileName strings.Builder
	modelFileName.Grow(len(resourceName) + 3) // +3 for ".go"
	modelFileName.WriteString(strings.ToLower(resourceName))
	modelFileName.WriteString(".go")
	modelPath := filepath.Join(c.config.Paths.Models, modelFileName.String())
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"model file %s does not exist. Generate the model first with: andurel generate model %s <table_name>",
			modelPath,
			resourceName,
		)
	}

	tableName := inflection.Plural(strings.ToLower(resourceName))

	if err := c.validator.ValidateTableName(tableName); err != nil {
		return fmt.Errorf("derived table name validation failed: %w", err)
	}

	routesFilePath := filepath.Join(c.config.Paths.Routes, "routes.go")
	if _, err := os.Stat(routesFilePath); os.IsNotExist(err) {
		return fmt.Errorf(
			"routes file %s does not exist. Please ensure your project has a routes.go file before generating controllers",
			routesFilePath,
		)
	}

	individualRoutePath := filepath.Join("router/routes", tableName+".go")
	if _, err := os.Stat(individualRoutePath); err == nil {
		return fmt.Errorf("routes file %s already exists", individualRoutePath)
	}

	controllerPath := filepath.Join(c.config.Paths.Controllers, tableName+".go")
	if _, err := os.Stat(controllerPath); err == nil {
		return fmt.Errorf("controller file %s already exists", controllerPath)
	}

	controllerFilePath := filepath.Join(c.config.Paths.Controllers, "controller.go")
	if _, err := os.Stat(controllerFilePath); os.IsNotExist(err) {
		return fmt.Errorf(
			"main controller file %s does not exist. Please ensure your project has a controller.go file before generating controllers",
			controllerFilePath,
		)
	}

	content, err := os.ReadFile(controllerFilePath)
	if err != nil {
		return fmt.Errorf("failed to read controller.go: %w", err)
	}

	controllerFieldName := resourceName + "s"
	controllerVarName := strings.ToLower(resourceName) + "s"
	controllerConstructor := controllerVarName + " := new" + resourceName + "s(db)"
	controllerReturnField := controllerVarName + ","
	contentStr := string(content)
	lines := strings.SplitSeq(contentStr, "\n")

	for line := range lines {
		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, controllerFieldName+" ") &&
			strings.HasSuffix(trimmedLine, " "+controllerFieldName) {
			return fmt.Errorf(
				"controller %s is already registered in %s (struct field found)",
				resourceName,
				controllerFilePath,
			)
		}

		if strings.Contains(trimmedLine, controllerConstructor) {
			return fmt.Errorf(
				"controller %s is already registered in %s (constructor call found)",
				resourceName,
				controllerFilePath,
			)
		}

		if trimmedLine == controllerReturnField {
			return fmt.Errorf(
				"controller %s is already registered in %s (return field found)",
				resourceName,
				controllerFilePath,
			)
		}
	}

	if withViews {
		if _, err := os.Stat(c.config.Paths.Views); os.IsNotExist(err) {
			return fmt.Errorf(
				"views directory %s does not exist. Please create the views directory structure before using --with-views",
				c.config.Paths.Views,
			)
		}

		viewPath := filepath.Join(c.config.Paths.Views, tableName+"_resource.templ")
		if _, err := os.Stat(viewPath); err == nil {
			return fmt.Errorf("view file %s already exists", viewPath)
		}
	}

	cat, err := c.migrationManager.BuildCatalogFromMigrations(tableName)
	if err != nil {
		return err
	}

	controllerType := controllers.ResourceControllerNoViews
	if withViews {
		controllerType = controllers.ResourceController
	}

	if err := c.controllerGenerator.GenerateController(cat, resourceName, controllerType, modulePath); err != nil {
		return fmt.Errorf("failed to generate controller: %w", err)
	}

	if withViews {
		if err := c.viewGenerator.GenerateViewWithController(cat, resourceName, modulePath, true); err != nil {
			return fmt.Errorf("failed to generate view: %w", err)
		}
		fmt.Printf("Successfully generated resource controller %s with views\n", resourceName)
	} else {
		fmt.Printf("Successfully generated resource controller %s (no views)\n", resourceName)
	}

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

func (c *Coordinator) RefreshModel(resourceName, tableName string) error {
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

	if err := c.projectManager.ValidateBobConfig(rootDir); err != nil {
		return fmt.Errorf("Bob configuration validation failed: %w", err)
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

	cat, err := c.migrationManager.BuildCatalogFromMigrations(tableName)
	if err != nil {
		return err
	}

	if err := c.modelGenerator.RefreshModel(cat, resourceName, pluralName, modelPath, sqlPath, modulePath); err != nil {
		return fmt.Errorf("failed to refresh model: %w", err)
	}

	if err := c.fileManager.RunBobGenerate(); err != nil {
		return fmt.Errorf("failed to run bob generate: %w", err)
	}

	fmt.Printf(
		"Successfully refreshed model %s with updated database schema while preserving custom code\n",
		resourceName,
	)
	return nil
}

func (c *Coordinator) RefreshQueries(resourceName, tableName string) error {
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

	if err := c.projectManager.ValidateBobConfig(rootDir); err != nil {
		return fmt.Errorf("Bob configuration validation failed: %w", err)
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

	cat, err := c.migrationManager.BuildCatalogFromMigrations(tableName)
	if err != nil {
		return err
	}

	if err := c.modelGenerator.RefreshQueries(cat, resourceName, pluralName, sqlPath); err != nil {
		return fmt.Errorf("failed to refresh queries: %w", err)
	}

	if err := c.fileManager.RunBobGenerate(); err != nil {
		return fmt.Errorf("failed to run bob generate: %w", err)
	}

	fmt.Printf(
		"Successfully refreshed SQL queries for %s while preserving custom model functions\n",
		resourceName,
	)
	return nil
}

func (c *Coordinator) RefreshConstructors(resourceName, tableName string) error {
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

	if err := c.projectManager.ValidateBobConfig(rootDir); err != nil {
		return fmt.Errorf("Bob configuration validation failed: %w", err)
	}

	var modelFileName strings.Builder
	modelFileName.Grow(len(resourceName) + 3) // +3 for ".go"
	modelFileName.WriteString(strings.ToLower(resourceName))
	modelFileName.WriteString(".go")
	modelPath := filepath.Join(c.config.Paths.Models, modelFileName.String())

	var sqlFileName strings.Builder
	sqlFileName.Grow(len(tableName) + 4) // +4 for ".sql"
	sqlFileName.WriteString(tableName)
	sqlFileName.WriteString(".sql")
	sqlPath := filepath.Join(c.config.Paths.Queries, sqlFileName.String())

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

	cat, err := c.migrationManager.BuildCatalogFromMigrations(tableName)
	if err != nil {
		return err
	}

	if err := c.modelGenerator.RefreshQueries(cat, resourceName, tableName, sqlPath); err != nil {
		return fmt.Errorf("failed to refresh queries: %w", err)
	}

	constructorFileName := fmt.Sprintf("%s_constructors.go", strings.ToLower(resourceName))
	constructorPath := filepath.Join(
		filepath.Join(c.config.Paths.Models, "internal", "db"),
		constructorFileName,
	)
	if err := c.modelGenerator.RefreshConstructors(cat, resourceName, tableName, constructorPath, modulePath); err != nil {
		return fmt.Errorf("failed to refresh constructor functions: %w", err)
	}

	if err := c.fileManager.RunBobGenerate(); err != nil {
		return fmt.Errorf("failed to run bob generate: %w", err)
	}

	fmt.Printf(
		"Successfully refreshed SQL queries and constructor functions for %s - schema changes are now compiler-enforced\n",
		resourceName,
	)
	return nil
}

func (c *Coordinator) GenerateViewFromModel(resourceName string, withController bool) error {
	modulePath, err := c.projectManager.GetModulePath()
	if err != nil {
		return err
	}

	var modelFileName strings.Builder
	modelFileName.Grow(len(resourceName) + 3) // +3 for ".go"
	modelFileName.WriteString(strings.ToLower(resourceName))
	modelFileName.WriteString(".go")
	modelPath := filepath.Join(c.config.Paths.Models, modelFileName.String())
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"model file %s does not exist. Generate the model first with: andurel generate model %s <table_name>",
			modelPath,
			resourceName,
		)
	}

	tableName := inflection.Plural(strings.ToLower(resourceName))

	if err := c.validator.ValidateTableName(tableName); err != nil {
		return fmt.Errorf("derived table name validation failed: %w", err)
	}

	if _, err := os.Stat(c.config.Paths.Views); os.IsNotExist(err) {
		return fmt.Errorf(
			"views directory %s does not exist. Please create the views directory structure",
			c.config.Paths.Views,
		)
	}

	viewPath := filepath.Join(c.config.Paths.Views, tableName+"_resource.templ")
	if _, err := os.Stat(viewPath); err == nil {
		return fmt.Errorf("view file %s already exists", viewPath)
	}

	if withController {
		routesFilePath := filepath.Join(c.config.Paths.Routes, "routes.go")
		if _, err := os.Stat(routesFilePath); os.IsNotExist(err) {
			return fmt.Errorf(
				"routes file %s does not exist. Please ensure your project has a routes.go file before generating controllers",
				routesFilePath,
			)
		}

		individualRoutePath := filepath.Join("router/routes", tableName+".go")
		if _, err := os.Stat(individualRoutePath); err == nil {
			return fmt.Errorf("routes file %s already exists", individualRoutePath)
		}

		controllerPath := filepath.Join(c.config.Paths.Controllers, tableName+".go")
		if _, err := os.Stat(controllerPath); err == nil {
			return fmt.Errorf("controller file %s already exists", controllerPath)
		}

		controllerFilePath := filepath.Join(c.config.Paths.Controllers, "controller.go")
		if _, err := os.Stat(controllerFilePath); os.IsNotExist(err) {
			return fmt.Errorf(
				"main controller file %s does not exist. Please ensure your project has a controller.go file before generating controllers",
				controllerFilePath,
			)
		}

		content, err := os.ReadFile(controllerFilePath)
		if err != nil {
			return fmt.Errorf("failed to read controller.go: %w", err)
		}

		controllerFieldName := resourceName + "s"
		controllerVarName := strings.ToLower(resourceName) + "s"
		controllerConstructor := controllerVarName + " := new" + resourceName + "s(db)"
		controllerReturnField := controllerVarName + ","
		contentStr := string(content)
		lines := strings.SplitSeq(contentStr, "\n")

		for line := range lines {
			trimmedLine := strings.TrimSpace(line)

			if strings.HasPrefix(trimmedLine, controllerFieldName+" ") &&
				strings.HasSuffix(trimmedLine, " "+controllerFieldName) {
				return fmt.Errorf(
					"controller %s is already registered in %s (struct field found)",
					resourceName,
					controllerFilePath,
				)
			}

			if strings.Contains(trimmedLine, controllerConstructor) {
				return fmt.Errorf(
					"controller %s is already registered in %s (constructor call found)",
					resourceName,
					controllerFilePath,
				)
			}

			if trimmedLine == controllerReturnField {
				return fmt.Errorf(
					"controller %s is already registered in %s (return field found)",
					resourceName,
					controllerFilePath,
				)
			}
		}
	}

	cat, err := c.migrationManager.BuildCatalogFromMigrations(tableName)
	if err != nil {
		return err
	}

	if err := c.viewGenerator.GenerateViewWithController(cat, resourceName, modulePath, withController); err != nil {
		return fmt.Errorf("failed to generate view: %w", err)
	}

	if withController {
		controllerType := controllers.ResourceController // with views since we're generating both
		if err := c.controllerGenerator.GenerateController(cat, resourceName, controllerType, modulePath); err != nil {
			return fmt.Errorf("failed to generate controller: %w", err)
		}
		fmt.Printf("Successfully generated resource view for %s with controller\n", resourceName)
	} else {
		fmt.Printf("Successfully generated resource view for %s\n", resourceName)
	}

	return nil
}
