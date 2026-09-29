package generator

import (
	"fmt"
	"go/format"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/mbvlabs/andurel/v2/generator/files"
	"github.com/mbvlabs/andurel/v2/generator/internal/catalog"
	"github.com/mbvlabs/andurel/v2/generator/models"
	"github.com/mbvlabs/andurel/v2/generator/templates"
	"github.com/mbvlabs/andurel/v2/internal/naming"
	"github.com/mbvlabs/andurel/v2/internal/testseed"
	"github.com/mbvlabs/andurel/v2/layout"
)

type factoryValidationHook struct {
	validate func(rootDir, factoryPath, content string) error
}

// ModelManager coordinates model operations.
type ModelManager struct {
	validator        *InputValidator
	fileManager      files.Manager
	modelGenerator   *models.Generator
	projectManager   *ProjectManager
	migrationManager *MigrationManager
	config           *UnifiedConfig
	pkResolver       PrimaryKeyResolver
	factoryValidator *factoryValidationHook
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
		factoryValidator: factoryValidatorForEnv(),
	}
}

// factoryValidatorForEnv skips packages.Load type-checks under andurel_golden.
// Golden fixtures stay minimal (no module graph); --check still reports drift.
func factoryValidatorForEnv() *factoryValidationHook {
	if testseed.Enabled() {
		return nil
	}
	return &factoryValidationHook{validate: validatePlannedFactory}
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

	if err := m.validator.ValidateModelResourceName(resourceName); err != nil {
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
	return m.GenerateModelWithMode(
		resourceName,
		tableNameOverride,
		skipFactory,
		primaryKeyColumn,
		models.ModelModeCRUD,
	)
}

// GenerateModelWithMode generates model files with a restricted operation mode.
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
	return m.ApplyModelPlan(plan)
}

// PlanModel computes every model generation output without writing files.
func (m *ModelManager) PlanModel(
	resourceName string,
	options ModelGenerationOptions,
) (*ModelGenerationPlan, error) {
	tableNameOverride := options.TableNameOverride
	tableName := tableNameOverride
	if tableName == "" {
		tableName = naming.DeriveTableName(resourceName)
	}

	if tableNameOverride != "" {
		if err := m.validator.ValidateTableNameOverride(
			resourceName,
			tableNameOverride,
		); err != nil {
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
	planningConfig.Database.MigrationDirs = append(
		[]string(nil),
		m.config.Database.MigrationDirs...)
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
	genModel, modelContent, err := m.modelGenerator.PlanModelSource(
		cat,
		ctx.ResourceName,
		ctx.TableName,
		ctx.ModulePath,
		tableNameOverride,
		nullType,
		pkInfo.ColumnName,
		!pkInfo.Found,
		mode,
	)
	if err != nil {
		return nil, fmt.Errorf("plan model source: %w", err)
	}
	queryContent, queryErr := m.modelGenerator.PlanQuerySource(genModel)
	if queryErr != nil {
		return nil, fmt.Errorf("plan query source: %w", queryErr)
	}
	queryPath := filepath.Join(
		filepath.Dir(ctx.ModelPath),
		"queries",
		naming.ToSnakeCase(resourceName)+".sql",
	)
	plan := &ModelGenerationPlan{
		ResourceName: resourceName,
		Files: []PlannedFile{
			{
				Path:       ctx.ModelPath,
				NewContent: modelContent,
			},
			{
				Path:       queryPath,
				NewContent: queryContent,
			},
		},
	}

	registryPath := filepath.Join(filepath.Dir(ctx.ModelPath), "model.go")
	if m.fileManager.FileExists(registryPath) {
		registryContent, readErr := m.fileManager.ReadFile(registryPath)
		if readErr != nil {
			return nil, fmt.Errorf("read model registry: %w", readErr)
		}
		updatedRegistry, formatErr := planModelRegistration(resourceName, registryContent)
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
		factoryPath := filepath.Join(
			filepath.Dir(ctx.ModelPath),
			"factories",
			naming.ToSnakeCase(resourceName)+".go",
		)
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

// GenerateCustomModel generates a non-table-backed model from field:type specs.
func (m *ModelManager) GenerateCustomModel(resourceName string, fieldSpecs []string) error {
	plan, err := m.PlanCustomModel(resourceName, fieldSpecs)
	if err != nil {
		return err
	}
	return m.ApplyModelPlan(plan)
}

// PlanCustomModel computes custom model generation output without writing files.
func (m *ModelManager) PlanCustomModel(
	resourceName string,
	fieldSpecs []string,
) (*ModelGenerationPlan, error) {
	fields, fieldStdImports, fieldExtImports, err := ParseCustomFieldSpecs(fieldSpecs)
	if err != nil {
		return nil, err
	}

	ctx, err := m.setupCustomModelContext(resourceName)
	if err != nil {
		return nil, err
	}

	if err := m.fileManager.ValidateFileNotExists(ctx.ModelPath); err != nil {
		return nil, err
	}

	serviceName := naming.ModelServiceName(resourceName)
	data := CustomModelData{
		EntityName:    resourceName,
		NamespaceVar:  serviceName,
		NamespaceType: serviceName,
		ReceiverName:  naming.ToReceiverName(resourceName),
		ModulePath:    ctx.ModulePath,
		Fields:        fields,
	}

	importSet := make(map[string]bool)
	for _, imp := range fieldStdImports {
		importSet[imp] = true
	}
	for _, imp := range fieldExtImports {
		importSet[imp] = true
	}
	if ctx.ModulePath != "" {
		importSet[ctx.ModulePath+"/models/internal/queries"] = true
	}
	importSet["github.com/mbvlabs/andurel/pkg/storage"] = true
	importSet["github.com/mbvlabs/andurel/pkg/validation"] = true
	data.StandardImports, data.ExternalImports = groupAndSortCustomImports(importSet)

	modelContent, err := renderCustomModelSource(data)
	if err != nil {
		return nil, fmt.Errorf("plan custom model source: %w", err)
	}

	queryContent, err := templates.RenderTemplateUsingGlobal("narsilc_query.tmpl", struct {
		PascalName string
		TableName  string
	}{
		PascalName: resourceName,
		TableName:  "",
	})
	if err != nil {
		return nil, fmt.Errorf("plan custom query stub: %w", err)
	}

	queryPath := filepath.Join(
		filepath.Dir(ctx.ModelPath),
		"queries",
		naming.ToSnakeCase(resourceName)+".sql",
	)
	plan := &ModelGenerationPlan{
		ResourceName: resourceName,
		Files: []PlannedFile{
			{
				Path:       ctx.ModelPath,
				NewContent: modelContent,
			},
			{
				Path:       queryPath,
				NewContent: queryContent,
			},
		},
	}

	registryPath := filepath.Join(filepath.Dir(ctx.ModelPath), "model.go")
	if m.fileManager.FileExists(registryPath) {
		registryContent, readErr := m.fileManager.ReadFile(registryPath)
		if readErr != nil {
			return nil, fmt.Errorf("read model registry: %w", readErr)
		}
		updatedRegistry, formatErr := planModelRegistration(resourceName, registryContent)
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

	return plan, nil
}

func (m *ModelManager) setupCustomModelContext(resourceName string) (*modelSetupContext, error) {
	modulePath := m.projectManager.GetModulePath()

	if err := m.validator.ValidateModelResourceName(resourceName); err != nil {
		return nil, err
	}
	if err := m.validator.ValidateModulePath(modulePath); err != nil {
		return nil, fmt.Errorf("module path validation failed: %w", err)
	}

	rootDir, err := m.fileManager.FindGoModRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to find go.mod root: %w", err)
	}

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
		PluralName:   naming.ModelServiceName(resourceName),
	}, nil
}

func renderCustomModelSource(data CustomModelData) (string, error) {
	templateContent, err := templates.Files.ReadFile("model_custom.tmpl")
	if err != nil {
		return "", fmt.Errorf("failed to read custom model template: %w", err)
	}

	tmpl, err := template.New("model_custom").Parse(string(templateContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse custom model template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute custom model template: %w", err)
	}

	formatted, err := format.Source([]byte(buf.String()))
	if err != nil {
		return "", fmt.Errorf("failed to format custom model source: %w", err)
	}
	return string(formatted), nil
}

// resolvePrimaryKey inspects the catalog for the table's primary key and
// interacts with the user if the PK is non-standard or missing.
func (m *ModelManager) resolvePrimaryKey(
	cat *catalog.Catalog,
	tableName string,
) (PrimaryKeyInfo, error) {
	pkInfo := DetectPrimaryKey(cat, tableName)

	if !pkInfo.Found {
		ok, err := m.pkResolver.ConfirmNoPK(tableName)
		if err != nil {
			return PrimaryKeyInfo{}, err
		}
		if !ok {
			return PrimaryKeyInfo{}, fmt.Errorf(
				"generation aborted: table %q has no primary key",
				tableName,
			)
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

func planModelRegistration(resourceName, source string) (string, error) {
	constructor := "New" + naming.ModelServiceName(resourceName)
	if strings.Contains(source, constructor+",") || strings.Contains(source, constructor+")") {
		formatted, err := format.Source([]byte(source))
		if err != nil {
			return "", err
		}
		return string(formatted), nil
	}

	provideIdx := strings.Index(source, "fx.Provide(")
	if provideIdx < 0 {
		return "", fmt.Errorf("failed to locate fx.Provide in models module")
	}
	openIdx := strings.Index(source[provideIdx:], "(")
	if openIdx < 0 {
		return "", fmt.Errorf("failed to locate fx.Provide opening parenthesis")
	}
	openIdx += provideIdx
	closeIdx := findMatchingParen(source, openIdx)
	if closeIdx < 0 {
		return "", fmt.Errorf("failed to locate fx.Provide closing parenthesis")
	}

	updated := source[:closeIdx] + "\t" + constructor + ",\n" + source[closeIdx:]
	formatted, err := format.Source([]byte(updated))
	if err != nil {
		return "", err
	}
	return string(formatted), nil
}

func findMatchingParen(src string, openIdx int) int {
	depth := 0
	for i := openIdx; i < len(src); i++ {
		switch src[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// readNullType reads the nullable type strategy from andurel.lock.
// Defaults to pgtype.Null when not configured.
func (m *ModelManager) readNullType(rootDir string) string {
	if lock, err := layout.ReadLockFile(
		rootDir,
	); err == nil && lock.DatabaseConfig != nil &&
		lock.DatabaseConfig.NullType != "" {
		return lock.DatabaseConfig.NullType
	}
	return layout.NullTypePGType
}
