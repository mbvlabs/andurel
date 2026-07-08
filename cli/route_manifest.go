package cli

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
)

type routeManifest struct {
	Routes  []routeManifestRoute   `json:"routes"`
	Skipped []routeManifestSkipped `json:"skipped,omitempty"`
}

type routeManifestRoute struct {
	Variable    string               `json:"variable"`
	Name        string               `json:"name"`
	Path        string               `json:"path"`
	Constructor string               `json:"constructor"`
	Kind        string               `json:"kind"`
	Params      []routeManifestParam `json:"params,omitempty"`
	SourceFile  string               `json:"source_file"`
	Line        int                  `json:"line"`
}

type routeManifestParam struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type routeManifestSkipped struct {
	Variable    string `json:"variable,omitempty"`
	Constructor string `json:"constructor,omitempty"`
	SourceFile  string `json:"source_file"`
	Line        int    `json:"line"`
	Reason      string `json:"reason"`
}

type parsedRouteFile struct {
	path string
	fset *token.FileSet
	file *ast.File
}

type routeConstCandidate struct {
	name string
	expr ast.Expr
}

func collectRouteManifest(rootDir string) (routeManifest, error) {
	routesDir := filepath.Join(rootDir, "router", "routes")
	if _, err := os.Stat(routesDir); err != nil {
		if os.IsNotExist(err) {
			return routeManifest{}, nil
		}
		return routeManifest{}, err
	}

	files, err := parseRouteFiles(routesDir)
	if err != nil {
		return routeManifest{}, err
	}

	consts := collectRouteConstants(files)

	var manifest routeManifest
	for _, file := range files {
		routes, skipped := collectRoutesFromFile(rootDir, file, consts)
		manifest.Routes = append(manifest.Routes, routes...)
		manifest.Skipped = append(manifest.Skipped, skipped...)
	}

	sort.SliceStable(manifest.Routes, func(i, j int) bool {
		if manifest.Routes[i].SourceFile == manifest.Routes[j].SourceFile {
			return manifest.Routes[i].Line < manifest.Routes[j].Line
		}
		return manifest.Routes[i].SourceFile < manifest.Routes[j].SourceFile
	})
	sort.SliceStable(manifest.Skipped, func(i, j int) bool {
		if manifest.Skipped[i].SourceFile == manifest.Skipped[j].SourceFile {
			return manifest.Skipped[i].Line < manifest.Skipped[j].Line
		}
		return manifest.Skipped[i].SourceFile < manifest.Skipped[j].SourceFile
	})

	return manifest, nil
}

func parseRouteFiles(routesDir string) ([]parsedRouteFile, error) {
	entries, err := os.ReadDir(routesDir)
	if err != nil {
		return nil, err
	}

	paths := []string{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			continue
		}
		paths = append(paths, filepath.Join(routesDir, entry.Name()))
	}
	sort.Strings(paths)

	files := make([]parsedRouteFile, 0, len(paths))
	for _, path := range paths {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parse route file %s: %w", path, err)
		}
		files = append(files, parsedRouteFile{path: path, fset: fset, file: file})
	}

	return files, nil
}

func collectRouteConstants(files []parsedRouteFile) map[string]string {
	consts := map[string]string{}
	candidates := routeConstCandidates(files)

	for len(candidates) > 0 {
		progress := false
		unresolved := candidates[:0]
		for _, candidate := range candidates {
			value, ok := evalRouteStringExpr(candidate.expr, consts)
			if !ok {
				unresolved = append(unresolved, candidate)
				continue
			}
			consts[candidate.name] = value
			progress = true
		}
		if !progress {
			break
		}
		candidates = unresolved
	}

	return consts
}

func routeConstCandidates(files []parsedRouteFile) []routeConstCandidate {
	candidates := []routeConstCandidate{}
	for _, file := range files {
		for _, decl := range file.file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.CONST {
				continue
			}

			for _, spec := range genDecl.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}

				for i, name := range valueSpec.Names {
					if i >= len(valueSpec.Values) {
						continue
					}
					candidates = append(candidates, routeConstCandidate{
						name: name.Name,
						expr: valueSpec.Values[i],
					})
				}
			}
		}
	}
	return candidates
}

func collectRoutesFromFile(rootDir string, file parsedRouteFile, consts map[string]string) ([]routeManifestRoute, []routeManifestSkipped) {
	sourceFile := routeManifestSourceFile(rootDir, file.path)
	routes := []routeManifestRoute{}
	skipped := []routeManifestSkipped{}

	for _, decl := range file.file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}

		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			for i, name := range valueSpec.Names {
				if i >= len(valueSpec.Values) {
					continue
				}
				route, skip, ok := routeManifestFromValue(file.fset, sourceFile, name.Name, valueSpec.Values[i], consts)
				if !ok {
					continue
				}
				if skip != nil {
					skipped = append(skipped, *skip)
					continue
				}
				routes = append(routes, route)
			}
		}
	}

	return routes, skipped
}

func routeManifestFromValue(
	fset *token.FileSet,
	sourceFile string,
	variable string,
	expr ast.Expr,
	consts map[string]string,
) (routeManifestRoute, *routeManifestSkipped, bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return routeManifestRoute{}, nil, false
	}

	constructor, ok := routeConstructorName(call.Fun)
	if !ok {
		return routeManifestRoute{}, nil, false
	}

	line := fset.Position(call.Pos()).Line
	skip := func(reason string) *routeManifestSkipped {
		return &routeManifestSkipped{
			Variable:    variable,
			Constructor: constructor,
			SourceFile:  sourceFile,
			Line:        line,
			Reason:      reason,
		}
	}

	if len(call.Args) < 3 {
		return routeManifestRoute{}, skip("route constructor must have path, name, and prefix arguments"), true
	}

	path, ok := evalRouteStringExpr(call.Args[0], consts)
	if !ok {
		return routeManifestRoute{}, skip("route path is not a static string expression"), true
	}
	name, ok := evalRouteStringExpr(call.Args[1], consts)
	if !ok {
		return routeManifestRoute{}, skip("route name is not a static string expression"), true
	}
	prefix, ok := evalRouteStringExpr(call.Args[2], consts)
	if !ok {
		return routeManifestRoute{}, skip("route prefix is not a static string expression"), true
	}

	routePath := configureRouteManifestPath(path, prefix)
	return routeManifestRoute{
		Variable:    variable,
		Name:        name,
		Path:        routePath,
		Constructor: constructor,
		Kind:        routeKind(constructor),
		Params:      routeParams(routePath, constructor),
		SourceFile:  sourceFile,
		Line:        line,
	}, nil, true
}

func routeConstructorName(expr ast.Expr) (string, bool) {
	switch fun := expr.(type) {
	case *ast.SelectorExpr:
		if ident, ok := fun.X.(*ast.Ident); ok && ident.Name == "routing" {
			return fun.Sel.Name, isRouteConstructor(fun.Sel.Name)
		}
	case *ast.IndexExpr:
		return routeConstructorName(fun.X)
	case *ast.IndexListExpr:
		return routeConstructorName(fun.X)
	}
	return "", false
}

func isRouteConstructor(name string) bool {
	switch name {
	case "NewSimpleRoute",
		"NewRouteWithUUIDID",
		"NewRouteWithSerialID",
		"NewRouteWithBigSerialID",
		"NewRouteWithStringID",
		"NewRouteWithSlug",
		"NewRouteWithToken",
		"NewRouteWithFile",
		"NewRouteWithParams",
		"NewRouteWithSlugs":
		return true
	default:
		return false
	}
}

func evalRouteStringExpr(expr ast.Expr, consts map[string]string) (string, bool) {
	switch typed := expr.(type) {
	case *ast.BasicLit:
		if typed.Kind != token.STRING {
			return "", false
		}
		value, err := strconv.Unquote(typed.Value)
		if err != nil {
			return "", false
		}
		return value, true
	case *ast.Ident:
		value, ok := consts[typed.Name]
		return value, ok
	case *ast.BinaryExpr:
		if typed.Op != token.ADD {
			return "", false
		}
		left, ok := evalRouteStringExpr(typed.X, consts)
		if !ok {
			return "", false
		}
		right, ok := evalRouteStringExpr(typed.Y, consts)
		if !ok {
			return "", false
		}
		return left + right, true
	case *ast.ParenExpr:
		return evalRouteStringExpr(typed.X, consts)
	default:
		return "", false
	}
}

func configureRouteManifestPath(path, prefix string) string {
	if prefix != "" {
		if path == "" {
			path = prefix
		} else {
			prefixEndsWithSlash := strings.HasSuffix(prefix, "/")
			pathStartsWithSlash := strings.HasPrefix(path, "/")
			switch {
			case prefixEndsWithSlash != pathStartsWithSlash:
				path = prefix + path
			case prefixEndsWithSlash && pathStartsWithSlash:
				path = prefix + path[1:]
			default:
				path = prefix + "/" + path
			}
		}
	}

	path = strings.TrimSpace(path)
	path = strings.ReplaceAll(path, "\\", "/")
	if idx := strings.Index(path, "?"); idx != -1 {
		path = path[:idx]
	}
	if idx := strings.Index(path, "#"); idx != -1 {
		path = path[:idx]
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}
	if len(path) > 1 {
		path = strings.TrimSuffix(path, "/")
	}
	return path
}

func routeKind(constructor string) string {
	switch constructor {
	case "NewSimpleRoute":
		return "simple"
	case "NewRouteWithUUIDID":
		return "uuid_id"
	case "NewRouteWithSerialID":
		return "serial_id"
	case "NewRouteWithBigSerialID":
		return "bigserial_id"
	case "NewRouteWithStringID":
		return "string_id"
	case "NewRouteWithSlug":
		return "slug"
	case "NewRouteWithToken":
		return "token"
	case "NewRouteWithFile":
		return "file"
	case "NewRouteWithParams", "NewRouteWithSlugs":
		return "params"
	default:
		return "unknown"
	}
}

func routeParams(path, constructor string) []routeManifestParam {
	routeType := "string"
	switch constructor {
	case "NewRouteWithUUIDID":
		routeType = "uuid"
	case "NewRouteWithSerialID":
		routeType = "int32"
	case "NewRouteWithBigSerialID":
		routeType = "int64"
	}

	params := []routeManifestParam{}
	for segment := range strings.SplitSeq(path, "/") {
		if !strings.HasPrefix(segment, ":") {
			continue
		}
		params = append(params, routeManifestParam{
			Name: strings.TrimPrefix(segment, ":"),
			Type: routeType,
		})
	}
	return params
}

func routeManifestSourceFile(rootDir, path string) string {
	rel, err := filepath.Rel(rootDir, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func renderRouteManifestHuman(w io.Writer, manifest routeManifest) error {
	if len(manifest.Routes) == 0 {
		if _, err := fmt.Fprintln(w, "No routes found."); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(w, "Routes (%d)\n", len(manifest.Routes)); err != nil {
			return err
		}
		table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		if _, err := fmt.Fprintln(table, "VARIABLE\tNAME\tURL PATH\tPARAMS\tSOURCE"); err != nil {
			return err
		}
		for _, route := range manifest.Routes {
			if _, err := fmt.Fprintf(
				table,
				"%s\t%s\t%s\t%s\t%s:%d\n",
				route.Variable,
				route.Name,
				route.Path,
				formatRouteManifestParams(route.Params),
				route.SourceFile,
				route.Line,
			); err != nil {
				return err
			}
		}
		if err := table.Flush(); err != nil {
			return err
		}
	}

	if len(manifest.Skipped) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(w, "\nSkipped (%d)\n", len(manifest.Skipped)); err != nil {
		return err
	}
	table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "VARIABLE\tCONSTRUCTOR\tSOURCE\tREASON"); err != nil {
		return err
	}
	for _, skipped := range manifest.Skipped {
		if _, err := fmt.Fprintf(
			table,
			"%s\t%s\t%s:%d\t%s\n",
			skipped.Variable,
			skipped.Constructor,
			skipped.SourceFile,
			skipped.Line,
			skipped.Reason,
		); err != nil {
			return err
		}
	}
	return table.Flush()
}

func formatRouteManifestParams(params []routeManifestParam) string {
	if len(params) == 0 {
		return "-"
	}
	formatted := make([]string, 0, len(params))
	for _, param := range params {
		formatted = append(formatted, param.Name+":"+param.Type)
	}
	return strings.Join(formatted, ",")
}
