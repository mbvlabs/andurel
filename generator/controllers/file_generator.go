package controllers

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
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
) error {
	pluralName := naming.DeriveTableName(resourceName)
	controllerPath := filepath.Join("controllers", tableName+".go")

	if _, err := os.Stat(controllerPath); err == nil {
		return fmt.Errorf("controller file %s already exists", controllerPath)
	}

	generator := NewGenerator(databaseType)
	controller, err := generator.Build(cat, Config{
		ResourceName:   resourceName,
		PluralName:     pluralName,
		TableName:      tableName,
		PackageName:    "controllers",
		ModulePath:     modulePath,
		ControllerType: controllerType,
	})
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	controllerContent, err := fg.templateRenderer.RenderControllerFile(controller)
	if err != nil {
		return fmt.Errorf("failed to render controller file: %w", err)
	}

	if err := fg.fileManager.EnsureDir("controllers"); err != nil {
		return err
	}

	if err := os.WriteFile(controllerPath, []byte(controllerContent), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write controller file: %w", err)
	}

	if err := fg.formatGoFile(controllerPath); err != nil {
		return fmt.Errorf("failed to format controller file: %w", err)
	}

	if err := fg.routeGenerator.GenerateRoutes(resourceName, pluralName); err != nil {
		return fmt.Errorf("failed to generate routes: %w", err)
	}

	if err := fg.registerController(resourceName); err != nil {
		return fmt.Errorf("failed to register controller: %w", err)
	}

	return nil
}

func (fg *FileGenerator) registerController(resourceName string) error {
	controllerFilePath := "controllers/controller.go"

	content, err := os.ReadFile(controllerFilePath)
	if err != nil {
		return fmt.Errorf("failed to read controller.go: %w", err)
	}

	// Parse to check if already registered
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, controllerFilePath, content, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse controller.go: %w", err)
	}

	pluralName := resourceName + "s"
	fieldName := pluralName
	varName := naming.ToCamelCase(naming.ToSnakeCase(resourceName)) + "s"
	constructorName := "new" + pluralName

	// Check if controller already registered
	if fg.isControllerRegistered(node, fieldName) {
		return nil
	}

	contentStr := string(content)
	lines := strings.Split(contentStr, "\n")

	// Find and modify the file using line-based approach
	var result []string
	inStruct := false
	inNew := false
	inReturn := false
	addedField := false
	addedConstructor := false
	addedToReturn := false

	for _, line := range lines {
		result = append(result, line)

		// Add field to struct
		if !addedField && strings.Contains(line, "type Controllers struct") {
			inStruct = true
		} else if inStruct && strings.TrimSpace(line) == "}" {
			// Add new field before closing brace with proper alignment
			result = result[:len(result)-1]

			// Calculate spacing to match existing fields
			maxFieldLen := len(fieldName)
			for i := len(result) - 1; i >= 0; i-- {
				if strings.Contains(result[i], "type Controllers struct") {
					break
				}
				trimmed := strings.TrimSpace(result[i])
				if trimmed != "" && !strings.HasPrefix(trimmed, "//") {
					parts := strings.Fields(trimmed)
					if len(parts) == 2 && parts[0] == parts[1] {
						if len(parts[0]) > maxFieldLen {
							maxFieldLen = len(parts[0])
						}
					}
				}
			}

			spacing := strings.Repeat(" ", maxFieldLen-len(fieldName)+1)
			result = append(result, "\t"+fieldName+spacing+fieldName)
			result = append(result, line)
			inStruct = false
			addedField = true
		}

		// Track when we're in the New function
		if strings.Contains(line, "func New(") {
			inNew = true
		}

		// Add constructor call before return statement (but not in error blocks)
		if inNew && !addedConstructor {
			if strings.Contains(line, "return Controllers{") &&
				!strings.Contains(line, "return Controllers{}, err") {
				// Remove return line and any preceding blank lines
				result = result[:len(result)-1]

				// Remove blank line if present before return
				if len(result) > 0 && strings.TrimSpace(result[len(result)-1]) == "" {
					result = result[:len(result)-1]
				}

				// Add new constructor call
				result = append(result, "\t"+varName+" := "+constructorName+"(db)")

				// Add blank line before return
				result = append(result, "")
				result = append(result, line)
				addedConstructor = true
				inReturn = true
			}
		}

		// Add to return statement
		if !addedToReturn && inReturn && strings.TrimSpace(line) == "}, nil" {
			// Add new controller before closing
			result = result[:len(result)-1]
			result = append(result, "\t\t"+varName+",")
			result = append(result, line)
			inReturn = false
			inNew = false
			addedToReturn = true
		}
	}

	// Join lines and add trailing newline
	output := strings.Join(result, "\n")
	if !strings.HasSuffix(output, "\n") {
		output += "\n"
	}

	if err := os.WriteFile(controllerFilePath, []byte(output), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write controller.go: %w", err)
	}

	// Run gofmt to fix alignment and formatting
	return fg.formatGoFile(controllerFilePath)
}

func (fg *FileGenerator) isControllerRegistered(node *ast.File, fieldName string) bool {
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != "Controllers" {
				continue
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			for _, field := range structType.Fields.List {
				for _, name := range field.Names {
					if name.Name == fieldName {
						return true
					}
				}
			}
		}
	}
	return false
}

func (fg *FileGenerator) addControllerField(
	node *ast.File,
	fieldName, typeName string,
	fset *token.FileSet,
) error {
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != "Controllers" {
				continue
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			// Add new field to the struct
			newField := &ast.Field{
				Names: []*ast.Ident{ast.NewIdent(fieldName)},
				Type:  ast.NewIdent(typeName),
			}
			structType.Fields.List = append(structType.Fields.List, newField)
			return nil
		}
	}
	return fmt.Errorf("Controllers struct not found")
}

func (fg *FileGenerator) addControllerToNew(
	node *ast.File,
	varName, constructorName, fieldName string,
	fset *token.FileSet,
) error {
	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Name.Name != "New" {
			continue
		}

		// Find the last variable declaration before the return statement
		var lastVarDeclIndex int
		var returnStmtIndex int

		for i, stmt := range funcDecl.Body.List {
			if _, ok := stmt.(*ast.AssignStmt); ok {
				lastVarDeclIndex = i
			}
			if _, ok := stmt.(*ast.ReturnStmt); ok {
				returnStmtIndex = i
				break
			}
		}

		// Add new controller constructor call after the last variable declaration
		newAssign := &ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent(varName)},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{
				&ast.CallExpr{
					Fun:  ast.NewIdent(constructorName),
					Args: []ast.Expr{ast.NewIdent("db")},
				},
			},
		}

		// Insert the new assignment after the last variable declaration
		newBody := make([]ast.Stmt, 0, len(funcDecl.Body.List)+1)
		newBody = append(newBody, funcDecl.Body.List[:lastVarDeclIndex+1]...)
		newBody = append(newBody, newAssign)
		newBody = append(newBody, funcDecl.Body.List[lastVarDeclIndex+1:]...)
		funcDecl.Body.List = newBody

		// Update return statement index after insertion
		returnStmtIndex++

		// Add to return statement
		returnStmt := funcDecl.Body.List[returnStmtIndex].(*ast.ReturnStmt)
		if len(returnStmt.Results) > 0 {
			if compositeLit, ok := returnStmt.Results[0].(*ast.CompositeLit); ok {
				// Check if it's using named fields or positional
				isNamed := false
				if len(compositeLit.Elts) > 0 {
					_, isNamed = compositeLit.Elts[0].(*ast.KeyValueExpr)
				}

				// Calculate position for new element (on a new line)
				var newEltPos token.Pos
				if len(compositeLit.Elts) > 0 {
					lastElt := compositeLit.Elts[len(compositeLit.Elts)-1]
					lastPos := fset.Position(lastElt.End())
					// Add the new element on the next line at the same indentation
					newEltPos = fset.File(lastElt.Pos()).Pos(lastPos.Offset + 1)
				}

				var newElt ast.Expr
				if isNamed {
					// Named fields: add "FieldName: varName,"
					keyIdent := ast.NewIdent(fieldName)
					if newEltPos.IsValid() {
						keyIdent.NamePos = newEltPos
					}

					valueIdent := ast.NewIdent(varName)
					if newEltPos.IsValid() {
						valueIdent.NamePos = newEltPos + token.Pos(len(fieldName)+2)
					}

					newElt = &ast.KeyValueExpr{
						Key:   keyIdent,
						Value: valueIdent,
					}
				} else {
					// Positional fields: add "varName,"
					ident := ast.NewIdent(varName)
					if newEltPos.IsValid() {
						ident.NamePos = newEltPos
					}
					newElt = ident
				}

				compositeLit.Elts = append(compositeLit.Elts, newElt)
			}
		}

		return nil
	}
	return fmt.Errorf("New function not found")
}

func (fg *FileGenerator) formatGoFile(filePath string) error {
	cmd := exec.Command("go", "fmt", filePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run go fmt on %s: %w", filePath, err)
	}
	return nil
}
