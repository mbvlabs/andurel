package layers

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"maps"

	"golang.org/x/tools/go/analysis"
)

type atomicOpsSite struct {
	call  *ast.CallExpr
	outer *ast.FuncDecl
}

type callbackTarget struct {
	body         *ast.BlockStmt
	info         *types.Info
	receiver     *types.Var
	tx           *types.Var
	lexicalOuter *ast.FuncDecl
}

func txEscapeAnalyzer(cfg Config) *analysis.Analyzer {
	return newAnalyzer("tx-escape",
		"a storage.AtomicOps callback binds captured dependencies with InTx",
		func(pass *analysis.Pass) error {
			current := packageOfPass(pass)
			for _, site := range atomicOpsSites(pass) {
				arg, ok := callbackArg(site.call)
				if !ok {
					pass.Report(analysis.Diagnostic{
						Pos:     site.call.Pos(),
						Message: "cannot analyze a storage.AtomicOps call without a callback argument",
					})
					continue
				}
				target, ok := resolveCallback(cfg, current, site, pass)
				if !ok {
					pass.Report(analysis.Diagnostic{
						Pos: arg.Pos(),
						Message: "cannot analyze the storage.AtomicOps callback; " +
							"pass a func literal or a named function or method so the transaction boundary can be verified",
					})
					continue
				}
				newEscapeWalker(cfg, target, pass.Report).run()
			}
			return nil
		})
}

func txPublicCallAnalyzer(cfg Config) *analysis.Analyzer {
	return newAnalyzer("tx-public-call",
		"a storage.AtomicOps callback does not call an exported method on its own receiver",
		func(pass *analysis.Pass) error {
			current := packageOfPass(pass)
			for _, site := range atomicOpsSites(pass) {
				target, ok := resolveCallback(cfg, current, site, pass)
				if !ok || target.receiver == nil {
					continue
				}
				reportPublicCalls(target, site, pass.Report)
			}
			return nil
		})
}

func packageOfPass(pass *analysis.Pass) *Package {
	return &Package{
		Path:       pass.Pkg.Path(),
		Name:       pass.Pkg.Name(),
		Syntax:     pass.Files,
		Types:      pass.Pkg,
		TypesInfo:  pass.TypesInfo,
		TypesSizes: pass.TypesSizes,
	}
}

func atomicOpsSites(pass *analysis.Pass) []atomicOpsSite {
	sites := make([]atomicOpsSite, 0)
	add := func(node ast.Node, outer *ast.FuncDecl) {
		if node == nil {
			return
		}
		ast.Inspect(node, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || !isStorageCall(pass.TypesInfo, call, "AtomicOps") {
				return true
			}
			sites = append(sites, atomicOpsSite{call: call, outer: outer})
			return true
		})
	}
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			switch typed := decl.(type) {
			case *ast.FuncDecl:
				if typed.Body != nil {
					add(typed.Body, typed)
				}
			case *ast.GenDecl:
				for _, spec := range typed.Specs {
					values, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for _, value := range values.Values {
						add(value, nil)
					}
				}
			}
		}
	}
	return sites
}

func callbackArg(call *ast.CallExpr) (ast.Expr, bool) {
	if call == nil || len(call.Args) == 0 {
		return nil, false
	}
	return call.Args[len(call.Args)-1], true
}

func resolveCallback(
	cfg Config,
	current *Package,
	site atomicOpsSite,
	pass *analysis.Pass,
) (*callbackTarget, bool) {
	arg, ok := callbackArg(site.call)
	if !ok {
		return nil, false
	}
	if literal, ok := unparen(arg).(*ast.FuncLit); ok {
		return &callbackTarget{
			body:         literal.Body,
			info:         pass.TypesInfo,
			receiver:     receiverObject(pass.TypesInfo, site.outer),
			tx:           callbackTx(pass.TypesInfo, literal.Type),
			lexicalOuter: site.outer,
		}, true
	}
	function, ok := selectedObject(pass.TypesInfo, arg).(*types.Func)
	if !ok {
		return nil, false
	}
	return resolveFunc(cfg, current, function)
}

func resolveFunc(
	cfg Config,
	current *Package,
	function *types.Func,
) (*callbackTarget, bool) {
	if function == nil {
		return nil, false
	}
	origin := function.Origin()
	pos := origin.Pos()

	for _, file := range current.Syntax {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Pos() != pos || fn.Body == nil {
				continue
			}
			return &callbackTarget{
				body:     fn.Body,
				info:     current.TypesInfo,
				receiver: receiverObject(current.TypesInfo, fn),
				tx:       callbackTx(current.TypesInfo, fn.Type),
			}, true
		}
	}
	if cfg.Project == nil {
		return nil, false
	}
	fn, pkg, ok := cfg.Project.funcDeclAt(pos)
	if !ok || fn.Body == nil || pkg == nil {
		return nil, false
	}
	return &callbackTarget{
		body:     fn.Body,
		info:     pkg.TypesInfo,
		receiver: receiverObject(pkg.TypesInfo, fn),
		tx:       callbackTx(pkg.TypesInfo, fn.Type),
	}, true
}

func receiverObject(info *types.Info, fn *ast.FuncDecl) *types.Var {
	if fn == nil || fn.Recv == nil || len(fn.Recv.List) == 0 {
		return nil
	}
	names := fn.Recv.List[0].Names
	if len(names) == 0 || names[0].Name == "_" {
		return nil
	}
	receiver, _ := info.Defs[names[0]].(*types.Var)
	return receiver
}

func callbackTx(info *types.Info, function *ast.FuncType) *types.Var {
	if function == nil || function.Params == nil {
		return nil
	}
	for _, field := range function.Params.List {
		for _, name := range field.Names {
			if name.Name == "_" {
				continue
			}
			variable, ok := info.Defs[name].(*types.Var)
			if ok && isStorageTx(variable.Type()) {
				return variable
			}
		}
	}
	return nil
}

type escapeState struct {
	bound map[types.Object]bool
	tx    map[types.Object]bool
}

func newEscapeState() *escapeState {
	return &escapeState{
		bound: make(map[types.Object]bool),
		tx:    make(map[types.Object]bool),
	}
}

func (s *escapeState) clone() *escapeState {
	clone := newEscapeState()
	maps.Copy(clone.bound, s.bound)
	maps.Copy(clone.tx, s.tx)
	return clone
}

func mergeEscapeStates(states ...*escapeState) *escapeState {
	merged := newEscapeState()
	if len(states) == 0 {
		return merged
	}
	boundKeys := make(map[types.Object]bool)
	txKeys := make(map[types.Object]bool)
	for _, state := range states {
		for object := range state.bound {
			boundKeys[object] = true
		}
		for object := range state.tx {
			txKeys[object] = true
		}
	}
	for object := range boundKeys {
		bound := true
		for _, state := range states {
			if !state.bound[object] {
				bound = false
				break
			}
		}
		merged.bound[object] = bound
	}
	for object := range txKeys {
		alias := true
		for _, state := range states {
			if !state.tx[object] {
				alias = false
				break
			}
		}
		merged.tx[object] = alias
	}
	return merged
}

type escapeWalker struct {
	cfg                Config
	target             *callbackTarget
	report             func(analysis.Diagnostic)
	state              *escapeState
	inTxReceivers      map[ast.Expr]bool
	selectorSelIdents  map[*ast.Ident]bool
	compositeKeyIdents map[*ast.Ident]bool
	nestedParams       map[types.Object]bool
	nestedCallbacks    map[*ast.FuncLit]bool
}

func newEscapeWalker(
	cfg Config,
	target *callbackTarget,
	report func(analysis.Diagnostic),
) *escapeWalker {
	walker := &escapeWalker{
		cfg:                cfg,
		target:             target,
		report:             report,
		state:              newEscapeState(),
		inTxReceivers:      make(map[ast.Expr]bool),
		selectorSelIdents:  make(map[*ast.Ident]bool),
		compositeKeyIdents: make(map[*ast.Ident]bool),
		nestedParams:       make(map[types.Object]bool),
		nestedCallbacks:    make(map[*ast.FuncLit]bool),
	}
	if target.tx != nil {
		walker.state.tx[target.tx] = true
	}
	for _, object := range target.info.Defs {
		variable, ok := object.(*types.Var)
		if !ok || !isBindable(variable.Type(), cfg) {
			continue
		}
		if variable.Pos() > target.body.Lbrace && variable.Pos() < target.body.Rbrace {
			walker.state.bound[variable] = false
		}
	}
	ast.Inspect(target.body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.FuncLit:
			for _, field := range typed.Type.Params.List {
				for _, name := range field.Names {
					if variable, ok := target.info.Defs[name].(*types.Var); ok {
						walker.nestedParams[variable] = true
					}
				}
			}
		case *ast.SelectorExpr:
			walker.selectorSelIdents[typed.Sel] = true
		case *ast.CallExpr:
			if isStorageCall(target.info, typed, "AtomicOps") {
				if argument, ok := callbackArg(typed); ok {
					if literal, ok := unparen(argument).(*ast.FuncLit); ok {
						walker.nestedCallbacks[literal] = true
					}
				}
			}
		case *ast.CompositeLit:
			value, ok := target.info.Types[typed]
			if !ok {
				return true
			}
			if _, isStruct := value.Type.Underlying().(*types.Struct); !isStruct {
				return true
			}
			for _, element := range typed.Elts {
				keyValue, ok := element.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if key, ok := keyValue.Key.(*ast.Ident); ok {
					walker.compositeKeyIdents[key] = true
				}
			}
		}
		return true
	})
	return walker
}

func (w *escapeWalker) run() {
	w.walkStatements(w.target.body.List)
}

func (w *escapeWalker) branch(analyze func()) *escapeState {
	saved := w.state
	w.state = saved.clone()
	analyze()
	result := w.state
	w.state = saved
	return result
}

func (w *escapeWalker) walkStatements(statements []ast.Stmt) {
	for _, statement := range statements {
		w.walkStatement(statement)
	}
}

func (w *escapeWalker) walkStatement(statement ast.Stmt) {
	switch typed := statement.(type) {
	case nil:
		return
	case *ast.AssignStmt:
		for _, value := range typed.Rhs {
			w.walkExpr(value)
		}
		w.assign(typed.Lhs, typed.Rhs)
	case *ast.DeclStmt:
		decl, ok := typed.Decl.(*ast.GenDecl)
		if !ok {
			return
		}
		for _, spec := range decl.Specs {
			values, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, value := range values.Values {
				w.walkExpr(value)
			}
			left := make([]ast.Expr, 0, len(values.Names))
			for _, name := range values.Names {
				left = append(left, name)
			}
			w.assign(left, values.Values)
		}
	case *ast.ExprStmt:
		w.walkExpr(typed.X)
	case *ast.ReturnStmt:
		for _, result := range typed.Results {
			w.walkExpr(result)
		}
	case *ast.BlockStmt:
		w.walkStatements(typed.List)
	case *ast.IfStmt:
		w.walkStatement(typed.Init)
		w.walkExpr(typed.Cond)
		body := w.branch(func() { w.walkStatement(typed.Body) })
		elseState := w.state
		if typed.Else != nil {
			elseState = w.branch(func() { w.walkStatement(typed.Else) })
		}
		w.state = mergeEscapeStates(body, elseState)
	case *ast.ForStmt:
		w.walkStatement(typed.Init)
		w.walkExpr(typed.Cond)
		entry := w.state.clone()
		body := w.branch(func() {
			w.walkStatement(typed.Body)
			w.walkStatement(typed.Post)
		})
		w.state = mergeEscapeStates(entry, body)
	case *ast.RangeStmt:
		w.walkExpr(typed.X)
		w.markUnbound(typed.Key)
		w.markUnbound(typed.Value)
		entry := w.state.clone()
		body := w.branch(func() {
			w.walkStatement(typed.Body)
		})
		w.state = mergeEscapeStates(entry, body)
	case *ast.SwitchStmt:
		w.walkStatement(typed.Init)
		w.walkExpr(typed.Tag)
		w.mergeClauses(typed.Body.List)
	case *ast.TypeSwitchStmt:
		w.walkStatement(typed.Init)
		w.walkStatement(typed.Assign)
		w.mergeClauses(typed.Body.List)
	case *ast.SelectStmt:
		w.mergeCommClauses(typed.Body.List)
	case *ast.CaseClause:
		for _, item := range typed.List {
			w.walkExpr(item)
		}
		w.walkStatements(typed.Body)
	case *ast.CommClause:
		w.walkStatement(typed.Comm)
		w.walkStatements(typed.Body)
	case *ast.GoStmt:
		w.walkExpr(typed.Call)
	case *ast.DeferStmt:
		w.walkExpr(typed.Call)
	case *ast.SendStmt:
		w.walkExpr(typed.Chan)
		w.walkExpr(typed.Value)
	case *ast.IncDecStmt:
		w.walkExpr(typed.X)
	case *ast.LabeledStmt:
		w.walkStatement(typed.Stmt)
	}
}

func (w *escapeWalker) mergeClauses(statements []ast.Stmt) {
	entry := w.state.clone()
	states := make([]*escapeState, 0, len(statements)+1)
	hasDefault := false
	for _, statement := range statements {
		clause, ok := statement.(*ast.CaseClause)
		if !ok {
			continue
		}
		if len(clause.List) == 0 {
			hasDefault = true
		}
		for _, item := range clause.List {
			w.walkExpr(item)
		}
		states = append(states, w.branch(func() { w.walkStatements(clause.Body) }))
	}
	if !hasDefault {
		states = append(states, entry)
	}
	if len(states) == 0 {
		states = append(states, entry)
	}
	w.state = mergeEscapeStates(states...)
}

func (w *escapeWalker) mergeCommClauses(statements []ast.Stmt) {
	entry := w.state.clone()
	states := make([]*escapeState, 0, len(statements)+1)
	hasDefault := false
	for _, statement := range statements {
		clause, ok := statement.(*ast.CommClause)
		if !ok {
			continue
		}
		if clause.Comm == nil {
			hasDefault = true
		}
		states = append(states, w.branch(func() {
			w.walkStatement(clause.Comm)
			w.walkStatements(clause.Body)
		}))
	}
	if !hasDefault {
		states = append(states, entry)
	}
	if len(states) == 0 {
		states = append(states, entry)
	}
	w.state = mergeEscapeStates(states...)
}

func (w *escapeWalker) markUnbound(expr ast.Expr) {
	ident, ok := unparen(expr).(*ast.Ident)
	if !ok {
		return
	}
	variable, ok := w.target.info.Defs[ident].(*types.Var)
	if !ok {
		variable, _ = w.target.info.Uses[ident].(*types.Var)
	}
	if variable == nil {
		return
	}
	if _, tracked := w.state.bound[variable]; tracked {
		w.state.bound[variable] = false
	}
	if _, tracked := w.state.tx[variable]; tracked {
		w.state.tx[variable] = false
	}
}

func (w *escapeWalker) assign(left, right []ast.Expr) {
	for index, lhs := range left {
		ident, ok := unparen(lhs).(*ast.Ident)
		if !ok {
			continue
		}
		variable, ok := w.target.info.Defs[ident].(*types.Var)
		if !ok {
			variable, _ = w.target.info.Uses[ident].(*types.Var)
		}
		if variable == nil {
			continue
		}
		if _, tracked := w.state.bound[variable]; tracked {
			bound := false
			switch {
			case len(right) == len(left):
				bound = w.exprBound(right[index])
			case len(right) == 1:
				bound = w.exprBound(right[0])
			}
			w.state.bound[variable] = bound
		}
		if len(right) == len(left) {
			w.state.tx[variable] = w.exprTxAlias(right[index])
		} else if len(right) == 1 {
			w.state.tx[variable] = w.exprTxAlias(right[0])
		}
	}
}

func (w *escapeWalker) exprTxAlias(expr ast.Expr) bool {
	ident, ok := unparen(expr).(*ast.Ident)
	if !ok {
		return false
	}
	variable, ok := w.target.info.Uses[ident].(*types.Var)
	if !ok {
		return false
	}
	return w.state.tx[variable]
}

func (w *escapeWalker) exprBound(expr ast.Expr) bool {
	switch typed := unparen(expr).(type) {
	case *ast.CallExpr:
		return w.isInTxCall(typed)
	case *ast.Ident:
		variable, ok := w.target.info.Uses[typed].(*types.Var)
		if !ok {
			return false
		}
		if w.nestedParams[variable] {
			return true
		}
		return w.state.bound[variable]
	case *ast.SelectorExpr:
		root := rootObject(w.target.info, typed)
		if root == nil {
			return false
		}
		if w.nestedParams[root] {
			return true
		}
		return w.state.bound[root]
	case *ast.StarExpr:
		return w.exprBound(typed.X)
	case *ast.UnaryExpr:
		return w.exprBound(typed.X)
	case *ast.TypeAssertExpr:
		return w.exprBound(typed.X)
	case *ast.IndexExpr:
		return w.exprBound(typed.X)
	default:
		return false
	}
}

func (w *escapeWalker) isInTxCall(call *ast.CallExpr) bool {
	function, ok := selectedObject(w.target.info, call.Fun).(*types.Func)
	if !ok || !isBindableInTxFunc(function, w.cfg) || len(call.Args) != 1 {
		return false
	}
	return w.exprTxAlias(call.Args[0])
}

func (w *escapeWalker) isInTxLikeCall(call *ast.CallExpr) bool {
	function, ok := selectedObject(w.target.info, call.Fun).(*types.Func)
	return ok && isBindableInTxFunc(function, w.cfg)
}

func (w *escapeWalker) walkExpr(expr ast.Expr) {
	if expr == nil {
		return
	}
	ast.Inspect(expr, func(node ast.Node) bool {
		if literal, ok := node.(*ast.FuncLit); ok {
			if w.nestedCallbacks[literal] {
				return false
			}
			w.branch(func() { w.walkStatements(literal.Body.List) })
			return false
		}
		if call, ok := node.(*ast.CallExpr); ok && w.isInTxCall(call) {
			if selector, ok := unparen(call.Fun).(*ast.SelectorExpr); ok {
				w.inTxReceivers[unparen(selector.X)] = true
			}
		}
		candidate, ok := node.(ast.Expr)
		if !ok || !w.isCandidate(candidate) {
			return true
		}
		if w.inTxReceivers[candidate] || w.exprBound(candidate) {
			return true
		}
		w.report(analysis.Diagnostic{
			Pos: candidate.Pos(),
			Message: fmt.Sprintf(
				"%s is used inside a storage.AtomicOps callback without binding it with .InTx(tx)",
				renderExpr(candidate),
			),
		})
		return true
	})
}

func (w *escapeWalker) isCandidate(expr ast.Expr) bool {
	value, ok := w.target.info.Types[expr]
	if !ok || !value.IsValue() || !isBindable(value.Type, w.cfg) {
		return false
	}
	switch typed := expr.(type) {
	case *ast.Ident:
		if w.selectorSelIdents[typed] || w.compositeKeyIdents[typed] {
			return false
		}
		_, ok := w.target.info.Uses[typed].(*types.Var)
		return ok
	case *ast.CompositeLit, *ast.ParenExpr, *ast.KeyValueExpr:
		return false
	case *ast.UnaryExpr:
		if _, ok := unparen(typed.X).(*ast.CompositeLit); ok {
			return false
		}
		return true
	case *ast.CallExpr:
		return !w.isInTxLikeCall(typed)
	default:
		return true
	}
}

func rootObject(info *types.Info, expr ast.Expr) types.Object {
	switch typed := unparen(expr).(type) {
	case *ast.Ident:
		return info.Uses[typed]
	case *ast.SelectorExpr:
		return rootObject(info, typed.X)
	case *ast.StarExpr:
		return rootObject(info, typed.X)
	case *ast.IndexExpr:
		return rootObject(info, typed.X)
	case *ast.TypeAssertExpr:
		return rootObject(info, typed.X)
	default:
		return nil
	}
}

type aliasPair struct {
	lhs *types.Var
	rhs ast.Expr
}

// reportPublicCalls tracks simple receiver aliases and method values. Provenance
// is flow-insensitive and unions assignments across branches: a false positive
// is preferred to silently missing a receiver method reference inside the
// callback. Shadowing creates a distinct types.Object and stays clean.
func reportPublicCalls(
	target *callbackTarget,
	site atomicOpsSite,
	report func(analysis.Diagnostic),
) {
	prePairs := collectAliasPairs(target.info, site.outer, site.call.Pos(), target.lexicalOuter != nil)
	pairs := append([]aliasPair(nil), prePairs...)
	pairs = append(pairs, collectAliasPairs(target.info, target.body, token.NoPos, true)...)

	aliases := map[types.Object]bool{}
	if target.receiver != nil {
		aliases[target.receiver] = true
	}
	for changed := true; changed; {
		changed = false
		for _, pair := range pairs {
			if exprIsAlias(target.info, pair.rhs, aliases) && !aliases[pair.lhs] {
				aliases[pair.lhs] = true
				changed = true
			}
		}
	}

	methods := map[types.Object]*types.Func{}
	for _, pair := range prePairs {
		if method := receiverMethodRef(target.info, pair.rhs, aliases); method != nil {
			methods[pair.lhs] = method
		}
	}

	ast.Inspect(target.body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.SelectorExpr:
			selection, ok := target.info.Selections[typed]
			if !ok || selection.Kind() != types.MethodVal {
				return true
			}
			method := selection.Obj()
			ident, ok := unparen(typed.X).(*ast.Ident)
			if !ok {
				return true
			}
			root := target.info.Uses[ident]
			if method == nil || !aliases[root] || method.Name() == "InTx" ||
				!ast.IsExported(method.Name()) {
				return true
			}
			report(analysis.Diagnostic{
				Pos: typed.Pos(),
				Message: fmt.Sprintf(
					"%s inside a storage.AtomicOps callback opens its own, independent transaction",
					renderExpr(typed),
				),
			})
		case *ast.CallExpr:
			ident, ok := unparen(typed.Fun).(*ast.Ident)
			if !ok {
				return true
			}
			variable, ok := target.info.Uses[ident].(*types.Var)
			if !ok {
				return true
			}
			method := methods[variable]
			if method == nil {
				return true
			}
			report(analysis.Diagnostic{
				Pos: typed.Pos(),
				Message: fmt.Sprintf(
					"calls %s inside a storage.AtomicOps callback; it opens its own, independent transaction",
					method.Name(),
				),
			})
		}
		return true
	})
}

func collectAliasPairs(
	info *types.Info,
	node ast.Node,
	before token.Pos,
	include bool,
) []aliasPair {
	pairs := make([]aliasPair, 0)
	if !include || node == nil {
		return pairs
	}
	add := func(lhs, rhs []ast.Expr) {
		if len(lhs) != len(rhs) {
			return
		}
		for index := range lhs {
			ident, ok := unparen(lhs[index]).(*ast.Ident)
			if !ok {
				continue
			}
			variable, ok := info.Defs[ident].(*types.Var)
			if !ok {
				variable, _ = info.Uses[ident].(*types.Var)
			}
			if variable == nil {
				continue
			}
			pairs = append(pairs, aliasPair{lhs: variable, rhs: rhs[index]})
		}
	}
	ast.Inspect(node, func(node ast.Node) bool {
		if node == nil {
			return true
		}
		if before.IsValid() && node.Pos() >= before {
			return false
		}
		switch typed := node.(type) {
		case *ast.AssignStmt:
			add(typed.Lhs, typed.Rhs)
		case *ast.ValueSpec:
			lhs := make([]ast.Expr, 0, len(typed.Names))
			for _, name := range typed.Names {
				lhs = append(lhs, name)
			}
			add(lhs, typed.Values)
		}
		return true
	})
	return pairs
}

func exprIsAlias(info *types.Info, expr ast.Expr, aliases map[types.Object]bool) bool {
	switch typed := unparen(expr).(type) {
	case *ast.Ident:
		variable, ok := info.Uses[typed].(*types.Var)
		return ok && aliases[variable]
	case *ast.UnaryExpr:
		return typingUnaryAddress(typed) && exprIsAlias(info, typed.X, aliases)
	case *ast.StarExpr:
		return exprIsAlias(info, typed.X, aliases)
	default:
		return false
	}
}

func typingUnaryAddress(expr *ast.UnaryExpr) bool {
	return expr.Op == token.AND
}

func receiverMethodRef(
	info *types.Info,
	expr ast.Expr,
	aliases map[types.Object]bool,
) *types.Func {
	selector, ok := unparen(expr).(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	selection, ok := info.Selections[selector]
	if !ok || selection.Kind() != types.MethodVal {
		return nil
	}
	method, ok := selection.Obj().(*types.Func)
	if !ok {
		return nil
	}
	ident, ok := unparen(selector.X).(*ast.Ident)
	if !ok {
		return nil
	}
	root := info.Uses[ident]
	if !aliases[root] || method.Name() == "InTx" || !ast.IsExported(method.Name()) {
		return nil
	}
	return method
}
