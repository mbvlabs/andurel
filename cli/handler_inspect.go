package cli

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
)

// handlerInfo is one handler in a handlers file: a type with a Register
// method, the constructor that builds it, and the routes Register mounts.
type handlerInfo struct {
	Type         string         `json:"type"`
	Constructor  string         `json:"constructor,omitempty"`
	Params       []handlerParam `json:"params"`
	ReturnsError bool           `json:"returns_error"`
	Routes       []handlerRoute `json:"routes"`
}

// handlerParam is one constructor parameter. Name is empty for an unnamed
// parameter.
type handlerParam struct {
	Name string `json:"name,omitempty"`
	Type string `json:"type"`
}

// handlerRoute is one rt.<Method>(...) call in a handler's Register method.
// Variable is the routes package variable (routes.SessionNew); Name and Path
// come from the route manifest. A literal path has no variable or name.
type handlerRoute struct {
	Variable string `json:"variable,omitempty"`
	Name     string `json:"name,omitempty"`
	Method   string `json:"method"`
	Path     string `json:"path,omitempty"`
	Action   string `json:"action,omitempty"`
	Line     int    `json:"line"`
}

var handlerRouteMethods = map[string]string{
	"Get":    "GET",
	"Post":   "POST",
	"Put":    "PUT",
	"Patch":  "PATCH",
	"Delete": "DELETE",
}

type parsedHandlerFile struct {
	rel  string
	fset *token.FileSet
	file *ast.File
}

// describeHandlerItems attaches handler metadata to the handler file items.
// Files are grouped by directory so a Register method or constructor in a
// different file of the same package still belongs to its type, which is
// reported on the file that declares it.
func describeHandlerItems(
	rootDir, module string,
	items []projectItem,
	manifest routeManifest,
) []projectItem {
	routesByVariable := make(map[string]routeManifestRoute, len(manifest.Routes))
	for _, route := range manifest.Routes {
		routesByVariable[route.Variable] = route
	}
	routesImport := module + "/routes"

	packages := map[string][]parsedHandlerFile{}
	dirs := []string{}
	for _, item := range items {
		if strings.HasSuffix(item.Path, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(
			fset,
			filepath.Join(rootDir, filepath.FromSlash(item.Path)),
			nil,
			parser.SkipObjectResolution,
		)
		if err != nil {
			continue
		}
		dir := filepath.ToSlash(filepath.Dir(item.Path))
		if _, ok := packages[dir]; !ok {
			dirs = append(dirs, dir)
		}
		packages[dir] = append(packages[dir], parsedHandlerFile{rel: item.Path, fset: fset, file: file})
	}

	byPath := map[string][]handlerInfo{}
	for _, dir := range dirs {
		for path, handlers := range describeHandlerPackage(packages[dir], routesImport, routesByVariable) {
			byPath[path] = append(byPath[path], handlers...)
		}
	}
	for i := range items {
		handlers := byPath[items[i].Path]
		sort.SliceStable(handlers, func(a, b int) bool { return handlers[a].Type < handlers[b].Type })
		items[i].Handlers = handlers
	}
	return items
}

// describeHandlerPackage returns the handlers of one package keyed by the
// file that declares each handler type.
func describeHandlerPackage(
	files []parsedHandlerFile,
	routesImport string,
	routesByVariable map[string]routeManifestRoute,
) map[string][]handlerInfo {
	typeFiles := map[string]string{}
	type registerDecl struct {
		file parsedHandlerFile
		fn   *ast.FuncDecl
	}
	registers := map[string]registerDecl{}
	registerOrder := []string{}
	constructors := map[string][]*ast.FuncDecl{}

	for _, parsed := range files {
		for _, decl := range parsed.file.Decls {
			switch decl := decl.(type) {
			case *ast.GenDecl:
				if decl.Tok != token.TYPE {
					continue
				}
				for _, spec := range decl.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						typeFiles[typeSpec.Name.Name] = parsed.rel
					}
				}
			case *ast.FuncDecl:
				if decl.Recv != nil {
					if decl.Name.Name != "Register" || decl.Body == nil {
						continue
					}
					recv := handlerReceiverType(decl)
					if recv == "" {
						continue
					}
					if _, ok := registers[recv]; !ok {
						registerOrder = append(registerOrder, recv)
					}
					registers[recv] = registerDecl{file: parsed, fn: decl}
					continue
				}
				if !strings.HasPrefix(decl.Name.Name, "New") ||
					decl.Type.Results == nil ||
					len(decl.Type.Results.List) == 0 {
					continue
				}
				if result := namedTypeName(decl.Type.Results.List[0].Type); result != "" {
					constructors[result] = append(constructors[result], decl)
				}
			}
		}
	}

	byPath := map[string][]handlerInfo{}
	for _, typeName := range registerOrder {
		register := registers[typeName]
		info := handlerInfo{
			Type:   typeName,
			Params: []handlerParam{},
			Routes: handlerRegisterRoutes(register.file, register.fn, routesImport, routesByVariable),
		}
		if constructor := pickHandlerConstructor(typeName, constructors[typeName]); constructor != nil {
			info.Constructor = constructor.Name.Name
			info.Params = handlerParams(constructor.Type.Params)
			info.ReturnsError = handlerReturnsError(constructor.Type.Results)
		}
		path := typeFiles[typeName]
		if path == "" {
			path = register.file.rel
		}
		byPath[path] = append(byPath[path], info)
	}
	return byPath
}

// handlerReceiverType returns T for a method on T or *T.
func handlerReceiverType(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	return namedTypeName(fn.Recv.List[0].Type)
}

// namedTypeName returns T for the expression T or *T, and "" otherwise.
func namedTypeName(expr ast.Expr) string {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// pickHandlerConstructor prefers New<Type>, then the first New* function
// returning the type.
func pickHandlerConstructor(typeName string, candidates []*ast.FuncDecl) *ast.FuncDecl {
	for _, candidate := range candidates {
		if candidate.Name.Name == "New"+typeName {
			return candidate
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return nil
}

func handlerParams(fields *ast.FieldList) []handlerParam {
	params := []handlerParam{}
	if fields == nil {
		return params
	}
	for _, field := range fields.List {
		typ := types.ExprString(field.Type)
		if len(field.Names) == 0 {
			params = append(params, handlerParam{Type: typ})
			continue
		}
		for _, name := range field.Names {
			params = append(params, handlerParam{Name: name.Name, Type: typ})
		}
	}
	return params
}

func handlerReturnsError(results *ast.FieldList) bool {
	if results == nil || len(results.List) == 0 {
		return false
	}
	last, ok := results.List[len(results.List)-1].Type.(*ast.Ident)
	return ok && last.Name == "error"
}

// handlerRegisterRoutes collects rt.<Method>(path, action, ...) calls in a
// Register method whose receiver chain starts at the router parameter, so
// rt.Host(...).Get(...) counts and h.cache.Get(...) does not.
func handlerRegisterRoutes(
	parsed parsedHandlerFile,
	fn *ast.FuncDecl,
	routesImport string,
	routesByVariable map[string]routeManifestRoute,
) []handlerRoute {
	routes := []handlerRoute{}
	params := fn.Type.Params
	if params == nil || len(params.List) == 0 || len(params.List[0].Names) == 0 {
		return routes
	}
	router := params.List[0].Names[0].Name
	routesName := localName(importNames(parsed.file), routesImport)

	recv := ""
	if fn.Recv != nil && len(fn.Recv.List) > 0 && len(fn.Recv.List[0].Names) > 0 {
		recv = fn.Recv.List[0].Names[0].Name
	}

	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		method, ok := handlerRouteMethods[selector.Sel.Name]
		if !ok || len(call.Args) < 2 || rootIdentName(selector.X) != router {
			return true
		}
		route := handlerRoute{
			Method: method,
			Action: handlerAction(call.Args[1], recv),
			Line:   parsed.fset.Position(call.Pos()).Line,
		}
		if variable := routePathVariable(call.Args[0], routesName); variable != "" {
			route.Variable = variable
			if manifestRoute, ok := routesByVariable[variable]; ok {
				route.Name = manifestRoute.Name
				route.Path = manifestRoute.Path
			}
		} else if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if value, err := strconv.Unquote(lit.Value); err == nil {
				route.Path = value
			}
		}
		routes = append(routes, route)
		return true
	})
	return routes
}

// rootIdentName walks selectors and calls down to the leftmost identifier:
// rt.Host(x) -> rt.
func rootIdentName(expr ast.Expr) string {
	for {
		switch e := expr.(type) {
		case *ast.Ident:
			return e.Name
		case *ast.SelectorExpr:
			expr = e.X
		case *ast.CallExpr:
			expr = e.Fun
		case *ast.ParenExpr:
			expr = e.X
		default:
			return ""
		}
	}
}

// routePathVariable returns X for routes.X.Path() or routes.X.Pattern().
func routePathVariable(expr ast.Expr, routesName string) string {
	if routesName == "" {
		return ""
	}
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return ""
	}
	pathSel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || (pathSel.Sel.Name != "Path" && pathSel.Sel.Name != "Pattern") {
		return ""
	}
	varSel, ok := pathSel.X.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	pkg, ok := varSel.X.(*ast.Ident)
	if !ok || pkg.Name != routesName {
		return ""
	}
	return varSel.Sel.Name
}

// handlerAction names the handler func: Method for recv.Method, otherwise the
// expression as written.
func handlerAction(expr ast.Expr, recv string) string {
	if selector, ok := expr.(*ast.SelectorExpr); ok {
		if ident, ok := selector.X.(*ast.Ident); ok && recv != "" && ident.Name == recv {
			return selector.Sel.Name
		}
	}
	return types.ExprString(expr)
}

func renderHandlersHuman(w io.Writer, items []projectItem) error {
	count := 0
	for _, item := range items {
		count += len(item.Handlers)
	}
	if count == 0 {
		_, err := fmt.Fprintf(w, "No handlers found (%d handler files).\n", len(items))
		return err
	}
	if _, err := fmt.Fprintf(w, "Handlers (%d)\n", count); err != nil {
		return err
	}
	for _, item := range items {
		for _, handler := range item.Handlers {
			constructor := "-"
			if handler.Constructor != "" {
				params := make([]string, 0, len(handler.Params))
				for _, param := range handler.Params {
					params = append(params, strings.TrimSpace(param.Name+" "+param.Type))
				}
				constructor = handler.Constructor + "(" + strings.Join(params, ", ") + ")"
			}
			if _, err := fmt.Fprintf(w, "\n%s  %s  %s\n", handler.Type, constructor, item.Path); err != nil {
				return err
			}
			if len(handler.Routes) == 0 {
				if _, err := fmt.Fprintln(w, "  no routes registered"); err != nil {
					return err
				}
				continue
			}
			table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
			for _, route := range handler.Routes {
				name := route.Name
				if name == "" {
					name = "-"
				}
				path := route.Path
				if path == "" {
					path = "?"
				}
				if _, err := fmt.Fprintf(
					table,
					"  %s\t%s\t%s\t%s\n",
					route.Method,
					path,
					name,
					route.Action,
				); err != nil {
					return err
				}
			}
			if err := table.Flush(); err != nil {
				return err
			}
		}
	}
	return nil
}
