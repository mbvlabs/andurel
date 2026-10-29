package cli

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/layout"
	"github.com/spf13/cobra"
)

type pageRoute struct {
	Name              string `json:"name"`
	Method            string `json:"method"`
	Path              string `json:"path"`
	Component         string `json:"component,omitempty"`
	ComponentFile     string `json:"component_file,omitempty"`
	ComponentResolved bool   `json:"component_resolved"`
	ComponentReason   string `json:"component_reason,omitempty"`
	SSRPolicy         string `json:"ssr_policy"`
	SSRReason         string `json:"ssr_reason,omitempty"`
}

type pageComponent struct {
	Component  string      `json:"component"`
	File       string      `json:"file,omitempty"`
	Exists     bool        `json:"exists"`
	FileReason string      `json:"file_reason,omitempty"`
	Routes     []pageRoute `json:"routes"`
}

type unresolvedPageRender struct {
	Handler    string `json:"handler"`
	Method     string `json:"method"`
	SourceFile string `json:"source_file"`
	Line       int    `json:"line"`
	Reason     string `json:"reason"`
}

type pagesInspection struct {
	Pages      []pageComponent        `json:"pages"`
	Routes     []pageRoute            `json:"routes"`
	Unresolved []unresolvedPageRender `json:"unresolved,omitempty"`
}

type handlerPageRender struct {
	HandlerType       string
	Method            string
	Component         string
	ComponentResolved bool
	ComponentReason   string
	SSR               bool
	SourceFile        string
	Line              int
}

func newPagesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pages",
		Short: "List frontend page components and route associations",
		Long: `List frontend components under resources/js/Pages with their files, the
routes whose handler renders them, and the SSR policy where statically known.

Associations come from a handler method bound to a route in its Register
method that calls renderer.Page with a static component name. A render whose
component argument is not a string literal is reported under unresolved with
its handler, method, and source line instead of being guessed. A page that
does not call .SSR() reports ssr_policy=unresolved because the renderer-wide
default is configured at runtime.`,
		Args:    cobra.NoArgs,
		Example: "  andurel inspect pages\n  andurel inspect pages --json\n  andurel inspect pages --jq .pages",
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}
			report, err := collectPagesInspection(rootDir)
			if err != nil {
				return err
			}
			opts, err := output.ParseOptions(cmd)
			if err != nil {
				return err
			}
			if opts.Mode == output.ModeHuman {
				if opts.Quiet {
					return nil
				}
				return renderPagesHuman(cmd.OutOrStdout(), report)
			}
			return output.OK(
				cmd,
				report,
				fmt.Sprintf("Listed %d page components", len(report.Pages)),
			)
		},
	}
	setAgentMetadata(
		cmd,
		"introspection",
		"Read-only page component inventory with route associations and statically known SSR policy; dynamic render calls stay unresolved.",
	)
	return cmd
}

func collectPagesInspection(rootDir string) (pagesInspection, error) {
	pages, pageIndex, err := collectPageComponents(rootDir)
	if err != nil {
		return pagesInspection{}, err
	}
	renders, err := collectHandlerPageRenders(rootDir)
	if err != nil {
		return pagesInspection{}, err
	}
	routesByHandlerMethod, err := handlerRoutesByMethod(rootDir)
	if err != nil {
		return pagesInspection{}, err
	}

	report := pagesInspection{Pages: pages, Routes: []pageRoute{}}
	seenRoutes := map[string]bool{}
	addRoute := func(route pageRoute) {
		key := strings.Join(
			[]string{route.Name, route.Method, route.Path, route.Component, route.SSRPolicy},
			"\x00",
		)
		if seenRoutes[key] {
			return
		}
		seenRoutes[key] = true
		report.Routes = append(report.Routes, route)
	}

	for _, render := range renders {
		handlerKey := render.HandlerType + "." + render.Method
		bound := routesByHandlerMethod[handlerKey]
		ssrPolicy, ssrReason := pageSSRPolicy(render.SSR)
		if !render.ComponentResolved {
			report.Unresolved = append(report.Unresolved, unresolvedPageRender{
				Handler:    render.HandlerType,
				Method:     render.Method,
				SourceFile: render.SourceFile,
				Line:       render.Line,
				Reason:     render.ComponentReason,
			})
			for _, route := range bound {
				addRoute(pageRoute{
					Name:              route.Name,
					Method:            route.Method,
					Path:              route.Path,
					ComponentReason:   render.ComponentReason,
					ComponentResolved: false,
					SSRPolicy:         ssrPolicy,
					SSRReason:         ssrReason,
				})
			}
			continue
		}

		index, onDisk := pageIndex[render.Component]
		if !onDisk {
			report.Pages = append(report.Pages, pageComponent{
				Component:  render.Component,
				Exists:     false,
				FileReason: "component is rendered by a handler but no matching file exists under resources/js/Pages",
				Routes:     []pageRoute{},
			})
			index = len(report.Pages) - 1
			pageIndex[render.Component] = index
		}
		for _, route := range bound {
			association := pageRoute{
				Name:              route.Name,
				Method:            route.Method,
				Path:              route.Path,
				Component:         render.Component,
				ComponentFile:     report.Pages[index].File,
				ComponentResolved: true,
				SSRPolicy:         ssrPolicy,
				SSRReason:         ssrReason,
			}
			report.Pages[index].Routes = appendMissingPageRoute(
				report.Pages[index].Routes,
				association,
			)
			addRoute(association)
		}
	}
	return report, nil
}

func appendMissingPageRoute(routes []pageRoute, route pageRoute) []pageRoute {
	for _, existing := range routes {
		if existing.Name == route.Name && existing.Method == route.Method &&
			existing.Path == route.Path && existing.Component == route.Component &&
			existing.SSRPolicy == route.SSRPolicy {
			return routes
		}
	}
	return append(routes, route)
}

func pageSSRPolicy(enabled bool) (policy, reason string) {
	if enabled {
		return "enabled", "page builder calls .SSR()"
	}
	return "unresolved", "renderer-wide SSR default is configured at runtime; this page does not call .SSR()"
}

func collectPageComponents(rootDir string) ([]pageComponent, map[string]int, error) {
	adapter := ""
	if lock, err := layout.ReadLockFile(rootDir); err == nil && lock.ScaffoldConfig != nil {
		adapter = lock.ScaffoldConfig.Inertia
	}
	extensions := pageExtensionsForAdapter(adapter)
	pagesDir := filepath.Join(rootDir, "resources", "js", "Pages")
	if _, err := os.Stat(pagesDir); err != nil {
		if os.IsNotExist(err) {
			return []pageComponent{}, map[string]int{}, nil
		}
		return nil, nil, err
	}

	components := []pageComponent{}
	err := filepath.WalkDir(pagesDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if _, ok := extensions[ext]; !ok {
			return nil
		}
		rel, err := filepath.Rel(pagesDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		component := strings.TrimSuffix(rel, filepath.Ext(rel))
		file, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}
		components = append(components, pageComponent{
			Component: component,
			File:      filepath.ToSlash(file),
			Exists:    true,
			Routes:    []pageRoute{},
		})
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	sort.SliceStable(components, func(i, j int) bool {
		return components[i].Component < components[j].Component
	})
	index := make(map[string]int, len(components))
	for i, component := range components {
		index[component.Component] = i
	}
	return components, index, nil
}

func pageExtensionsForAdapter(adapter string) map[string]struct{} {
	switch {
	case strings.HasPrefix(adapter, "react"):
		return map[string]struct{}{".tsx": {}, ".jsx": {}}
	case strings.HasPrefix(adapter, "vue"):
		return map[string]struct{}{".vue": {}}
	case strings.HasPrefix(adapter, "svelte"):
		return map[string]struct{}{".svelte": {}}
	default:
		return map[string]struct{}{
			".tsx": {}, ".jsx": {}, ".vue": {}, ".svelte": {},
		}
	}
}

func collectHandlerPageRenders(rootDir string) ([]handlerPageRender, error) {
	handlersDir := filepath.Join(rootDir, "handlers")
	if _, err := os.Stat(handlersDir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	renders := []handlerPageRender{}
	err := filepath.WalkDir(handlersDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" ||
			strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		sourceFile := routeManifestSourceFile(rootDir, path)
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Recv == nil {
				continue
			}
			handlerType := handlerReceiverType(fn)
			if handlerType == "" {
				continue
			}
			ssrPages := map[token.Pos]bool{}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "SSR" {
					return true
				}
				if page := pageCallInChain(selector.X); page != nil {
					ssrPages[page.Pos()] = true
				}
				return true
			})
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "Page" {
					return true
				}
				render := handlerPageRender{
					HandlerType: handlerType,
					Method:      fn.Name.Name,
					SSR:         ssrPages[call.Pos()],
					SourceFile:  sourceFile,
					Line:        fset.Position(call.Pos()).Line,
				}
				if len(call.Args) < 2 {
					render.ComponentReason = "renderer.Page call has no component argument"
				} else if literal, ok := call.Args[1].(*ast.BasicLit); ok && literal.Kind == token.STRING {
					component, err := strconv.Unquote(literal.Value)
					if err == nil && component != "" {
						render.Component = component
						render.ComponentResolved = true
					} else {
						render.ComponentReason = "renderer.Page component is not a static string"
					}
				} else {
					render.ComponentReason = fmt.Sprintf(
						"renderer.Page component is not a static string (found %s)",
						types.ExprString(call.Args[1]),
					)
				}
				renders = append(renders, render)
				return true
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return renders, nil
}

// pageCallInChain walks a builder chain such as
// renderer.Page(...).SSR().Render() back to the Page call.
func pageCallInChain(expr ast.Expr) *ast.CallExpr {
	for {
		call, ok := expr.(*ast.CallExpr)
		if !ok {
			return nil
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return nil
		}
		if selector.Sel.Name == "Page" {
			return call
		}
		expr = selector.X
	}
}

func handlerRoutesByMethod(rootDir string) (map[string][]handlerRoute, error) {
	manifest, err := collectRouteManifest(rootDir)
	if err != nil {
		return nil, err
	}
	items, err := listProjectFiles(rootDir, "handlers", ".go", "handler")
	if err != nil {
		return nil, err
	}
	module, _, err := readGoModMetadata(rootDir)
	if err != nil {
		return nil, err
	}
	items = describeHandlerItems(rootDir, module, items, manifest)
	routes := map[string][]handlerRoute{}
	for _, item := range items {
		for _, handler := range item.Handlers {
			for _, route := range handler.Routes {
				key := handler.Type + "." + route.Action
				routes[key] = append(routes[key], route)
			}
		}
	}
	return routes, nil
}

func renderPagesHuman(w io.Writer, report pagesInspection) error {
	if len(report.Pages) == 0 {
		if _, err := fmt.Fprintln(w, "No page components found."); err != nil {
			return err
		}
	} else {
		table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		if _, err := fmt.Fprintln(table, "COMPONENT\tFILE\tROUTES\tSSR"); err != nil {
			return err
		}
		for _, page := range report.Pages {
			file := page.File
			if file == "" {
				file = "-"
			}
			routes := make([]string, 0, len(page.Routes))
			ssr := "unresolved"
			for _, route := range page.Routes {
				routes = append(routes, route.Name)
				if route.SSRPolicy == "enabled" {
					ssr = "enabled"
				}
			}
			if len(routes) == 0 {
				routes = []string{"-"}
			}
			if _, err := fmt.Fprintf(
				table,
				"%s\t%s\t%s\t%s\n",
				page.Component,
				file,
				strings.Join(routes, ","),
				ssr,
			); err != nil {
				return err
			}
		}
		if err := table.Flush(); err != nil {
			return err
		}
	}
	if len(report.Unresolved) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "Unresolved render calls:"); err != nil {
		return err
	}
	for _, unresolved := range report.Unresolved {
		if _, err := fmt.Fprintf(
			w,
			"  %s.%s (%s:%d): %s\n",
			unresolved.Handler,
			unresolved.Method,
			unresolved.SourceFile,
			unresolved.Line,
			unresolved.Reason,
		); err != nil {
			return err
		}
	}
	return nil
}
