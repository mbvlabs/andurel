package generator

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/models"
	"github.com/mbvlabs/andurel/generator/templates"
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
	"[]byte":   true,
	"[]int32":  true,
	"[]string": true,
	"any":      true,
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
	createDataName := "Create" + resourceName + "Data"
	updateDataName := "Update" + resourceName + "Data"

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
	newCreateDataStr := renderCreateDataStruct(resourceName, newModel)
	newUpdateDataStr := renderUpdateDataStruct(resourceName, newModel)

	// Generate the full file from the template so we can extract new
	// method bodies for Create, Update, and Upsert.
	templateContent, err := templates.Files.ReadFile("model.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to read model template: %w", err)
	}
	newFullContent, err := m.modelGenerator.GenerateModelFile(newModel, string(templateContent))
	if err != nil {
		return nil, fmt.Errorf("failed to generate model file from template: %w", err)
	}

	// Collect replacements: (start offset, end offset, replacement text).
	// All start/end values come from the ORIGINAL src so they remain valid
	// when we apply in descending start order.
	type splice struct {
		start int
		end   int
		text  string
	}
	var splices []splice

	receiverType := newModel.NamespaceType

	// Method replacements — extract from the generated full file.
	if newStart, newEnd, err := findFuncOffsets([]byte(newFullContent), receiverType, "Upsert"); err == nil {
		if oldStart, oldEnd, err := findFuncOffsets(src, receiverType, "Upsert"); err == nil {
			splices = append(splices, splice{oldStart, oldEnd, newFullContent[newStart:newEnd]})
		}
	}
	if cdStart, cdEnd, err := findStructOffsets(src, updateDataName); err == nil {
		splices = append(splices, splice{cdStart, cdEnd, newUpdateDataStr})
	}
	if newStart, newEnd, err := findFuncOffsets([]byte(newFullContent), receiverType, "Update"); err == nil {
		if oldStart, oldEnd, err := findFuncOffsets(src, receiverType, "Update"); err == nil {
			splices = append(splices, splice{oldStart, oldEnd, newFullContent[newStart:newEnd]})
		}
	}
	if cdStart, cdEnd, err := findStructOffsets(src, createDataName); err == nil {
		splices = append(splices, splice{cdStart, cdEnd, newCreateDataStr})
	}
	if newStart, newEnd, err := findFuncOffsets([]byte(newFullContent), receiverType, "Create"); err == nil {
		if oldStart, oldEnd, err := findFuncOffsets(src, receiverType, "Create"); err == nil {
			splices = append(splices, splice{oldStart, oldEnd, newFullContent[newStart:newEnd]})
		}
	}
	splices = append(splices, splice{structStart, structEnd, newEntityStr})

	// Sort by start descending so earlier offsets remain valid.
	sort.Slice(splices, func(i, j int) bool {
		return splices[i].start > splices[j].start
	})

	// Collect old text before modifying content.
	oldCreateStr, oldUpdateStr, oldUpsertStr := "", "", ""
	for _, s := range splices {
		switch {
		case strings.Contains(s.text, "func (") && strings.Contains(s.text, " Upsert("):
			oldUpsertStr = string(src[s.start:s.end])
		case strings.Contains(s.text, "func (") && strings.Contains(s.text, " Update("):
			oldUpdateStr = string(src[s.start:s.end])
		case strings.Contains(s.text, "func (") && strings.Contains(s.text, " Create("):
			oldCreateStr = string(src[s.start:s.end])
		}
	}

	content := string(src)
	for _, s := range splices {
		content = content[:s.start] + s.text + content[s.end:]
	}

	formatted, err := format.Source([]byte(content))
	if err != nil {
		formatted = []byte(content)
	}

	// Collect all old and new struct+method definitions for diffing.
	appendIf := func(sb *strings.Builder, text string) {
		if text != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n\n")
			}
			sb.WriteString(text)
		}
	}
	extractStruct := func(src []byte, name string) string {
		if start, end, err := findStructOffsets(src, name); err == nil {
			return string(src[start:end])
		}
		return ""
	}
	extractFunc := func(src []byte, recv, name string) string {
		if start, end, err := findFuncOffsets(src, recv, name); err == nil {
			return string(src[start:end])
		}
		return ""
	}

	var oldParts, newParts strings.Builder
	appendIf(&oldParts, string(src[structStart:structEnd]))
	appendIf(&newParts, newEntityStr)
	appendIf(&oldParts, extractStruct(src, createDataName))
	appendIf(&newParts, extractStruct([]byte(newFullContent), createDataName))
	appendIf(&oldParts, extractStruct(src, updateDataName))
	appendIf(&newParts, extractStruct([]byte(newFullContent), updateDataName))
	appendIf(&oldParts, oldCreateStr)
	appendIf(&newParts, extractFunc([]byte(newFullContent), receiverType, "Create"))
	appendIf(&oldParts, oldUpdateStr)
	appendIf(&newParts, extractFunc([]byte(newFullContent), receiverType, "Update"))
	appendIf(&oldParts, oldUpsertStr)
	appendIf(&newParts, extractFunc([]byte(newFullContent), receiverType, "Upsert"))

	return &UpdateModelResult{
		OldStruct:      oldParts.String(),
		NewStruct:      newParts.String(),
		OldFileContent: string(src),
		NewFileContent: string(formatted),
		ModelPath:      modelPath,
		HasChanges:     string(formatted) != string(src),
	}, nil
}

// ApplyModelUpdate writes the updated file content and runs the Go formatter.
func (m *ModelManager) ApplyModelUpdate(result *UpdateModelResult) error {
	if err := os.WriteFile(result.ModelPath, []byte(result.NewFileContent), 0o600); err != nil {
		return fmt.Errorf("failed to write model file: %w", err)
	}
	if err := files.FormatGoFile(result.ModelPath); err != nil {
		return fmt.Errorf("failed to format model file: %w", err)
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
