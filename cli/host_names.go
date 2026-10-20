package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type discoveredHost struct {
	Name    string
	Value   string
	PkgName string
	GoExpr  string
}

func discoverAppHostNames(rootDir string) ([]discoveredHost, error) {
	var hosts []discoveredHost
	seen := map[string]struct{}{}

	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || name == "testdata" || name == "node_modules" ||
				name == ".git" || name == "assets" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil
		}
		alias, ok := routingImportName(file)
		if !ok {
			return nil
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok || !isRoutingHostNameType(valueSpec.Type, alias) {
					continue
				}
				for i, ident := range valueSpec.Names {
					if ident.Name == "_" || i >= len(valueSpec.Values) {
						continue
					}
					value, ok := evalHostString(valueSpec.Values[i])
					if !ok || value == "" {
						continue
					}
					key := file.Name.Name + "." + ident.Name
					if _, exists := seen[key]; exists {
						continue
					}
					seen[key] = struct{}{}
					relDir, err := filepath.Rel(rootDir, filepath.Dir(path))
					if err != nil || relDir == "." {
						relDir = file.Name.Name
					}
					hosts = append(hosts, discoveredHost{
						Name:    ident.Name,
						Value:   value,
						PkgName: filepath.ToSlash(relDir),
						GoExpr:  file.Name.Name + "." + ident.Name,
					})
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(hosts, func(i, j int) bool {
		if hosts[i].Value == hosts[j].Value {
			return hosts[i].GoExpr < hosts[j].GoExpr
		}
		return hosts[i].Value < hosts[j].Value
	})
	return hosts, nil
}

func routingImportName(file *ast.File) (string, bool) {
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		if path != "github.com/mbvlabs/andurel/pkg/routing" &&
			!strings.HasSuffix(path, "/pkg/routing") {
			continue
		}
		if imp.Name != nil {
			if imp.Name.Name == "." || imp.Name.Name == "_" {
				return "", false
			}
			return imp.Name.Name, true
		}
		return "routing", true
	}
	return "", false
}

func isRoutingHostNameType(expr ast.Expr, routingAlias string) bool {
	if routingAlias == "" {
		ident, ok := expr.(*ast.Ident)
		return ok && ident.Name == "HostName"
	}
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	return ok && ident.Name == routingAlias && sel.Sel.Name == "HostName"
}

func evalHostString(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return value, true
}
