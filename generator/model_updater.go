package generator

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/models"
	"github.com/mbvlabs/andurel/generator/templates"
	"github.com/mbvlabs/andurel/pkg/naming"
	"github.com/pmezard/go-difflib/difflib"
)

// standardGoTypes is the complete set of types the TypeMapper can emit.
// Any field type not in this set is treated as user-customized and preserved.
var standardGoTypes = map[string]bool{
	"string": true, "*string": true,
	"bool": true, "*bool": true,
	"int16": true, "*int16": true,
	"int32": true, "*int32": true,
	"int64": true, "*int64": true,
	"float32": true, "*float32": true,
	"float64": true, "*float64": true,
	"time.Time": true, "*time.Time": true,
	"uuid.UUID": true, "*uuid.UUID": true,
	"[]byte":           true,
	"json.RawMessage":  true,
	"*json.RawMessage": true,
	"[]int32":          true,
	"[]string":         true,
	"any":              true,
	// sql.Null types
	"sql.NullString":  true,
	"sql.NullBool":    true,
	"sql.NullInt16":   true,
	"sql.NullInt32":   true,
	"sql.NullInt64":   true,
	"sql.NullFloat64": true,
	"sql.NullTime":    true,
	// bun.Null types
	"bun.NullString":  true,
	"bun.NullBool":    true,
	"bun.NullInt32":   true,
	"bun.NullInt64":   true,
	"bun.NullFloat64": true,
	"bun.NullTime":    true,
}

type parsedField struct {
	Name     string
	TypeStr  string
	BunTag   string
	IsCustom bool
}

// UpdateModelResult holds the before/after state for a model update.
type UpdateModelResult struct {
	OldStruct      string
	NewStruct      string
	OldFileContent string
	NewFileContent string
	ModelPath      string
	HasChanges     bool

	FactoryPath       string
	OldFactoryContent string
	NewFactoryContent string
	FactoryHasChanges bool
}

// Diff returns a unified diff of the struct definitions and method bodies
// (Entity, CreateData, UpdateData, Create, Update, Upsert). bun.BaseModel
// lines are excluded — their alignment changes with field widths and is not
// schema content.
func (r *UpdateModelResult) Diff() (string, error) {
	d := difflib.UnifiedDiff{
		A:        difflib.SplitLines(dropBaseModelLine(r.OldStruct)),
		B:        difflib.SplitLines(dropBaseModelLine(r.NewStruct)),
		FromFile: "current",
		ToFile:   "updated",
		Context:  2,
	}
	return difflib.GetUnifiedDiffString(d)
}

// FactoryDiff returns a unified diff of the old vs new factory file content.
func (r *UpdateModelResult) FactoryDiff() (string, error) {
	d := difflib.UnifiedDiff{
		A:        difflib.SplitLines(r.OldFactoryContent),
		B:        difflib.SplitLines(r.NewFactoryContent),
		FromFile: "current",
		ToFile:   "updated",
		Context:  2,
	}
	return difflib.GetUnifiedDiffString(d)
}

// dropBaseModelLine removes the bun.BaseModel embedding line (and any blank
// line immediately following it) from a struct string before diffing.
func dropBaseModelLine(structStr string) string {
	lines := strings.Split(structStr, "\n")
	out := make([]string, 0, len(lines))
	skipNext := false
	for _, line := range lines {
		if strings.Contains(line, "bun.BaseModel") {
			skipNext = true
			continue
		}
		if skipNext && strings.TrimSpace(line) == "" {
			skipNext = false
			continue
		}
		skipNext = false
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// UpdateModel inspects the existing model file for resourceName, rebuilds the
// Entity struct from migrations (preserving custom field types), and returns a
// result describing the change without writing anything.
func (m *ModelManager) UpdateModel(resourceName string) (*UpdateModelResult, error) {
	modelPath := BuildModelPath(m.config.Paths.Models, resourceName)

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return nil, fmt.Errorf(
			"model file not found: %s\nRun 'andurel model %s create' to create it",
			modelPath, resourceName,
		)
	}

	src, err := os.ReadFile(modelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read model file: %w", err)
	}

	entityName := resourceName + "Entity"

	existingFields, structStart, structEnd, err := parseEntityStruct(src, entityName)
	if err != nil {
		return nil, err
	}

	tableName := ResolveTableName(m.config.Paths.Models, resourceName)

	cat, err := m.migrationManager.BuildCatalogFromMigrations(tableName, m.config)
	if err != nil {
		return nil, err
	}

	rootDir, _ := m.fileManager.FindGoModRoot()
	nullType := m.readNullType(rootDir)
	newModel, err := m.modelGenerator.Build(cat, models.Config{
		TableName:    tableName,
		ResourceName: resourceName,
		PackageName:  "models",
		DatabaseType: m.config.Database.Type,
		ModulePath:   m.projectManager.GetModulePath(),
		NullType:     nullType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build model: %w", err)
	}

	// Collect custom-typed fields from the existing struct.
	customFields := make(map[string]parsedField)
	for _, f := range existingFields {
		if f.IsCustom {
			customFields[f.Name] = f
		}
	}

	// Track which field names the migration-derived model contains.
	newFieldNames := make(map[string]bool, len(newModel.Fields))
	for _, field := range newModel.Fields {
		newFieldNames[field.Name] = true
	}

	// Override generated types with user-customized ones for fields that exist
	// in both the old and new model.
	for i, field := range newModel.Fields {
		if custom, ok := customFields[field.Name]; ok {
			newModel.Fields[i].Type = custom.TypeStr
		}
	}

	// Preserve custom-typed fields (e.g. enums) that exist in the current file
	// but are not produced by the migration-derived model.
	for _, f := range existingFields {
		if f.IsCustom && !newFieldNames[f.Name] {
			newModel.Fields = append(newModel.Fields, models.GeneratedField{
				Name:   f.Name,
				Type:   f.TypeStr,
				BunTag: f.BunTag,
			})
		}
	}

	newEntityStr := renderEntityStruct(entityName, tableName, newModel.Fields)

	content := string(src)
	content = content[:structStart] + newEntityStr + content[structEnd:]

	formatted, err := format.Source([]byte(content))
	if err != nil {
		formatted = []byte(content)
	}

	oldParts := string(src[structStart:structEnd])
	newParts := newEntityStr

	// Generate factory content for the updated model
	factoryPath := fmt.Sprintf("%s/models/factories/%s.go", rootDir, naming.ToSnakeCase(resourceName))
	var oldFactoryContent, newFactoryContent string
	if existingSrc, err := os.ReadFile(factoryPath); err == nil {
		oldFactoryContent = string(existingSrc)
	}

	factoryGenFactory, factoryErr := m.modelGenerator.BuildFactory(cat, models.Config{
		TableName:    tableName,
		ResourceName: resourceName,
		PackageName:  "factories",
		DatabaseType: m.config.Database.Type,
		ModulePath:   m.projectManager.GetModulePath(),
		NullType:     nullType,
	}, newModel)
	if factoryErr == nil {
		factoryTmplContent, tmplErr := templates.Files.ReadFile("factory.tmpl")
		if tmplErr == nil {
			if genFactoryContent, genErr := m.modelGenerator.GenerateFactoryFile(factoryGenFactory, string(factoryTmplContent)); genErr == nil {
				formattedFactory, fmtErr := format.Source([]byte(genFactoryContent))
				if fmtErr == nil {
					newFactoryContent = string(formattedFactory)
				} else {
					newFactoryContent = genFactoryContent
				}
			}
		}
	}

	return &UpdateModelResult{
		OldStruct:      oldParts,
		NewStruct:      newParts,
		OldFileContent: string(src),
		NewFileContent: string(formatted),
		ModelPath:      modelPath,
		HasChanges:     string(formatted) != string(src),

		FactoryPath:       factoryPath,
		OldFactoryContent: oldFactoryContent,
		NewFactoryContent: newFactoryContent,
		FactoryHasChanges: oldFactoryContent != newFactoryContent,
	}, nil
}

// ApplyModelUpdate writes the updated model and factory file content and runs the Go formatter.
func (m *ModelManager) ApplyModelUpdate(result *UpdateModelResult) error {
	if err := os.WriteFile(result.ModelPath, []byte(result.NewFileContent), 0o600); err != nil {
		return fmt.Errorf("failed to write model file: %w", err)
	}
	if err := files.FormatGoFile(result.ModelPath); err != nil {
		return fmt.Errorf("failed to format model file: %w", err)
	}

	// Write updated factory file if we have new content
	if result.NewFactoryContent != "" {
		factoryDir := filepath.Dir(result.FactoryPath)
		if err := os.MkdirAll(factoryDir, 0755); err != nil {
			return fmt.Errorf("failed to create factories directory: %w", err)
		}
		if err := os.WriteFile(result.FactoryPath, []byte(result.NewFactoryContent), 0o600); err != nil {
			return fmt.Errorf("failed to write factory file: %w", err)
		}
		if err := files.FormatGoFile(result.FactoryPath); err != nil {
			return fmt.Errorf("failed to format factory file: %w", err)
		}
	}

	return nil
}

// parseEntityStruct parses src to find the named Entity struct.
// Returns the struct fields, and the byte offsets [start, end) covering the
// "type X struct { ... }" declaration.
func parseEntityStruct(src []byte, entityName string) ([]parsedField, int, int, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to parse Go source: %w", err)
	}

	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != entityName {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			startOff := fset.Position(genDecl.Pos()).Offset
			endOff := fset.Position(genDecl.End()).Offset

			var fields []parsedField
			for _, field := range structType.Fields.List {
				if len(field.Names) == 0 {
					// Embedded field (bun.BaseModel), skip.
					continue
				}
				fieldName := field.Names[0].Name
				typeStart := fset.Position(field.Type.Pos()).Offset
				typeEnd := fset.Position(field.Type.End()).Offset
				typeStr := strings.TrimSpace(string(src[typeStart:typeEnd]))

				var bunTag string
				if field.Tag != nil {
					// field.Tag.Value is the raw string literal including backticks.
					raw := strings.Trim(field.Tag.Value, "`")
					bunTag = reflect.StructTag(raw).Get("bun")
				}

				fields = append(fields, parsedField{
					Name:     fieldName,
					TypeStr:  typeStr,
					BunTag:   bunTag,
					IsCustom: !standardGoTypes[typeStr],
				})
			}

			return fields, startOff, endOff, nil
		}
	}

	return nil, 0, 0, fmt.Errorf("entity struct %q not found in file", entityName)
}

// renderEntityStruct generates the "type X struct { ... }" text for the given fields.
func renderEntityStruct(entityName, tableName string, fields []models.GeneratedField) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "type %s struct {\n", entityName)
	fmt.Fprintf(&sb, "\tbun.BaseModel `bun:\"table:%s,alias:%s\"`\n", tableName, tableName)
	sb.WriteString("\n")
	for _, f := range fields {
		fmt.Fprintf(&sb, "\t%s %s `bun:\"%s\"`\n", f.Name, f.Type, f.BunTag)
	}
	sb.WriteString("}")
	return sb.String()
}

// renderCreateDataStruct generates the "type CreateXData struct { ... }" text.
func renderCreateDataStruct(resourceName string, model *models.GeneratedModel) string {
	var sb strings.Builder
	idGoField := model.IDGoFieldName
	if idGoField == "" {
		idGoField = "ID"
	}
	fmt.Fprintf(&sb, "type Create%sData struct {\n", resourceName)
	for _, f := range model.Fields {
		if f.Name == idGoField || f.Name == "CreatedAt" || f.Name == "UpdatedAt" {
			continue
		}
		fmt.Fprintf(&sb, "\t%s %s\n", f.Name, f.Type)
	}
	if !model.IsAutoIncrementID && model.IDType != "" && model.IDType != "uuid.UUID" {
		fmt.Fprintf(&sb, "\t%s %s\n", idGoField, model.IDType)
	}
	sb.WriteString("}")
	return sb.String()
}

// renderUpdateDataStruct generates the "type UpdateXData struct { ... }" text.
func renderUpdateDataStruct(resourceName string, model *models.GeneratedModel) string {
	var sb strings.Builder
	idGoField := model.IDGoFieldName
	if idGoField == "" {
		idGoField = "ID"
	}
	idType := model.IDType
	if idType == "" {
		idType = "uuid.UUID"
	}
	fmt.Fprintf(&sb, "type Update%sData struct {\n", resourceName)
	fmt.Fprintf(&sb, "\t%s %s\n", idGoField, idType)
	for _, f := range model.Fields {
		if f.Name == idGoField || f.Name == "CreatedAt" {
			continue
		}
		fmt.Fprintf(&sb, "\t%s %s\n", f.Name, f.Type)
	}
	sb.WriteString("}")
	return sb.String()
}

// findFuncOffsets returns the byte offsets [start, end) of a method
// declaration matching the given receiver type and function name.
func findFuncOffsets(src []byte, receiverType string, funcName string) (int, int, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse Go source: %w", err)
	}

	for _, decl := range f.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if funcDecl.Name.Name != funcName {
			continue
		}
		if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
			continue
		}
		recvType := funcDecl.Recv.List[0].Type
		typeStart := fset.Position(recvType.Pos()).Offset
		typeEnd := fset.Position(recvType.End()).Offset
		typeStr := strings.TrimSpace(string(src[typeStart:typeEnd]))
		typeStr = strings.TrimPrefix(typeStr, "*")
		if typeStr != receiverType {
			continue
		}
		return fset.Position(funcDecl.Pos()).Offset, fset.Position(funcDecl.End()).Offset, nil
	}

	return 0, 0, fmt.Errorf("func %q on receiver %q not found", funcName, receiverType)
}

// findStructOffsets returns the byte offsets [start, end) of a named struct
// declaration in Go source.
func findStructOffsets(src []byte, structName string) (int, int, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse Go source: %w", err)
	}

	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != structName {
				continue
			}
			if _, ok := typeSpec.Type.(*ast.StructType); !ok {
				continue
			}
			return fset.Position(genDecl.Pos()).Offset, fset.Position(genDecl.End()).Offset, nil
		}
	}
	return 0, 0, fmt.Errorf("struct %q not found in file", structName)
}
