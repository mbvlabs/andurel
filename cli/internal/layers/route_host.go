package layers

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"sync"

	"golang.org/x/tools/go/analysis"
)

const (
	anduinImportPath  = "github.com/mbvlabs/andurel/pkg/anduin"
	routingImportPath = "github.com/mbvlabs/andurel/pkg/routing"

	// routingHostPrimary is the value of routing.HostPrimary. Routes on the
	// primary host need no host-aware registration.
	routingHostPrimary = "primary"
)

// routeHostAnalyzer flags a route registered by path on a host it does not
// declare. rt.Get(routes.X.Pattern(), h) serves X on the primary host whatever
// host X was declared on, so an admin route quietly lands on the public site.
// rt.Route(routes.X).Get(h) registers X on its declared host.
//
// The declared host is read from the route variable's constructor: a
// routing.Host(name) option whose name is a constant. Routes declared in other
// packages are found through the loaded project; without a project (analysistest)
// only the analyzed package's routes are known.
func routeHostAnalyzer(cfg Config) *analysis.Analyzer {
	var (
		once  sync.Once
		hosts map[string]string
	)

	return newAnalyzer("route-host",
		"a route declared on a non-primary host is registered with rt.Route, not by its Pattern() path",
		func(pass *analysis.Pass) error {
			once.Do(func() { hosts = declaredRouteHosts(cfg, pass) })

			for _, file := range pass.Files {
				ast.Inspect(file, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					reportRouteHostCall(pass, hosts, call)

					return true
				})
			}

			return nil
		})
}

// reportRouteHostCall reports call when it is rt.<Method>(X.Pattern(), ...) on
// an anduin router or host and X declares a non-primary host.
func reportRouteHostCall(pass *analysis.Pass, hosts map[string]string, call *ast.CallExpr) {
	method, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) == 0 || !isRouteRegistrationMethod(method.Sel.Name) {
		return
	}
	if !isAnduinRegistrar(pass.TypesInfo.TypeOf(method.X)) {
		return
	}
	pattern, ok := unparen(call.Args[0]).(*ast.CallExpr)
	if !ok || len(pattern.Args) != 0 {
		return
	}
	patternSel, ok := pattern.Fun.(*ast.SelectorExpr)
	if !ok || patternSel.Sel.Name != "Pattern" {
		return
	}
	route, ok := selectedObject(pass.TypesInfo, patternSel.X).(*types.Var)
	if !ok || route.Pkg() == nil {
		return
	}
	host, ok := hosts[routeVarKey(route)]
	if !ok || host == "" || host == routingHostPrimary {
		return
	}

	routeExpr := renderExpr(patternSel.X)
	receiver := renderExpr(method.X)
	pass.Report(analysis.Diagnostic{
		Pos: call.Pos(),
		Message: fmt.Sprintf(
			"registers %s.Pattern() with %s.%s, but the route declares host %q; use %s.Route(%s).%s(...) so the route's host decides the domain",
			routeExpr, receiver, method.Sel.Name, host, receiver, routeExpr, method.Sel.Name,
		),
	})
}

func isRouteRegistrationMethod(name string) bool {
	switch name {
	case "Get", "Post", "Put", "Patch", "Delete":
		return true
	default:
		return false
	}
}

// isAnduinRegistrar reports whether t is anduin.Router or anduin.Host, the
// receivers that register routes.
func isAnduinRegistrar(t types.Type) bool {
	named := namedType(t)
	if named == nil || named.Obj().Pkg() == nil {
		return false
	}

	return named.Obj().Pkg().Path() == anduinImportPath &&
		(named.Obj().Name() == "Router" || named.Obj().Name() == "Host")
}

func routeVarKey(route *types.Var) string {
	return route.Pkg().Path() + "." + route.Name()
}

// declaredRouteHosts maps each package-level route variable to the host its
// constructor declares. Routes without a routing.Host option, or with a host
// name that is not a constant, are left out.
func declaredRouteHosts(cfg Config, pass *analysis.Pass) map[string]string {
	hosts := map[string]string{}
	if cfg.Project == nil {
		collectRouteHosts(hosts, pass.Files, pass.TypesInfo)

		return hosts
	}
	for _, pkg := range cfg.Project.Packages {
		collectRouteHosts(hosts, pkg.Syntax, pkg.TypesInfo)
	}

	return hosts
}

func collectRouteHosts(hosts map[string]string, files []*ast.File, info *types.Info) {
	if info == nil {
		return
	}
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range valueSpec.Names {
					if i >= len(valueSpec.Values) {
						break
					}
					host, ok := routeDeclaredHost(valueSpec.Values[i], info)
					if !ok {
						continue
					}
					route, ok := info.Defs[name].(*types.Var)
					if !ok || route == nil || route.Pkg() == nil {
						continue
					}
					hosts[routeVarKey(route)] = host
				}
			}
		}
	}
}

// routeDeclaredHost returns the constant host name passed to routing.Host in a
// route constructor call.
func routeDeclaredHost(expr ast.Expr, info *types.Info) (string, bool) {
	call, ok := unparen(expr).(*ast.CallExpr)
	if !ok {
		return "", false
	}
	for _, arg := range call.Args {
		hostCall, ok := unparen(arg).(*ast.CallExpr)
		if !ok || len(hostCall.Args) != 1 {
			continue
		}
		if !isRoutingObject(selectedObject(info, hostCall.Fun), "Host") {
			continue
		}
		value := info.Types[hostCall.Args[0]].Value
		if value == nil || value.Kind() != constant.String {
			return "", false
		}

		return constant.StringVal(value), true
	}

	return "", false
}

func isRoutingObject(object types.Object, name string) bool {
	return object != nil &&
		object.Pkg() != nil &&
		object.Pkg().Path() == routingImportPath &&
		object.Name() == name
}
