package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/generator/controllers"
	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/pkg/naming"
)

type ControllerManager struct {
	validator        *InputValidator
	projectManager   *ProjectManager
	migrationManager *MigrationManager
	config           *UnifiedConfig
	pkResolver       PrimaryKeyResolver
}

func NewControllerManager(
	validator *InputValidator,
	projectManager *ProjectManager,
	migrationManager *MigrationManager,
	config *UnifiedConfig,
) *ControllerManager {
	return &ControllerManager{
		validator:        validator,
		projectManager:   projectManager,
		migrationManager: migrationManager,
		config:           config,
		pkResolver:       DefaultPrimaryKeyResolver{},
	}
}

func (c *ControllerManager) SetPrimaryKeyResolver(resolver PrimaryKeyResolver) {
	c.pkResolver = resolver
}

func (c *ControllerManager) resolvePK(cat *catalog.Catalog, tableName string) (PrimaryKeyInfo, error) {
	pkInfo := DetectPrimaryKey(cat, tableName)
	if !pkInfo.Found {
		ok, err := c.pkResolver.ConfirmNoPK(tableName)
		if err != nil {
			return PrimaryKeyInfo{}, err
		}
		if !ok {
			return PrimaryKeyInfo{}, fmt.Errorf("generation aborted: table %q has no primary key", tableName)
		}
		return PrimaryKeyInfo{Found: false}, nil
	}
	if !pkInfo.IsNamedID {
		resolved, err := c.pkResolver.ResolveAlternatePK(pkInfo, tableName)
		if err != nil {
			return PrimaryKeyInfo{}, err
		}
		return resolved, nil
	}
	return pkInfo, nil
}

func (c *ControllerManager) GenerateController(
	resourceName, namespace string,
	inertia string,
) error {
	return c.GenerateControllerWithActions(resourceName, namespace, nil, inertia)
}

func (c *ControllerManager) GenerateControllerWithActions(
	resourceName, namespace string,
	actions []string,
	inertia string,
) error {
	return c.GenerateControllerWithActionsForModel(resourceName, namespace, resourceName, "", actions, inertia, false)
}

func (c *ControllerManager) GenerateControllerWithActionsForModel(
	resourceName, namespace, modelName, tableName string,
	actions []string,
	inertia string,
	isAPI bool,
) error {
	modulePath := c.projectManager.GetModulePath()
	if modelName == "" {
		modelName = resourceName
	}
	if err := c.validator.ValidateResourceName(modelName); err != nil {
		return fmt.Errorf("model name validation failed: %w", err)
	}

	tableNameOverridden := tableName != "" && tableName != naming.DeriveTableName(resourceName)

	if tableName == "" {
		tableName = naming.DeriveTableName(resourceName)
	}
	modelTableName := tableName
	modelTableNameOverridden := tableNameOverridden
	if modelName != resourceName {
		modelTableName = naming.DeriveTableName(modelName)
		modelTableNameOverridden = false
	}

	if tableNameOverridden {
		if err := c.validator.ValidateTableNameOverride(resourceName, tableName); err != nil {
			return fmt.Errorf("table name validation failed: %w", err)
		}
	} else {
		if err := c.validator.ValidateAll(resourceName, tableName, modulePath); err != nil {
			return err
		}
	}

	var modelFileName strings.Builder
	modelFileName.Grow(len(modelName) + 3) // +3 for ".go"
	modelFileName.WriteString(naming.ToSnakeCase(modelName))
	modelFileName.WriteString(".go")
	modelPath := filepath.Join(c.config.Paths.Models, modelFileName.String())
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"model file %s does not exist. Generate model first",
			modelPath,
		)
	}

	cat, err := c.migrationManager.BuildCatalogFromMigrations(modelTableName, c.config)
	if err != nil {
		return err
	}

	// Resolve primary key
	pkInfo, err := c.resolvePK(cat, modelTableName)
	if err != nil {
		return err
	}

	controllerType := controllers.ResourceController

	nullType := c.readNullType()

	fileGen := controllers.NewFileGenerator()
	if err := fileGen.GenerateControllerWithActionsForModel(cat, resourceName, namespace, modelName, tableName, modelTableName, controllerType, modulePath, c.config.Database.Type, tableNameOverridden, modelTableNameOverridden, nullType, pkInfo.ColumnName, inertia, actions, isAPI); err != nil {
		return fmt.Errorf("failed to generate controller: %w", err)
	}

	if isAPI {
		fmt.Printf("Successfully generated API resource controller %s\n", resourceName)
	} else {
		fmt.Printf("Successfully generated resource controller %s with views\n", resourceName)
	}

	return nil
}

func (c *ControllerManager) GenerateControllerFromModel(resourceName string) error {
	modulePath := c.projectManager.GetModulePath()

	var modelFileName strings.Builder
	modelFileName.Grow(len(resourceName) + 3) // +3 for ".go"
	modelFileName.WriteString(naming.ToSnakeCase(resourceName))
	modelFileName.WriteString(".go")
	modelPath := filepath.Join(c.config.Paths.Models, modelFileName.String())
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"model file %s does not exist. Create the model first: andurel model %s create",
			modelPath,
			resourceName,
		)
	}

	if err := c.validator.ValidateResourceName(resourceName); err != nil {
		return err
	}

	tableName, tableNameOverridden := ResolveTableNameWithFlag(c.config.Paths.Models, resourceName)

	if tableNameOverridden {
		if err := c.validator.ValidateTableNameOverride(resourceName, tableName); err != nil {
			return fmt.Errorf("table name validation failed: %w", err)
		}
	} else {
		if err := c.validator.ValidateTableName(tableName); err != nil {
			return fmt.Errorf("table name validation failed: %w", err)
		}
	}

	validationCtx := newControllerValidationContext(resourceName, tableName, "", c.config)
	if err := validateControllerNotExists(validationCtx); err != nil {
		return err
	}

	if _, err := os.Stat(c.config.Paths.Views); os.IsNotExist(err) {
		return fmt.Errorf(
			"views directory %s does not exist. Create views directory before generating controllers",
			c.config.Paths.Views,
		)
	}

	viewPath := filepath.Join(c.config.Paths.Views, controllerNamespacePrefix("")+tableName+"_resource.templ")
	if _, err := os.Stat(viewPath); err == nil {
		return fmt.Errorf("view file %s already exists", viewPath)
	}
	cat, err := c.migrationManager.BuildCatalogFromMigrations(tableName, c.config)
	if err != nil {
		return err
	}

	// Resolve primary key
	pkInfo, err := c.resolvePK(cat, tableName)
	if err != nil {
		return err
	}

	controllerType := controllers.ResourceController

	nullType := c.readNullType()
	inertia := ""

	fileGen := controllers.NewFileGenerator()
	if err := fileGen.GenerateController(cat, resourceName, "", tableName, controllerType, modulePath, c.config.Database.Type, tableNameOverridden, nullType, pkInfo.ColumnName, inertia); err != nil {
		return fmt.Errorf("failed to generate controller: %w", err)
	}

	fmt.Printf("Successfully generated resource controller %s with views\n", resourceName)

	return nil
}

// readNullType reads the nullable type strategy from andurel.lock.
// Defaults to "sql.Null" when not configured.
func (c *ControllerManager) readNullType() string {
	return ReadNullType()
}

// ReadNullType reads the nullable type strategy from andurel.lock.
// Defaults to "sql.Null" when not configured.
func ReadNullType() string {
	fm := files.NewUnifiedFileManager()
	rootDir, err := fm.FindGoModRoot()
	if err != nil {
		return "sql.Null"
	}
	if lock, err := layout.ReadLockFile(rootDir); err == nil && lock.DatabaseConfig != nil && lock.DatabaseConfig.NullType != "" {
		return lock.DatabaseConfig.NullType
	}
	return "sql.Null"
}

func controllerNamespacePrefix(namespace string) string {
	return naming.NamespaceFilePrefix(namespace)
}

// Returns "" when not configured (templ-only mode).
func ReadInertia() string {
	fm := files.NewUnifiedFileManager()
	rootDir, err := fm.FindGoModRoot()
	if err != nil {
		return ""
	}
	if lock, err := layout.ReadLockFile(rootDir); err == nil && lock.ScaffoldConfig != nil && lock.ScaffoldConfig.Inertia != "" {
		return lock.ScaffoldConfig.Inertia
	}
	return ""
}
