package layers

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Config configures the typed analyzers. Tests construct a Config directly;
// production callers use Analyze or Project.Run.
type Config struct {
	RootDir string
	Module  string
	Fset    *token.FileSet
	Project *Project
}

// Analyzers returns the ordered typed layer analyzers. Each analyzer's Name is
// the catalog rule ID with hyphens replaced by underscores, which is the Go
// identifier go/analysis requires; runAnalyzer maps it back to the catalog ID.
func Analyzers(cfg Config) []*analysis.Analyzer {
	return []*analysis.Analyzer{
		queriesImportAnalyzer(cfg),
		dbHandleAnalyzer(cfg),
		txUnwrapAnalyzer(cfg),
		txEscapeAnalyzer(cfg),
		txPublicCallAnalyzer(cfg),
		txLayerAnalyzer(cfg),
		payloadsImportAnalyzer(cfg),
		domainTransportAnalyzer(cfg),
		entityColumnsAnalyzer(cfg),
		routeHostAnalyzer(cfg),
	}
}

func newAnalyzer(id, doc string, run func(*analysis.Pass) error) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: analyzerName(id),
		Doc:  doc + " (rule " + id + ")",
		Run: func(pass *analysis.Pass) (any, error) {
			return nil, run(pass)
		},
	}
}

// analyzerName converts a rule ID (which may contain hyphens) into the Go
// identifier an analysis.Analyzer requires. The catalog ID remains the
// user-facing rule name.
func analyzerName(ruleID string) string {
	return strings.ReplaceAll(ruleID, "-", "_")
}

// ruleIDForAnalyzer maps an analyzer back to its catalog rule ID.
func ruleIDForAnalyzer(analyzer *analysis.Analyzer) string {
	if analyzer == nil {
		return ""
	}
	for _, rule := range LayerRules() {
		if analyzerName(rule.ID) == analyzer.Name {
			return rule.ID
		}
	}
	return analyzer.Name
}

func runAnalyzer(
	analyzer *analysis.Analyzer,
	pkg *Package,
	fset *token.FileSet,
	cfg Config,
) ([]Violation, error) {
	found := make([]Violation, 0)
	pass := &analysis.Pass{
		Analyzer:   analyzer,
		Fset:       fset,
		Files:      pkg.Syntax,
		Pkg:        pkg.Types,
		TypesInfo:  pkg.TypesInfo,
		TypesSizes: pkg.TypesSizes,
		ResultOf:   map[*analysis.Analyzer]any{},
		ImportObjectFact: func(types.Object, analysis.Fact) bool {
			return false
		},
		ImportPackageFact: func(*types.Package, analysis.Fact) bool {
			return false
		},
		ExportObjectFact:  func(types.Object, analysis.Fact) {},
		ExportPackageFact: func(analysis.Fact) {},
		AllObjectFacts:    func() []analysis.ObjectFact { return nil },
		AllPackageFacts:   func() []analysis.PackageFact { return nil },
	}
	pass.Report = func(diagnostic analysis.Diagnostic) {
		ruleID := ruleIDForAnalyzer(analyzer)
		position := fset.PositionFor(diagnostic.Pos, true)
		file := relativeFile(cfg.RootDir, position.Filename)
		found = append(found, Violation{
			Rule:    ruleID,
			File:    file,
			Line:    position.Line,
			Message: diagnostic.Message,
			Fix:     fixFor(ruleID),
		})
	}
	if _, err := analyzer.Run(pass); err != nil {
		return nil, err
	}
	return found, nil
}

func relativeFile(rootDir, filename string) string {
	if filename == "" {
		return ""
	}
	if rootDir == "" {
		return filepath.ToSlash(filename)
	}
	rel, err := filepath.Rel(rootDir, filename)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(filename)
	}
	return filepath.ToSlash(rel)
}

// pkgDir returns the package directory relative to the configured module.
// Loaded projects use the package's actual source directory, so external test
// variants stay in the layer they exercise; analysistest falls back to import
// paths because it has no filesystem metadata.
func pkgDir(cfg Config, pass *analysis.Pass) string {
	if cfg.Project != nil && pass.Pkg != nil {
		if pkg := cfg.Project.packageTypes[pass.Pkg]; pkg != nil {
			return pkg.layerDir(cfg.Module, cfg.RootDir)
		}
	}
	return layerDirFromPath(cfg.Module, pass.Pkg.Path())
}

func isStorageObject(object types.Object, name string) bool {
	return object != nil &&
		object.Pkg() != nil &&
		object.Pkg().Path() == StorageImportPath &&
		object.Name() == name
}

func isStorageCall(info *types.Info, call *ast.CallExpr, name string) bool {
	return isStorageObject(selectedObject(info, call.Fun), name)
}

func selectedObject(info *types.Info, expr ast.Expr) types.Object {
	switch typed := unparen(expr).(type) {
	case *ast.Ident:
		return info.Uses[typed]
	case *ast.SelectorExpr:
		return info.Uses[typed.Sel]
	case *ast.IndexExpr:
		return selectedObject(info, typed.X)
	case *ast.IndexListExpr:
		return selectedObject(info, typed.X)
	default:
		return nil
	}
}

func unparen(expr ast.Expr) ast.Expr {
	for {
		paren, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = paren.X
	}
}

func namedType(t types.Type) *types.Named {
	t = types.Unalias(t)
	for {
		switch typed := t.(type) {
		case *types.Named:
			return typed
		case *types.Pointer:
			t = types.Unalias(typed.Elem())
		default:
			return nil
		}
	}
}

func isStorageTx(t types.Type) bool {
	named, ok := types.Unalias(t).(*types.Named)
	return ok && isStorageObject(named.Obj(), "Tx")
}

func isBindable(t types.Type, cfg Config) bool {
	named := namedType(t)
	if named == nil {
		return false
	}
	object := named.Obj()
	if object == nil || object.Pkg() == nil {
		return false
	}
	switch object.Pkg().Path() {
	case StorageImportPath:
		return object.Name() == "Transactor" || object.Name() == "InsertQueue"
	case cfg.Module + "/domains/internal/queries":
		return object.Name() == "Queries"
	case cfg.Module + "/domains":
		return domainHasInTx(named)
	default:
		return false
	}
}

func domainHasInTx(named *types.Named) bool {
	for _, candidate := range []types.Type{named, types.NewPointer(named)} {
		methods := types.NewMethodSet(candidate)
		for method := range methods.Methods() {
			method := method.Obj()
			if method.Name() != "InTx" {
				continue
			}
			signature, ok := method.Type().(*types.Signature)
			if !ok || signature.Params().Len() != 1 || signature.Results().Len() != 1 {
				continue
			}
			if !isStorageTx(signature.Params().At(0).Type()) {
				continue
			}
			result := types.Unalias(signature.Results().At(0).Type())
			if types.Identical(result, named) || types.Identical(result, types.NewPointer(named)) {
				return true
			}
		}
	}
	return false
}

// isBindableInTxFunc reports whether function is an InTx method on a bindable
// receiver that returns that same receiver type. It rejects unrelated types
// which happen to expose a method named InTx.
func isBindableInTxFunc(function *types.Func, cfg Config) bool {
	if function == nil || function.Name() != "InTx" {
		return false
	}
	signature, ok := function.Type().(*types.Signature)
	if !ok || signature.Recv() == nil ||
		signature.Params().Len() != 1 || signature.Results().Len() != 1 {
		return false
	}
	if !isStorageTx(signature.Params().At(0).Type()) {
		return false
	}
	receiver := signature.Recv().Type()
	if !isBindable(receiver, cfg) {
		return false
	}
	receiverNamed := namedType(receiver)
	if receiverNamed == nil {
		return false
	}
	result := types.Unalias(signature.Results().At(0).Type())
	return types.Identical(result, receiverNamed) ||
		types.Identical(result, types.NewPointer(receiverNamed))
}

// storageSQLMethods is the SQL surface of the pkg/storage interfaces. It
// includes the pgx batch and copy methods a narrow local interface may embed
// next to storage.Connection.
var storageSQLMethods = map[string]bool{
	"Exec":      true,
	"Query":     true,
	"QueryRow":  true,
	"SendBatch": true,
	"CopyFrom":  true,
}

// isStorageSQLMethod reports whether the selected method is SQL executed
// through a pkg/storage type or interface. The receiver is a storage type when
// its declaration lives in pkg/storage (storage.Connection, *storage.Postgres,
// a type embedding storage.Postgres), or an interface that embeds a storage
// interface or copies one of its SQL method signatures. That last case is the
// narrow local interface the name-based rule used to miss.
func isStorageSQLMethod(
	pass *analysis.Pass,
	selector *ast.SelectorExpr,
	method *types.Func,
) bool {
	if method == nil || !storageSQLMethods[method.Name()] {
		return false
	}
	if method.Pkg() != nil && method.Pkg().Path() == StorageImportPath {
		return true
	}
	receiver := pass.TypesInfo.TypeOf(selector.X)
	if receiver == nil {
		return false
	}
	for {
		receiver = types.Unalias(receiver)
		pointer, ok := receiver.(*types.Pointer)
		if !ok {
			break
		}
		receiver = pointer.Elem()
	}
	if named, ok := receiver.(*types.Named); ok && named.Obj() != nil &&
		named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == StorageImportPath {
		return true
	}
	if _, ok := receiver.Underlying().(*types.Interface); !ok {
		return false
	}
	return matchesStorageSQLSignature(pass.Pkg, method)
}

// matchesStorageSQLSignature compares a locally declared interface method with
// the methods pkg/storage declares. Comparing declarations rather than names
// keeps an unrelated Exec(string) from being reported as SQL.
func matchesStorageSQLSignature(pkg *types.Package, method *types.Func) bool {
	storagePkg := importedPackage(pkg, StorageImportPath)
	if storagePkg == nil {
		return false
	}
	for _, name := range []string{"Connection", "Transaction", "Postgres", "InsertQueue"} {
		object := storagePkg.Scope().Lookup(name)
		if object == nil {
			continue
		}
		typ := object.Type()
		if _, ok := typ.Underlying().(*types.Interface); !ok {
			typ = types.NewPointer(typ)
		}
		methods := types.NewMethodSet(typ)
		for candidate := range methods.Methods() {
			candidateFunc, ok := candidate.Obj().(*types.Func)
			if !ok || candidateFunc.Name() != method.Name() {
				continue
			}
			if types.Identical(candidateFunc.Type(), method.Type()) {
				return true
			}
		}
	}
	return false
}

func importedPackage(pkg *types.Package, path string) *types.Package {
	if pkg == nil {
		return nil
	}
	for _, imported := range pkg.Imports() {
		if imported.Path() == path {
			return imported
		}
	}
	return nil
}

func postgresForbiddenMethods(t types.Type) map[string]bool {
	named := namedType(t)
	if named == nil || !isStorageObject(named.Obj(), "Postgres") {
		return nil
	}
	methods := types.NewMethodSet(types.NewPointer(named))
	forbidden := make(map[string]bool, methods.Len())
	for method := range methods.Methods() {
		name := method.Obj().Name()
		if name == "Health" || name == "Close" {
			continue
		}
		forbidden[name] = true
	}
	return forbidden
}

// postgresForbiddenMethod reports whether function is an SQL/handle method
// declared by storage.Postgres. Reading the receiver from the method
// declaration catches promoted methods selected through an embedding type,
// where selection.Recv() is the embedding type instead.
func postgresForbiddenMethod(function *types.Func) bool {
	if function == nil {
		return false
	}
	signature, ok := function.Type().(*types.Signature)
	if !ok || signature.Recv() == nil {
		return false
	}
	return postgresForbiddenMethods(signature.Recv().Type())[function.Name()]
}

func renderExpr(expr ast.Expr) string {
	return types.ExprString(expr)
}
