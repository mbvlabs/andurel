package layers

import (
	"go/ast"
	"go/types"
	"golang.org/x/tools/go/analysis"
	"strconv"
	"strings"
)

var forbiddenDomainTransportImports = []string{
	"github.com/mbvlabs/andurel/pkg/anduin",
	"github.com/mbvlabs/andurel/pkg/inertia",
	"github.com/mbvlabs/andurel/pkg/kiks",
	"github.com/mbvlabs/andurel/pkg/routing",
	"net/http",
}

func queriesImportAnalyzer(cfg Config) *analysis.Analyzer {
	return newAnalyzer("queries-import",
		"only domains may import generated domains/internal/queries",
		func(pass *analysis.Pass) error {
			queriesImport := cfg.Module + "/domains/internal/queries"
			if inDir(pkgDir(cfg, pass), "domains") {
				return nil
			}
			for _, file := range pass.Files {
				for _, imported := range file.Imports {
					if importPath(imported) != queriesImport {
						continue
					}
					pass.Report(analysis.Diagnostic{
						Pos: imported.Pos(),
						Message: "imports " + queriesImport +
							"; only domains/** may import generated queries",
					})
				}
			}
			return nil
		})
}

func dbHandleAnalyzer(cfg Config) *analysis.Analyzer {
	return newAnalyzer("db-handle",
		"outside domains nothing holds a raw database handle or imports a SQL driver",
		func(pass *analysis.Pass) error {
			dir := pkgDir(cfg, pass)
			inDomains := inDir(dir, "domains")
			persistence := inPersistenceDir(dir)
			for _, file := range pass.Files {
				for _, imported := range file.Imports {
					path := importPath(imported)
					if inDomains || !isSQLDriverImport(path) {
						continue
					}
					pass.Report(analysis.Diagnostic{
						Pos: imported.Pos(),
						Message: "imports " + path +
							"; only domains/** talks to the database",
					})
				}
			}

			for ident, object := range pass.TypesInfo.Uses {
				if !persistence &&
					(isStorageObject(object, "Connection") || isStorageObject(object, "Transaction")) {
					pass.Report(analysis.Diagnostic{
						Pos: ident.Pos(),
						Message: "references storage." + object.Name() +
							"; only persistence layers hold the database handle",
					})
				}
			}

			if inDomains {
				return nil
			}
			for _, file := range pass.Files {
				ast.Inspect(file, func(node ast.Node) bool {
					selector, ok := node.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					selection, ok := pass.TypesInfo.Selections[selector]
					if !ok {
						return true
					}
					if selection.Kind() != types.MethodVal && selection.Kind() != types.MethodExpr {
						return true
					}
					name := selection.Obj().Name()
					method, ok := selection.Obj().(*types.Func)
					if !ok || !postgresForbiddenMethod(method) {
						return true
					}
					pass.Report(analysis.Diagnostic{
						Pos: selector.Pos(),
						Message: "calls storage.Postgres." + name +
							"; only domains/** executes SQL",
					})
					return true
				})
			}
			return nil
		})
}

func txUnwrapAnalyzer(cfg Config) *analysis.Analyzer {
	return newAnalyzer("tx-unwrap",
		"storage.Unwrap is referenced only under domains",
		func(pass *analysis.Pass) error {
			if inDir(pkgDir(cfg, pass), "domains") {
				return nil
			}
			for ident, object := range pass.TypesInfo.Uses {
				if !isStorageObject(object, "Unwrap") {
					continue
				}
				pass.Report(analysis.Diagnostic{
					Pos:     ident.Pos(),
					Message: "references storage.Unwrap; only domains/** may unwrap a storage.Tx",
				})
			}
			return nil
		})
}

func txLayerAnalyzer(cfg Config) *analysis.Analyzer {
	return newAnalyzer("tx-layer",
		"storage.AtomicOps and storage.Transactor stay in services, domains, and internal/runtime",
		func(pass *analysis.Pass) error {
			dir := pkgDir(cfg, pass)
			if inDir(dir, "services") || inDir(dir, "domains") || inDir(dir, "internal/runtime") {
				return nil
			}
			for ident, object := range pass.TypesInfo.Uses {
				if !isStorageObject(object, "AtomicOps") && !isStorageObject(object, "Transactor") {
					continue
				}
				pass.Report(analysis.Diagnostic{
					Pos: ident.Pos(),
					Message: "references storage." + object.Name() +
						"; transaction orchestration belongs in services/**",
				})
			}
			return nil
		})
}

func domainTransportAnalyzer(cfg Config) *analysis.Analyzer {
	return newAnalyzer("domain-transport",
		"domains import no HTTP, Inertia, routing, or delivery packages",
		func(pass *analysis.Pass) error {
			if !inDir(pkgDir(cfg, pass), "domains") {
				return nil
			}
			forbidden := make(map[string]bool, len(forbiddenDomainTransportImports)+3)
			for _, path := range forbiddenDomainTransportImports {
				forbidden[path] = true
			}
			for _, name := range []string{"handlers", "middleware", "routes"} {
				forbidden[cfg.Module+"/"+name] = true
			}
			for _, file := range pass.Files {
				for _, imported := range file.Imports {
					path := importPath(imported)
					if !forbidden[path] {
						continue
					}
					pass.Report(analysis.Diagnostic{
						Pos: imported.Pos(),
						Message: "domains/** must not import " + path +
							"; keep transport concerns outside the domain layer",
					})
				}
			}
			return nil
		})
}

func importPath(spec *ast.ImportSpec) string {
	path, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		return strings.Trim(spec.Path.Value, `"`)
	}
	return path
}

func isSQLDriverImport(path string) bool {
	if path == "database/sql" {
		return true
	}
	if path == "github.com/jackc/pgx/v5" || strings.HasPrefix(path, "github.com/jackc/pgx/v5/") {
		return path != "github.com/jackc/pgx/v5/pgtype"
	}
	return false
}
