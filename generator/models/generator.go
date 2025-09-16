package models

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/generator/internal/ddl"
	"github.com/mbvlabs/andurel/generator/internal/migrations"
	"github.com/mbvlabs/andurel/generator/internal/types"
	"github.com/mbvlabs/andurel/generator/templates"
	"github.com/mbvlabs/andurel/pkg/constants"
	"github.com/mbvlabs/andurel/pkg/errors"
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
	RequiresErrorHandling   bool
	IsNullable              bool
}

type GeneratedModel struct {
	Name         string
	Package      string
	Fields       []GeneratedField
	Imports      []string
	TableName    string
	ModulePath   string
	DatabaseType string
	HasRelations bool
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
		Name:         config.ResourceName,
		Package:      config.PackageName,
		TableName:    config.TableName,
		ModulePath:   config.ModulePath,
		DatabaseType: g.typeMapper.GetDatabaseType(),
		Fields:       make([]GeneratedField, 0, len(table.Columns)),
		Imports:      make([]string, 0),
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

		typeImports := g.addTypeImports(field.SQLCType, field.Type)
		for imp := range typeImports {
			importSet[imp] = true
		}
		model.Fields = append(model.Fields, field)
	}

	importSet["time"] = true
	importSet["github.com/google/uuid"] = true

	for imp := range importSet {
		model.Imports = append(model.Imports, imp)
	}
	sort.Strings(model.Imports)

	return model, nil
}

func (g *Generator) buildField(col *catalog.Column) (GeneratedField, error) {
	var goType, sqlcType, pkg string
	var err error

	// Special handling for ID fields in SQLite - always use uuid.UUID
	if col.Name == "id" && g.typeMapper.GetDatabaseType() == "sqlite" {
		goType = "uuid.UUID"
		sqlcType = "string"
		pkg = "github.com/google/uuid"
	} else {
		goType, sqlcType, pkg, err = g.typeMapper.MapSQLTypeToGo(col.DataType, col.IsNullable)
		if err != nil {
			return GeneratedField{}, err
		}
	}

	field := GeneratedField{
		Name:                  types.FormatFieldName(col.Name),
		Type:                  goType,
		SQLCType:              sqlcType,
		Package:               pkg,
		RequiresErrorHandling: col.Name == "id" && g.typeMapper.GetDatabaseType() == "sqlite",
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

func (g *Generator) addTypeImports(sqlcType, goType string) map[string]bool {
	importSet := map[string]bool{}
	if strings.Contains(sqlcType, "pgtype.") {
		importSet["github.com/jackc/pgx/v5/pgtype"] = true
	}
	if strings.Contains(sqlcType, "sql.Null") {
		importSet["database/sql"] = true
	}
	if strings.Contains(goType, "time.Time") || strings.Contains(sqlcType, "time.Time") {
		importSet["time"] = true
	}
	if strings.Contains(goType, "uuid.UUID") || strings.Contains(sqlcType, "uuid.UUID") {
		importSet["github.com/google/uuid"] = true
	}

	return importSet
}

func (g *Generator) GenerateModelFile(model *GeneratedModel, templateStr string) (string, error) {
	funcMap := template.FuncMap{
		// "SQLCTypeName": func(tableName string) string {
		// 	singular := strings.TrimSuffix(tableName, "s") // Simple singularization
		// 	return types.FormatFieldName(singular)
		// },
		"lower": func(s string) string {
			return strings.ToLower(s)
		},
		"uuidParam": func(param string) string {
			if model.DatabaseType == "sqlite" {
				return param + ".String()"
			}
			return param
		},
		"hasErrorHandling": func() bool {
			for _, field := range model.Fields {
				if field.RequiresErrorHandling {
					return true
				}
			}
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
) error {
	_, err := cat.GetTable("", pluralName)
	if err != nil {
		return fmt.Errorf("table '%s' not found in catalog: %w", pluralName, err)
	}

	// if err := g.GenerateSQLFile(resourceName, pluralName, table, sqlPath); err != nil {
	// 	return fmt.Errorf("failed to generate SQL file: %w", err)
	// }

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

	modelContent, err := g.GenerateModelFile(model, string(templateContent))
	if err != nil {
		return fmt.Errorf("failed to render model file: %w", err)
	}

	if err := os.WriteFile(modelPath, []byte(modelContent), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write model file: %w", err)
	}

	if err := g.formatGoFile(modelPath); err != nil {
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

	tmpl, err := templates.GetCachedTemplate("crud_operations.tmpl", template.FuncMap{})
	if err != nil {
		return errors.NewTemplateError("crud_operations.tmpl", "get template", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return errors.NewTemplateError("crud_operations.tmpl", "execute template", err)
	}

	return os.WriteFile(sqlPath, []byte(buf.String()), constants.FilePermissionPrivate)
}

func (g *Generator) GenerateSQLContent(
	resourceName string,
	pluralName string,
	table *catalog.Table,
) (string, error) {
	data := g.prepareSQLData(resourceName, pluralName, table)

	tmpl, err := templates.GetCachedTemplate("crud_operations.tmpl", template.FuncMap{})
	if err != nil {
		return "", errors.NewTemplateError("crud_operations.tmpl", "get template", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", errors.NewTemplateError("crud_operations.tmpl", "execute template", err)
	}

	return buf.String(), nil
}

func (g *Generator) prepareSQLData(
	resourceName string,
	pluralName string,
	table *catalog.Table,
) SQLData {
	var insertColumns []string
	var insertPlaceholders []string
	var updateColumns []string

	var placeholderFunc func(int) string
	var nowFunc string
	var idPlaceholder string
	var limitOffsetClause string

	if g.typeMapper.GetDatabaseType() == "sqlite" {
		placeholderFunc = func(i int) string { return "?" }
		nowFunc = "datetime('now')"
		idPlaceholder = "?"
		limitOffsetClause = "limit ? offset ?"
	}
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
			} else {
				updateColumns = append(
					updateColumns,
					fmt.Sprintf("%s=%s", col.Name, placeholderFunc(placeholderIndex)),
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
	stmt, err := ddl.ParseDDLStatement(statement, "", g.databaseType)
	if err != nil {
		// Don't filter out statements that fail to parse - let them be processed
		// by ApplyDDL so validation errors can be properly reported
		return strings.Contains(strings.ToLower(statement), strings.ToLower(tableName))
	}

	if stmt == nil {
		return false
	}

	switch stmt.Type {
	case ddl.CreateTable, ddl.AlterTable, ddl.DropTable:
		return stmt.TableName == tableName
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

	return g.GenerateModel(cat, resourceName, tableName, modelPath, sqlPath, modulePath)
}

func (g *Generator) formatGoFile(filePath string) error {
	cmd := exec.Command("go", "fmt", filePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run go fmt on %s: %w", filePath, err)
	}
	return nil
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

func (g *Generator) GenerateConstructorFile(
	model *GeneratedModel,
	templateStr string,
) (string, error) {
	funcMap := template.FuncMap{
		"lower": func(s string) string {
			return strings.ToLower(s)
		},
		"uuidParam": func(param string) string {
			if model.DatabaseType == "sqlite" {
				return param + ".String()"
			}
			return param
		},
	}

	tmpl, err := template.New("constructors").Funcs(funcMap).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse constructor template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, model); err != nil {
		return "", fmt.Errorf("failed to execute constructor template: %w", err)
	}

	return buf.String(), nil
}

func (g *Generator) GenerateConstructors(
	cat *catalog.Catalog,
	resourceName, pluralName string,
	constructorPath string,
	modulePath string,
) error {
	model, err := g.Build(cat, Config{
		TableName:    pluralName,
		ResourceName: resourceName,
		PackageName:  "db",
		DatabaseType: g.typeMapper.GetDatabaseType(),
		ModulePath:   modulePath,
	})
	if err != nil {
		return fmt.Errorf("failed to build model for constructors: %w", err)
	}

	model.Imports = g.calculateConstructorImports(model)

	templateContent, err := templates.Files.ReadFile("constructors.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read constructor template: %w", err)
	}

	constructorContent, err := g.GenerateConstructorFile(model, string(templateContent))
	if err != nil {
		return fmt.Errorf("failed to render constructor file: %w", err)
	}

	if err := os.WriteFile(constructorPath, []byte(constructorContent), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write constructor file: %w", err)
	}

	if err := g.formatGoFile(constructorPath); err != nil {
		return fmt.Errorf("failed to format constructor file: %w", err)
	}

	return nil
}

func (g *Generator) calculateConstructorImports(model *GeneratedModel) []string {
	importSet := make(map[string]bool)

	importSet["github.com/google/uuid"] = true

	for _, field := range model.Fields {
		if field.Name == "ID" || field.Name == "CreatedAt" || field.Name == "UpdatedAt" {
			continue
		}

		if strings.Contains(field.SQLCType, "pgtype.") {
			importSet["github.com/jackc/pgx/v5/pgtype"] = true
		}
		if strings.Contains(field.SQLCType, "sql.") {
			importSet["database/sql"] = true
		}
		if strings.Contains(field.SQLCType, "time.Time") {
			importSet["time"] = true
		}
		if strings.Contains(field.SQLCType, "uuid.UUID") {
			importSet["github.com/google/uuid"] = true
		}
	}

	var imports []string
	for imp := range importSet {
		imports = append(imports, imp)
	}
	sort.Strings(imports)

	return imports
}

func (g *Generator) RefreshConstructors(
	cat *catalog.Catalog,
	resourceName string,
	pluralName string,
	constructorPath string,
	modulePath string,
) error {
	if err := g.validateIDColumnConstraints(cat, pluralName); err != nil {
		return fmt.Errorf("ID validation failed: %w", err)
	}

	if err := g.GenerateConstructors(cat, resourceName, pluralName, constructorPath, modulePath); err != nil {
		return fmt.Errorf("failed to generate constructor functions: %w", err)
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
		return fmt.Errorf("table '%s' not found in catalog: %w", pluralName, err)
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

	if err := g.formatGoFile(modelPath); err != nil {
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

	signatures := []string{
		fmt.Sprintf("type %s struct", resourceName),
		fmt.Sprintf("type Create%sData struct", resourceName),
		fmt.Sprintf("type Update%sData struct", resourceName),
		fmt.Sprintf("type Paginated%ss struct", resourceName),
		fmt.Sprintf("func Find%s(", resourceName),
		fmt.Sprintf("func Create%s(", resourceName),
		fmt.Sprintf("func Update%s(", resourceName),
		fmt.Sprintf("func Destroy%s(", resourceName),
		fmt.Sprintf("func All%ss(", resourceName),
		fmt.Sprintf("func Paginate%ss(", resourceName),
		fmt.Sprintf("func rowTo%s(", resourceName),
		fmt.Sprintf("func newInsert%sParams(", resourceName),
		fmt.Sprintf("func newUpdate%sParams(", resourceName),
		fmt.Sprintf("func newQueryPaginated%ssParams(", resourceName),
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

// GenerateModelFromBob generates a model by parsing bob-generated structs
func (g *Generator) GenerateModelFromBob(
	resourceName, tableName, modelPath, modulePath string,
) error {
	// Find the bob-generated file in models/internal/db/
	dbDir := filepath.Join("models", "internal", "db")

	// Look for generated Go files in the db directory
	files, err := filepath.Glob(filepath.Join(dbDir, "*.go"))
	if err != nil {
		return fmt.Errorf("failed to find generated db files: %w", err)
	}

	var bobStruct *BobStruct
	for _, file := range files {
		bobStruct, err = g.parseBobStruct(file, resourceName)
		if err != nil {
			continue // Try next file
		}
		if bobStruct != nil {
			break
		}
	}

	if bobStruct == nil {
		return fmt.Errorf("could not find bob-generated struct for %s in %s", resourceName, dbDir)
	}

	model := g.convertBobStructToModel(bobStruct, resourceName, tableName, modulePath)

	templateContent, err := templates.Files.ReadFile("model_bob.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read bob model template: %w", err)
	}

	modelContent, err := g.GenerateModelFileForBob(model, string(templateContent))
	if err != nil {
		return fmt.Errorf("failed to render model file: %w", err)
	}

	if err := os.WriteFile(modelPath, []byte(modelContent), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write model file: %w", err)
	}

	if err := g.formatGoFile(modelPath); err != nil {
		return fmt.Errorf("failed to format model file: %w", err)
	}

	return nil
}

type BobStruct struct {
	Name   string
	Fields []BobField
}

type BobField struct {
	Name       string
	Type       string
	IsNullable bool
}

// parseBobStruct parses a Go file to find a struct with the given name
func (g *Generator) parseBobStruct(filename, structName string) (*BobStruct, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, content, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file %s: %w", filename, err)
	}

	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != structName {
				continue
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			// Found the struct, extract fields
			bobStruct := &BobStruct{
				Name:   structName,
				Fields: make([]BobField, 0, len(structType.Fields.List)),
			}

			for _, field := range structType.Fields.List {
				for _, name := range field.Names {
					fieldType := g.extractTypeFromAst(field.Type)
					isNullable := g.isNullableType(fieldType)
					bobStruct.Fields = append(bobStruct.Fields, BobField{
						Name:       name.Name,
						Type:       fieldType,
						IsNullable: isNullable,
					})
				}
			}

			return bobStruct, nil
		}
	}

	return nil, fmt.Errorf("struct %s not found in %s", structName, filename)
}

// extractTypeFromAst converts an AST type expression to a string
func (g *Generator) extractTypeFromAst(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		pkg := g.extractTypeFromAst(t.X)
		return pkg + "." + t.Sel.Name
	case *ast.ArrayType:
		elem := g.extractTypeFromAst(t.Elt)
		return "[]" + elem
	case *ast.StarExpr:
		elem := g.extractTypeFromAst(t.X)
		return "*" + elem
	case *ast.IndexExpr:
		// Handle generic types like null.Val[int32]
		base := g.extractTypeFromAst(t.X)
		index := g.extractTypeFromAst(t.Index)
		return base + "[" + index + "]"
	default:
		return "interface{}" // fallback
	}
}

// isNullableType determines if a bob type is nullable
func (g *Generator) isNullableType(bobType string) bool {
	return strings.HasPrefix(bobType, "null.Val[") ||
		strings.HasPrefix(bobType, "omitnull.Val[") ||
		strings.Contains(bobType, "Null")
}

// convertBobStructToModel converts a bob struct to our GeneratedModel format
func (g *Generator) convertBobStructToModel(
	bobStruct *BobStruct,
	resourceName, tableName, modulePath string,
) *GeneratedModel {
	model := &GeneratedModel{
		Name:         resourceName,
		Package:      "models",
		TableName:    tableName,
		ModulePath:   modulePath,
		DatabaseType: g.databaseType,
		Fields:       make([]GeneratedField, 0, len(bobStruct.Fields)),
		Imports:      make([]string, 0),
	}

	importSet := make(map[string]bool)

	hasNullable := false
	hasNonNullable := false

	// Detect if bob struct includes relationship container
	for _, bf := range bobStruct.Fields {
		if bf.Name == "R" {
			model.HasRelations = true
			break
		}
	}

	for _, bobField := range bobStruct.Fields {
		// Skip bob relationship container field. It's not a real column
		// and should not appear in the domain model or DTOs.
		if bobField.Name == "R" {
			continue
		}
		goType, imports := g.convertBobTypeToGoType(bobField.Type)

		// Track usage for import decisions
		if bobField.IsNullable {
			hasNullable = true
		} else {
			hasNonNullable = true
		}

		field := GeneratedField{
			Name:     bobField.Name,
			Type:     goType,
			SQLCType: bobField.Type, // Keep original bob type for conversions
			ConversionFromDB: g.generateBobConversionFromDB(
				bobField.Name,
				bobField.Type,
				goType,
			),
			ConversionToDB: g.generateBobConversionToDB(
				bobField.IsNullable,
				goType,
				"data."+bobField.Name,
			),
			ConversionToDBForUpdate: g.generateBobConversionToDB(
				bobField.IsNullable,
				goType,
				"data."+bobField.Name,
			),
			ZeroCheck:  g.generateZeroCheck(goType, "data."+bobField.Name),
			IsNullable: bobField.IsNullable,
		}

		model.Fields = append(model.Fields, field)

		// Add imports
		for _, imp := range imports {
			importSet[imp] = true
		}
	}

	// Add bob-specific imports based on actual usage
	if hasNullable {
		importSet["github.com/aarondl/opt/omitnull"] = true
	}
	if hasNonNullable {
		importSet["github.com/aarondl/opt/omit"] = true
	}
	importSet["github.com/stephenafamo/bob"] = true

	// Add required imports for bob models based on usage
	// time and uuid are added based on actual field usage

	for imp := range importSet {
		model.Imports = append(model.Imports, imp)
	}
	sort.Strings(model.Imports)

	return model
}

// convertBobTypeToGoType converts bob types to standard Go types
func (g *Generator) convertBobTypeToGoType(bobType string) (string, []string) {
	imports := make([]string, 0)

	// Handle common bob types
	switch {
	case bobType == "uuid.UUID":
		imports = append(imports, "github.com/google/uuid")
		return "uuid.UUID", imports
	case strings.HasPrefix(bobType, "null.Val["):
		// Extract the inner type from null.Val[T]
		innerType := strings.TrimPrefix(bobType, "null.Val[")
		innerType = strings.TrimSuffix(innerType, "]")

		// Convert inner type
		convertedInner, innerImports := g.convertBobTypeToGoType(innerType)
		imports = append(imports, innerImports...)

		return convertedInner, imports
	case bobType == "int32":
		return "int32", imports
	case bobType == "bool":
		return "bool", imports
	case bobType == "string":
		return "string", imports
	case bobType == "time.Time":
		imports = append(imports, "time")
		return "time.Time", imports
	default:
		return bobType, imports
	}
}

// generateBobConversionFromDB generates code to convert from bob struct field to Go type
func (g *Generator) generateBobConversionFromDB(fieldName, bobType, goType string) string {
	// Handle null.Val types
	if strings.HasPrefix(bobType, "null.Val[") {
		// For null.Val[T], access the value with .GetOrZero()
		return fmt.Sprintf("row.%s.GetOrZero()", fieldName)
	}

	// Direct conversion for non-nullable types
	return fmt.Sprintf("row.%s", fieldName)
}

// generateBobConversionToDB generates code to convert from Go type to bob parameter
func (g *Generator) generateBobConversionToDB(isNullable bool, goType, valueExpr string) string {
	if isNullable {
		return fmt.Sprintf("omitnull.From(%s)", valueExpr)
	}
	return fmt.Sprintf("omit.From(%s)", valueExpr)
}

// generateZeroCheck generates code to check if a value is non-zero
func (g *Generator) generateZeroCheck(goType, valueExpr string) string {
	switch goType {
	case "string":
		return fmt.Sprintf(`%s != ""`, valueExpr)
	case "int", "int32", "int64":
		return fmt.Sprintf(`%s != 0`, valueExpr)
	case "bool":
		return fmt.Sprintf(`%s`, valueExpr)
	case "uuid.UUID":
		return fmt.Sprintf(`%s != uuid.Nil`, valueExpr)
	default:
		return fmt.Sprintf(`%s != nil`, valueExpr)
	}
}

// GenerateModelFileForBob generates model content using bob-specific template functions
func (g *Generator) GenerateModelFileForBob(
	model *GeneratedModel,
	templateStr string,
) (string, error) {
	funcMap := template.FuncMap{
		"SQLCTypeName": func(tableName string) string {
			// For bob, the struct name is the singular resource name
			return model.Name
		},
		"lower": func(s string) string {
			return strings.ToLower(s)
		},
		"uuidParam": func(param string) string {
			// Bob handles UUIDs natively
			return param
		},
		"hasErrorHandling": func() bool {
			// For now, assume no special error handling needed
			return false
		},
		"hasImport": func(imp string) bool {
			for _, i := range model.Imports {
				if i == imp {
					return true
				}
			}
			return false
		},
		"isExternal": func(imp string) bool {
			return strings.HasPrefix(imp, "github.com/")
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
