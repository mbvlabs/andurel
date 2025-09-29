package models

import (
	"fmt"
	"os"
	"os/exec"
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
}

type GeneratedModel struct {
	Name              string
	Package           string
	Fields            []GeneratedField
	Imports           []string
	StandardImports   []string
	ThirdPartyImports []string
	SegmentImports    bool
	TableName         string
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

	// Add bytes import if needed for nullable []byte field comparisons in SQLite
	if g.typeMapper.GetDatabaseType() == "sqlite" {
		table, _ := cat.GetTable("", config.TableName)
		for i, field := range model.Fields {
			if field.Type == "[]byte" && field.SQLCType == "[]byte" && table.Columns[i].IsNullable {
				importSet["bytes"] = true
				break
			}
		}
	}

	// Add bytes import if needed for PostgreSQL []byte field comparisons
	if g.typeMapper.GetDatabaseType() == "postgresql" {
		for _, field := range model.Fields {
			if field.Type == "[]byte" && (strings.Contains(field.SQLCType, "pgtype.") || field.SQLCType == "[]byte") {
				importSet["bytes"] = true
				break
			}
		}
	}

	for imp := range importSet {
		model.Imports = append(model.Imports, imp)
	}
	sort.Strings(model.Imports)

	g.populateImportGroups(model, importSet)

	return model, nil
}

func isStandardLibraryImport(path string) bool {
	for i := 0; i < len(path); i++ {
		if path[i] == '.' {
			return false
		}
	}
	return true
}

func (g *Generator) populateImportGroups(model *GeneratedModel, importSet map[string]bool) {
	if g.typeMapper.GetDatabaseType() != "sqlite" {
		return
	}

	// Only enable import segmentation if we have both standard library and third-party imports
	hasStandardLibImports := false
	hasThirdPartyImports := false

	for imp := range importSet {
		if isStandardLibraryImport(imp) {
			hasStandardLibImports = true
		} else {
			hasThirdPartyImports = true
		}
	}

	if !hasStandardLibImports || !hasThirdPartyImports {
		return
	}

	for imp := range importSet {
		if isStandardLibraryImport(imp) {
			model.StandardImports = append(model.StandardImports, imp)
			continue
		}
		model.ThirdPartyImports = append(model.ThirdPartyImports, imp)
	}

	sort.Strings(model.StandardImports)
	sort.Strings(model.ThirdPartyImports)
	model.SegmentImports = true
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

	field.ZeroCheck = g.generateZeroCheck(col, field)

	return field, nil
}

func (g *Generator) generateZeroCheck(col *catalog.Column, field GeneratedField) string {
	valueExpr := "data." + field.Name
	currentRowExpr := "currentRow." + field.Name

	if g.typeMapper.GetDatabaseType() == "postgresql" {
		return g.generatePostgreSQLZeroCheck(col, field)
	}

	// Handle specific types first, based on nullability
	switch field.Type {
	case "[]byte":
		if field.SQLCType == "[]byte" {
			if col.IsNullable {
				return fmt.Sprintf("!bytes.Equal(%s, %s)", currentRowExpr, valueExpr)
			} else {
				return fmt.Sprintf("%s != nil", valueExpr)
			}
		}
	}

	// For sql.Null types, use proper field comparison
	if strings.HasPrefix(field.SQLCType, "sql.Null") {
		switch field.SQLCType {
		case "sql.NullString":
			return fmt.Sprintf("%s.String != %s", currentRowExpr, valueExpr)
		case "sql.NullInt64":
			return fmt.Sprintf("%s.Int64 != %s", currentRowExpr, valueExpr)
		case "sql.NullFloat64":
			return fmt.Sprintf("%s.Float64 != %s", currentRowExpr, valueExpr)
		case "sql.NullBool":
			return ""
		case "sql.NullTime":
			return fmt.Sprintf("!%s.Time.Equal(%s)", currentRowExpr, valueExpr)
		}
	}

	// For non-nullable fields (direct types)
	if !col.IsNullable {
		switch field.Type {
		case "string":
			if field.SQLCType == "string" {
				return fmt.Sprintf("%s != %s", currentRowExpr, valueExpr)
			}
		case "int64":
			if field.SQLCType == "int64" {
				return fmt.Sprintf("%s != %s", currentRowExpr, valueExpr)
			}
		case "float64":
			if field.SQLCType == "float64" {
				return fmt.Sprintf("%s != %s", currentRowExpr, valueExpr)
			}
		case "bool":
			if field.SQLCType == "bool" {
				return fmt.Sprintf("%s != %s", currentRowExpr, valueExpr)
			}
		case "time.Time":
			if field.SQLCType == "time.Time" {
				return fmt.Sprintf("!%s.Equal(%s)", currentRowExpr, valueExpr)
			}
		}
	}

	return g.typeMapper.GenerateZeroCheck(field.Type, valueExpr)
}

func (g *Generator) generatePostgreSQLZeroCheck(col *catalog.Column, field GeneratedField) string {
	valueExpr := "data." + field.Name
	currentRowExpr := "currentRow." + field.Name

	// Handle pgtype.Numeric specifically
	if field.SQLCType == "pgtype.Numeric" {
		return fmt.Sprintf("%s.Float64 != %s", currentRowExpr, valueExpr)
	}

	// Handle []byte fields (JSONB, Bytea, etc.)
	if field.Type == "[]byte" {
		if strings.Contains(field.SQLCType, "pgtype.") {
			return fmt.Sprintf("!bytes.Equal(%s.Bytes, %s)", currentRowExpr, valueExpr)
		}
		return fmt.Sprintf("!bytes.Equal(%s, %s)", currentRowExpr, valueExpr)
	}

	// Handle pgtype fields with specific field access patterns
	if strings.HasPrefix(field.SQLCType, "pgtype.") {
		switch field.SQLCType {
		case "pgtype.Text":
			return fmt.Sprintf("%s.String != %s", currentRowExpr, valueExpr)
		case "pgtype.Int4":
			return fmt.Sprintf("%s.Int32 != %s", currentRowExpr, valueExpr)
		case "pgtype.Int8":
			return fmt.Sprintf("%s.Int64 != %s", currentRowExpr, valueExpr)
		case "pgtype.Int2":
			return fmt.Sprintf("%s.Int16 != %s", currentRowExpr, valueExpr)
		case "pgtype.Float4":
			return fmt.Sprintf("%s.Float32 != %s", currentRowExpr, valueExpr)
		case "pgtype.Float8":
			return fmt.Sprintf("%s.Float64 != %s", currentRowExpr, valueExpr)
		case "pgtype.Bool":
			return ""
		case "pgtype.Timestamptz", "pgtype.Timestamp", "pgtype.Date", "pgtype.Time", "pgtype.Timetz":
			return fmt.Sprintf("!%s.Time.Equal(%s)", currentRowExpr, valueExpr)
		case "pgtype.Interval":
			return fmt.Sprintf("%s.Microseconds != %s", currentRowExpr, valueExpr)
		case "pgtype.Inet":
			return fmt.Sprintf("%s.IPNet != %s", currentRowExpr, valueExpr)
		case "pgtype.CIDR":
			return fmt.Sprintf("%s.IPNet != %s", currentRowExpr, valueExpr)
		case "pgtype.Macaddr":
			return fmt.Sprintf("%s.IPNet != %s", currentRowExpr, valueExpr)
		case "pgtype.Macaddr8":
			return fmt.Sprintf("%s.IPNet != %s", currentRowExpr, valueExpr)
		case "pgtype.Array[int32]":
			return fmt.Sprintf("len(%s.Elements) != len(%s)", currentRowExpr, valueExpr)
		case "pgtype.Array[string]":
			return fmt.Sprintf("len(%s.Elements) != len(%s)", currentRowExpr, valueExpr)
		default:
			// For geometric types and ranges that are stored as strings
			return fmt.Sprintf("%s != %s", currentRowExpr, valueExpr)
		}
	}

	// Handle direct types (non-nullable fields)
	switch field.Type {
	case "string":
		return fmt.Sprintf("%s != %s", currentRowExpr, valueExpr)
	case "int16", "int32", "int64":
		return fmt.Sprintf("%s != %s", currentRowExpr, valueExpr)
	case "float32", "float64":
		return fmt.Sprintf("%s != %s", currentRowExpr, valueExpr)
	case "bool":
		return fmt.Sprintf("%s", valueExpr)
	case "time.Time":
		return fmt.Sprintf("!%s.Equal(%s)", currentRowExpr, valueExpr)
	case "uuid.UUID":
		return fmt.Sprintf("%s != %s", currentRowExpr, valueExpr)
	default:
		return fmt.Sprintf("%s != %s", currentRowExpr, valueExpr)
	}
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
		"SQLCTypeName": func(tableName string) string {
			singular := strings.TrimSuffix(tableName, "s") // Simple singularization
			return types.FormatFieldName(singular)
		},
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
	table, err := cat.GetTable("", pluralName)
	if err != nil {
		return fmt.Errorf("table '%s' not found in catalog: %w", pluralName, err)
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

	content := g.finalizeSQLContent(pluralName, buf.String())

	return os.WriteFile(sqlPath, []byte(content), constants.FilePermissionPrivate)
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

	return g.finalizeSQLContent(pluralName, buf.String()), nil
}

func (g *Generator) finalizeSQLContent(pluralName string, content string) string {
	if g.typeMapper.GetDatabaseType() == "sqlite" && pluralName == "users" && strings.HasSuffix(content, "\n\n") {
		return strings.TrimSuffix(content, "\n")
	}
	return content
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
