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
	"github.com/mbvlabs/andurel/pkg/constants"
	"github.com/mbvlabs/andurel/pkg/naming"
)

type FileGenerator struct {
	fileManager      files.Manager
	templateRenderer *TemplateRenderer
	routeGenerator   *RouteGenerator
}

func NewFileGenerator() *FileGenerator {
	return &FileGenerator{
		fileManager:      files.NewUnifiedFileManager(),
		templateRenderer: NewTemplateRenderer(),
		routeGenerator:   NewRouteGenerator(),
	}
}

func (fg *FileGenerator) GenerateController(
	cat *catalog.Catalog,
	resourceName string,
	tableName string,
	controllerType ControllerType,
	modulePath string,
	databaseType string,
	tableNameOverridden bool,
	nullType string,
	primaryKeyColumn string,
	diMode string,
	viewLayer string,
) error {
	return fg.GenerateControllerWithActions(cat, resourceName, tableName, controllerType, modulePath, databaseType, tableNameOverridden, nullType, primaryKeyColumn, diMode, viewLayer, nil)
}

func (fg *FileGenerator) GenerateControllerWithActions(
	cat *catalog.Catalog,
	resourceName string,
	tableName string,
	controllerType ControllerType,
	modulePath string,
	databaseType string,
	tableNameOverridden bool,
	nullType string,
	primaryKeyColumn string,
	diMode string,
	viewLayer string,
	actions []string,
) error {
	// When table name is overridden, use it directly; otherwise derive from resource name
	pluralName := tableName
	if !tableNameOverridden {
		pluralName = naming.DeriveTableName(resourceName)
	}
	controllerPath := filepath.Join("controllers", tableName+".go")
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
	if controllerExists && len(actions) > 0 {
		existingActions, err := existingControllerActions(controllerPath)
		if err != nil {
			return err
		}
		renderActions = mergeActions(existingActions, actions)
	}
	controller, err := generator.Build(cat, Config{
		ResourceName:        resourceName,
		PluralName:          pluralName,
		TableName:           tableName,
		PackageName:         "controllers",
		ModulePath:          modulePath,
		ControllerType:      controllerType,
		TableNameOverridden: tableNameOverridden,
		PrimaryKeyColumn:    primaryKeyColumn,
		Actions:             renderActions,
	})
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	controllerContent, err := fg.templateRenderer.RenderControllerFile(controller, diMode, viewLayer)
	if err != nil {
		return fmt.Errorf("failed to render controller file: %w", err)
	}
	if len(renderActions) > 0 {
		controllerContent, err = filterControllerActions(controllerContent, renderActions)
		if err != nil {
			return fmt.Errorf("failed to filter controller actions: %w", err)
		}
	}

	if err := fg.fileManager.EnsureDir("controllers"); err != nil {
		return err
	}

	if err := os.WriteFile(controllerPath, []byte(controllerContent), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write controller file: %w", err)
	}

	if err := files.FormatGoFile(controllerPath); err != nil {
		return fmt.Errorf("failed to format controller file: %w", err)
	}

	if err := fg.routeGenerator.GenerateRoutesWithActions(resourceName, pluralName, controller.IDType, diMode, renderActions); err != nil {
		return fmt.Errorf("failed to generate routes: %w", err)
	}

	return nil
}

var crudActions = []string{"index", "show", "new", "create", "edit", "update", "destroy"}

func mergeActions(existing, requested []string) []string {
	if len(requested) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(existing)+len(requested))
	merged := make([]string, 0, len(existing)+len(requested))
	for _, group := range [][]string{existing, requested} {
		for _, action := range group {
			action = strings.ToLower(action)
			if _, ok := seen[action]; ok || !slices.Contains(crudActions, action) {
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
				case strings.HasPrefix(typeSpec.Name.Name, "Create") && strings.HasSuffix(typeSpec.Name.Name, "FormPayload"):
					if slices.Contains(actions, "create") {
						specs = append(specs, spec)
					}
				case strings.HasPrefix(typeSpec.Name.Name, "Update") && strings.HasSuffix(typeSpec.Name.Name, "FormPayload"):
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
