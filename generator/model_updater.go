package generator

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/models"
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

// Diff returns a unified diff of the struct's column fields.
// bun.BaseModel is excluded — its alignment changes with field widths and is
// not schema content.
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

	existingFields, structStart, structEnd, err := parseEntityStruct(src, entityName)
	if err != nil {
		return nil, err
	}

	tableName := ResolveTableName(m.config.Paths.Models, resourceName)

	cat, err := m.migrationManager.BuildCatalogFromMigrations(tableName, m.config)
	if err != nil {
		return nil, err
	}

	newModel, err := m.modelGenerator.Build(cat, models.Config{
		TableName:    tableName,
		ResourceName: resourceName,
		PackageName:  "models",
		DatabaseType: m.config.Database.Type,
		ModulePath:   m.projectManager.GetModulePath(),
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

	oldStructStr := string(src[structStart:structEnd])
	newStructStr := renderEntityStruct(entityName, tableName, newModel.Fields)

	// Splice the new struct into the file and format.
	spliced := string(src[:structStart]) + newStructStr + string(src[structEnd:])
	formatted, err := format.Source([]byte(spliced))
	if err != nil {
		// Fall back to unformatted; goimports will clean it up on write.
		formatted = []byte(spliced)
	}

	// Re-extract the struct from the formatted content for a clean diff.
	formattedStructStr := newStructStr
	if _, fStart, fEnd, err := parseEntityStruct(formatted, entityName); err == nil {
		formattedStructStr = string(formatted[fStart:fEnd])
	}

	return &UpdateModelResult{
		OldStruct:      oldStructStr,
		NewStruct:      formattedStructStr,
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
	fmt.Fprintf(&sb, "\tbun.BaseModel `bun:\"table:%s\"`\n", tableName)
	sb.WriteString("\n")
	for _, f := range fields {
		fmt.Fprintf(&sb, "\t%s %s `bun:\"%s\"`\n", f.Name, f.Type, f.BunTag)
	}
	sb.WriteString("}")
	return sb.String()
}
