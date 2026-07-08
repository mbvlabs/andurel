package controllers

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/pkg/constants"
	"github.com/mbvlabs/andurel/pkg/naming"
)

// FileGenerator generates file artifacts.
type FileGenerator struct {
	fileManager      files.Manager
	templateRenderer *TemplateRenderer
	routeGenerator   *RouteGenerator
	mainInjector     *MainInjector
}

// NewFileGenerator creates a new file generator.
func NewFileGenerator() *FileGenerator {
	return &FileGenerator{
		fileManager:      files.NewUnifiedFileManager(),
		templateRenderer: NewTemplateRenderer(),
		routeGenerator:   NewRouteGenerator(),
		mainInjector:     NewMainInjector(),
	}
}

// GenerateController performs the generate controller operation.
func (fg *FileGenerator) GenerateController(
	cat *catalog.Catalog,
	resourceName string,
	namespace string,
	tableName string,
	controllerType ControllerType,
	modulePath string,
	databaseType string,
	tableNameOverridden bool,
	nullType string,
	primaryKeyColumn string,
	inertia string,
) error {
	return fg.GenerateControllerWithActionsForModel(cat, resourceName, namespace, resourceName, tableName, tableName, controllerType, modulePath, databaseType, tableNameOverridden, tableNameOverridden, nullType, primaryKeyColumn, inertia, nil, false)
}

// GenerateControllerWithActionsForModel performs the generate controller with actions for model operation.
func (fg *FileGenerator) GenerateControllerWithActionsForModel(
	cat *catalog.Catalog,
	resourceName string,
	namespace string,
	modelName string,
	tableName string,
	modelTableName string,
	controllerType ControllerType,
	modulePath string,
	databaseType string,
	tableNameOverridden bool,
	modelTableNameOverridden bool,
	nullType string,
	primaryKeyColumn string,
	inertia string,
	actions []string,
	isAPI bool,
) error {
	if modelName == "" {
		modelName = resourceName
	}
	if modelTableName == "" {
		modelTableName = tableName
	}
	// When table name is overridden, use it directly; otherwise derive from resource name
	pluralName := tableName
	if !tableNameOverridden {
		pluralName = naming.DeriveTableName(resourceName)
	}
	modelPluralName := modelTableName
	if !modelTableNameOverridden {
		modelPluralName = naming.DeriveTableName(modelName)
	}
	controllerDir := filepath.Join("controllers", namespace)
	controllerPath := filepath.Join(controllerDir, tableName+".go")
	controllerExists := false
	if _, err := os.Stat(controllerPath); err == nil {
		controllerExists = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat controller file %s: %w", controllerPath, err)
	}

	generator := NewGenerator(databaseType)
	if nullType != "" {
		generator.SetNullType(nullType)
	}
	renderActions := actions
	routeActions := actions
	mergeIntoExistingController := false
	var existingControllerContent string
	if controllerExists && len(actions) > 0 {
		content, err := os.ReadFile(controllerPath)
		if err != nil {
			return fmt.Errorf("failed to read existing controller %s: %w", controllerPath, err)
		}
		existingControllerContent = string(content)
		existingActions, err := existingControllerActions(controllerPath)
		if err != nil {
			return err
		}
		routeActions = mergeActions(existingActions, actions)
		existingFrontend := detectControllerFrontend(existingControllerContent)
		requestedFrontend := controllerFrontendTempl
		if layout.IsSupportedInertiaAdapter(inertia) {
			requestedFrontend = controllerFrontendInertia
		}
		if existingFrontend != controllerFrontendUnknown && existingFrontend != requestedFrontend {
			mergeIntoExistingController = true
		} else {
			renderActions = routeActions
		}
	}
	// For API controllers with no specific actions, default to JSON-relevant CRUD actions
	if isAPI && len(renderActions) == 0 {
		renderActions = []string{"index", "show", "create", "update", "destroy"}
	}
	if isAPI && len(routeActions) == 0 {
		routeActions = []string{"index", "show", "create", "update", "destroy"}
	}

	controller, err := generator.Build(cat, Config{
		ResourceName:             resourceName,
		ModelName:                modelName,
		PluralName:               pluralName,
		ModelPluralName:          modelPluralName,
		TableName:                tableName,
		ModelTableName:           modelTableName,
		PackageName:              naming.ControllerPackageName(namespace),
		Namespace:                namespace,
		ModulePath:               modulePath,
		ControllerType:           controllerType,
		TableNameOverridden:      tableNameOverridden,
		ModelTableNameOverridden: modelTableNameOverridden,
		PrimaryKeyColumn:         primaryKeyColumn,
		Actions:                  renderActions,
		IsAPI:                    isAPI,
	})
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	controllerContent, err := fg.templateRenderer.RenderControllerFile(controller, inertia)
	if err != nil {
		return fmt.Errorf("failed to render controller file: %w", err)
	}
	if len(renderActions) > 0 {
		controllerContent, err = filterControllerActions(controllerContent, renderActions)
		if err != nil {
			return fmt.Errorf("failed to filter controller actions: %w", err)
		}
	}
	if mergeIntoExistingController {
		controllerContent, err = mergeControllerSources(existingControllerContent, controllerContent)
		if err != nil {
			return fmt.Errorf("failed to merge controller file: %w", err)
		}
		controllerContent = ensureRegisterRoutes(controllerContent, controller.ReceiverName, controller.PluralResourceName, namespace, resourceName, actions)
	}

	if err := fg.fileManager.EnsureDir(controllerDir); err != nil {
		return err
	}

	if err := os.WriteFile(controllerPath, []byte(controllerContent), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write controller file: %w", err)
	}

	if err := files.FormatGoFile(controllerPath); err != nil {
		return fmt.Errorf("failed to format controller file: %w", err)
	}

	if err := fg.mainInjector.InjectController(resourceName, namespace, pluralName); err != nil {
		return fmt.Errorf("failed to inject controller: %w", err)
	}

	if err := fg.routeGenerator.GenerateRoutes(resourceName, namespace, pluralName, controller.IDType, routeActions); err != nil {
		return fmt.Errorf("failed to generate routes: %w", err)
	}

	return nil
}

var crudActions = []string{"index", "show", "new", "create", "edit", "update", "destroy"}

type controllerFrontend string

const (
	controllerFrontendUnknown controllerFrontend = ""
	controllerFrontendTempl   controllerFrontend = "templ"
	controllerFrontendInertia controllerFrontend = "inertia"
	controllerFrontendAPI     controllerFrontend = "api"
)

func detectControllerFrontend(content string) controllerFrontend {
	switch {
	case strings.Contains(content, "/internal/inertia\""):
		return controllerFrontendInertia
	case strings.Contains(content, "/internal/hypermedia\""):
		return controllerFrontendTempl
	default:
		return controllerFrontendUnknown
	}
}

func mergeControllerSources(existingContent, generatedContent string) (string, error) {
	fset := token.NewFileSet()
	existingFile, err := parser.ParseFile(fset, "existing_controller.go", existingContent, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("failed to parse existing controller: %w", err)
	}
	generatedFile, err := parser.ParseFile(fset, "generated_controller.go", generatedContent, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("failed to parse generated controller: %w", err)
	}

	mergeControllerImports(existingFile, generatedFile)

	existingKeys := make(map[string]struct{}, len(existingFile.Decls))
	for _, decl := range existingFile.Decls {
		for _, key := range controllerDeclKeys(decl) {
			existingKeys[key] = struct{}{}
		}
	}

	for _, decl := range generatedFile.Decls {
		if importDecl(decl) {
			continue
		}
		keys := controllerDeclKeys(decl)
		if len(keys) == 0 {
			continue
		}

		shouldAppend := false
		for _, key := range keys {
			if _, ok := existingKeys[key]; !ok {
				shouldAppend = true
				break
			}
		}
		if !shouldAppend {
			continue
		}

		existingFile.Decls = append(existingFile.Decls, decl)
		for _, key := range keys {
			existingKeys[key] = struct{}{}
		}
	}

	var out strings.Builder
	if err := format.Node(&out, fset, existingFile); err != nil {
		return "", err
	}
	return out.String(), nil
}

func mergeControllerImports(existingFile, generatedFile *ast.File) {
	existingImports := make(map[string]struct{}, len(existingFile.Imports))
	for _, spec := range existingFile.Imports {
		existingImports[spec.Path.Value] = struct{}{}
	}

	var importDecl *ast.GenDecl
	for _, decl := range existingFile.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if ok && genDecl.Tok == token.IMPORT {
			importDecl = genDecl
			break
		}
	}
	if importDecl == nil {
		importDecl = &ast.GenDecl{Tok: token.IMPORT}
		existingFile.Decls = append([]ast.Decl{importDecl}, existingFile.Decls...)
	}

	for _, spec := range generatedFile.Imports {
		if _, ok := existingImports[spec.Path.Value]; ok {
			continue
		}
		existingImports[spec.Path.Value] = struct{}{}
		importDecl.Specs = append(importDecl.Specs, &ast.ImportSpec{
			Name: spec.Name,
			Path: &ast.BasicLit{Kind: token.STRING, Value: spec.Path.Value},
		})
	}
}

func importDecl(decl ast.Decl) bool {
	genDecl, ok := decl.(*ast.GenDecl)
	return ok && genDecl.Tok == token.IMPORT
}

func controllerDeclKeys(decl ast.Decl) []string {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		prefix := "func:"
		if d.Recv != nil {
			prefix = "method:"
		}
		return []string{prefix + d.Name.Name}
	case *ast.GenDecl:
		if d.Tok != token.TYPE {
			return nil
		}
		keys := make([]string, 0, len(d.Specs))
		for _, spec := range d.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			keys = append(keys, "type:"+typeSpec.Name.Name)
		}
		return keys
	default:
		return nil
	}
}

func ensureRegisterRoutes(content, receiverName, controllerName, namespace, resourceName string, actions []string) string {
	if !strings.Contains(content, "RegisterRoutes(r *router.Router)") {
		return strings.TrimRight(content, "\n") + "\n\n" +
			strings.TrimRight(registerRoutesMethod(receiverName, controllerName, namespace, resourceName, actions), "\n") + "\n"
	}

	var additions strings.Builder
	routePrefix := naming.NamespaceToPascal(namespace) + resourceName
	for _, action := range actions {
		action = strings.ToLower(action)
		methodName := naming.ToPascalCase(action)
		routeRef := fmt.Sprintf("routes.%s%s.Path()", routePrefix, methodName)
		if strings.Contains(content, routeRef) {
			continue
		}
		if block := routeRegistrationBlock(receiverName, namespace, resourceName, action); block != "" {
			additions.WriteString(block)
		}
	}
	if additions.Len() == 0 {
		return content
	}

	needle := "\n\treturn errors.Join(errs...)"
	if strings.Contains(content, needle) {
		return strings.Replace(content, needle, "\n"+strings.TrimRight(additions.String(), "\n")+needle, 1)
	}
	return strings.TrimRight(content, "\n") + "\n\n" + strings.TrimRight(additions.String(), "\n") + "\n"
}

func registerRoutesMethod(receiverName, controllerName, namespace, resourceName string, actions []string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("func (%s %s) RegisterRoutes(r *router.Router) error {\n", receiverName, controllerName))
	sb.WriteString("\tvar errs []error\n")
	sb.WriteString("\tvar err error\n\n")
	for _, action := range actions {
		if block := routeRegistrationBlock(receiverName, namespace, resourceName, strings.ToLower(action)); block != "" {
			sb.WriteString(block)
		}
	}
	sb.WriteString("\treturn errors.Join(errs...)\n")
	sb.WriteString("}\n")
	return sb.String()
}

func routeRegistrationBlock(receiverName, namespace, resourceName, action string) string {
	methodName := naming.ToPascalCase(action)
	httpMethod := map[string]string{
		"index":   "http.MethodGet",
		"show":    "http.MethodGet",
		"new":     "http.MethodGet",
		"create":  "http.MethodPost",
		"edit":    "http.MethodGet",
		"update":  "http.MethodPut",
		"destroy": "http.MethodDelete",
	}[action]
	if httpMethod == "" {
		return ""
	}
	routePrefix := naming.NamespaceToPascal(namespace) + resourceName

	return fmt.Sprintf("\t_, err = r.AddRoute(echo.Route{\n\t\tMethod:  %s,\n\t\tPath:    routes.%s%s.Path(),\n\t\tName:    routes.%s%s.Name(),\n\t\tHandler: %s.%s,\n\t})\n\tif err != nil {\n\t\terrs = append(errs, err)\n\t}\n\n",
		httpMethod,
		routePrefix,
		methodName,
		routePrefix,
		methodName,
		receiverName,
		methodName,
	)
}

func mergeActions(existing, requested []string) []string {
	if len(requested) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(existing)+len(requested))
	merged := make([]string, 0, len(existing)+len(requested))
	for _, group := range [][]string{existing, requested} {
		for _, action := range group {
			action = strings.ToLower(action)
			if _, ok := seen[action]; ok {
				continue
			}
			seen[action] = struct{}{}
			merged = append(merged, action)
		}
	}
	return merged
}

func existingControllerActions(controllerPath string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, controllerPath, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to parse existing controller %s: %w", controllerPath, err)
	}

	actions := make([]string, 0, len(crudActions))
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil {
			continue
		}
		action := strings.ToLower(fn.Name.Name)
		if slices.Contains(crudActions, action) {
			actions = append(actions, action)
		}
	}
	return actions, nil
}

func filterControllerActions(content string, actions []string) (string, error) {
	allowedMethods := make(map[string]struct{}, len(actions))
	for _, action := range actions {
		allowedMethods[naming.ToPascalCase(action)] = struct{}{}
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "controller.go", content, parser.ParseComments)
	if err != nil {
		return "", err
	}

	filteredDecls := make([]ast.Decl, 0, len(file.Decls))
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil || d.Name.Name == "RegisterRoutes" {
				filteredDecls = append(filteredDecls, decl)
				continue
			}
			if _, ok := allowedMethods[d.Name.Name]; ok {
				filteredDecls = append(filteredDecls, decl)
			}
		case *ast.GenDecl:
			if d.Tok != token.TYPE {
				filteredDecls = append(filteredDecls, decl)
				continue
			}
			specs := make([]ast.Spec, 0, len(d.Specs))
			for _, spec := range d.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					specs = append(specs, spec)
					continue
				}
				switch {
				case strings.HasPrefix(typeSpec.Name.Name, "Create") &&
					(strings.HasSuffix(typeSpec.Name.Name, "FormPayload") || strings.HasSuffix(typeSpec.Name.Name, "Payload")):
					if slices.Contains(actions, "create") {
						specs = append(specs, spec)
					}
				case strings.HasPrefix(typeSpec.Name.Name, "Update") &&
					(strings.HasSuffix(typeSpec.Name.Name, "FormPayload") || strings.HasSuffix(typeSpec.Name.Name, "Payload")):
					if slices.Contains(actions, "update") {
						specs = append(specs, spec)
					}
				default:
					specs = append(specs, spec)
				}
			}
			if len(specs) > 0 {
				d.Specs = specs
				filteredDecls = append(filteredDecls, d)
			}
		default:
			filteredDecls = append(filteredDecls, decl)
		}
	}
	file.Decls = filteredDecls

	var out strings.Builder
	if err := format.Node(&out, fset, file); err != nil {
		return "", err
	}
	return out.String(), nil
}
