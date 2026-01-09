package models

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/generator/internal/ddl"
	"github.com/mbvlabs/andurel/generator/internal/migrations"
	"github.com/mbvlabs/andurel/generator/internal/types"
	"github.com/mbvlabs/andurel/generator/templates"
	"github.com/mbvlabs/andurel/pkg/constants"
	"github.com/mbvlabs/andurel/pkg/errors"
	"github.com/mbvlabs/andurel/pkg/naming"
)

type GeneratedField struct {
	Name                    string
	Type                    string
	Comment                 string
	Package                 string
	SQLCType                string
	ConversionFromDB        string
	ConversionToDB          string
	ConversionToDBForUpdate string
	ZeroCheck               string
	IsForeignKey            bool
}

type GeneratedModel struct {
	Name              string
	Package           string
	Fields            []GeneratedField
	StandardImports   []string
	ExternalImports   []string
	Imports           []string
	TableName         string
	TableNameOverride string
	ModulePath        string
	DatabaseType      string
}

type Config struct {
	TableName    string
	ResourceName string
	PackageName  string
	DatabaseType string
	ModulePath   string
	CustomTypes  []types.TypeOverride
}

type SQLData struct {
	ResourceName       string
	PluralName         string
	InsertColumns      string
	InsertPlaceholders string
	UpdateColumns      string
	DatabaseType       string
	IDPlaceholder      string
	LimitOffsetClause  string
	NowFunction        string
	UpsertUpdateSet    string
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

func (g *Generator) Build(cat *catalog.Catalog, config Config) (*GeneratedModel, error) {
	table, err := cat.GetTable("", config.TableName)
	if err != nil {
		return nil, errors.NewDatabaseError("get table", config.TableName, err)
	}

	g.typeMapper.Overrides = append(g.typeMapper.Overrides, config.CustomTypes...)

	model := &GeneratedModel{
		Name:            config.ResourceName,
		Package:         config.PackageName,
		TableName:       config.TableName,
		ModulePath:      config.ModulePath,
		DatabaseType:    g.typeMapper.GetDatabaseType(),
		Fields:          make([]GeneratedField, 0, len(table.Columns)),
		StandardImports: make([]string, 0),
		ExternalImports: make([]string, 0),
		Imports:         make([]string, 0),
	}

	importSet := make(map[string]bool)

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

		if strings.Contains(field.SQLCType, "pgtype.") {
			importSet["github.com/jackc/pgx/v5/pgtype"] = true
		}

		model.Fields = append(model.Fields, field)
	}

	// Don't force all imports - only add them if they're actually needed
	importSet["github.com/google/uuid"] = true

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
	var goType, sqlcType, pkg string
	var err error

	goType, sqlcType, _, err = g.typeMapper.MapSQLTypeToGo(col.DataType, col.IsNullable)
	if err != nil {
		return GeneratedField{}, err
	}

	goType = g.getSimpleGoType(goType, sqlcType)
	pkg = g.getSimpleGoTypePackage(goType)

	field := GeneratedField{
		Name:         types.FormatFieldName(col.Name),
		Type:         goType,
		SQLCType:     sqlcType,
		Package:      pkg,
		IsForeignKey: col.ForeignKey != nil,
	}

	field.ConversionFromDB = g.typeMapper.GenerateConversionFromDB(
		field.Name,
		field.SQLCType,
		field.Type,
	)

	if col.Name == "created_at" || col.Name == "updated_at" {
		field.ConversionToDB = ""
	} else {
		field.ConversionToDB = g.typeMapper.GenerateConversionToDB(field.SQLCType, field.Type, "data."+field.Name)
	}

	if col.Name == "updated_at" {
		field.ConversionToDBForUpdate = ""
	} else {
		field.ConversionToDBForUpdate = g.typeMapper.GenerateConversionToDB(
			field.SQLCType,
			field.Type,
			"data."+field.Name,
		)
	}

	field.ZeroCheck = g.typeMapper.GenerateZeroCheck(field.Type, "data."+field.Name)

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

func (g *Generator) getSimpleGoType(goType, sqlcType string) string {
	// If it's already a simple Go type, keep it
	if !strings.Contains(goType, "pgtype.") && !strings.Contains(goType, "sql.") {
		return goType
	}

	// Convert pgtype and sql types to simple Go types
	switch {
	case strings.Contains(sqlcType, "pgtype.Int4"):
		return "int32"
	case strings.Contains(sqlcType, "pgtype.Int8"):
		return "int64"
	case strings.Contains(sqlcType, "pgtype.Int2"):
		return "int16"
	case strings.Contains(sqlcType, "pgtype.Float4"):
		return "float32"
	case strings.Contains(sqlcType, "pgtype.Float8"):
		return "float64"
	case strings.Contains(sqlcType, "pgtype.Bool"):
		return "bool"
	case strings.Contains(sqlcType, "pgtype.Text"):
		return "string"
	case strings.Contains(sqlcType, "pgtype.Timestamp"),
		strings.Contains(sqlcType, "pgtype.Date"),
		strings.Contains(sqlcType, "pgtype.Time"):
		return "time.Time"
	case strings.Contains(sqlcType, "pgtype.JSONB"),
		strings.Contains(sqlcType, "pgtype.JSON"):
		return "[]byte"
	case strings.Contains(sqlcType, "sql.NullString"):
		return "string"
	case strings.Contains(sqlcType, "sql.NullInt64"):
		return "int64"
	case strings.Contains(sqlcType, "sql.NullFloat64"):
		return "float64"
	case strings.Contains(sqlcType, "sql.NullBool"):
		return "bool"
	case strings.Contains(sqlcType, "sql.NullTime"):
		return "time.Time"
	default:
		return goType
	}
}

func (g *Generator) getSimpleGoTypePackage(goType string) string {
	switch {
	case strings.Contains(goType, "time.Time"):
		return "" // time is handled by addModelTypeImports
	case strings.Contains(goType, "uuid.UUID"):
		return "" // uuid is handled by addModelTypeImports
	default:
		return ""
	}
}

func (g *Generator) GenerateModelFile(model *GeneratedModel, templateStr string) (string, error) {
	funcMap := template.FuncMap{
		"lower": func(s string) string {
			return strings.ToLower(s)
		},
		"toUpper": func(s string) string {
			return strings.ToUpper(s)
		},
		"uuidParam": func(param string) string {
			return param
		},
		"hasErrorHandling": func() bool {
			return false
		},
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
	modelPath, sqlPath string,
	modulePath string,
	tableNameOverride string,
) error {
	table, err := cat.GetTable("", pluralName)
	if err != nil {
		return fmt.Errorf(`table '%s' not found in catalog: %w

Convention: Model names must be singular, table names must be plural snake_case.
Example: Model 'UserAccount' expects table 'user_accounts'

To use a different table name, run:
  andurel generate model %s --table-name=your_table_name`,
			pluralName, err, resourceName)
	}

	if err := g.GenerateSQLFile(resourceName, pluralName, table, sqlPath); err != nil {
		return fmt.Errorf("failed to generate SQL file: %w", err)
	}

	model, err := g.Build(cat, Config{
		TableName:    pluralName,
		ResourceName: resourceName,
		PackageName:  "models",
		DatabaseType: g.typeMapper.GetDatabaseType(),
		ModulePath:   modulePath,
	})
	if err != nil {
		return fmt.Errorf("failed to build model: %w", err)
	}

	model.TableNameOverride = tableNameOverride

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

func (g *Generator) GenerateSQLFile(
	resourceName string,
	pluralName string,
	table *catalog.Table,
	sqlPath string,
) error {
	data := g.prepareSQLData(resourceName, pluralName, table)

	// Use the unified template service
	service := templates.GetGlobalTemplateService()
	content, err := service.RenderTemplate("crud_operations.tmpl", data)
	if err != nil {
		return errors.WrapTemplateError(err, "generate SQL", "crud_operations.tmpl")
	}

	if err := os.WriteFile(sqlPath, []byte(content), constants.FilePermissionPrivate); err != nil {
		return errors.WrapFileError(err, "write SQL file", sqlPath)
	}
	return nil
}

func (g *Generator) GenerateSQLContent(
	resourceName string,
	pluralName string,
	table *catalog.Table,
) (string, error) {
	data := g.prepareSQLData(resourceName, pluralName, table)

	// Use the unified template service
	service := templates.GetGlobalTemplateService()
	result, err := service.RenderTemplate("crud_operations.tmpl", data)
	if err != nil {
		return "", errors.WrapTemplateError(err, "generate SQL content", "crud_operations.tmpl")
	}
	return result, nil
}

func (g *Generator) prepareSQLData(
	resourceName string,
	pluralName string,
	table *catalog.Table,
) SQLData {
	var insertColumns []string
	var insertPlaceholders []string
	var updateColumns []string
	var upsertUpdateColumns []string

	var placeholderFunc func(int) string
	var nowFunc string
	var idPlaceholder string
	var limitOffsetClause string

	if g.typeMapper.GetDatabaseType() == "postgresql" {
		placeholderFunc = func(i int) string { return fmt.Sprintf("$%d", i) }
		nowFunc = "now()"
		idPlaceholder = "$1"
		limitOffsetClause = "limit sqlc.arg('limit')::bigint offset sqlc.arg('offset')::bigint"
	}

	placeholderIndex := 1

	for _, col := range table.Columns {
		insertColumns = append(insertColumns, col.Name)

		if col.Name == "created_at" || col.Name == "updated_at" {
			insertPlaceholders = append(insertPlaceholders, nowFunc)
		} else {
			insertPlaceholders = append(
				insertPlaceholders,
				placeholderFunc(placeholderIndex),
			)
			placeholderIndex++
		}
	}

	placeholderIndex = 2
	for _, col := range table.Columns {
		if col.Name != "id" && col.Name != "created_at" {
			if col.Name == "updated_at" {
				updateColumns = append(updateColumns, "updated_at="+nowFunc)
				upsertUpdateColumns = append(upsertUpdateColumns, "updated_at="+nowFunc)
			} else {
				updateColumns = append(
					updateColumns,
					fmt.Sprintf("%s=%s", col.Name, placeholderFunc(placeholderIndex)),
				)
				upsertUpdateColumns = append(
					upsertUpdateColumns,
					fmt.Sprintf("%s=excluded.%s", col.Name, col.Name),
				)
				placeholderIndex++
			}
		}
	}

	return SQLData{
		ResourceName:       resourceName,
		PluralName:         pluralName,
		InsertColumns:      strings.Join(insertColumns, ", "),
		InsertPlaceholders: strings.Join(insertPlaceholders, ", "),
		UpdateColumns:      strings.Join(updateColumns, ", "),
		DatabaseType:       g.typeMapper.GetDatabaseType(),
		IDPlaceholder:      idPlaceholder,
		LimitOffsetClause:  limitOffsetClause,
		NowFunction:        nowFunc,
		UpsertUpdateSet:    strings.Join(upsertUpdateColumns, ", "),
	}
}

func (g *Generator) buildCatalogFromTableMigrations(
	tableName string,
	migrationDirs []string,
) (*catalog.Catalog, error) {
	allMigrations, err := migrations.DiscoverMigrations(migrationDirs)
	if err != nil {
		return nil, fmt.Errorf("failed to discover migrations: %w", err)
	}

	relevantMigrations := g.filterMigrationsForTable(tableName, allMigrations)

	cat := catalog.NewCatalog("public")

	for _, migration := range relevantMigrations {
		for _, statement := range migration.Statements {
			if err := ddl.ApplyDDL(cat, statement, migration.FilePath, g.databaseType); err != nil {
				return nil, fmt.Errorf("failed to apply DDL from %s: %w", migration.FilePath, err)
			}
		}
	}

	return cat, nil
}

func (g *Generator) filterMigrationsForTable(
	tableName string,
	allMigrations []migrations.Migration,
) []migrations.Migration {
	var relevantMigrations []migrations.Migration

	for _, migration := range allMigrations {
		isRelevant := false

		for _, statement := range migration.Statements {
			if g.statementAffectsTable(statement, tableName) {
				isRelevant = true
				break
			}
		}

		if isRelevant {
			relevantMigrations = append(relevantMigrations, migration)
		}
	}

	return relevantMigrations
}

func (g *Generator) statementAffectsTable(statement, tableName string) bool {
	parser := ddl.NewDDLParser()
	stmt, err := parser.Parse(statement, "", g.databaseType)
	if err != nil {
		// Don't filter out statements that fail to parse - let them be processed
		// by ApplyDDL so validation errors can be properly reported
		return strings.Contains(strings.ToLower(statement), strings.ToLower(tableName))
	}

	if stmt == nil {
		return false
	}

	// Check based on statement type
	switch s := stmt.(type) {
	case *ddl.CreateTableStatement:
		return s.TableName == tableName
	case *ddl.AlterTableStatement:
		return s.TableName == tableName
	case *ddl.DropTableStatement:
		return s.TableName == tableName
	default:
		return false
	}
}

func (g *Generator) GenerateModelFromMigrations(
	tableName, resourceName string,
	migrationDirs []string,
	modelPath, sqlPath string,
	modulePath string,
) error {
	cat, err := g.buildCatalogFromTableMigrations(tableName, migrationDirs)
	if err != nil {
		return fmt.Errorf("failed to build catalog from migrations: %w", err)
	}

	return g.GenerateModel(cat, resourceName, tableName, modelPath, sqlPath, modulePath, "")
}

func (g *Generator) RefreshModel(
	cat *catalog.Catalog,
	resourceName, pluralName string,
	modelPath, sqlPath string,
	modulePath string,
) error {
	if err := g.refreshSQLFile(resourceName, pluralName, cat, sqlPath); err != nil {
		return fmt.Errorf("failed to refresh SQL file: %w", err)
	}

	if err := g.refreshModelFile(cat, resourceName, pluralName, modelPath, modulePath); err != nil {
		return fmt.Errorf("failed to refresh model file: %w", err)
	}

	return nil
}

func (g *Generator) RefreshQueries(
	cat *catalog.Catalog,
	resourceName, pluralName string,
	sqlPath string,
) error {
	if err := g.validateIDColumnConstraints(cat, pluralName); err != nil {
		return fmt.Errorf("ID validation failed: %w", err)
	}

	if err := g.refreshSQLFile(resourceName, pluralName, cat, sqlPath); err != nil {
		return fmt.Errorf("failed to refresh SQL file: %w", err)
	}

	return nil
}


func (g *Generator) validateIDColumnConstraints(cat *catalog.Catalog, tableName string) error {
	table, err := cat.GetTable("", tableName)
	if err != nil {
		return fmt.Errorf("table '%s' not found in catalog: %w", tableName, err)
	}

	for _, col := range table.Columns {
		if col.Name == "id" && col.IsPrimaryKey {
			if err := col.ValidatePrimaryKeyDatatype(g.databaseType, "refresh operation"); err != nil {
				return err
			}
			return nil
		}
	}

	return fmt.Errorf("no primary key 'id' column found in table '%s'", tableName)
}

func (g *Generator) refreshSQLFile(
	resourceName string,
	pluralName string,
	cat *catalog.Catalog,
	sqlPath string,
) error {
	existingContent, err := os.ReadFile(sqlPath)
	if err != nil {
		return fmt.Errorf("failed to read existing SQL file: %w", err)
	}

	table, err := cat.GetTable("", pluralName)
	if err != nil {
		return fmt.Errorf(`table '%s' not found in catalog: %w

Convention: Model names must be singular, table names must be plural snake_case.
Example: Model 'UserAccount' expects table 'user_accounts'

To use a different table name, add the override comment to your model file`,
			pluralName, err)
	}

	newSQLContent, err := g.GenerateSQLContent(resourceName, pluralName, table)
	if err != nil {
		return fmt.Errorf("failed to generate SQL content: %w", err)
	}

	updatedContent := g.replaceGeneratedSQLQueries(
		string(existingContent),
		newSQLContent,
		resourceName,
	)

	if err := os.WriteFile(sqlPath, []byte(updatedContent), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write SQL file: %w", err)
	}

	return nil
}

func (g *Generator) refreshModelFile(
	cat *catalog.Catalog,
	resourceName, pluralName string,
	modelPath, modulePath string,
) error {
	existingContent, err := os.ReadFile(modelPath)
	if err != nil {
		return fmt.Errorf("failed to read existing model file: %w", err)
	}

	model, err := g.Build(cat, Config{
		TableName:    pluralName,
		ResourceName: resourceName,
		PackageName:  "models",
		DatabaseType: g.typeMapper.GetDatabaseType(),
		ModulePath:   modulePath,
	})
	if err != nil {
		return fmt.Errorf("failed to build model: %w", err)
	}

	templateContent, err := templates.Files.ReadFile("model.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read model template: %w", err)
	}

	newModelContent, err := g.GenerateModelFile(model, string(templateContent))
	if err != nil {
		return fmt.Errorf("failed to render model file: %w", err)
	}

	updatedContent, err := g.replaceGeneratedParts(
		string(existingContent),
		newModelContent,
		resourceName,
	)
	if err != nil {
		return fmt.Errorf("failed to replace generated parts: %w", err)
	}

	if err := os.WriteFile(modelPath, []byte(updatedContent), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write updated model file: %w", err)
	}

	if err := files.FormatGoFile(modelPath); err != nil {
		return fmt.Errorf("failed to format model file: %w", err)
	}

	return nil
}

func (g *Generator) replaceGeneratedParts(
	existingContent, newContent, resourceName string,
) (string, error) {
	newParts := g.extractGeneratedParts(newContent, resourceName)

	updatedContent := existingContent

	for signature, newContent := range newParts {
		updatedContent = g.replacePartBySignature(updatedContent, signature, newContent)
	}

	return updatedContent, nil
}

func (g *Generator) extractGeneratedParts(content, resourceName string) map[string]string {
	parts := make(map[string]string)
	lines := strings.Split(content, "\n")

	// On --refresh: ONLY replace the main model struct and rowToModel function
	// The model struct needs to match the current DB schema
	// The rowToModel function maps sqlc rows to the domain model
	// Do NOT replace domain data structs or CRUD functions - user will fix compilation errors
	signatures := []string{
		fmt.Sprintf("type %s struct", resourceName),
		fmt.Sprintf("func rowTo%s(", resourceName),
	}

	for _, signature := range signatures {
		part := g.extractPartBySignature(lines, signature)
		if part != "" {
			parts[signature] = part
		}
	}

	return parts
}

func (g *Generator) extractPartBySignature(lines []string, signature string) string {
	var result []string
	inBlock := false
	braceCount := 0
	inFunctionParams := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, signature) {
			inBlock = true
			inFunctionParams = true
			result = []string{line}
			braceCount = strings.Count(line, "{") - strings.Count(line, "}")

			if strings.Contains(line, "{") {
				inFunctionParams = false
			}

			if braceCount == 0 && !inFunctionParams {
				return strings.Join(result, "\n")
			}
			continue
		}

		if inBlock {
			result = append(result, line)

			if inFunctionParams && strings.Contains(line, "{") {
				inFunctionParams = false
			}

			braceCount += strings.Count(line, "{") - strings.Count(line, "}")

			if braceCount == 0 && !inFunctionParams {
				return strings.Join(result, "\n")
			}
		}
	}

	return ""
}

func (g *Generator) replacePartBySignature(content, signature, newPart string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inBlock := false
	braceCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, signature) {
			inBlock = true
			braceCount = strings.Count(line, "{") - strings.Count(line, "}")

			if braceCount == 0 {
				result = append(result, newPart)
				inBlock = false
				continue
			}
			continue
		}

		if inBlock {
			braceCount += strings.Count(line, "{") - strings.Count(line, "}")

			if braceCount == 0 {
				result = append(result, newPart)
				inBlock = false
				continue
			}

			continue
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

func (g *Generator) replaceGeneratedSQLQueries(
	existingContent, newContent, resourceName string,
) string {
	newQueries := g.extractGeneratedSQLQueries(newContent, resourceName)

	updatedContent := existingContent

	for queryName, newQuery := range newQueries {
		if g.queryExistsInContent(updatedContent, queryName) {
			updatedContent = g.replaceSQLQueryByName(updatedContent, queryName, newQuery)
		} else {
			updatedContent = strings.TrimSpace(updatedContent) + "\n\n" + newQuery + "\n"
		}
	}

	return updatedContent
}

func (g *Generator) extractGeneratedSQLQueries(content, resourceName string) map[string]string {
	queries := make(map[string]string)
	lines := strings.Split(content, "\n")

	queryNames := []string{
		fmt.Sprintf("Query%sByID", resourceName),
		fmt.Sprintf("Query%ss", resourceName),
		fmt.Sprintf("QueryAll%ss", resourceName),
		fmt.Sprintf("Insert%s", resourceName),
		fmt.Sprintf("Update%s", resourceName),
		fmt.Sprintf("Delete%s", resourceName),
		fmt.Sprintf("QueryPaginated%ss", resourceName),
		fmt.Sprintf("Count%ss", resourceName),
		fmt.Sprintf("Upsert%s", resourceName),
		fmt.Sprintf("%sExists", resourceName),
	}

	for _, queryName := range queryNames {
		query := g.extractSQLQueryByName(lines, queryName)
		if query != "" {
			queries[queryName] = query
		}
	}

	return queries
}

func (g *Generator) extractSQLQueryByName(lines []string, queryName string) string {
	var result []string
	inQuery := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(trimmed, fmt.Sprintf("-- name: %s ", queryName)) {
			inQuery = true
			result = []string{line}
			continue
		}

		if inQuery {
			if trimmed == "" || strings.HasPrefix(trimmed, "-- name:") {
				return strings.Join(result, "\n")
			}
			result = append(result, line)
		}
	}

	if inQuery {
		return strings.Join(result, "\n")
	}

	return ""
}

func (g *Generator) replaceSQLQueryByName(content, queryName, newQuery string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inQuery := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(trimmed, fmt.Sprintf("-- name: %s ", queryName)) {
			inQuery = true
			result = append(result, newQuery)
			continue
		}

		if inQuery {
			if trimmed == "" || strings.HasPrefix(trimmed, "-- name:") {
				inQuery = false
				result = append(result, line) // Keep the empty line or next query
				continue
			}
			continue
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

func (g *Generator) queryExistsInContent(content, queryName string) bool {
	return strings.Contains(content, fmt.Sprintf("-- name: %s ", queryName))
}

// GeneratedFactory represents a factory for a model
type GeneratedFactory struct {
	ModelName         string
	Package           string
	Fields            []FactoryField
	ModulePath        string
	StandardImports   []string
	ExternalImports   []string
	HasCreateFunction bool
	HasForeignKeys    bool           // True if there are 1+ FKs
	ForeignKeyFields  []FactoryField // All FK fields
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

	// Collect imports
	standardImports := []string{"context", "fmt"}
	externalImports := []string{
		"github.com/go-faker/faker/v4",
		"github.com/google/uuid",
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
		Package:           "factories",
		Fields:            factoryFields,
		ModulePath:        config.ModulePath,
		StandardImports:   standardImports,
		ExternalImports:   externalImports,
		HasCreateFunction: true, // Assume Create function exists
		HasForeignKeys:    len(fkFields) > 0,
		ForeignKeyFields:  fkFields,
	}, nil
}

// analyzeFactoryField analyzes a field and returns factory metadata
func (g *Generator) analyzeFactoryField(field GeneratedField, tableName string) FactoryField {
	info := FactoryField{
		Name:          field.Name,
		ArgumentName:  naming.ToLowerCamelCase(field.Name),
		Type:          field.Type,
		OptionName:    fmt.Sprintf("With%s%s", toCamelCase(tableName), field.Name),
		IsID:          field.Name == "ID",
		IsTimestamp:   field.Type == "time.Time" || strings.Contains(field.Type, "Time"),
		IsAutoManaged: field.Name == "ID" || field.Name == "CreatedAt" || field.Name == "UpdatedAt",
		IsFK:          field.IsForeignKey,
	}

	// Determine default value
	info.DefaultValue = g.determineFactoryDefault(field.Name, field.Type, field.SQLCType)
	info.GoZero = g.getFactoryGoZero(field.Type)

	return info
}

func (g *Generator) determineFactoryDefault(fieldName, goType, sqlcType string) string {
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
		return "faker.Bool()"
	case "time.Time":
		return "time.Time{}"
	case "uuid.UUID":
		return "uuid.UUID{}"
	case "[]byte":
		return "[]byte{}"
	}

	// Handle pgtype wrappers
	if strings.Contains(goType, "pgtype") {
		return fmt.Sprintf("%s{}", goType)
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

func toCamelCase(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
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
