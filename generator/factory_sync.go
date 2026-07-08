package generator

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mbvlabs/andurel/generator/models"
	"github.com/mbvlabs/andurel/pkg/naming"
	"github.com/pmezard/go-difflib/difflib"
)

type FactorySyncOptions struct {
	Check bool
	Sync  bool
	Diff  bool
}

type FactorySyncResult struct {
	ResourceName string   `json:"resource_name"`
	Path         string   `json:"path"`
	Missing      bool     `json:"missing"`
	Stale        bool     `json:"stale"`
	Written      bool     `json:"written"`
	Diff         string   `json:"diff,omitempty"`
	Messages     []string `json:"messages,omitempty"`

	newContent string
}

func (r FactorySyncResult) HasDrift() bool {
	return r.Missing || r.Stale
}

func (m *ModelManager) SyncFactory(resourceName string, opts FactorySyncOptions) (*FactorySyncResult, error) {
	genModel, tableName, err := m.factoryModelFromEntity(resourceName)
	if err != nil {
		return nil, err
	}

	result, err := m.planFactorySync(resourceName, tableName, genModel, opts)
	if err != nil {
		return nil, err
	}
	if opts.Sync && result.HasDrift() {
		if err := os.MkdirAll(filepath.Dir(result.Path), 0o755); err != nil {
			return nil, fmt.Errorf("create factories directory: %w", err)
		}
		if err := os.WriteFile(result.Path, []byte(result.newContent), 0o600); err != nil {
			return nil, fmt.Errorf("write factory file: %w", err)
		}
		result.Written = true
	}
	return result, nil
}

func (m *ModelManager) SyncFactories(opts FactorySyncOptions) ([]*FactorySyncResult, error) {
	names, err := m.discoverFactoryResourceNames()
	if err != nil {
		return nil, err
	}

	results := make([]*FactorySyncResult, 0, len(names))
	for _, name := range names {
		result, err := m.SyncFactory(name, opts)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func (m *ModelManager) discoverFactoryResourceNames() ([]string, error) {
	entries, err := os.ReadDir(m.config.Paths.Models)
	if err != nil {
		return nil, fmt.Errorf("read models directory: %w", err)
	}

	names := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(m.config.Paths.Models, entry.Name())
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read model file %s: %w", path, err)
		}
		for _, name := range entityNames(src) {
			names = append(names, strings.TrimSuffix(name, "Entity"))
		}
	}
	slices.Sort(names)
	return names, nil
}

func (m *ModelManager) factoryModelFromEntity(resourceName string) (*models.GeneratedModel, string, error) {
	modelPath := BuildModelPath(m.config.Paths.Models, resourceName)
	src, err := os.ReadFile(modelPath)
	if err != nil {
		return nil, "", fmt.Errorf("read model file: %w", err)
	}

	entityName := resourceName + "Entity"
	fields, _, _, err := parseEntityStruct(src, entityName)
	if err != nil {
		return nil, "", err
	}

	tableName := ResolveTableName(m.config.Paths.Models, resourceName)
	genModel := generatedModelFromParsedEntity(resourceName, tableName, m.projectManager.GetModulePath(), fields)
	return genModel, tableName, nil
}

func generatedModelFromParsedEntity(resourceName, tableName, modulePath string, fields []parsedField) *models.GeneratedModel {
	genModel := &models.GeneratedModel{
		Name:          resourceName,
		PluralName:    naming.DeriveTableName(resourceName),
		EntityName:    resourceName + "Entity",
		NamespaceVar:  resourceName,
		Package:       "models",
		TableName:     tableName,
		ModulePath:    modulePath,
		Fields:        make([]models.GeneratedField, 0, len(fields)),
		HasPrimaryKey: false,
	}

	for _, field := range fields {
		generated := models.GeneratedField{
			Name:         field.Name,
			Type:         field.TypeStr,
			BunTag:       field.BunTag,
			IsForeignKey: field.Name != "ID" && strings.HasSuffix(field.Name, "ID"),
			IsNullable:   strings.HasPrefix(field.TypeStr, "*") || strings.HasPrefix(field.TypeStr, "sql.Null") || strings.HasPrefix(field.TypeStr, "bun.Null"),
			IsPrimaryKey: field.Name == "ID" || strings.Contains(field.BunTag, "pk"),
		}
		genModel.Fields = append(genModel.Fields, generated)

		if generated.IsPrimaryKey && !genModel.HasPrimaryKey {
			genModel.HasPrimaryKey = true
			genModel.IDType = generated.Type
			genModel.IDGoType = generated.Type
			genModel.IDGoFieldName = generated.Name
		}
		if field.Name == "CreatedAt" {
			genModel.HasCreatedAt = true
		}
		if field.Name == "UpdatedAt" {
			genModel.HasUpdatedAt = true
		}
	}

	if genModel.IDGoFieldName == "" {
		genModel.IDGoFieldName = "ID"
		genModel.IDType = "uuid.UUID"
		genModel.IDGoType = "uuid.UUID"
	}

	return genModel
}

func (m *ModelManager) planFactorySync(resourceName, tableName string, genModel *models.GeneratedModel, opts FactorySyncOptions) (*FactorySyncResult, error) {
	rootDir, err := m.fileManager.FindGoModRoot()
	if err != nil {
		return nil, fmt.Errorf("find project root: %w", err)
	}

	factoryPath := filepath.Join(rootDir, "models", "factories", naming.ToSnakeCase(resourceName)+".go")
	oldContent := ""
	missing := false
	if src, err := os.ReadFile(factoryPath); err == nil {
		oldContent = string(src)
	} else if os.IsNotExist(err) {
		missing = true
	} else {
		return nil, fmt.Errorf("read factory file: %w", err)
	}

	genFactory, err := m.modelGenerator.BuildFactory(nil, models.Config{
		TableName:    tableName,
		ResourceName: resourceName,
		PackageName:  "factories",
		DatabaseType: m.config.Database.Type,
		ModulePath:   m.projectManager.GetModulePath(),
	}, genModel)
	if err != nil {
		return nil, fmt.Errorf("build factory metadata: %w", err)
	}

	newContent, err := renderSyncedFactoryFile(genFactory, oldContent)
	if err != nil {
		return nil, err
	}

	stale := oldContent != newContent
	result := &FactorySyncResult{
		ResourceName: resourceName,
		Path:         factoryPath,
		Missing:      missing,
		Stale:        !missing && stale,
		newContent:   newContent,
	}

	if opts.Diff && stale {
		result.Diff, err = factoryUnifiedDiff(oldContent, newContent)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func renderSyncedFactoryFile(factory *models.GeneratedFactory, oldContent string) (string, error) {
	generatedOptions := expectedFactoryOptionNames(factory)
	customDecls, oldImports, err := customFactoryDecls(oldContent, factory, generatedOptions)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("package factories\n\n")
	writeFactoryImports(&sb, factory, oldImports)
	sb.WriteString("\n// Factory declarations below are generated by Andurel.\n\n")
	writeFactoryCore(&sb, factory)
	sb.WriteString("\n")
	writeFactoryOptions(&sb, factory)
	if strings.TrimSpace(customDecls) != "" {
		sb.WriteString("\n")
		sb.WriteString(strings.TrimSpace(customDecls))
		sb.WriteString("\n")
	}

	formatted, err := format.Source([]byte(sb.String()))
	if err != nil {
		return sb.String(), nil
	}
	return string(formatted), nil
}

func writeFactoryImports(sb *strings.Builder, factory *models.GeneratedFactory, oldImports []string) {
	imports := map[string]bool{
		"context":                                true,
		"fmt":                                    true,
		factory.ModulePath + "/internal/storage": true,
		factory.ModulePath + "/models":           true,
		"github.com/go-faker/faker/v4":           true,
	}
	if !factory.IsAutoIncrementID && (factory.IDType == "" || factory.IDType == "uuid.UUID") {
		imports["github.com/google/uuid"] = true
	}
	if factory.HasCreatedAt || factory.HasUpdatedAt {
		imports["time"] = true
	}
	for _, field := range factory.Fields {
		switch {
		case strings.Contains(field.Type, "time.Time") || strings.Contains(field.Type, "NullTime"):
			imports["time"] = true
		case strings.HasPrefix(field.Type, "sql."):
			imports["database/sql"] = true
		case strings.HasPrefix(field.Type, "bun."):
			imports["github.com/uptrace/bun"] = true
		case strings.HasPrefix(field.Type, "json."):
			imports["encoding/json"] = true
		}
	}
	for _, oldImport := range oldImports {
		imports[oldImport] = true
	}

	ordered := make([]string, 0, len(imports))
	for imp := range imports {
		if imp != "" {
			ordered = append(ordered, imp)
		}
	}
	slices.Sort(ordered)

	sb.WriteString("import (\n")
	for _, imp := range ordered {
		fmt.Fprintf(sb, "\t%q\n", imp)
	}
	sb.WriteString(")\n")
}

func writeFactoryCore(sb *strings.Builder, factory *models.GeneratedFactory) {
	fmt.Fprintf(sb, "type %sFactory struct {\n\tmodels.%s\n}\n\n", factory.ModelName, factory.EntityName)
	fmt.Fprintf(sb, "type %sOption func(*%sFactory)\n\n", factory.ModelName, factory.ModelName)

	fmt.Fprintf(sb, "func Build%s(", factory.ModelName)
	writeFactoryFKParams(sb, factory)
	fmt.Fprintf(sb, "opts ...%sOption) models.%s {\n", factory.ModelName, factory.EntityName)
	fmt.Fprintf(sb, "\tf := &%sFactory{\n\t\t%s: models.%s{\n", factory.ModelName, factory.EntityName, factory.EntityName)
	for _, field := range factory.Fields {
		if field.IsAutoManaged {
			continue
		}
		if field.IsFK {
			fmt.Fprintf(sb, "\t\t\t%s: %s,\n", field.Name, field.ArgumentName)
			continue
		}
		fmt.Fprintf(sb, "\t\t\t%s: %s,\n", field.Name, field.DefaultValue)
	}
	sb.WriteString("\t\t},\n\t}\n\n")
	sb.WriteString("\tfor _, opt := range opts {\n\t\topt(f)\n\t}\n\n")
	fmt.Fprintf(sb, "\treturn f.%s\n}\n\n", factory.EntityName)

	writeFactoryCreateFunctions(sb, factory)
}

func writeFactoryCreateFunctions(sb *strings.Builder, factory *models.GeneratedFactory) {
	fmt.Fprintf(sb, "func Create%s(ctx context.Context, exec storage.Executor, ", factory.ModelName)
	writeFactoryFKParams(sb, factory)
	fmt.Fprintf(sb, "opts ...%sOption) (models.%s, error) {\n", factory.ModelName, factory.EntityName)
	fmt.Fprintf(sb, "\tbuilt := Build%s(", factory.ModelName)
	writeFactoryFKArgs(sb, factory)
	sb.WriteString("opts...)\n\n")
	fmt.Fprintf(sb, "\tentity := models.%s{\n", factory.EntityName)
	if !factory.IsAutoIncrementID && factory.IDGoFieldName != "" {
		if factory.IDType == "" || factory.IDType == "uuid.UUID" {
			fmt.Fprintf(sb, "\t\t%s: uuid.New(),\n", factory.IDGoFieldName)
		} else {
			fmt.Fprintf(sb, "\t\t%s: built.%s,\n", factory.IDGoFieldName, factory.IDGoFieldName)
		}
	}
	if factory.HasCreatedAt {
		sb.WriteString("\t\tCreatedAt: time.Now(),\n")
	}
	if factory.HasUpdatedAt {
		sb.WriteString("\t\tUpdatedAt: time.Now(),\n")
	}
	for _, field := range factory.Fields {
		if field.IsAutoManaged {
			continue
		}
		fmt.Fprintf(sb, "\t\t%s: built.%s,\n", field.Name, field.Name)
	}
	sb.WriteString("\t}\n\n")
	sb.WriteString("\tif err := exec.NewInsert().Model(&entity).Returning(\"*\").Scan(ctx); err != nil {\n")
	fmt.Fprintf(sb, "\t\treturn models.%s{}, err\n\t}\n\n", factory.EntityName)
	sb.WriteString("\treturn entity, nil\n}\n\n")

	fmt.Fprintf(sb, "func Create%ss(ctx context.Context, exec storage.Executor, ", factory.ModelName)
	writeFactoryFKParams(sb, factory)
	fmt.Fprintf(sb, "count int, opts ...%sOption) ([]models.%s, error) {\n", factory.ModelName, factory.EntityName)
	lower := strings.ToLower(factory.ModelName)
	fmt.Fprintf(sb, "\t%ss := make([]models.%s, 0, count)\n\n", lower, factory.EntityName)
	sb.WriteString("\tfor i := 0; i < count; i++ {\n")
	fmt.Fprintf(sb, "\t\tentity, err := Create%s(ctx, exec, ", factory.ModelName)
	writeFactoryFKArgs(sb, factory)
	sb.WriteString("opts...)\n")
	sb.WriteString("\t\tif err != nil {\n")
	fmt.Fprintf(sb, "\t\t\treturn nil, fmt.Errorf(\"failed to create %s %%d: %%w\", i+1, err)\n\t\t}\n", lower)
	fmt.Fprintf(sb, "\t\t%ss = append(%ss, entity)\n\t}\n\n", lower, lower)
	fmt.Fprintf(sb, "\treturn %ss, nil\n}\n", lower)
}

func writeFactoryOptions(sb *strings.Builder, factory *models.GeneratedFactory) {
	for _, field := range factory.Fields {
		if field.IsAutoManaged {
			continue
		}
		fmt.Fprintf(sb, "func %s(value %s) %sOption {\n", field.OptionName, field.Type, factory.ModelName)
		fmt.Fprintf(sb, "\treturn func(f *%sFactory) {\n", factory.ModelName)
		fmt.Fprintf(sb, "\t\tf.%s.%s = value\n", factory.EntityName, field.Name)
		sb.WriteString("\t}\n}\n\n")
	}
}

func writeFactoryFKParams(sb *strings.Builder, factory *models.GeneratedFactory) {
	for _, field := range factory.ForeignKeyFields {
		fmt.Fprintf(sb, "%s %s, ", field.ArgumentName, field.Type)
	}
}

func writeFactoryFKArgs(sb *strings.Builder, factory *models.GeneratedFactory) {
	for _, field := range factory.ForeignKeyFields {
		fmt.Fprintf(sb, "%s, ", field.ArgumentName)
	}
}

func expectedFactoryOptionNames(factory *models.GeneratedFactory) map[string]bool {
	names := make(map[string]bool)
	for _, field := range factory.Fields {
		if field.IsAutoManaged {
			continue
		}
		names[field.OptionName] = true
	}
	return names
}

func customFactoryDecls(src string, factory *models.GeneratedFactory, expectedOptions map[string]bool) (string, []string, error) {
	if src == "" {
		return "", nil, nil
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return "", nil, fmt.Errorf("parse existing factory: %w", err)
	}

	var imports []existingFactoryImport
	for _, imp := range file.Imports {
		imports = append(imports, existingFactoryImport{
			Path: strings.Trim(imp.Path.Value, `"`),
			Name: importLocalName(imp),
		})
	}

	var custom strings.Builder
	var retainedDecls []ast.Decl
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
			continue
		}
		if isGeneratedFactoryDecl(decl, factory) ||
			isExpectedFactoryOptionDecl(decl, expectedOptions) {
			continue
		}
		start := fset.Position(decl.Pos()).Offset
		end := fset.Position(decl.End()).Offset
		custom.WriteString(strings.TrimSpace(src[start:end]))
		custom.WriteString("\n\n")
		retainedDecls = append(retainedDecls, decl)
	}
	return custom.String(), retainedCustomImportPaths(imports, retainedDecls), nil
}

type existingFactoryImport struct {
	Path string
	Name string
}

func importLocalName(imp *ast.ImportSpec) string {
	if imp.Name != nil {
		return imp.Name.Name
	}
	base := path.Base(strings.Trim(imp.Path.Value, `"`))
	if base == "" || strings.Contains(base, ".") || strings.Contains(base, "-") {
		return ""
	}
	return base
}

func retainedCustomImportPaths(imports []existingFactoryImport, decls []ast.Decl) []string {
	if len(decls) == 0 {
		return nil
	}

	usedNames := make(map[string]bool)
	for _, decl := range decls {
		ast.Inspect(decl, func(node ast.Node) bool {
			if selector, ok := node.(*ast.SelectorExpr); ok {
				if ident, ok := selector.X.(*ast.Ident); ok {
					usedNames[ident.Name] = true
				}
			}
			return true
		})
	}

	paths := make([]string, 0, len(imports))
	for _, imp := range imports {
		switch imp.Name {
		case "", ".", "_":
			paths = append(paths, imp.Path)
		default:
			if usedNames[imp.Name] {
				paths = append(paths, imp.Path)
			}
		}
	}
	return paths
}

func isExpectedFactoryOptionDecl(decl ast.Decl, expectedOptions map[string]bool) bool {
	fn, ok := decl.(*ast.FuncDecl)
	return ok && expectedOptions[fn.Name.Name]
}

func isGeneratedFactoryDecl(decl ast.Decl, factory *models.GeneratedFactory) bool {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		name := d.Name.Name
		return name == "Build"+factory.ModelName ||
			name == "Create"+factory.ModelName ||
			name == "Create"+factory.ModelName+"s"
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				return false
			}
			if typeSpec.Name.Name != factory.ModelName+"Factory" && typeSpec.Name.Name != factory.ModelName+"Option" {
				return false
			}
		}
		return len(d.Specs) > 0
	default:
		return false
	}
}

func factoryUnifiedDiff(oldContent, newContent string) (string, error) {
	d := difflib.UnifiedDiff{
		A:        difflib.SplitLines(oldContent),
		B:        difflib.SplitLines(newContent),
		FromFile: "current",
		ToFile:   "updated",
		Context:  2,
	}
	return difflib.GetUnifiedDiffString(d)
}

func entityNames(src []byte) []string {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		return nil
	}
	var names []string
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || !strings.HasSuffix(typeSpec.Name.Name, "Entity") {
				continue
			}
			if _, ok := typeSpec.Type.(*ast.StructType); ok {
				names = append(names, typeSpec.Name.Name)
			}
		}
	}
	return names
}
