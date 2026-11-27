package cli

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// frontendGoFile retains import names: matching a selector by spelling alone
// would allow an unrelated package (or a comment) to stand in for the wiring.
type frontendGoFile struct {
	file    *ast.File
	path    string
	imports map[string]string
}

func frontendGoFiles(root, dir string) ([]frontendGoFile, error) {
	entries, err := os.ReadDir(filepath.Join(root, dir))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var files []frontendGoFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		name := filepath.Join(dir, entry.Name())
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, name), nil, 0)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		item := frontendGoFile{file: f, path: filepath.ToSlash(name), imports: map[string]string{}}
		for _, imp := range f.Imports {
			p, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return nil, err
			}
			n := path.Base(p)
			if imp.Name != nil {
				n = imp.Name.Name
			}
			item.imports[n] = p
		}
		files = append(files, item)
	}
	return files, nil
}

func frontendSelector(e ast.Expr, f frontendGoFile, pkg, name string) bool {
	s, ok := e.(*ast.SelectorExpr)
	if !ok || s.Sel.Name != name {
		return false
	}
	id, ok := s.X.(*ast.Ident)
	if !ok {
		return false
	}
	p := f.imports[id.Name]
	return p != "" && (p == pkg || strings.HasSuffix(p, "/"+pkg))
}

func frontendCall(e ast.Expr) *ast.CallExpr {
	c, _ := e.(*ast.CallExpr)
	return c
}

func frontendString(e ast.Expr, constants map[string]ast.Expr, depth int) (string, bool) {
	if depth > 10 {
		return "", false
	}
	switch x := e.(type) {
	case *ast.ParenExpr:
		return frontendString(x.X, constants, depth+1)
	case *ast.BasicLit:
		if x.Kind == token.STRING {
			s, err := strconv.Unquote(x.Value)
			return s, err == nil
		}
	case *ast.Ident:
		if v, ok := constants[x.Name]; ok {
			return frontendString(v, constants, depth+1)
		}
	case *ast.BinaryExpr:
		if x.Op == token.ADD {
			a, okA := frontendString(x.X, constants, depth+1)
			b, okB := frontendString(x.Y, constants, depth+1)
			return a + b, okA && okB
		}
	}
	return "", false
}

func frontendViolation(file, message, fix string) verifyViolation {
	return verifyViolation{Rule: "frontend-setup", File: file, Message: message, Fix: fix}
}

// frontendBindings collects only straight-line assignments. Conditional or
// helper-based wiring is deliberately not treated as evidence.
func frontendBindings(body *ast.BlockStmt) map[string]ast.Expr {
	m := map[string]ast.Expr{}
	if body == nil {
		return m
	}
	for _, stmt := range body.List {
		switch s := stmt.(type) {
		case *ast.ReturnStmt:
			return m
		case *ast.AssignStmt:
			if len(s.Rhs) == 1 {
				for _, lhs := range s.Lhs {
					if id, ok := lhs.(*ast.Ident); ok {
						m[id.Name] = s.Rhs[0]
					}
				}
			} else if len(s.Lhs) == len(s.Rhs) {
				for i, lhs := range s.Lhs {
					if id, ok := lhs.(*ast.Ident); ok {
						m[id.Name] = s.Rhs[i]
					}
				}
			}
		case *ast.DeclStmt:
			if d, ok := s.Decl.(*ast.GenDecl); ok {
				for _, spec := range d.Specs {
					v, ok := spec.(*ast.ValueSpec)
					if ok && len(v.Names) == 1 && len(v.Values) == 1 {
						m[v.Names[0].Name] = v.Values[0]
					}
				}
			}
		}
	}
	return m
}

func frontendResolve(e ast.Expr, bindings map[string]ast.Expr) ast.Expr {
	for range 10 {
		switch x := e.(type) {
		case *ast.ParenExpr:
			e = x.X
		case *ast.Ident:
			next, ok := bindings[x.Name]
			if !ok {
				return e
			}
			e = next
		default:
			return e
		}
	}
	return e
}

// Follow only locally bound identifiers; package names and parameters are not
// bindings. This lets mutation checks cover aliases and intermediate values.
func frontendTrace(e ast.Expr, bindings map[string]ast.Expr, names map[string]bool) {
	if e == nil {
		return
	}
	ast.Inspect(e, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok || names[id.Name] {
			return true
		}
		if value, ok := bindings[id.Name]; ok {
			names[id.Name] = true
			frontendTrace(value, bindings, names)
		}
		return true
	})
}

// Reassignment under control flow, or mutation of a field of a traced value,
// invalidates the straight-line snapshot. Multiple writes also make aliases
// time-dependent, so leave those cases for a human rather than guessing.
func frontendMutated(body *ast.BlockStmt, names map[string]bool) bool {
	counts := map[string]int{}
	mutated := false
	for _, stmt := range body.List {
		if _, ok := stmt.(*ast.ReturnStmt); ok {
			break
		}
		ast.Inspect(stmt, func(n ast.Node) bool {
			if _, ok := n.(*ast.FuncLit); ok {
				return false
			}
			assign, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for _, lhs := range assign.Lhs {
				switch x := lhs.(type) {
				case *ast.Ident:
					if names[x.Name] {
						counts[x.Name]++
						if n != stmt || counts[x.Name] > 1 {
							mutated = true
						}
					}
				case *ast.SelectorExpr:
					root := x.X
					for {
						s, ok := root.(*ast.SelectorExpr)
						if !ok {
							break
						}
						root = s.X
					}
					if id, ok := root.(*ast.Ident); ok && names[id.Name] {
						mutated = true
					}
				}
			}
			return true
		})
	}
	return mutated
}

// Inspect the application composition itself, not just an unused handler.
func frontendAppRegistration(files []frontendGoFile) (string, *verifyViolation) {
	fn, f := frontendFunc(files, "New", "")
	if fn == nil {
		v := frontendViolation("app", "app.New does not construct and register the asset handler", "Construct handlers.NewAssetsHandler and call its Register method on the application router.")
		return "", &v
	}
	bindings := map[string]ast.Expr{}
	handlerNames := map[string]bool{}
	routerNames := map[string]bool{}
	registered := false
	indirect := false
	for _, stmt := range fn.Body.List {
		if _, ok := stmt.(*ast.ReturnStmt); ok {
			break
		}
		if a, ok := stmt.(*ast.AssignStmt); ok && len(a.Rhs) == 1 && len(a.Lhs) > 0 {
			if id, ok := a.Lhs[0].(*ast.Ident); ok {
				bindings[id.Name] = a.Rhs[0]
				if c := frontendCall(frontendResolve(a.Rhs[0], bindings)); c != nil {
					if frontendSelector(c.Fun, f, "handlers", "NewAssetsHandler") {
						handlerNames[id.Name] = true
					}
					if frontendSelector(c.Fun, f, "anduin", "NewRouter") {
						routerNames[id.Name] = true
					}
				}
			}
		}
		if expr, ok := stmt.(*ast.ExprStmt); ok {
			if c := frontendCall(expr.X); c != nil && len(c.Args) == 1 {
				if sel, ok := c.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Register" {
					h, hok := sel.X.(*ast.Ident)
					r, rok := c.Args[0].(*ast.Ident)
					if hok && rok && handlerNames[h.Name] && routerNames[r.Name] {
						registered = true
						continue
					}
					if hok && rok && routerNames[r.Name] {
						indirect = true
					}
				} else if _, ok := c.Fun.(*ast.Ident); ok {
					if r, ok := c.Args[0].(*ast.Ident); ok && routerNames[r.Name] {
						indirect = true
					}
				}
			}
		}
		// Passing the handler to another call, or registration in control
		// flow, may be legitimate but is outside this bounded analysis.
		inspect := ast.Node(stmt)
		if a, ok := stmt.(*ast.AssignStmt); ok {
			// Initializing an alias is supported, but a call receiving the
			// handler could perform registration or mutate it.
			for _, rhs := range a.Rhs {
				if c := frontendCall(rhs); c != nil {
					for _, arg := range c.Args {
						if id, ok := arg.(*ast.Ident); ok && handlerNames[id.Name] {
							indirect = true
						}
					}
				}
			}
			inspect = nil
		}
		if inspect != nil {
			ast.Inspect(inspect, func(n ast.Node) bool {
				if _, ok := n.(*ast.FuncLit); ok {
					return false
				}
				if id, ok := n.(*ast.Ident); ok && handlerNames[id.Name] {
					indirect = true
				}
				return true
			})
		}
	}
	names := map[string]bool{}
	for n := range handlerNames {
		names[n] = true
	}
	for n := range routerNames {
		names[n] = true
	}
	if frontendMutated(fn.Body, names) {
		return f.path + ": asset handler or router is reassigned in app.New", nil
	}
	if indirect {
		return f.path + ": indirect or conditional asset handler registration in app.New", nil
	}
	if registered {
		return "", nil
	}
	v := frontendViolation(f.path, "app.New does not register its asset handler on the application router", "Construct handlers.NewAssetsHandler and call assetsHandler.Register(router) before returning.")
	return "", &v
}

func frontendFunc(files []frontendGoFile, name, receiver string) (*ast.FuncDecl, frontendGoFile) {
	for _, f := range files {
		for _, decl := range f.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != name || fn.Body == nil {
				continue
			}
			if receiver == "" && fn.Recv == nil {
				return fn, f
			}
			if receiver != "" && fn.Recv != nil && len(fn.Recv.List) == 1 {
				typ := fn.Recv.List[0].Type
				if star, ok := typ.(*ast.StarExpr); ok {
					typ = star.X
				}
				if id, ok := typ.(*ast.Ident); ok && id.Name == receiver {
					return fn, f
				}
			}
		}
	}
	return nil, frontendGoFile{}
}

// frontendRendererEntry returns the default entry and, when the generated
// env-backed config is used, the key whose value overrides that default.
func frontendRendererEntry(rootDir string) (entry, envKey, blocked string) {
	files, err := frontendGoFiles(rootDir, "app")
	if err != nil {
		return "", "", "cannot inspect renderer entry point: " + err.Error()
	}
	fn, f := frontendFunc(files, "NewRenderer", "")
	if fn == nil {
		return "", "", "app: cannot find NewRenderer to inspect entry point"
	}
	bindings := frontendBindings(fn.Body)
	for _, file := range files {
		for _, decl := range file.file.Decls {
			d, ok := decl.(*ast.GenDecl)
			if !ok || d.Tok != token.CONST {
				continue
			}
			for _, spec := range d.Specs {
				v := spec.(*ast.ValueSpec)
				if len(v.Names) == 1 && len(v.Values) == 1 {
					if _, exists := bindings[v.Names[0].Name]; !exists {
						bindings[v.Names[0].Name] = v.Values[0]
					}
				}
			}
		}
	}
	for _, stmt := range fn.Body.List {
		ret, ok := stmt.(*ast.ReturnStmt)
		if !ok || len(ret.Results) == 0 {
			continue
		}
		c := frontendCall(frontendResolve(ret.Results[0], bindings))
		if c == nil || !frontendSelector(c.Fun, f, "inertia", "NewRenderer") || len(c.Args) == 0 {
			break
		}
		cfg, ok := frontendResolve(c.Args[0], bindings).(*ast.CompositeLit)
		if !ok || (!frontendSelector(cfg.Type, f, "inertia", "RendererConfig") && !frontendSelector(cfg.Type, f, "inertia", "Config")) {
			break
		}
		traced := map[string]bool{}
		frontendTrace(c.Args[0], bindings, traced)
		if frontendMutated(fn.Body, traced) {
			return "", "", f.path + ": renderer entry point configuration is mutated"
		}
		for _, elt := range cfg.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				return "", "", f.path + ": renderer entry point uses positional configuration"
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != "EntryPoint" {
				continue
			}
			if value, ok := frontendString(kv.Value, bindings, 0); ok {
				return value, "", ""
			}
			// The generated renderer passes cfg.Inertia.EntryPoint through
			// unchanged; only accept a parameter explicitly typed config.Config.
			if field, ok := kv.Value.(*ast.SelectorExpr); ok && field.Sel.Name == "EntryPoint" {
				if inertiaField, ok := field.X.(*ast.SelectorExpr); ok && inertiaField.Sel.Name == "Inertia" {
					if id, ok := inertiaField.X.(*ast.Ident); ok {
						for _, param := range fn.Type.Params.List {
							if frontendSelector(param.Type, f, "config", "Config") {
								for _, name := range param.Names {
									if id.Name == name.Name && !frontendMutated(fn.Body, map[string]bool{id.Name: true}) {
										return frontendConfiguredEntry(rootDir)
									}
								}
							}
						}
					}
				}
			}
			return "", "", f.path + ": renderer EntryPoint is dynamic; cannot compare it with Vite inputs"
		}
		return "", "", f.path + ": renderer EntryPoint is missing"
	}
	return "", "", f.path + ": cannot inspect returned renderer entry point"
}

func frontendConfiguredEntry(rootDir string) (string, string, string) {
	files, err := frontendGoFiles(rootDir, "config")
	if err != nil {
		return "", "", "cannot inspect Inertia config: " + err.Error()
	}
	fn, f := frontendFunc(files, "NewInertia", "")
	if fn == nil {
		return "", "", "config: cannot find NewInertia to inspect entry point"
	}
	bindings := frontendBindings(fn.Body)
	for _, file := range files {
		for _, decl := range file.file.Decls {
			if d, ok := decl.(*ast.GenDecl); ok && d.Tok == token.CONST {
				for _, spec := range d.Specs {
					v := spec.(*ast.ValueSpec)
					if len(v.Names) == 1 && len(v.Values) == 1 {
						if _, exists := bindings[v.Names[0].Name]; !exists {
							bindings[v.Names[0].Name] = v.Values[0]
						}
					}
				}
			}
		}
	}
	for _, stmt := range fn.Body.List {
		ret, ok := stmt.(*ast.ReturnStmt)
		if !ok || len(ret.Results) == 0 {
			continue
		}
		lit, ok := frontendResolve(ret.Results[0], bindings).(*ast.CompositeLit)
		if !ok {
			break
		}
		typ, ok := lit.Type.(*ast.Ident)
		if !ok || typ.Name != "Inertia" {
			break
		}
		traced := map[string]bool{}
		frontendTrace(ret.Results[0], bindings, traced)
		if frontendMutated(fn.Body, traced) {
			return "", "", f.path + ": Inertia configuration is mutated"
		}
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				break
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != "EntryPoint" {
				continue
			}
			call := frontendCall(kv.Value)
			if call == nil || len(call.Args) != 2 {
				break
			}
			method, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || method.Sel.Name != "String" {
				break
			}
			envNew := frontendCall(frontendResolve(method.X, bindings))
			if envNew == nil || !frontendSelector(envNew.Fun, f, "env", "New") || len(envNew.Args) != 0 {
				break
			}
			envKey, keyOK := frontendString(call.Args[0], bindings, 0)
			entry, entryOK := frontendString(call.Args[1], bindings, 0)
			if keyOK && entryOK && envKey != "" {
				return entry, envKey, ""
			}
			break
		}
		break
	}
	return "", "", f.path + ": cannot resolve NewInertia EntryPoint environment key and default"
}

func verifyFrontendAssetRoutes(rootDir string) (violations []verifyViolation, blocked string) {
	routes, err := frontendGoFiles(rootDir, "routes")
	if err != nil {
		return nil, "cannot inspect routes: " + err.Error()
	}
	app, err := frontendGoFiles(rootDir, "app")
	if err != nil {
		return nil, "cannot inspect renderer: " + err.Error()
	}
	handlers, err := frontendGoFiles(rootDir, "handlers")
	if err != nil {
		return nil, "cannot inspect asset handler: " + err.Error()
	}
	var reasons []string
	constants := map[string]ast.Expr{}
	for _, f := range routes {
		for _, decl := range f.file.Decls {
			d, ok := decl.(*ast.GenDecl)
			if !ok || d.Tok != token.CONST {
				continue
			}
			for _, spec := range d.Specs {
				v := spec.(*ast.ValueSpec)
				if len(v.Names) == 1 && len(v.Values) == 1 {
					constants[v.Names[0].Name] = v.Values[0]
				}
			}
		}
	}
	foundRoute := false
	for _, f := range routes {
		for _, decl := range f.file.Decls {
			d, ok := decl.(*ast.GenDecl)
			if !ok || d.Tok != token.VAR {
				continue
			}
			for _, spec := range d.Specs {
				v := spec.(*ast.ValueSpec)
				for i, name := range v.Names {
					if name.Name != "ViteBuild" {
						if i >= len(v.Values) {
							continue
						}
						c := frontendCall(v.Values[i])
						if c == nil || !frontendSelector(c.Fun, f, "routing", "NewSimpleRoute") || len(c.Args) < 3 {
							continue
						}
						routePath, pathOK := frontendString(c.Args[0], constants, 0)
						prefix, prefixOK := frontendString(c.Args[2], constants, 0)
						if !pathOK || !prefixOK {
							reasons = append(reasons, f.path+": dynamic route "+name.Name+" could overlap the Vite asset namespace")
							continue
						}
						// Mirror NewSimpleRoute's path/prefix joining. The
						// subtree belongs exclusively to the Vite build.
						joined := routePath
						if prefix != "" {
							switch {
							case routePath == "":
								joined = prefix
							case routePath == "/":
								joined = strings.TrimSuffix(prefix, "/")
							case strings.HasSuffix(prefix, "/") && strings.HasPrefix(routePath, "/"):
								joined = prefix + routePath[1:]
							case !strings.HasSuffix(prefix, "/") && !strings.HasPrefix(routePath, "/"):
								joined = prefix + "/" + routePath
							default:
								joined = prefix + routePath
							}
						}
						joined = strings.TrimSpace(joined)
						joined = strings.ReplaceAll(joined, "\\", "/")
						if i := strings.IndexAny(joined, "?#"); i >= 0 {
							joined = joined[:i]
						}
						if !strings.HasPrefix(joined, "/") {
							joined = "/" + joined
						}
						for strings.Contains(joined, "//") {
							joined = strings.ReplaceAll(joined, "//", "/")
						}
						if joined == "/assets/dist" || strings.HasPrefix(joined, "/assets/dist/") {
							violations = append(violations, frontendViolation(f.path, "route "+name.Name+" overlaps the reserved Vite asset namespace", "Move the route outside /assets/dist; reserve that subtree for routes.ViteBuild."))
						}
						continue
					}
					foundRoute = true
					if i >= len(v.Values) {
						reasons = append(reasons, f.path+": cannot inspect ViteBuild initializer")
						continue
					}
					c := frontendCall(v.Values[i])
					if c == nil || !frontendSelector(c.Fun, f, "routing", "NewSimpleRoute") || len(c.Args) != 3 {
						reasons = append(reasons, f.path+": unsupported ViteBuild initializer")
						continue
					}
					var values [3]string
					known := true
					for j, arg := range c.Args {
						value, ok := frontendString(arg, constants, 0)
						if !ok {
							reasons = append(reasons, f.path+": dynamic ViteBuild route arguments")
							known = false
							break
						}
						values[j] = value
					}
					if known && values[2] != "" && strings.HasSuffix(values[2], "/") == strings.HasPrefix(values[0], "/") {
						reasons = append(reasons, f.path+": ViteBuild prefix/path requires routing normalization")
						known = false
					}
					if known && (values[2]+values[0] != "/assets/dist/*" || values[1] != "vite.build") {
						violations = append(violations, frontendViolation(f.path, "ViteBuild does not declare the canonical asset route", `Declare routing.NewSimpleRoute("/dist/*", "vite.build", "/assets").`))
					}
				}
			}
		}
	}
	if !foundRoute {
		violations = append(violations, frontendViolation("routes", "ViteBuild route is missing", `Declare routing.NewSimpleRoute("/dist/*", "vite.build", "/assets") as routes.ViteBuild.`))
	}
	if reason, violation := frontendAppRegistration(app); reason != "" {
		reasons = append(reasons, reason)
	} else if violation != nil {
		violations = append(violations, *violation)
	}

	fn, f := frontendFunc(app, "NewRenderer", "")
	if fn == nil {
		reasons = append(reasons, "app: cannot find NewRenderer to inspect renderer wiring")
	} else {
		bindings := frontendBindings(fn.Body)
		found := false
		for _, stmt := range fn.Body.List {
			ret, ok := stmt.(*ast.ReturnStmt)
			if !ok || len(ret.Results) == 0 {
				continue
			}
			c := frontendCall(frontendResolve(ret.Results[0], bindings))
			if c == nil || !frontendSelector(c.Fun, f, "inertia", "NewRenderer") || len(c.Args) == 0 {
				continue
			}
			cfg, ok := frontendResolve(c.Args[0], bindings).(*ast.CompositeLit)
			if !ok || (!frontendSelector(cfg.Type, f, "inertia", "RendererConfig") && !frontendSelector(cfg.Type, f, "inertia", "Config")) {
				reasons = append(reasons, f.path+": dynamic renderer config")
				found = true
				continue
			}
			found = true
			assetFS := false
			assetFSDynamic := false
			for _, option := range c.Args[1:] {
				opt := frontendCall(frontendResolve(option, bindings))
				if opt == nil {
					assetFSDynamic = true
					continue
				}
				if frontendSelector(opt.Fun, f, "inertia", "WithAssetFS") {
					if len(opt.Args) == 1 && frontendSelector(opt.Args[0], f, "assets", "Files") {
						assetFS = true
					} else {
						assetFSDynamic = true
					}
					continue
				}
				sel, ok := opt.Fun.(*ast.SelectorExpr)
				if !ok {
					assetFSDynamic = true
				} else if id, ok := sel.X.(*ast.Ident); !ok || (f.imports[id.Name] != "inertia" && !strings.HasSuffix(f.imports[id.Name], "/inertia")) {
					assetFSDynamic = true
				}
			}
			if assetFSDynamic {
				reasons = append(reasons, f.path+": cannot determine renderer WithAssetFS wiring")
			} else if !assetFS {
				violations = append(violations, frontendViolation(f.path, "renderer does not use the embedded asset filesystem", "Pass inertia.WithAssetFS(assets.Files) to inertia.NewRenderer."))
			}
			traced := map[string]bool{}
			frontendTrace(c.Args[0], bindings, traced)
			if frontendMutated(fn.Body, traced) {
				reasons = append(reasons, f.path+": renderer config or its inputs are mutated")
				break
			}
			valid := false
			dynamic := false
			for _, elt := range cfg.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					dynamic = true
					continue
				}
				key, ok := kv.Key.(*ast.Ident)
				if !ok || key.Name != "BuildPathURL" {
					continue
				}
				value := frontendResolve(kv.Value, bindings)
				call := frontendCall(value)
				if call != nil && len(call.Args) == 1 {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "FullURL" && frontendSelector(sel.X, f, "routes", "ViteBuild") {
						if id, ok := call.Args[0].(*ast.Ident); ok && id.Name != "nil" {
							valid = true
							continue
						}
					}
				}
				if _, ok := frontendString(value, nil, 0); !ok {
					dynamic = true
				}
			}
			if !valid && dynamic {
				reasons = append(reasons, f.path+": cannot determine BuildPathURL wiring")
			} else if !valid {
				violations = append(violations, frontendViolation(f.path, "renderer BuildPathURL is not derived from ViteBuild", "Set BuildPathURL to routes.ViteBuild.FullURL(hosts)."))
			}
			break
		}
		if !found {
			reasons = append(reasons, f.path+": cannot inspect NewRenderer return wiring")
		}
	}

	constructor, cf := frontendFunc(handlers, "NewAssetsHandler", "")
	register, rf := frontendFunc(handlers, "Register", "AssetsHandler")
	if constructor == nil || register == nil {
		reasons = append(reasons, "handlers: cannot find NewAssetsHandler and AssetsHandler.Register")
	} else {
		field := ""
		for _, stmt := range register.Body.List {
			if _, ok := stmt.(*ast.ReturnStmt); ok {
				break
			}
			expr, ok := stmt.(*ast.ExprStmt)
			if !ok {
				continue
			}
			get := frontendCall(expr.X)
			if get == nil || len(get.Args) == 0 {
				continue
			}
			method, ok := get.Fun.(*ast.SelectorExpr)
			if !ok || method.Sel.Name != "Get" {
				continue
			}
			route := frontendCall(method.X)
			if route == nil || len(route.Args) != 1 || !frontendSelector(route.Args[0], rf, "routes", "ViteBuild") {
				continue
			}
			routeMethod, ok := route.Fun.(*ast.SelectorExpr)
			if !ok || routeMethod.Sel.Name != "Route" {
				continue
			}
			router, ok := routeMethod.X.(*ast.Ident)
			if !ok || register.Type.Params == nil || len(register.Type.Params.List) != 1 ||
				len(register.Type.Params.List[0].Names) != 1 || router.Name != register.Type.Params.List[0].Names[0].Name {
				continue
			}
			sel, ok := get.Args[0].(*ast.SelectorExpr)
			if !ok || len(register.Recv.List[0].Names) != 1 {
				continue
			}
			id, ok := sel.X.(*ast.Ident)
			if ok && id.Name == register.Recv.List[0].Names[0].Name {
				field = sel.Sel.Name
			}
		}
		if field == "" {
			violations = append(violations, frontendViolation(rf.path, "AssetsHandler does not register ViteBuild with its static handler", "Register rt.Route(routes.ViteBuild).Get(a.dist)."))
		} else {
			bindings := frontendBindings(constructor.Body)
			var handler ast.Expr
			traced := map[string]bool{}
			for _, stmt := range constructor.Body.List {
				ret, ok := stmt.(*ast.ReturnStmt)
				if !ok || len(ret.Results) == 0 {
					continue
				}
				lit, ok := frontendResolve(ret.Results[0], bindings).(*ast.CompositeLit)
				if !ok {
					break
				}
				id, ok := lit.Type.(*ast.Ident)
				if !ok || id.Name != "AssetsHandler" {
					break
				}
				for _, elt := range lit.Elts {
					if kv, ok := elt.(*ast.KeyValueExpr); ok {
						if key, ok := kv.Key.(*ast.Ident); ok && key.Name == field {
							frontendTrace(kv.Value, bindings, traced)
							handler = frontendResolve(kv.Value, bindings)
						}
					}
				}
				break
			}
			receiver := register.Recv.List[0].Names[0].Name
			router := register.Type.Params.List[0].Names[0].Name
			if frontendMutated(constructor.Body, traced) || frontendMutated(register.Body, map[string]bool{receiver: true, router: true}) {
				reasons = append(reasons, cf.path+": asset handler wiring is mutated")
				return violations, strings.Join(reasons, "; ")
			}
			static := frontendCall(handler)
			if static == nil || !frontendSelector(static.Fun, cf, "anduin", "StaticFS") || len(static.Args) == 0 {
				reasons = append(reasons, cf.path+": cannot trace registered handler to anduin.StaticFS")
			} else {
				sub := frontendCall(frontendResolve(static.Args[0], bindings))
				if sub == nil || !frontendSelector(sub.Fun, cf, "io/fs", "Sub") || len(sub.Args) != 2 {
					reasons = append(reasons, cf.path+": cannot trace StaticFS to fs.Sub(assets.Files, \"dist\")")
				} else if !frontendSelector(sub.Args[0], cf, "assets", "Files") {
					violations = append(violations, frontendViolation(cf.path, "asset filesystem is not assets.Files", `Use fs.Sub(assets.Files, "dist") for the Vite handler.`))
				} else if subtree, ok := frontendString(frontendResolve(sub.Args[1], bindings), nil, 0); !ok {
					reasons = append(reasons, cf.path+": dynamic asset filesystem subtree")
				} else if subtree != "dist" {
					violations = append(violations, frontendViolation(cf.path, "asset filesystem serves the wrong subtree", `Use fs.Sub(assets.Files, "dist") for the Vite handler.`))
				}
			}
		}
	}
	return violations, strings.Join(reasons, "; ")
}
