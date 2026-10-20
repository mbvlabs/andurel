package cli

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"unicode"
)

const andurelStorageImport = "github.com/mbvlabs/andurel/pkg/storage"

// persistenceDirs may hold the raw database handle: the domains package owns
// all persistence, and the composition root, process entry points, seeds, and
// migrations construct or hand it over.
var persistenceDirs = []string{"domains/", "internal/runtime/", "cmd/", "seeds/", "migrations/"}

// forbiddenStorageIdents are the storage identifiers that expose the raw
// database handle. Code outside persistenceDirs reaches the database only
// through domain receivers and storage.AtomicOps.
var forbiddenStorageIdents = map[string]bool{
	"Connection":  true,
	"Transaction": true,
}

// bindableStorageTypes are the pkg/storage types with an InTx method.
var bindableStorageTypes = map[string]bool{
	"Transactor":  true,
	"InsertQueue": true,
}

// parsedGoFile is one project Go file prepared for the boundary checks.
type parsedGoFile struct {
	rel     string
	dir     string
	file    *ast.File
	imports map[string]string // local package name → import path
}

// fieldDecl is a struct field's declared type plus the imports of the file
// that declares it, so qualified types can be resolved.
type fieldDecl struct {
	typ     ast.Expr
	imports map[string]string
}

// boundaryIndex is the syntactic type information the transaction rules
// need: which types can be bound with InTx, and the fields of every struct.
type boundaryIndex struct {
	module       string
	domainsInTx  map[string]bool                            // domains types with an InTx method
	localInTx    map[string]map[string]bool                 // dir → local types with an InTx method
	structFields map[string]map[string]map[string]fieldDecl // dir → type → field → decl
}

// verifyLayerBoundaries enforces the persistence and transaction boundary:
//
//   - queries-import: only files under domains/ may import domains/internal/queries.
//   - db-handle: outside persistenceDirs, no file references storage.Connection,
//     or storage.Transaction, or imports pgx (pgtype is allowed because
//     entities carry pgtype values).
//   - tx-unwrap: storage.Unwrap is called only under domains/.
//   - tx-escape: inside a func literal passed to storage.AtomicOps, the
//     enclosing method's receiver fields that can be bound with InTx are used
//     only as <recv>.<field>.InTx(...). Any other use runs outside the
//     transaction.
//   - tx-public-call: inside such a func literal, no exported method other
//     than InTx is called on the enclosing receiver; a nested public use case
//     opens its own, independent transaction.
//
// The checks are syntactic. Field types come from the receiver's struct
// declaration in the same package directory; "can be bound" means a local or
// domains type with an InTx method, storage.Transactor, storage.InsertQueue,
// or the generated *queries.Queries.
func verifyLayerBoundaries(rootDir string) verifyStep {
	module, _, err := readGoModMetadata(rootDir)
	if err != nil {
		step := verifyStepFailed("layers")
		step.Detail = err.Error()
		return step
	}

	fset := token.NewFileSet()
	files := make([]parsedGoFile, 0)
	violations := make([]verifyViolation, 0)
	walkErr := filepath.WalkDir(rootDir, func(filePath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "bin", "node_modules", "vendor", "dist", "resources":
				return fs.SkipDir
			}
			return nil
		}
		if filepath.Ext(filePath) != ".go" {
			return nil
		}

		rel, err := filepath.Rel(rootDir, filePath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		file, err := parser.ParseFile(fset, filePath, nil, parser.SkipObjectResolution)
		if err != nil {
			violations = append(violations, verifyViolation{
				Rule:    "parse",
				File:    rel,
				Message: err.Error(),
			})
			return nil
		}

		files = append(files, parsedGoFile{
			rel:     rel,
			dir:     path.Dir(rel),
			file:    file,
			imports: importNames(file),
		})
		return nil
	})
	if walkErr != nil {
		step := verifyStepFailed("layers")
		step.Detail = walkErr.Error()
		return step
	}

	index := buildBoundaryIndex(module, files)
	queriesImport := module + "/domains/internal/queries"
	for _, parsed := range files {
		violations = append(violations, fileBoundaryViolations(fset, parsed, queriesImport)...)
		violations = append(violations, txCallbackViolations(fset, parsed, index)...)
	}

	if len(violations) == 0 {
		return verifyStepOK("layers")
	}
	step := verifyStepFailed("layers")
	step.Violations = violations
	return step
}

// importNames maps each import's local package name to its path.
func importNames(file *ast.File) map[string]string {
	names := make(map[string]string, len(file.Imports))
	for _, imported := range file.Imports {
		value := strings.Trim(imported.Path.Value, `"`)
		name := path.Base(value)
		if imported.Name != nil {
			name = imported.Name.Name
		}
		names[name] = value
	}
	return names
}

// localName returns the name file uses for importPath, or "" when the file
// does not import it (or imports it blank or dot).
func localName(imports map[string]string, importPath string) string {
	for name, value := range imports {
		if value == importPath && name != "_" && name != "." {
			return name
		}
	}
	return ""
}

func buildBoundaryIndex(module string, files []parsedGoFile) boundaryIndex {
	index := boundaryIndex{
		module:       module,
		domainsInTx:  make(map[string]bool),
		localInTx:    make(map[string]map[string]bool),
		structFields: make(map[string]map[string]map[string]fieldDecl),
	}

	for _, parsed := range files {
		if strings.HasSuffix(parsed.rel, "_test.go") {
			continue
		}
		for _, decl := range parsed.file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if decl.Name.Name != "InTx" || decl.Recv == nil || len(decl.Recv.List) == 0 {
					continue
				}
				typeName := receiverTypeName(decl.Recv.List[0].Type)
				if typeName == "" {
					continue
				}
				if index.localInTx[parsed.dir] == nil {
					index.localInTx[parsed.dir] = make(map[string]bool)
				}
				index.localInTx[parsed.dir][typeName] = true
				if parsed.dir == "domains" {
					index.domainsInTx[typeName] = true
				}
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					structType, ok := typeSpec.Type.(*ast.StructType)
					if !ok {
						continue
					}
					fields := make(map[string]fieldDecl)
					for _, field := range structType.Fields.List {
						for _, name := range field.Names {
							fields[name.Name] = fieldDecl{typ: field.Type, imports: parsed.imports}
						}
					}
					if index.structFields[parsed.dir] == nil {
						index.structFields[parsed.dir] = make(map[string]map[string]fieldDecl)
					}
					index.structFields[parsed.dir][typeSpec.Name.Name] = fields
				}
			}
		}
	}

	return index
}

// receiverTypeName strips pointers and type parameters from a method
// receiver type and returns the type's name.
func receiverTypeName(expr ast.Expr) string {
	for {
		switch typed := expr.(type) {
		case *ast.StarExpr:
			expr = typed.X
		case *ast.IndexExpr:
			expr = typed.X
		case *ast.IndexListExpr:
			expr = typed.X
		case *ast.Ident:
			return typed.Name
		default:
			return ""
		}
	}
}

// bindable reports whether a field declared in dir can be bound to a
// transaction, i.e. must be used through .InTx(tx) inside AtomicOps.
func (index boundaryIndex) bindable(dir string, decl fieldDecl) bool {
	expr := decl.typ
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	switch typed := expr.(type) {
	case *ast.Ident:
		return index.localInTx[dir][typed.Name]
	case *ast.SelectorExpr:
		pkg, ok := typed.X.(*ast.Ident)
		if !ok {
			return false
		}
		switch decl.imports[pkg.Name] {
		case andurelStorageImport:
			return bindableStorageTypes[typed.Sel.Name]
		case index.module + "/domains":
			return index.domainsInTx[typed.Sel.Name]
		case index.module + "/domains/internal/queries":
			return true
		}
	}
	return false
}

func inPersistenceDir(rel string) bool {
	for _, dir := range persistenceDirs {
		if strings.HasPrefix(rel, dir) {
			return true
		}
	}
	return false
}

func fileBoundaryViolations(
	fset *token.FileSet,
	parsed parsedGoFile,
	queriesImport string,
) []verifyViolation {
	violations := make([]verifyViolation, 0)
	rel := parsed.rel
	inDomains := strings.HasPrefix(rel, "domains/")
	allowed := inPersistenceDir(rel)

	for _, imported := range parsed.file.Imports {
		value := strings.Trim(imported.Path.Value, `"`)
		line := fset.Position(imported.Pos()).Line

		if value == queriesImport && !inDomains {
			violations = append(violations, verifyViolation{
				Rule:    "queries-import",
				File:    rel,
				Line:    line,
				Message: fmt.Sprintf("imports %s; only domains/** may import it", queriesImport),
				Fix:     "call a method on a domains receiver instead of generated queries",
			})
		}
		if !allowed && isPgxImport(value) {
			violations = append(violations, verifyViolation{
				Rule:    "db-handle",
				File:    rel,
				Line:    line,
				Message: fmt.Sprintf("imports %s; only domains/** talks to the database", value),
				Fix:     "move the database access into a domains method and call it on a receiver",
			})
		}
	}

	storageName := localName(parsed.imports, andurelStorageImport)
	if storageName == "" || (allowed && inDomains) {
		return violations
	}

	ast.Inspect(parsed.file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if !ok || ident.Name != storageName {
			return true
		}
		switch {
		case !allowed && forbiddenStorageIdents[selector.Sel.Name]:
			violations = append(violations, verifyViolation{
				Rule:    "db-handle",
				File:    rel,
				Line:    fset.Position(selector.Pos()).Line,
				Message: fmt.Sprintf("references storage.%s; only domains/** holds the database handle", selector.Sel.Name),
				Fix:     "take domain receivers and a storage.Transactor, and run multi-step writes in storage.AtomicOps",
			})
		case !inDomains && selector.Sel.Name == "Unwrap":
			violations = append(violations, verifyViolation{
				Rule:    "tx-unwrap",
				File:    rel,
				Line:    fset.Position(selector.Pos()).Line,
				Message: "calls storage.Unwrap; only domains/** may unwrap a storage.Tx",
				Fix:     "bind a domain receiver with receiver.InTx(tx) instead of unwrapping the transaction",
			})
		}
		return true
	})

	return violations
}

func isPgxImport(importPath string) bool {
	if importPath != "github.com/jackc/pgx/v5" && !strings.HasPrefix(importPath, "github.com/jackc/pgx/v5/") {
		return false
	}
	return importPath != "github.com/jackc/pgx/v5/pgtype"
}

// txCallbackViolations applies tx-escape and tx-public-call to every
// storage.AtomicOps callback inside a method of parsed.
func txCallbackViolations(fset *token.FileSet, parsed parsedGoFile, index boundaryIndex) []verifyViolation {
	storageName := localName(parsed.imports, andurelStorageImport)
	if storageName == "" {
		return nil
	}

	violations := make([]verifyViolation, 0)
	for _, decl := range parsed.file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Recv == nil || len(fn.Recv.List) == 0 {
			continue
		}
		names := fn.Recv.List[0].Names
		if len(names) == 0 || names[0].Name == "_" {
			continue
		}
		receiver := names[0].Name
		fields := index.structFields[parsed.dir][receiverTypeName(fn.Recv.List[0].Type)]

		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || !isAtomicOpsCall(call, storageName) || len(call.Args) == 0 {
				return true
			}
			callback, ok := call.Args[len(call.Args)-1].(*ast.FuncLit)
			if !ok {
				return true
			}
			violations = append(violations, callbackViolations(fset, parsed, index, receiver, fields, callback)...)
			return true
		})
	}
	return violations
}

func callbackViolations(
	fset *token.FileSet,
	parsed parsedGoFile,
	index boundaryIndex,
	receiver string,
	fields map[string]fieldDecl,
	callback *ast.FuncLit,
) []verifyViolation {
	// <recv>.<field> selectors that are immediately bound with .InTx(...).
	boundUses := make(map[*ast.SelectorExpr]bool)
	// <recv>.<Method> selectors that are called.
	calledMethods := make(map[*ast.SelectorExpr]bool)
	ast.Inspect(callback.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		fun, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if inner, ok := fun.X.(*ast.SelectorExpr); ok && fun.Sel.Name == "InTx" {
			boundUses[inner] = true
		}
		if root, ok := fun.X.(*ast.Ident); ok && root.Name == receiver {
			calledMethods[fun] = true
		}
		return true
	})

	violations := make([]verifyViolation, 0)
	ast.Inspect(callback.Body, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		root, ok := selector.X.(*ast.Ident)
		if !ok || root.Name != receiver {
			return true
		}
		name := selector.Sel.Name
		line := fset.Position(selector.Pos()).Line

		if decl, isField := fields[name]; isField {
			if !boundUses[selector] && index.bindable(parsed.dir, decl) {
				violations = append(violations, verifyViolation{
					Rule: "tx-escape",
					File: parsed.rel,
					Line: line,
					Message: fmt.Sprintf(
						"%s.%s inside a storage.AtomicOps callback runs outside the transaction",
						receiver,
						name,
					),
					Fix: fmt.Sprintf(
						"bind it at the top of the callback (%s := %s.%s.InTx(tx)) and use the bound value",
						name,
						receiver,
						name,
					),
				})
			}
			return true
		}

		if calledMethods[selector] && name != "InTx" && isExported(name) {
			violations = append(violations, verifyViolation{
				Rule: "tx-public-call",
				File: parsed.rel,
				Line: line,
				Message: fmt.Sprintf(
					"%s.%s inside a storage.AtomicOps callback opens its own, independent transaction",
					receiver,
					name,
				),
				Fix: "move the shared logic into an unexported helper that takes bound dependencies as parameters",
			})
		}
		return true
	})
	return violations
}

// isAtomicOpsCall reports whether call is <storage>.AtomicOps(...), including
// an explicit generic instantiation.
func isAtomicOpsCall(call *ast.CallExpr, storageName string) bool {
	fun := call.Fun
	switch indexed := fun.(type) {
	case *ast.IndexExpr:
		fun = indexed.X
	case *ast.IndexListExpr:
		fun = indexed.X
	}
	selector, ok := fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "AtomicOps" {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Name == storageName
}

func isExported(name string) bool {
	for _, r := range name {
		return unicode.IsUpper(r)
	}
	return false
}
