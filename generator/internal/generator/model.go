package generator

// import (
// 	"fmt"
// 	"mbvlabs/andurel/generator/internal/catalog"
// 	"sort"
// 	"strings"
// 	"text/template"
// )
//
// type GeneratorConfig struct {
// 	TableName    string
// 	ResourceName string
// 	PackageName  string
// 	DatabaseType string
// 	StructTags   TagConfig
// 	CustomTypes  []TypeOverride
// }
//
// func GenerateModel(
// 	cat *catalog.Catalog,
// 	config GeneratorConfig,
// ) (*GeneratedModel, error) {
// 	table, err := cat.GetTable("", config.TableName)
// 	if err != nil {
// 		return nil, fmt.Errorf("table %s not found: %w", config.TableName, err)
// 	}
//
// 	typeMapper := NewTypeMapper(config.DatabaseType)
// 	typeMapper.Overrides = append(typeMapper.Overrides, config.CustomTypes...)
//
// 	model := &GeneratedModel{
// 		Name:      config.ResourceName,
// 		Package:   config.PackageName,
// 		TableName: config.TableName,
// 		Fields:    make([]GeneratedField, 0, len(table.Columns)),
// 		Imports:   make([]string, 0),
// 	}
//
// 	importSet := make(map[string]bool)
//
// 	for _, col := range table.Columns {
// 		goType, sqlcType, pkg, err := typeMapper.MapSQLTypeToGo(
// 			col.DataType,
// 			col.IsNullable,
// 		)
// 		if err != nil {
// 			return nil, fmt.Errorf(
// 				"failed to map type for column %s: %w",
// 				col.Name,
// 				err,
// 			)
// 		}
//
// 		field := GeneratedField{
// 			Name:     FormatFieldName(col.Name),
// 			Type:     goType,
// 			SQLCType: sqlcType,
// 			Tag:      GenerateStructTags(col, config.StructTags),
// 		}
//
// 		field.ConversionFromDB = typeMapper.GenerateConversionFromDB(field)
// 		if col.Name == "created_at" || col.Name == "updated_at" {
// 			field.ConversionToDB = typeMapper.GenerateConversionToDB(
// 				field,
// 				"resource."+field.Name,
// 			)
// 		} else {
// 			field.ConversionToDB = typeMapper.GenerateConversionToDB(field, "data."+field.Name)
// 		}
//
// 		field.ConversionToDBForUpdate = typeMapper.GenerateConversionToDB(
// 			field,
// 			"data."+field.Name,
// 		)
//
// 		field.ZeroCheck = typeMapper.GenerateZeroCheck(
// 			field,
// 			"data."+field.Name,
// 		)
//
// 		if pkg != "" {
// 			importSet[pkg] = true
// 		}
//
// 		switch sqlcType {
// 		case "sql.NullString",
// 			"sql.NullBool",
// 			"sql.NullInt32",
// 			"sql.NullInt64",
// 			"sql.NullFloat64":
// 			importSet["database/sql"] = true
// 		case "pgtype.Timestamptz", "pgtype.Timestamp", "pgtype.Numeric":
// 			importSet["github.com/jackc/pgx/v5/pgtype"] = true
// 		}
//
// 		model.Fields = append(model.Fields, field)
// 	}
//
// 	for imp := range importSet {
// 		model.Imports = append(model.Imports, imp)
// 	}
// 	sort.Strings(model.Imports)
//
// 	return model, nil
// }
//
// func GenerateModelFile(
// 	model *GeneratedModel,
// 	templateStr string,
// ) (string, error) {
// 	tmpl, err := template.New("model").Parse(templateStr)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to parse template: %w", err)
// 	}
//
// 	var buf strings.Builder
// 	if err := tmpl.Execute(&buf, model); err != nil {
// 		return "", fmt.Errorf("failed to execute template: %w", err)
// 	}
//
// 	return buf.String(), nil
// }
