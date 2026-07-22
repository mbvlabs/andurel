package generator

import (
	"fmt"
	"go/format"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/generator/models"
	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/pkg/naming"
)

// ModelManager coordinates model operations.
type ModelManager struct {
	validator        *InputValidator
	fileManager      files.Manager
	modelGenerator   *models.Generator
	projectManager   *ProjectManager
	migrationManager *MigrationManager
	config           *UnifiedConfig
	pkResolver       PrimaryKeyResolver
	factoryValidator func(rootDir, factoryPath, content string) error
}

// ModelGenerationOptions controls pure model generation planning.
type ModelGenerationOptions struct {
	TableNameOverride string
	SkipFactory       bool
	PrimaryKeyColumn  string
	Mode              ModelMode
}

// PlannedFile describes one complete file transformation.
type PlannedFile struct {
	Path       string
	OldContent string
	NewContent string
	Exists     bool
}

// ModelGenerationPlan contains every file produced by model generation.
type ModelGenerationPlan struct {
	ResourceName string
	Files        []PlannedFile
}

type modelSetupContext struct {
	ModulePath   string
	RootDir      string
	ModelPath    string
	ResourceName string
	TableName    string
	PluralName   string
}

// NewModelManager creates a new model manager.
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
		factoryValidator: validatePlannedFactory,
	}
}

// SetPrimaryKeyResolver overrides primary key resolution during model generation.
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
	modelsPath := m.config.Paths.Models
	if !filepath.IsAbs(modelsPath) {
		modelsPath = filepath.Join(rootDir, modelsPath)
	}
	modelPath := filepath.Join(modelsPath, modelFileName.String())

	return &modelSetupContext{
		ModulePath:   modulePath,
		RootDir:      rootDir,
		ModelPath:    modelPath,
		ResourceName: resourceName,
		TableName:    tableName,
		PluralName:   pluralName,
	}, nil
}

// GenerateModel generates model files for a resource from project migrations.
func (m *ModelManager) GenerateModel(
	resourceName string,
	tableNameOverride string,
	skipFactory bool,
	primaryKeyColumn string,
) error {
	return m.GenerateModelWithMode(resourceName, tableNameOverride, skipFactory, primaryKeyColumn, models.ModelModeCRUD)
}

// GenerateModelWithMode generates model files with a persisted operation mode.
func (m *ModelManager) GenerateModelWithMode(
	resourceName string,
	tableNameOverride string,
	skipFactory bool,
	primaryKeyColumn string,
	mode models.ModelMode,
) error {
	plan, err := m.PlanModel(resourceName, ModelGenerationOptions{
		TableNameOverride: tableNameOverride,
		SkipFactory:       skipFactory,
		PrimaryKeyColumn:  primaryKeyColumn,
		Mode:              mode,
	})
	if err != nil {
		return err
	}
	if err := m.ApplyModelPlan(plan); err != nil {
		return err
	}

	if !skipFactory {
		fmt.Printf("✓ Generated factory: models/factories/%s.go\n", naming.ToSnakeCase(resourceName))
	}
	fmt.Printf("Successfully generated complete model for %s with database functions\n", resourceName)
	return nil
}

// PlanModel computes every model generation output without writing files.
func (m *ModelManager) PlanModel(resourceName string, options ModelGenerationOptions) (*ModelGenerationPlan, error) {
	tableNameOverride := options.TableNameOverride
	tableName := tableNameOverride
	if tableName == "" {
		tableName = naming.DeriveTableName(resourceName)
	}

	if tableNameOverride != "" {
		if err := m.validator.ValidateTableNameOverride(resourceName, tableNameOverride); err != nil {
			return nil, err
		}
	}

	ctx, err := m.setupModelContext(resourceName, tableName, tableNameOverride != "")
	if err != nil {
		return nil, err
	}

	if err := m.fileManager.ValidateFileNotExists(ctx.ModelPath); err != nil {
		return nil, err
	}

	planningConfig := *m.config
	planningConfig.Database = m.config.Database
	planningConfig.Database.MigrationDirs = append([]string(nil), m.config.Database.MigrationDirs...)
	for index, migrationDir := range planningConfig.Database.MigrationDirs {
		if !filepath.IsAbs(migrationDir) {
			planningConfig.Database.MigrationDirs[index] = filepath.Join(ctx.RootDir, migrationDir)
		}
	}
	cat, err := m.migrationManager.BuildCatalogFromMigrations(ctx.TableName, &planningConfig)
	if err != nil {
		return nil, err
	}

	// Resolve primary key
	var pkInfo PrimaryKeyInfo
	if options.PrimaryKeyColumn != "" {
		pkInfo = PrimaryKeyInfo{
			ColumnName: options.PrimaryKeyColumn,
			Found:      true,
			IsNamedID:  options.PrimaryKeyColumn == "id",
		}
	} else {
		var err error
		pkInfo, err = m.resolvePrimaryKey(cat, ctx.TableName)
		if err != nil {
			return nil, err
		}
	}

	nullType := m.readNullType(ctx.RootDir)
	mode := options.Mode
	if mode == "" {
		mode = models.ModelModeCRUD
	}
	genModel, modelContent, err := m.modelGenerator.PlanModelSource(cat, ctx.ResourceName, ctx.TableName, ctx.ModulePath, tableNameOverride, nullType, pkInfo.ColumnName, !pkInfo.Found, mode)
	if err != nil {
		return nil, fmt.Errorf("plan model source: %w", err)
	}
	plan := &ModelGenerationPlan{
		ResourceName: resourceName,
		Files: []PlannedFile{{
			Path:       ctx.ModelPath,
			NewContent: modelContent,
		}},
	}

	registryPath := filepath.Join(filepath.Dir(ctx.ModelPath), "model.go")
	if m.fileManager.FileExists(registryPath) {
		registryContent, readErr := m.fileManager.ReadFile(registryPath)
		if readErr != nil {
			return nil, fmt.Errorf("read model registry: %w", readErr)
		}
		updatedRegistry, formatErr := planNamespaceRegistration(resourceName, registryContent)
		if formatErr != nil {
			return nil, fmt.Errorf("plan model registry: %w", formatErr)
		}
		if updatedRegistry != registryContent {
			plan.Files = append(plan.Files, PlannedFile{
				Path:       registryPath,
				OldContent: registryContent,
				NewContent: updatedRegistry,
				Exists:     true,
			})
		}
	}

	if !options.SkipFactory {
		genFactory, buildErr := m.modelGenerator.BuildFactory(cat, models.Config{
			TableName:         ctx.TableName,
			ResourceName:      ctx.ResourceName,
			PackageName:       "factories",
			DatabaseType:      m.config.Database.Type,
			ModulePath:        ctx.ModulePath,
			NullType:          nullType,
			PrimaryKeyColumn:  pkInfo.ColumnName,
			GenerateWithoutPK: !pkInfo.Found,
			ModelMode:         mode,
		}, genModel)
		if buildErr != nil {
			return nil, fmt.Errorf("plan factory metadata: %w", buildErr)
		}
		factoryContent, renderErr := m.modelGenerator.PlanFactorySource(genFactory)
		if renderErr != nil {
			return nil, fmt.Errorf("plan factory source: %w", renderErr)
		}
		factoryPath := filepath.Join(filepath.Dir(ctx.ModelPath), "factories", naming.ToSnakeCase(resourceName)+".go")
		plannedFactory := PlannedFile{Path: factoryPath, NewContent: factoryContent}
		if m.fileManager.FileExists(factoryPath) {
			oldFactory, readErr := m.fileManager.ReadFile(factoryPath)
			if readErr != nil {
				return nil, fmt.Errorf("read existing factory: %w", readErr)
			}
			plannedFactory.Exists = true
			plannedFactory.OldContent = oldFactory
		}
		if !plannedFactory.Exists || plannedFactory.OldContent != plannedFactory.NewContent {
			plan.Files = append(plan.Files, plannedFactory)
		}
	}

	return plan, nil
}

// ApplyModelPlan writes the exact content returned by PlanModel.
func (m *ModelManager) ApplyModelPlan(plan *ModelGenerationPlan) error {
	if plan == nil {
		return fmt.Errorf("model generation plan is required")
	}
	for _, file := range plan.Files {
		if err := m.fileManager.WriteFile(file.Path, file.NewContent); err != nil {
			return fmt.Errorf("write planned file %s: %w", file.Path, err)
		}
	}
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

func planNamespaceRegistration(resourceName, source string) (string, error) {
	namespaceType := naming.ToLowerCamelCaseFromAny(resourceName)
	updated := ensureLineInBlock(source, "type (", "\t"+namespaceType+" struct{}")
	updated = ensureLineInBlock(updated, "var (", "\t"+resourceName+" "+namespaceType)
	formatted, err := format.Source([]byte(updated))
	if err != nil {
		return "", err
	}
	return string(formatted), nil
}

// ensureLineInBlock inserts entry as a new line just before the `)` that
// closes the block opened by blockHeader. If the entry is already present in
// the file the source is returned unchanged. If the block does not exist a
// new one is appended.
func ensureLineInBlock(src, blockHeader, entry string) string {
	if strings.Contains(src, entry+"\n") {
		return src
	}

	openIdx := strings.Index(src, blockHeader)
	if openIdx < 0 {
		return src + "\n" + blockHeader + "\n" + entry + "\n)\n"
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
