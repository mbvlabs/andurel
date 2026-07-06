package views

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/generator/internal/types"
	"github.com/mbvlabs/andurel/generator/internal/validation"
	"github.com/mbvlabs/andurel/generator/templates"
	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/pkg/constants"
	"github.com/mbvlabs/andurel/pkg/errors"
	"github.com/mbvlabs/andurel/pkg/naming"
)

type ViewField struct {
	Name            string
	GoType          string
	GoFormType      string
	DisplayName     string
	IsTimestamp     bool
	InputType       string
	StringConverter string
	DBName          string
	CamelCase       string
	IsSystemField   bool
}

type InertiaPageData struct {
	*GeneratedView
	ComponentName string
}

type GeneratedView struct {
	ResourceName    string
	ModelName       string
	EntityName      string
	PluralName      string
	ModelPluralName string
	Namespace       string
	NamespacePascal string
	Fields          []ViewField
	ModulePath      string
	IDType          string // "uuid.UUID", "int32", "int64", "string"
	IDFieldName     string
	Actions         []string
}

type Config struct {
	ResourceName    string
	ModelName       string
	EntityName      string
	PluralName      string
	ModelPluralName string
	TableName       string
	ModelTableName  string
	Namespace       string
	ModulePath      string
	Actions         []string
}

type Generator struct {
	typeMapper  *types.TypeMapper
	fileManager files.Manager
}

func NewGenerator(databaseType string) *Generator {
	return &Generator{
		typeMapper:  types.NewTypeMapper(databaseType),
		fileManager: files.NewUnifiedFileManager(),
	}
}

func (g *Generator) Build(cat *catalog.Catalog, config Config) (*GeneratedView, error) {
	modelName := config.ModelName
	if modelName == "" {
		modelName = config.ResourceName
	}
	modelPluralName := config.ModelPluralName
	if modelPluralName == "" {
		modelPluralName = config.PluralName
	}
	view := &GeneratedView{
		ResourceName:    config.ResourceName,
		ModelName:       modelName,
		EntityName:      config.EntityName,
		PluralName:      config.PluralName,
		ModelPluralName: modelPluralName,
		Namespace:       config.Namespace,
		NamespacePascal: naming.ToPascalCase(config.Namespace),
		ModulePath:      config.ModulePath,
		Fields:          make([]ViewField, 0),
		IDType:          "uuid.UUID", // Default to UUID
		IDFieldName:     "ID",
		Actions:         config.Actions,
	}

	tableName := config.TableName
	if config.ModelTableName != "" {
		tableName = config.ModelTableName
	}
	if tableName == "" {
		tableName = modelPluralName
	}
	table, err := cat.GetTable("", tableName)
	if err != nil {
		return nil, errors.NewDatabaseError("get table", tableName, err)
	}

	for _, col := range table.Columns {
		// Detect ID type from primary key column
		if col.IsPrimaryKey {
			pkType, _ := validation.ClassifyPrimaryKeyType(col.DataType)
			view.IDType = validation.GoType(pkType)
			view.IDFieldName = types.FormatFieldName(col.Name)
			continue
		}
		if col.Name == "id" {
			continue
		}

		field, err := g.buildViewField(col)
		if err != nil {
			return nil, fmt.Errorf("failed to build field for column %s: %w", col.Name, err)
		}
		view.Fields = append(view.Fields, field)
	}

	return view, nil
}

// resolveViewBaseType strips null type wrappers and pointer prefixes to get
// the underlying scalar type for view rendering purposes.
func resolveViewBaseType(goType string) string {
	switch goType {
	case "sql.NullString", "bun.NullString":
		return "string"
	case "sql.NullBool", "bun.NullBool":
		return "bool"
	case "sql.NullInt16":
		return "int16"
	case "sql.NullInt32", "bun.NullInt32":
		return "int32"
	case "sql.NullInt64", "bun.NullInt64":
		return "int64"
	case "sql.NullFloat64", "bun.NullFloat64":
		return "float64"
	case "sql.NullTime", "bun.NullTime":
		return "time.Time"
	}
	return strings.TrimPrefix(goType, "*")
}

func hasNullFields(fields []ViewField) bool {
	for _, field := range fields {
		if isNullType(field.GoType) {
			return true
		}
	}
	return false
}

func isNullType(goType string) bool {
	return strings.HasPrefix(goType, "sql.Null") || strings.HasPrefix(goType, "bun.Null")
}

func usesViewDataType(fields []ViewField, goType string) bool {
	for _, field := range fields {
		if strings.TrimPrefix(viewDataType(field), "*") == goType {
			return true
		}
	}
	return false
}

func viewDataType(field ViewField) string {
	if isNullType(field.GoType) {
		return resolveViewBaseType(field.GoType)
	}
	return field.GoType
}

func viewDataValue(field ViewField, source string) string {
	switch field.GoType {
	case "sql.NullString", "bun.NullString":
		return "func() string { if !" + source + ".Valid { return \"\" }; return " + source + ".String }()"
	case "sql.NullBool", "bun.NullBool":
		return "func() bool { if !" + source + ".Valid { return false }; return " + source + ".Bool }()"
	case "sql.NullInt16":
		return "func() int16 { if !" + source + ".Valid { return 0 }; return " + source + ".Int16 }()"
	case "sql.NullInt32", "bun.NullInt32":
		return "func() int32 { if !" + source + ".Valid { return 0 }; return " + source + ".Int32 }()"
	case "sql.NullInt64", "bun.NullInt64":
		return "func() int64 { if !" + source + ".Valid { return 0 }; return " + source + ".Int64 }()"
	case "sql.NullFloat64", "bun.NullFloat64":
		return "func() float64 { if !" + source + ".Valid { return 0 }; return " + source + ".Float64 }()"
	case "sql.NullTime", "bun.NullTime":
		return "func() time.Time { if !" + source + ".Valid { return time.Time{} }; return " + source + ".Time }()"
	default:
		return source
	}
}

func viewDataRef(namespacePascal, resourceName, entityRef string, useDTO bool) string {
	if !useDTO {
		return entityRef
	}
	return fmt.Sprintf("new%s%sData(%s)", namespacePascal, resourceName, entityRef)
}

func viewDataRowRef(namespacePascal, resourceName, rowRef string, useDTO bool) string {
	if !useDTO {
		return rowRef
	}
	if namespacePascal == "" {
		return rowRef + "Data"
	}
	dtoPrefix := naming.ToLowerCamelCase(namespacePascal) + resourceName
	return dtoPrefix + "Data"
}

func viewDataLoopAssignment(namespacePascal, resourceName, rowRef string, useDTO bool) string {
	if !useDTO {
		return "{"
	}
	qualifiedName := namespacePascal + resourceName
	dtoVar := rowRef + "Data"
	if namespacePascal != "" {
		dtoVar = naming.ToLowerCamelCase(namespacePascal) + resourceName + "Data"
	}
	return fmt.Sprintf("{\n\t\t\t\t\t\t\t\t\t{{ %s := new%sData(%s) }}", dtoVar, qualifiedName, rowRef)
}

func viewDataImports(fields []ViewField) string {
	if !hasNullFields(fields) {
		return ""
	}

	var b strings.Builder
	if usesViewDataType(fields, "json.RawMessage") {
		b.WriteString("\t\"encoding/json\"\n")
	}
	if usesViewDataType(fields, "time.Time") {
		b.WriteString("\t\"time\"\n")
	}
	if usesViewDataType(fields, "uuid.UUID") {
		b.WriteString("\t\"github.com/google/uuid\"\n")
	}
	return b.String()
}

func viewDataDefinition(view *GeneratedView) string {
	if !hasNullFields(view.Fields) {
		return ""
	}

	prefix := view.NamespacePascal

	var b strings.Builder
	fmt.Fprintf(&b, "\ntype %s%sData struct {\n", prefix, view.ResourceName)
	for _, field := range view.Fields {
		fmt.Fprintf(&b, "\t%s %s\n", field.Name, viewDataType(field))
	}
	b.WriteString("}\n\n")
	fmt.Fprintf(
		&b,
		"func new%s%sData(entity models.%s) %s%sData {\n",
		prefix, view.ResourceName,
		view.EntityName,
		prefix, view.ResourceName,
	)
	fmt.Fprintf(&b, "\treturn %s%sData{\n", prefix, view.ResourceName)
	for _, field := range view.Fields {
		fmt.Fprintf(&b, "\t\t%s: %s,\n", field.Name, viewDataValue(field, "entity."+field.Name))
	}
	b.WriteString("\t}\n")
	b.WriteString("}\n")
	return b.String()
}

func (g *Generator) buildViewField(col *catalog.Column) (ViewField, error) {
	goType, _, err := g.typeMapper.MapSQLTypeToGo(col.DataType, col.IsNullable)
	if err != nil {
		goType = "string"
	}

	viewGoType := resolveViewBaseType(goType)

	field := ViewField{
		Name:          types.FormatFieldName(col.Name),
		DisplayName:   types.FormatDisplayName(col.Name),
		DBName:        col.Name,
		CamelCase:     types.FormatCamelCase(col.Name),
		IsSystemField: col.Name == "created_at" || col.Name == "updated_at",
		GoType:        goType,
	}

	switch viewGoType {
	case "time.Time":
		field.IsTimestamp = true
		field.InputType = "date"
		field.StringConverter = "%s.String()"
	case "string":
		field.InputType = "text"
		field.StringConverter = ""
	case "int16":
		field.InputType = "number"
		field.StringConverter = "fmt.Sprintf(\"%d\", %s)"
	case "int32":
		field.InputType = "number"
		field.StringConverter = "fmt.Sprintf(\"%d\", %s)"
	case "int64":
		field.InputType = "number"
		field.StringConverter = "fmt.Sprintf(\"%d\", %s)"
	case "float32":
		field.InputType = "number"
		field.StringConverter = "fmt.Sprintf(\"%f\", %s)"
	case "float64":
		field.InputType = "number"
		field.StringConverter = "fmt.Sprintf(\"%f\", %s)"
	case "bool":
		field.InputType = "checkbox"
		field.StringConverter = "fmt.Sprintf(\"%t\", %s)"
	case "uuid.UUID":
		field.InputType = "text"
		field.StringConverter = "%s.String()"
	case "[]byte":
		field.InputType = "text"
		field.StringConverter = "string(%s)"
	case "[]int32":
		field.InputType = "text"
		field.StringConverter = "fmt.Sprintf(\"%v\", %s)"
	case "[]string":
		field.InputType = "text"
		field.StringConverter = "strings.Join(%s, \", \")"
	case "interface{}":
		field.InputType = "text"
		field.StringConverter = "fmt.Sprintf(\"%v\", %s)"
	default:
		field.InputType = "text"
		field.StringConverter = "fmt.Sprintf(\"%v\", %s)"
	}

	switch viewGoType {
	case "time.Time":
		field.GoFormType = "time.Time"
	case "int16":
		field.GoFormType = "int16"
	case "int32":
		field.GoFormType = "int32"
	case "int64":
		field.GoFormType = "int64"
	case "float32":
		field.GoFormType = "float32"
	case "float64":
		field.GoFormType = "float64"
	case "bool":
		field.GoFormType = "bool"
	default:
		field.GoFormType = "string"
	}

	return field, nil
}

func (g *Generator) templatePrefix(lock *layout.AndurelLock) string {
	hasCssComponents := false

	if lock != nil {
		if _, ok := lock.Extensions["css-components"]; ok {
			hasCssComponents = true
		}
	}

	if hasCssComponents {
		return "tw_"
	}

	return "tw_bare_"
}

func (g *Generator) GenerateViewFile(view *GeneratedView, withController bool, templatePrefix string) (string, error) {
	// Custom template functions for view-specific operations
	customFuncs := template.FuncMap{
		"HasNullFields":    hasNullFields,
		"UsesViewDataType": usesViewDataType,
		"ViewDataType":     viewDataType,
		"ViewDataValue":    viewDataValue,
		"ViewDataRef":      viewDataRef,
		"ViewDataRowRef":   viewDataRowRef,
		"ViewDataLoop":     viewDataLoopAssignment,
		"ViewDataImports":  viewDataImports,
		"ViewData":         viewDataDefinition,
		"UsesPackage": func(fields []ViewField, packageName string) bool {
			for _, field := range fields {
				if strings.Contains(field.StringConverter, packageName+".") {
					return true
				}
			}
			return false
		},
		"FieldRef": func(field ViewField, objRef string) string {
			return fmt.Sprintf("%s.%s", objRef, field.Name)
		},
		"StringDisplay": func(field ViewField, objRef string) string {
			if field.StringConverter == "" {
				return fmt.Sprintf(
					"{ %s.%s }",
					objRef,
					field.Name,
				)
			}
			converter := strings.ReplaceAll(
				field.StringConverter,
				"%s",
				fmt.Sprintf("%s.%s", objRef, field.Name),
			)
			return fmt.Sprintf("{ %s }", converter)
		},
		"StringTableDisplay": func(field ViewField, objRef string) string {
			if field.StringConverter == "" {
				return fmt.Sprintf(
					"{ %s.%s }",
					objRef,
					field.Name,
				)
			}
			converter := strings.ReplaceAll(
				field.StringConverter,
				"%s",
				fmt.Sprintf("%s.%s", objRef, field.Name),
			)
			return fmt.Sprintf("{ %s }", converter)
		},
		"StringValue": func(field ViewField, objRef string) string {
			if field.StringConverter == "" {
				return fmt.Sprintf(
					"%s.%s",
					objRef,
					field.Name,
				)
			}
			return strings.ReplaceAll(
				field.StringConverter,
				"%s",
				fmt.Sprintf("%s.%s", objRef, field.Name),
			)
		},
		"HasAction": func(action string) bool {
			if len(view.Actions) == 0 {
				return true
			}
			return slices.Contains(view.Actions, action)
		},
	}

	templateName := templatePrefix + "resource_view_no_controller.tmpl"
	if withController {
		templateName = templatePrefix + "resource_view.tmpl"
	}

	// Use the unified template service with custom functions
	service := templates.GetGlobalTemplateService()
	result, err := service.RenderTemplateWithCustomFunctions(templateName, view, customFuncs)
	if err != nil {
		return "", errors.WrapTemplateError(err, "render view", templateName)
	}
	return result, nil
}

func (g *Generator) GenerateInertiaViewFiles(view *GeneratedView, templatePrefix, extension string) (map[string]string, error) {
	service := templates.GetGlobalTemplateService()
	fileNames := make(map[string]string, 4)

	defaultComponents := []string{"Index", "Show", "Create", "Edit"}
	components := defaultComponents
	if len(view.Actions) > 0 {
		actionSet := make(map[string]struct{}, len(defaultComponents))
		for _, a := range defaultComponents {
			actionSet[strings.ToLower(a)] = struct{}{}
		}
		components = make([]string, 0, len(view.Actions))
		for _, action := range view.Actions {
			pascal := naming.ToPascalCase(action)
			if _, ok := actionSet[strings.ToLower(pascal)]; ok {
				components = append(components, pascal)
			}
		}
	}

	templateName := templatePrefix + "resource_view.tmpl"
	for _, componentName := range components {
		result, err := service.RenderTemplate(templateName, InertiaPageData{
			GeneratedView: view,
			ComponentName: componentName,
		})
		if err != nil {
			return nil, errors.WrapTemplateError(err, "render inertia view", templateName)
		}
		fileNames[componentName+extension] = result
	}

	return fileNames, nil
}

func namespacePrefix(namespace string) string {
	if namespace == "" {
		return ""
	}
	return namespace + "_"
}

func (g *Generator) GenerateView(
	cat *catalog.Catalog,
	resourceName string,
	tableName string,
	modulePath string,
	namespace string,
) error {
	return g.GenerateViewWithController(cat, resourceName, tableName, modulePath, false, "", namespace)
}

func (g *Generator) GenerateViewWithController(
	cat *catalog.Catalog,
	resourceName string,
	tableName string,
	modulePath string,
	withController bool,
	inertia string,
	namespace string,
) error {
	return g.GenerateViewWithControllerActions(cat, resourceName, tableName, modulePath, withController, nil, inertia, namespace)
}

func (g *Generator) GenerateViewWithControllerActions(
	cat *catalog.Catalog,
	resourceName string,
	tableName string,
	modulePath string,
	withController bool,
	actions []string,
	inertia string,
	namespace string,
) error {
	return g.GenerateViewWithControllerActionsForModel(cat, resourceName, resourceName, tableName, tableName, modulePath, namespace, withController, actions, inertia)
}

func (g *Generator) GenerateViewWithControllerActionsForModel(
	cat *catalog.Catalog,
	resourceName string,
	modelName string,
	tableName string,
	modelTableName string,
	modulePath string,
	namespace string,
	withController bool,
	actions []string,
	inertia string,
) error {
	if modelName == "" {
		modelName = resourceName
	}
	if modelTableName == "" {
		modelTableName = tableName
	}
	pluralName := naming.DeriveTableName(resourceName)
	modelPluralName := naming.DeriveTableName(modelName)
	viewPath := filepath.Join("views", namespacePrefix(namespace)+tableName+"_resource.templ")

	viewExists := false
	if _, err := os.Stat(viewPath); err == nil {
		viewExists = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat view file %s: %w", viewPath, err)
	}
	isInertia := layout.IsSupportedInertiaAdapter(inertia)
	renderActions := actions
	if !isInertia && viewExists && len(actions) > 0 {
		existingActions, err := existingResourceViewActions(viewPath, resourceName, namespace)
		if err != nil {
			return err
		}
		renderActions = mergeResourceViewActions(existingActions, actions)
	}

	// Read lock file to determine extensions and view layer.
	templatePrefix := "tw_bare_"
	var lock *layout.AndurelLock
	if rootDir, err := g.fileManager.FindGoModRoot(); err == nil {
		if projectLock, err := layout.ReadLockFile(rootDir); err == nil {
			lock = projectLock
			templatePrefix = g.templatePrefix(lock)
		}
	}

	// Override inertia mode from parameter if explicitly set
	if isInertia {
		templatePrefix = inertiaViewTemplatePrefix(inertia)
	}

	view, err := g.Build(cat, Config{
		ResourceName:    resourceName,
		ModelName:       modelName,
		EntityName:      modelName + "Entity",
		PluralName:      pluralName,
		ModelPluralName: modelPluralName,
		TableName:       tableName,
		ModelTableName:  modelTableName,
		Namespace:       namespace,
		ModulePath:      modulePath,
		Actions:         renderActions,
	})
	if err != nil {
		return fmt.Errorf("failed to build view: %w", err)
	}

	if isInertia {
		return g.generateInertiaViews(view, templatePrefix, resourceName, inertia)
	}

	if viewExists && len(actions) == 0 {
		return fmt.Errorf("view file %s already exists", viewPath)
	}

	viewContent, err := g.GenerateViewFile(view, withController, templatePrefix)
	if err != nil {
		return fmt.Errorf("failed to render view file: %w", err)
	}

	if err := g.fileManager.EnsureDir("views"); err != nil {
		return err
	}

	if err := os.WriteFile(viewPath, []byte(viewContent), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write view file: %w", err)
	}

	if err := g.formatTemplFile(viewPath); err != nil {
		return fmt.Errorf("failed to format view file: %w", err)
	}

	if err := g.runCompileTemplates(); err != nil {
		return fmt.Errorf("failed to compile templates: %w", err)
	}

	fmt.Printf("Successfully generated view at %s\n", viewPath)
	return nil
}

func inertiaViewTemplatePrefix(adapter string) string {
	switch adapter {
	case "react":
		return "inertia_react_tw_bare_"
	default:
		return "inertia_vue_tw_bare_"
	}
}

func inertiaViewExtension(adapter string) string {
	switch adapter {
	case "react":
		return ".tsx"
	default:
		return ".vue"
	}
}

func (g *Generator) generateInertiaViews(view *GeneratedView, templatePrefix, resourceName, adapter string) error {
	inertiaFiles, err := g.GenerateInertiaViewFiles(view, templatePrefix, inertiaViewExtension(adapter))
	if err != nil {
		return fmt.Errorf("failed to render inertia view files: %w", err)
	}

	pagesDir := filepath.Join("resources", "js", "Pages", view.NamespacePascal, resourceName)
	if err := g.fileManager.EnsureDir(pagesDir); err != nil {
		return err
	}

	for fileName, content := range inertiaFiles {
		filePath := filepath.Join(pagesDir, fileName)
		if _, err := os.Stat(filePath); err == nil {
			return fmt.Errorf("view file %s already exists", filePath)
		}
		if err := os.WriteFile(filePath, []byte(content), constants.FilePermissionPrivate); err != nil {
			return fmt.Errorf("failed to write inertia view file %s: %w", fileName, err)
		}
	}

	fmt.Printf("Successfully generated inertia views at %s\n", pagesDir)
	return nil
}

var resourceViewActions = []string{"index", "show", "new", "create", "edit", "update", "destroy"}

func mergeResourceViewActions(existing, requested []string) []string {
	if len(requested) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(existing)+len(requested))
	merged := make([]string, 0, len(existing)+len(requested))
	for _, group := range [][]string{existing, requested} {
		for _, action := range group {
			action = strings.ToLower(action)
			if _, ok := seen[action]; ok || !slices.Contains(resourceViewActions, action) {
				continue
			}
			seen[action] = struct{}{}
			merged = append(merged, action)
		}
	}
	return merged
}

func existingResourceViewActions(viewPath, resourceName, namespace string) ([]string, error) {
	content, err := os.ReadFile(viewPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read view file %s: %w", viewPath, err)
	}

	contentStr := string(content)
	prefix := naming.ToPascalCase(namespace)
	actions := make([]string, 0, len(resourceViewActions))
	for _, action := range []string{"index", "show", "new", "edit"} {
		typeName := prefix + resourceName + naming.ToPascalCase(action)
		if strings.Contains(contentStr, "type "+typeName) {
			actions = append(actions, action)
		}
	}
	for _, action := range []string{"create", "update", "destroy"} {
		routeName := "routes." + prefix + resourceName + naming.ToPascalCase(action)
		if strings.Contains(contentStr, routeName) {
			actions = append(actions, action)
		}
	}
	return actions, nil
}

func (g *Generator) formatTemplFile(filePath string) error {
	rootDir, err := g.fileManager.FindGoModRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	templBin := filepath.Join(rootDir, "bin", "templ")
	cmd := exec.Command(templBin, "fmt", filePath)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run templ fmt on %s: %w", filePath, err)
	}

	return nil
}

func (g *Generator) runCompileTemplates() error {
	rootDir, err := g.fileManager.FindGoModRoot()
	if err != nil {
		return nil
	}

	templBin := filepath.Join(rootDir, "bin", "templ")
	cmd := exec.Command(templBin, "generate")
	cmd.Dir = rootDir

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run templ generate: %w", err)
	}
	return nil
}
