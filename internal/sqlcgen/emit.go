package sqlcgen

import (
	"fmt"
	"go/format"
	"sort"
	"strings"

	"github.com/sqlc-dev/plugin-sdk-go/plugin"

	"github.com/mbvlabs/andurel/pkg/naming"
)

// emitModelFile renders one Go file containing the model's base struct,
// public Var, and one method per query in the group.
//
// Returned File.Name is relative to the codegen.out directory in sqlc.yaml
// (i.e. "<snake>_gen.go").
func emitModelFile(cfg *Config, group modelGroup) (*plugin.File, error) {
	model := group.Model
	if model == "" {
		return nil, fmt.Errorf("empty model name")
	}

	view := fileView{
		Package:        cfg.Package,
		Model:          model,
		BaseType:       baseTypeName(model),
		EntityType:     model + "Entity",
		RowMapper:      rowMapperName(model),
		DBImport:       cfg.DBPackageImport,
		StorageImport:  cfg.StoragePackageImport,
		FileNameSuffix: "_gen.go",
	}

	imports := importSet{}
	imports.add(goImport{Path: "context"})
	imports.add(goImport{Path: cfg.StoragePackageImport})
	imports.add(goImport{Path: cfg.DBPackageImport})

	for _, q := range group.Queries {
		method, err := buildMethod(model, q)
		if err != nil {
			return nil, fmt.Errorf("query %q: %w", q.GetName(), err)
		}
		for _, imp := range method.Imports {
			imports.add(imp)
		}
		view.Methods = append(view.Methods, method)
	}

	view.Imports = imports.sorted()

	src, err := renderFile(view)
	if err != nil {
		return nil, err
	}
	formatted, err := format.Source(src)
	if err != nil {
		// Surface the unformatted source on error so the user can debug what
		// the template emitted (otherwise gofmt's error is opaque).
		return nil, fmt.Errorf("gofmt: %w\n--- source ---\n%s", err, src)
	}

	return &plugin.File{
		Name:     naming.ToSnakeCase(model) + view.FileNameSuffix,
		Contents: formatted,
	}, nil
}

// methodView is the per-query data used by the template to emit one method
// plus its raw helper.
type methodView struct {
	// Public method on the base struct, e.g. "Find", "All", "Insert".
	MethodName string
	// Underlying sqlc-generated function name on db.Queries, e.g. "QueryServerByID".
	RawName string
	// Op string, e.g. "OpFind". Must match a constant in the storage package.
	Op string
	// Cmd is the raw sqlc command, e.g. ":one", ":many", ":exec", ":execrows".
	Cmd string
	// Params are the public method parameters (excluding ctx + exec).
	Params []paramView
	// CallArgs is what to pass through to the raw db.Queries call after
	// (ctx, exec). Almost always the same names as Params, but kept separate
	// so we can reshape the call (e.g. "params" struct field references) later.
	CallArgs []string
	// ReturnEntity is "" for :exec, "<EntityType>" for :one, "[]<EntityType>" for :many,
	// "int64" for :execrows.
	ReturnEntity string
	// HasRowMapping is true when ReturnEntity wraps a db row that needs to
	// run through rowToXEntity.
	HasRowMapping bool
	// IsMany is true when the method returns a slice (drives a loop in the template).
	IsMany bool
	// IsExec is true for :exec (no value, just error).
	IsExec bool
	// IsExecRows is true for :execrows (returns int64 affected).
	IsExecRows bool
	// Imports requested by this method's parameter types.
	Imports []goImport
}

type paramView struct {
	Name string // identifier name as it appears in the public signature
	Type string // Go type expression
}

// buildMethod assembles a methodView from a single sqlc Query.
func buildMethod(model string, q *plugin.Query) (methodView, error) {
	op := deriveOp(q.GetName())
	methodName := deriveMethodName(q.GetName(), model, q.GetCmd())

	mv := methodView{
		MethodName: methodName,
		RawName:    q.GetName(),
		Op:         string(op),
		Cmd:        q.GetCmd(),
	}

	imports := importSet{}

	params, callArgs, paramImports, err := buildParamSignature(q)
	if err != nil {
		return methodView{}, fmt.Errorf("params: %w", err)
	}
	mv.Params = params
	mv.CallArgs = callArgs
	for _, imp := range paramImports {
		imports.add(imp)
	}

	switch q.GetCmd() {
	case ":one":
		mv.ReturnEntity = model + "Entity"
		mv.HasRowMapping = true
	case ":many":
		mv.ReturnEntity = "[]" + model + "Entity"
		mv.HasRowMapping = true
		mv.IsMany = true
	case ":exec":
		mv.IsExec = true
	case ":execrows":
		mv.IsExecRows = true
	default:
		// :execlastid, :batchexec, :batchmany, etc. are not supported in v1.
		return methodView{}, fmt.Errorf("unsupported sqlc cmd %q", q.GetCmd())
	}

	mv.Imports = imports.list()
	return mv, nil
}

// buildParamSignature derives the public method's parameter list and the
// arguments the raw helper needs.
//
// Mirrors sqlc-gen-go's convention:
//   - 0 params  -> no extra args
//   - 1 param   -> single inline arg with the param's Go type
//   - 2+ params -> single 'params <ParamsStruct>' arg
func buildParamSignature(q *plugin.Query) ([]paramView, []string, []goImport, error) {
	params := q.GetParams()
	switch len(params) {
	case 0:
		return nil, nil, nil, nil
	case 1:
		col := params[0].GetColumn()
		gt, err := mapColumnType(col)
		if err != nil {
			return nil, nil, nil, err
		}
		ident := identifierFromColumn(col)
		return []paramView{{Name: ident, Type: gt.Expr}}, []string{ident}, gt.Imports, nil
	default:
		// sqlc-gen-go names the multi-param struct {QueryName}Params, in package db.
		structName := q.GetName() + "Params"
		return []paramView{{Name: "params", Type: "db." + structName}},
			[]string{"params"},
			nil,
			nil
	}
}

// identifierFromColumn picks a sensible Go identifier for a single-param
// query. sqlc usually names columns ("id", "user_id", ...); fall back to "arg"
// when the name is empty or not a valid identifier.
func identifierFromColumn(col *plugin.Column) string {
	name := col.GetName()
	if name == "" {
		return "arg"
	}
	camel := naming.ToLowerCamelCase(name)
	if camel == "" {
		return "arg"
	}
	return camel
}

// fileView is the top-level template data.
type fileView struct {
	Package        string
	Model          string
	BaseType       string
	EntityType     string
	RowMapper      string
	DBImport       string
	StorageImport  string
	FileNameSuffix string
	Imports        []goImport
	Methods        []methodView
}

// baseTypeName is the unexported zero-sized struct the user adds methods to
// (CanAccess, BeforeFind, etc.). e.g. "Server" -> "serverBase".
func baseTypeName(model string) string {
	return naming.ToLowerCamelCase(model) + "Base"
}

// rowMapperName is the user-defined function that maps db.{Row} -> {Model}Entity.
func rowMapperName(model string) string {
	return "rowTo" + model + "Entity"
}

// importSet deduplicates imports by path. The first alias wins.
type importSet map[string]goImport

func (s importSet) add(imp goImport) {
	if imp.Path == "" {
		return
	}
	if _, exists := s[imp.Path]; exists {
		return
	}
	s[imp.Path] = imp
}

func (s importSet) list() []goImport {
	out := make([]goImport, 0, len(s))
	for _, v := range s {
		out = append(out, v)
	}
	return out
}

func (s importSet) sorted() []goImport {
	out := s.list()
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// stringer for debug/logging.
func (mv methodView) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%s)", mv.MethodName, mv.Op)
	return b.String()
}
