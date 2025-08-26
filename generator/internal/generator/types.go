package generator

// type TypeMapper struct {
// 	DatabaseType string
// 	TypeMap      map[string]string
// 	Overrides    []TypeOverride
// }
//
// type TypeOverride struct {
// 	DatabaseType string
// 	GoType       string
// 	Package      string
// 	Nullable     bool
// }
//
// type GeneratedField struct {
// 	Name                    string
// 	Type                    string
// 	Tag                     string
// 	Comment                 string
// 	Package                 string
// 	SQLCType                string // The type used by sqlc
// 	ConversionFromDB        string // How to convert from sqlc type to Go type
// 	ConversionToDB          string // How to convert from Go type to sqlc type
// 	ConversionToDBForUpdate string // How to convert from Go type to sqlc type in update operations
// 	ZeroCheck               string // How to check if the value is zero/empty
// }
//
// type GeneratedModel struct {
// 	Name      string
// 	Package   string
// 	Fields    []GeneratedField
// 	Imports   []string
// 	TableName string
// }
//
// // func GenerateStructTags(column *catalog.Column, config TagConfig) string {
// // 	var tags []string
// //
// // 	if config.JSON {
// // 		tags = append(tags, fmt.Sprintf("json:\"%s\"", column.Name))
// // 	}
// //
// // 	if config.DB {
// // 		tags = append(tags, fmt.Sprintf("db:\"%s\"", column.Name))
// // 	}
// //
// // 	if config.Validate {
// // 		validationTags := generateValidationTags(column)
// // 		if validationTags != "" {
// // 			tags = append(tags, fmt.Sprintf("validate:\"%s\"", validationTags))
// // 		}
// // 	}
// //
// // 	for key, value := range config.Custom {
// // 		tags = append(tags, fmt.Sprintf("%s:\"%s\"", key, value))
// // 	}
// //
// // 	if len(tags) > 0 {
// // 		return "`" + strings.Join(tags, " ") + "`"
// // 	}
// //
// // 	return ""
// // }
// //
// // func generateValidationTags(column *catalog.Column) string {
// // 	var tags []string
// //
// // 	if !column.IsNullable {
// // 		tags = append(tags, "required")
// // 	}
// //
// // 	if strings.Contains(strings.ToLower(column.Name), "email") {
// // 		tags = append(tags, "email")
// // 	}
// //
// // 	if column.DataType == "uuid" {
// // 		tags = append(tags, "uuid")
// // 	}
// //
// // 	return strings.Join(tags, ",")
// // }
