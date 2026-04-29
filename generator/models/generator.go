package models

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"

	"github.com/jinzhu/inflection"
	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/generator/internal/ddl"
	"github.com/mbvlabs/andurel/generator/internal/migrations"
	"github.com/mbvlabs/andurel/generator/internal/types"
	"github.com/mbvlabs/andurel/generator/internal/validation"
	"github.com/mbvlabs/andurel/generator/templates"
	"github.com/mbvlabs/andurel/pkg/constants"
	"github.com/mbvlabs/andurel/pkg/errors"
	"github.com/mbvlabs/andurel/pkg/naming"
)

type GeneratedField struct {
	Name         string
	Type         string
	Comment      string
	Package      string
	BunTag       string // Full bun struct tag (e.g., `bun:"id,pk,type:uuid"`)
	IsForeignKey bool
	IsNullable   bool
	IsPrimaryKey bool
}

type GeneratedModel struct {
	Name                string
	PluralName          string // The pluralized form of Name for function names (respects --table-name override)
	Package             string
	Fields              []GeneratedField
	StandardImports     []string
	ExternalImports     []string
	Imports             []string
	TableName           string
	TableNameOverride   string
	TableNameOverridden bool
	ModulePath          string
	DatabaseType        string
	IDType              string // "uuid.UUID", "int32", "int64", "string"
	IDGoType            string // Same as IDType (for template clarity)
	IsAutoIncrementID   bool   // True for serial/bigserial
	EntityName          string // ServerEntity (resource name + "Entity")
	NamespaceVar        string // Server (exported, package-scope)
	NamespaceType       string // server (unexported receiver type)
	ReceiverName        string // s (for the namespace methods)
}

type Config struct {
	TableName    string
	ResourceName string
	PackageName  string
	DatabaseType string
	ModulePath   string
	CustomTypes  []types.TypeOverride
}

// BunModelConfig holds configuration for bun model generation
type BunModelConfig struct {
	ResourceName       string
	TableName         string
	ModulePath        string
	PackageName       string
	UseSoftDelete     bool // If true, use deleted_at for soft deletes
}

type Generator struct {
	typeMapper   *types.TypeMapper
	databaseType string
}

func NewGenerator(databaseType string) *Generator {
	return &Generator{
		typeMapper:   types.NewTypeMapper(databaseType),
		databaseType: databaseType,
	}
}

// BuildCatalogFromMigrations builds a catalog from migration files
func (g *Generator) BuildCatalogFromMigrations(tableName string, migrationDirs []string) (*catalog.Catalog, error) {
	allMigrations, err := migrations.DiscoverMigrations(migrationDirs)
	if err != nil {
		return nil, fmt.Errorf("failed to discover migrations: %w", err)
	}

	cat := catalog.NewCatalog("public")

	for _, migration := range allMigrations {
		for _, statement := range migration.Statements {
			if err := ddl.ApplyDDL(cat, statement, migration.FilePath, g.databaseType); err != nil {
				return nil, fmt.Errorf("failed to apply DDL from %s: %w", migration.FilePath, err)
			}
		}
	}

	return cat, nil
}

func (g *Generator) Build(cat *catalog.Catalog, config Config) (*GeneratedModel, error) {
	table, err := cat.GetTable("", config.TableName)
	if err != nil {
		return nil, errors.NewDatabaseError("get table", config.TableName, err)
	}

	g.typeMapper.Overrides = append(g.typeMapper.Overrides, config.CustomTypes...)

	entityName := config.ResourceName + "Entity"
	namespaceVar := config.ResourceName
	namespaceType := naming.ToLowerCamelCaseFromAny(config.ResourceName)
	receiverName := naming.ToReceiverName(config.ResourceName)

	model := &GeneratedModel{
		Name:             config.ResourceName,
		PluralName:       inflection.Plural(config.ResourceName),
		EntityName:       entityName,
		NamespaceVar:     namespaceVar,
		NamespaceType:    namespaceType,
		ReceiverName:     receiverName,
		Package:          config.PackageName,
		TableName:        config.TableName,
		ModulePath:       config.ModulePath,
		DatabaseType:     g.typeMapper.GetDatabaseType(),
		Fields:           make([]GeneratedField, 0, len(table.Columns)),
		StandardImports:  make([]string, 0),
		ExternalImports:  make([]string, 0),
		Imports:          make([]string, 0),
	}

	importSet := make(map[string]bool)
	importSet["context"] = true
	importSet["errors"] = true
	importSet["time"] = true
	importSet["github.com/uptrace/bun"] = true
	if config.ModulePath != "" {
		importSet[config.ModulePath+"/internal/storage"] = true
	}

	for _, col := range table.Columns {
		field, err := g.buildField(col)
		if err != nil {
			return nil, errors.NewGeneratorError("build field", col.Name, err)
		}

		if field.Package != "" {
			importSet[field.Package] = true
		}

		typeImports := g.addModelTypeImports(field.Type)
		for imp := range typeImports {
			importSet[imp] = true
		}

		model.Fields = append(model.Fields, field)

		if col.Name == "id" && col.IsPrimaryKey {
			pkType, _ := validation.ClassifyPrimaryKeyType(col.DataType)
			model.IDType = validation.GoType(pkType)
			model.IDGoType = model.IDType
			model.IsAutoIncrementID = validation.IsAutoIncrement(col.DataType)
		}
	}

	if model.IDType == "uuid.UUID" || model.IDType == "" {
		importSet["github.com/google/uuid"] = true
	}

	stdImports, extImports := groupAndSortImports(importSet)
	model.StandardImports = stdImports
	model.ExternalImports = extImports
	model.Imports = append(
		append(make([]string, 0, len(stdImports)+len(extImports)), stdImports...),
		extImports...)

	return model, nil
}

func groupAndSortImports(importSet map[string]bool) (stdImports []string, extImports []string) {
	for imp := range importSet {
		if strings.Contains(imp, ".") {
			extImports = append(extImports, imp)
		} else {
			stdImports = append(stdImports, imp)
		}
	}
	sort.Strings(stdImports)
	sort.Strings(extImports)
	return stdImports, extImports
}

func (g *Generator) buildField(col *catalog.Column) (GeneratedField, error) {
	goType, pkg, err := g.typeMapper.MapSQLTypeToGo(col.DataType, col.IsNullable)
	if err != nil {
		return GeneratedField{}, err
	}

	bunTag := g.typeMapper.BuildBunTag(col)

	field := GeneratedField{
		Name:         types.FormatFieldName(col.Name),
		Type:         goType,
		Package:      pkg,
		BunTag:       bunTag,
		IsForeignKey: col.ForeignKey != nil,
		IsNullable:   col.IsNullable,
		IsPrimaryKey: col.IsPrimaryKey,
	}

	return field, nil
}

func (g *Generator) addModelTypeImports(goType string) map[string]bool {
	importSet := map[string]bool{}
	if strings.Contains(goType, "time.Time") {
		importSet["time"] = true
	}
	if strings.Contains(goType, "uuid.UUID") {
		importSet["github.com/google/uuid"] = true
	}
	return importSet
}

func (g *Generator) GenerateModelFile(model *GeneratedModel, templateStr string) (string, error) {
	funcMap := template.FuncMap{
		"lower": func(s string) string {
			return strings.ToLower(s)
		},
		"Plural": inflection.Plural,
	}

	tmpl, err := template.New("model").Funcs(funcMap).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, model); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func (g *Generator) GenerateModel(
	cat *catalog.Catalog,
	resourceName string,
	pluralName string,
	modelPath string,
	modulePath string,
	tableNameOverride string,
) error {
	tableName := pluralName
	if tableNameOverride != "" {
		tableName = tableNameOverride
	}

	model, err := g.Build(cat, Config{
		TableName:    tableName,
		ResourceName: resourceName,
		PackageName:  "models",
		DatabaseType: g.typeMapper.GetDatabaseType(),
		ModulePath:   modulePath,
	})
	if err != nil {
		return fmt.Errorf("failed to build model: %w", err)
	}

	model.TableNameOverride = tableNameOverride
	model.TableNameOverridden = tableNameOverride != ""
	// When table name is overridden, don't pluralize the resource name for function names
	if tableNameOverride != "" {
		model.PluralName = resourceName
	} else {
		model.PluralName = inflection.Plural(resourceName)
	}

	templateContent, err := templates.Files.ReadFile("model.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read model template: %w", err)
	}

	modelContent, err := g.GenerateModelFile(model, string(templateContent))
	if err != nil {
		return fmt.Errorf("failed to render model file: %w", err)
	}

	if err := os.WriteFile(modelPath, []byte(modelContent), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write model file: %w", err)
	}

	if err := files.FormatGoFile(modelPath); err != nil {
		return fmt.Errorf("failed to format model file: %w", err)
	}

	return nil
}

// GeneratedFactory represents a factory for a model
type GeneratedFactory struct {
	ModelName         string
	EntityName        string // ServerEntity (resource name + "Entity")
	NamespaceVar      string // Server (exported, package-scope)
	Package           string
	Fields            []FactoryField
	ModulePath        string
	StandardImports   []string
	ExternalImports   []string
	HasCreateFunction bool
	HasForeignKeys    bool           // True if there are 1+ FKs
	ForeignKeyFields  []FactoryField // All FK fields
	IDType            string         // "uuid.UUID", "int32", "int64", "string"
	IsAutoIncrementID bool           // True for serial/bigserial
}

// FactoryField represents a field in a factory
type FactoryField struct {
	Name          string
	ArgumentName  string
	Type          string
	GoZero        string
	DefaultValue  string
	OptionName    string
	IsFK          bool
	IsTimestamp   bool
	IsID          bool
	IsAutoManaged bool
}

// BuildFactory generates factory metadata from a model
func (g *Generator) BuildFactory(cat *catalog.Catalog, config Config, genModel *GeneratedModel) (*GeneratedFactory, error) {
	// Import the factory analyzer
	factoryFields := make([]FactoryField, 0, len(genModel.Fields))

	for _, field := range genModel.Fields {
		fieldInfo := g.analyzeFactoryField(field, config.TableName)
		factoryFields = append(factoryFields, fieldInfo)
	}

	// Check for foreign keys
	var fkFields []FactoryField
	for _, field := range factoryFields {
		if field.IsFK {
			fkFields = append(fkFields, field)
		}
	}

	// Collect imports - context and fmt are already in the template
	standardImports := []string{}
	externalImports := []string{
		"github.com/go-faker/faker/v4",
	}

	// Only add uuid import if ID type uses UUID
	if genModel.IDType == "uuid.UUID" || genModel.IDType == "" {
		externalImports = append(externalImports, "github.com/google/uuid")
	}

	// Add time import if needed
	for _, field := range factoryFields {
		if field.IsTimestamp {
			standardImports = append(standardImports, "time")
			break
		}
	}

	return &GeneratedFactory{
		ModelName:         genModel.Name,
		EntityName:        genModel.EntityName,
		NamespaceVar:      genModel.NamespaceVar,
		Package:           "factories",
		Fields:            factoryFields,
		ModulePath:        config.ModulePath,
		StandardImports:   standardImports,
		ExternalImports:   externalImports,
		HasCreateFunction: true, // Assume Create function exists
		HasForeignKeys:    len(fkFields) > 0,
		ForeignKeyFields:  fkFields,
		IDType:            genModel.IDType,
		IsAutoIncrementID: genModel.IsAutoIncrementID,
	}, nil
}

// analyzeFactoryField analyzes a field and returns factory metadata
func (g *Generator) analyzeFactoryField(field GeneratedField, tableName string) FactoryField {
	info := FactoryField{
		Name:          field.Name,
		ArgumentName:  naming.ToLowerCamelCase(field.Name),
		Type:          field.Type,
		OptionName:    fmt.Sprintf("With%s%s", naming.Capitalize(naming.ToCamelCase(tableName)), field.Name),
		IsID:          field.Name == "ID",
		IsTimestamp:   field.Type == "time.Time" || strings.Contains(field.Type, "Time"),
		IsAutoManaged: field.Name == "ID" || field.Name == "CreatedAt" || field.Name == "UpdatedAt",
		IsFK:          field.IsForeignKey,
	}

	// Determine default value
	info.DefaultValue = g.determineFactoryDefault(field.Name, field.Type)
	info.GoZero = g.getFactoryGoZero(field.Type)

	return info
}

func (g *Generator) determineFactoryDefault(fieldName, goType string) string {
	// Handle by type first
	switch goType {
	case "string":
		return "faker.Word()"
	case "int32", "int":
		return "randomInt(1, 1000, 100)"
	case "int64":
		return "randomInt64(1, 1000, 100)"
	case "int16":
		return "randomInt16(1, 1000, 100)"
	case "bool":
		return "randomBool()"
	case "time.Time":
		return "time.Time{}"
	case "uuid.UUID":
		return "uuid.UUID{}"
	case "[]byte":
		return "[]byte{}"
	}

	// Default fallback
	return fmt.Sprintf("%s{}", goType)
}

func (g *Generator) stringFactoryDefault(fieldName string) string {
	lower := strings.ToLower(fieldName)

	// Field name heuristics
	switch {
	case lower == "email":
		return "faker.Email()"
	case lower == "name" || strings.HasSuffix(lower, "name"):
		return "faker.Name()"
	case lower == "phone" || strings.Contains(lower, "phone"):
		return "faker.Phonenumber()"
	case lower == "url" || strings.Contains(lower, "url"):
		return "faker.URL()"
	case lower == "description" || strings.HasSuffix(lower, "description"):
		return "faker.Sentence()"
	case lower == "title" || strings.HasSuffix(lower, "title"):
		return "faker.Word()"
	case lower == "address" || strings.Contains(lower, "address"):
		return "faker.GetRealAddress().Address"
	case lower == "city":
		return "faker.GetRealAddress().City"
	case lower == "country":
		return "faker.GetRealAddress().Country"
	case lower == "zipcode" || lower == "postalcode":
		return "faker.GetRealAddress().PostalCode"
	case strings.Contains(lower, "color"):
		return "faker.GetRandomColor()"
	default:
		return "faker.Word()"
	}
}

func (g *Generator) intFactoryDefault(fieldName string) string {
	lower := strings.ToLower(fieldName)

	switch {
	case strings.Contains(lower, "price") || strings.Contains(lower, "amount"):
		return "faker.RandomInt(100, 10000)" // Price in cents
	case strings.Contains(lower, "count") || strings.Contains(lower, "quantity"):
		return "faker.RandomInt(1, 100)"
	case strings.Contains(lower, "age"):
		return "faker.RandomInt(18, 80)"
	default:
		return "faker.RandomInt(1, 1000)"
	}
}

func (g *Generator) getFactoryGoZero(goType string) string {
	switch goType {
	case "string":
		return `""`
	case "int", "int32", "int64", "float32", "float64":
		return "0"
	case "bool":
		return "false"
	case "time.Time":
		return "time.Time{}"
	case "uuid.UUID":
		return "uuid.UUID{}"
	case "[]byte":
		return "nil"
	default:
		if strings.HasPrefix(goType, "[]") {
			return "nil"
		}
		return fmt.Sprintf("%s{}", goType)
	}
}

// GenerateFactoryFile renders a factory file from a template
func (g *Generator) GenerateFactoryFile(factory *GeneratedFactory, templateStr string) (string, error) {
	funcMap := template.FuncMap{
		"toLower": func(s string) string {
			return strings.ToLower(s)
		},
	}

	tmpl, err := template.New("factory").Funcs(funcMap).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse factory template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, factory); err != nil {
		return "", fmt.Errorf("failed to execute factory template: %w", err)
	}

	return buf.String(), nil
}

// WriteFactoryFile writes a factory file to disk
func (g *Generator) WriteFactoryFile(factory *GeneratedFactory, outputDir string) error {
	// Read factory template
	templateContent, err := templates.Files.ReadFile("factory.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read factory template: %w", err)
	}

	// Generate factory content
	factoryContent, err := g.GenerateFactoryFile(factory, string(templateContent))
	if err != nil {
		return fmt.Errorf("failed to render factory file: %w", err)
	}

	// Determine output path using snake_case for consistency with model files
	fileName := fmt.Sprintf("%s.go", naming.ToSnakeCase(factory.ModelName))
	outputPath := fmt.Sprintf("%s/models/factories/%s", outputDir, fileName)

	// Ensure directory exists
	if err := os.MkdirAll(fmt.Sprintf("%s/models/factories", outputDir), 0755); err != nil {
		return fmt.Errorf("failed to create factories directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(outputPath, []byte(factoryContent), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write factory file: %w", err)
	}

	// Format the file
	if err := files.FormatGoFile(outputPath); err != nil {
		return fmt.Errorf("failed to format factory file: %w", err)
	}

	return nil
}
