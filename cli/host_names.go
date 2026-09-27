package cli

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mbvlabs/andurel/v2/internal/naming"
)

type discoveredHost struct {
	Name    string
	Value   string
	PkgName string
	GoExpr  string
}

func resolvePrefixAndHost(
	rootDir, name, prefixFlag, hostFlag string,
) (namespace, resourceName, hostExpr string, err error) {
	namespace, resourceName, err = naming.ParseNamespacedResource(name)
	if err != nil {
		return "", "", "", err
	}
	prefixFlag = strings.TrimSpace(prefixFlag)
	if prefixFlag != "" {
		if !naming.IsValidNamespace(prefixFlag) {
			return "", "", "", fmt.Errorf(
				"invalid prefix %q: prefix must be a valid Go package name and not a reserved path",
				prefixFlag,
			)
		}
		if namespace != "" && namespace != prefixFlag {
			return "", "", "", fmt.Errorf(
				"--prefix=%q conflicts with namespaced name %q",
				prefixFlag,
				name,
			)
		}
		namespace = prefixFlag
	}

	hostExpr, err = resolveHostFlag(rootDir, hostFlag)
	if err != nil {
		return "", "", "", err
	}
	return namespace, resourceName, hostExpr, nil
}

func resolveHostFlag(rootDir, hostFlag string) (string, error) {
	hostFlag = strings.TrimSpace(hostFlag)
	if hostFlag == "" || hostFlag == "primary" {
		return "", nil
	}

	hosts, err := discoverAppHostNames(rootDir)
	if err != nil {
		return "", err
	}

	known := []string{"primary"}
	for _, host := range hosts {
		known = append(known, host.Value)
	}
	sort.Strings(known)

	for _, host := range hosts {
		if host.Value == hostFlag || host.Name == hostFlag {
			return host.GoExpr, nil
		}
		trimmed := strings.TrimPrefix(host.Name, "Host")
		if strings.EqualFold(trimmed, hostFlag) {
			return host.GoExpr, nil
		}
	}

	return "", fmt.Errorf(
		"unknown host %q; known hosts: %s",
		hostFlag,
		strings.Join(uniqueStrings(known), ", "),
	)
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
					hosts = append(hosts, discoveredHost{
						Name:    ident.Name,
						Value:   value,
						PkgName: file.Name.Name,
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

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
