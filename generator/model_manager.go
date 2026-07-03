package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/generator/models"
	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/pkg/naming"
)

type ModelManager struct {
	validator        *InputValidator
	fileManager      files.Manager
	modelGenerator   *models.Generator
	projectManager   *ProjectManager
	migrationManager *MigrationManager
	config           *UnifiedConfig
	pkResolver       PrimaryKeyResolver
}

type modelSetupContext struct {
	ModulePath   string
	RootDir      string
	ModelPath    string
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
		pkResolver:       DefaultPrimaryKeyResolver{},
	}
}

func (m *ModelManager) SetPrimaryKeyResolver(resolver PrimaryKeyResolver) {
	m.pkResolver = resolver
}

func (m *ModelManager) setupModelContext(
	resourceName, tableName string,
	tableNameOverridden bool,
) (*modelSetupContext, error) {
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

	pluralName := naming.DeriveTableName(resourceName)

	var modelFileName strings.Builder
	modelFileName.Grow(len(resourceName) + 3)
	modelFileName.WriteString(naming.ToSnakeCase(resourceName))
	modelFileName.WriteString(".go")
	modelPath := filepath.Join(m.config.Paths.Models, modelFileName.String())

	return &modelSetupContext{
		ModulePath:   modulePath,
		RootDir:      rootDir,
		ModelPath:    modelPath,
		ResourceName: resourceName,
		TableName:    tableName,
		PluralName:   pluralName,
	}, nil
}

func (m *ModelManager) GenerateModel(
	resourceName string,
	tableNameOverride string,
	skipFactory bool,
	primaryKeyColumn string,
) error {
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

	cat, err := m.migrationManager.BuildCatalogFromMigrations(ctx.TableName, m.config)
	if err != nil {
		return err
	}

	// Resolve primary key
	var pkInfo PrimaryKeyInfo
	if primaryKeyColumn != "" {
		pkInfo = PrimaryKeyInfo{
			ColumnName: primaryKeyColumn,
			Found:      true,
			IsNamedID:  primaryKeyColumn == "id",
		}
	} else {
		var err error
		pkInfo, err = m.resolvePrimaryKey(cat, ctx.TableName)
		if err != nil {
			return err
		}
	}

	nullType := m.readNullType(ctx.RootDir)

	if err := m.modelGenerator.GenerateModel(cat, ctx.ResourceName, ctx.TableName, ctx.ModelPath, ctx.ModulePath, tableNameOverride, nullType, pkInfo.ColumnName, !pkInfo.Found); err != nil {
		return fmt.Errorf("failed to generate model: %w", err)
	}

	if err := m.registerNamespace(ctx.ResourceName); err != nil {
		return fmt.Errorf("failed to register namespace in models/model.go: %w", err)
	}

	// Generate factory (unless skipped)
	if !skipFactory {
		if err := m.generateFactory(cat, ctx, pkInfo); err != nil {
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

// resolvePrimaryKey inspects the catalog for the table's primary key and
// interacts with the user if the PK is non-standard or missing.
func (m *ModelManager) resolvePrimaryKey(cat *catalog.Catalog, tableName string) (PrimaryKeyInfo, error) {
	pkInfo := DetectPrimaryKey(cat, tableName)

	if !pkInfo.Found {
		ok, err := m.pkResolver.ConfirmNoPK(tableName)
		if err != nil {
			return PrimaryKeyInfo{}, err
		}
		if !ok {
			return PrimaryKeyInfo{}, fmt.Errorf("generation aborted: table %q has no primary key", tableName)
		}
		return PrimaryKeyInfo{Found: false}, nil
	}

	if !pkInfo.IsNamedID {
		resolved, err := m.pkResolver.ResolveAlternatePK(pkInfo, tableName)
		if err != nil {
			return PrimaryKeyInfo{}, err
		}
		return resolved, nil
	}

	return pkInfo, nil
}

// generateFactory creates a factory file for the model
func (m *ModelManager) generateFactory(cat *catalog.Catalog, ctx *modelSetupContext, pkInfo PrimaryKeyInfo) error {
	// Get root directory
	rootDir, err := m.fileManager.FindGoModRoot()
	if err != nil {
		return fmt.Errorf("failed to find go.mod root: %w", err)
	}

	nullType := m.readNullType(rootDir)

	// Build the model first
	genModel, err := m.modelGenerator.Build(cat, models.Config{
		TableName:         ctx.TableName,
		ResourceName:      ctx.ResourceName,
		PackageName:       "models",
		DatabaseType:      m.config.Database.Type,
		ModulePath:        ctx.ModulePath,
		NullType:          nullType,
		PrimaryKeyColumn:  pkInfo.ColumnName,
		GenerateWithoutPK: !pkInfo.Found,
	})
	if err != nil {
		return fmt.Errorf("failed to build model for factory: %w", err)
	}

	// Build factory metadata
	genFactory, err := m.modelGenerator.BuildFactory(cat, models.Config{
		TableName:         ctx.TableName,
		ResourceName:      ctx.ResourceName,
		PackageName:       "factories",
		DatabaseType:      m.config.Database.Type,
		ModulePath:        ctx.ModulePath,
		NullType:          nullType,
		PrimaryKeyColumn:  pkInfo.ColumnName,
		GenerateWithoutPK: !pkInfo.Found,
	}, genModel)
	if err != nil {
		return fmt.Errorf("failed to build factory: %w", err)
	}

	// Write factory file
	if err := m.modelGenerator.WriteFactoryFile(genFactory, rootDir); err != nil {
		return fmt.Errorf("failed to write factory: %w", err)
	}

	return nil
}

// registerNamespace ensures the project's models/model.go declares the
// `<type> struct{}` and `<Var> <type>` entries for the new resource so
// per-resource files (which only define methods on the namespace type)
// compile.
func (m *ModelManager) registerNamespace(resourceName string) error {
	modelGoPath := filepath.Join(m.config.Paths.Models, "model.go")

	src, err := os.ReadFile(modelGoPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	namespaceVar := resourceName
	namespaceType := naming.ToLowerCamelCaseFromAny(resourceName)
	typeEntry := "\t" + namespaceType + " struct{}"
	varEntry := "\t" + namespaceVar + " " + namespaceType

	updated := ensureLineInBlock(string(src), "type (", typeEntry)
	updated = ensureLineInBlock(updated, "var (", varEntry)

	if updated == string(src) {
		return nil
	}

	return os.WriteFile(modelGoPath, []byte(updated), 0o644)
}

// ensureLineInBlock inserts entry as a new line just before the `)` that
// closes the block opened by openMarker. If the entry is already present in
// the file the source is returned unchanged. If the block does not exist a
// new one is appended.
func ensureLineInBlock(src, openMarker, entry string) string {
	if strings.Contains(src, entry+"\n") {
		return src
	}

	openIdx := strings.Index(src, openMarker)
	if openIdx < 0 {
		return src + "\n" + openMarker + "\n" + entry + "\n)\n"
	}

	closeRel := strings.Index(src[openIdx:], "\n)")
	if closeRel < 0 {
		return src
	}
	insertAt := openIdx + closeRel + 1

	return src[:insertAt] + entry + "\n" + src[insertAt:]
}

// readNullType reads the nullable type strategy from andurel.lock.
// Defaults to "sql.Null" when not configured.
func (m *ModelManager) readNullType(rootDir string) string {
	if lock, err := layout.ReadLockFile(rootDir); err == nil && lock.DatabaseConfig != nil && lock.DatabaseConfig.NullType != "" {
		return lock.DatabaseConfig.NullType
	}
	return "sql.Null"
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
