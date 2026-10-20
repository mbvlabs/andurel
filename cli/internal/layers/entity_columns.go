package layers

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"maps"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/mbvlabs/andurel/v2/cli/internal/columns"
	"golang.org/x/tools/go/analysis"
)

// NarsilcImportPath is the runtime package whose Scan tag rules this check
// mirrors. The CLI does not import it; the generated query package's type
// information is enough to identify Transformer implementations by identity.
const NarsilcImportPath = "github.com/mbvlabs/narsilc"

type taggedField struct {
	field  *types.Var
	column string
}

type queryColumnMapping struct {
	query       string
	row         *types.Named
	columns     []string
	transformer bool
}

func (m queryColumnMapping) key() string {
	parts := []string{m.query, namedIdentity(m.row), strings.Join(m.columns, "\x00")}
	if m.transformer {
		parts = append(parts, "transformer")
	}
	return strings.Join(parts, "\x01")
}

type columnEntityState struct {
	entity         *types.Named
	mappings       map[string]queryColumnMapping
	hasAutomatic   bool
	hasTransformer bool
}

func entityColumnsAnalyzer(cfg Config) *analysis.Analyzer {
	return newAnalyzer("entity-columns",
		"every automatic generated query column maps to an exported andurel tag",
		func(pass *analysis.Pass) error {
			if cfg.Project == nil || pass.Pkg == nil {
				return nil
			}
			pkg := &Package{
				Path:       pass.Pkg.Path(),
				Types:      pass.Pkg,
				TypesInfo:  pass.TypesInfo,
				TypesSizes: pass.TypesSizes,
			}
			states, err := packageColumnStates(cfg.Project, pkg)
			if err != nil {
				return err
			}
			names := make([]string, 0, len(states))
			for name := range states {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				for _, mismatch := range states[name].status(cfg.Project).Mismatches {
					if mismatch.Kind != columns.KindMissingColumn &&
						mismatch.Kind != columns.KindDuplicateTag {
						continue
					}
					pass.Report(analysis.Diagnostic{
						Pos:     positionFor(cfg.Project, mismatch.File, mismatch.Line),
						Message: mismatch.Message,
					})
				}
			}
			return nil
		})
}

// InspectEntityColumns reports automatic query mappings for domain entities.
// It loads the project once so an entity with no applicable query mapping is
// reported as unchecked rather than guessed from source alone.
func InspectEntityColumns(rootDir string) (map[string]columns.EntityStatus, error) {
	project, err := Load(rootDir)
	if err != nil {
		return nil, fmt.Errorf("load project for entity-column check: %w", err)
	}
	states, err := projectColumnStates(project)
	if err != nil {
		return nil, err
	}
	statuses := make(map[string]columns.EntityStatus, len(states))
	for name, state := range states {
		statuses[name] = state.status(project)
	}
	return statuses, nil
}

func projectColumnStates(project *Project) (map[string]*columnEntityState, error) {
	states := make(map[string]*columnEntityState)
	for _, pkg := range project.Packages {
		found, err := packageColumnStates(project, pkg)
		if err != nil {
			return nil, err
		}
		mergeColumnStates(states, found)
	}
	return states, nil
}

func mergeColumnStates(dst, src map[string]*columnEntityState) {
	for name, incoming := range src {
		current := dst[name]
		if current == nil {
			current = &columnEntityState{
				entity:   incoming.entity,
				mappings: make(map[string]queryColumnMapping, len(incoming.mappings)),
			}
			dst[name] = current
		}
		current.hasAutomatic = current.hasAutomatic || incoming.hasAutomatic
		current.hasTransformer = current.hasTransformer || incoming.hasTransformer
		maps.Copy(current.mappings, incoming.mappings)
	}
}

func packageColumnStates(project *Project, pkg *Package) (map[string]*columnEntityState, error) {
	states := make(map[string]*columnEntityState)
	if pkg == nil || pkg.TypesInfo == nil {
		return states, nil
	}

	queryPackage := project.Module + "/domains/internal/queries"
	for ident, instance := range pkg.TypesInfo.Instances {
		function, ok := pkg.TypesInfo.Uses[ident].(*types.Func)
		if !ok {
			continue
		}
		origin := function.Origin()
		if origin == nil || origin.Pkg() == nil || origin.Pkg().Path() != queryPackage {
			continue
		}
		signature, ok := origin.Type().(*types.Signature)
		if !ok || signature.TypeParams().Len() == 0 {
			continue
		}
		if instance.TypeArgs == nil || instance.TypeArgs.Len() != 1 {
			continue
		}
		entity := domainEntityType(instance.TypeArgs.At(0), project.Module)
		if entity == nil {
			continue
		}

		mapping, err := project.queryColumnMapping(origin, instance, entity)
		if err != nil {
			return nil, fmt.Errorf(
				"analyze query %s for domain entity %s: %w",
				origin.Name(),
				entity.Obj().Name(),
				err,
			)
		}
		name := entity.Obj().Name()
		state := states[name]
		if state == nil {
			state = &columnEntityState{
				entity:   entity,
				mappings: make(map[string]queryColumnMapping),
			}
			states[name] = state
		}
		key := mapping.key()
		if _, exists := state.mappings[key]; exists {
			continue
		}
		state.mappings[key] = mapping
		if mapping.transformer {
			state.hasTransformer = true
		} else {
			state.hasAutomatic = true
		}
	}
	return states, nil
}

func domainEntityType(t types.Type, module string) *types.Named {
	t = types.Unalias(t)
	named, ok := t.(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return nil
	}
	if named.Obj().Pkg().Path() != module+"/domains" {
		return nil
	}
	if _, ok := named.Underlying().(*types.Struct); !ok {
		return nil
	}
	return named
}

func (p *Project) queryColumnMapping(
	origin *types.Func,
	instance types.Instance,
	entity *types.Named,
) (queryColumnMapping, error) {
	signature, ok := instance.Type.(*types.Signature)
	if !ok || signature.Results().Len() == 0 {
		return queryColumnMapping{}, fmt.Errorf("instantiated method has no result")
	}

	if builder := queryBuilderType(signature.Results().At(0).Type(), origin.Pkg()); builder != nil {
		all, allPkg, err := p.builderAllMethod(origin.Pkg(), builder)
		if err != nil {
			return queryColumnMapping{}, err
		}
		return p.mappingFromBody(origin, all, allPkg, entity)
	}

	decl, pkg, err := p.queryFuncDecl(origin)
	if err != nil {
		return queryColumnMapping{}, err
	}
	return p.mappingFromBody(origin, decl, pkg, entity)
}

func queryBuilderType(t types.Type, queryPkg *types.Package) *types.Named {
	t = types.Unalias(t)
	if pointer, ok := t.(*types.Pointer); ok {
		t = types.Unalias(pointer.Elem())
	}
	named, ok := t.(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return nil
	}
	if named.Obj().Pkg() != queryPkg {
		return nil
	}
	if types.NewMethodSet(types.NewPointer(named)).Lookup(queryPkg, "All") == nil {
		return nil
	}
	return named
}

func (p *Project) builderAllMethod(
	queryPkg *types.Package,
	builder *types.Named,
) (*ast.FuncDecl, *Package, error) {
	selection := types.NewMethodSet(types.NewPointer(builder)).Lookup(queryPkg, "All")
	if selection == nil {
		return nil, nil, fmt.Errorf("builder %s has no All method", builder.Obj().Name())
	}
	all, ok := selection.Obj().(*types.Func)
	if !ok {
		return nil, nil, fmt.Errorf("builder %s All is not a function", builder.Obj().Name())
	}
	return p.queryFuncDecl(all.Origin())
}

func (p *Project) queryFuncDecl(function *types.Func) (*ast.FuncDecl, *Package, error) {
	if decl, pkg, ok := p.funcDeclAt(function.Pos()); ok {
		return decl, pkg, nil
	}
	// Test variants parse the same source with new token positions. Fall back
	// to the variant whose type package is the function's own package.
	for _, pkg := range p.Packages {
		if pkg.Types != function.Pkg() {
			continue
		}
		for _, file := range pkg.Syntax {
			for _, item := range file.Decls {
				decl, ok := item.(*ast.FuncDecl)
				if ok && decl.Name.Name == function.Name() {
					return decl, pkg, nil
				}
			}
		}
	}
	return nil, nil, fmt.Errorf(
		"cannot find generated body for %s; run 'andurel sync queries'",
		function.Name(),
	)
}

func (p *Project) mappingFromBody(
	origin *types.Func,
	decl *ast.FuncDecl,
	pkg *Package,
	entity *types.Named,
) (queryColumnMapping, error) {
	found, err := queryBodyMapping(pkg.TypesInfo, decl.Body, origin.Pkg().Path())
	if err != nil {
		return queryColumnMapping{}, fmt.Errorf("%s: %w", origin.Name(), err)
	}
	return queryColumnMapping{
		query:       origin.Name(),
		row:         found.row,
		columns:     found.columns,
		transformer: implementsNarsilcTransformer(origin.Pkg(), found.row, entity),
	}, nil
}

type bodyColumnMapping struct {
	row     *types.Named
	columns []string
}

func queryBodyMapping(
	info *types.Info,
	body *ast.BlockStmt,
	queryPackage string,
) (bodyColumnMapping, error) {
	if body == nil {
		return bodyColumnMapping{}, fmt.Errorf("generated method has no body")
	}

	found := make(map[string]bodyColumnMapping)
	unresolved := make([]string, 0)
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if !isNarsilcCall(info, call, "Read") && !isNarsilcCall(info, call, "Scan") {
			return true
		}
		indices := callTypeArgs(call.Fun)
		if len(indices) < 1 || len(call.Args) < 2 {
			return true
		}
		row := rowTypeArgument(info, indices, queryPackage)
		if row == nil {
			return true
		}
		columnList, err := columns.ResolveStringSlice(body, call.Args[1])
		if err != nil {
			unresolved = append(unresolved, err.Error())
			return true
		}
		mapping := bodyColumnMapping{row: row, columns: columnList}
		found[mapping.key()] = mapping
		return true
	})

	if len(found) == 0 {
		if len(unresolved) > 0 {
			return bodyColumnMapping{}, fmt.Errorf(
				"cannot read generated query columns: %s",
				strings.Join(unresolved, "; "),
			)
		}
		return bodyColumnMapping{}, fmt.Errorf(
			"does not contain a narsilc.Read or narsilc.Scan mapping; run 'andurel sync queries'",
		)
	}
	if len(found) > 1 {
		return bodyColumnMapping{}, fmt.Errorf("has more than one generated column mapping")
	}
	for _, mapping := range found {
		return mapping, nil
	}
	return bodyColumnMapping{}, fmt.Errorf("has no generated column mapping")
}

func (m bodyColumnMapping) key() string {
	return namedIdentity(m.row) + "\x01" + strings.Join(m.columns, "\x00")
}

func isNarsilcCall(info *types.Info, call *ast.CallExpr, name string) bool {
	object := selectedObject(info, call.Fun)
	function, ok := object.(*types.Func)
	return ok && function.Pkg() != nil &&
		function.Pkg().Path() == NarsilcImportPath &&
		function.Name() == name
}

func callTypeArgs(expr ast.Expr) []ast.Expr {
	switch typed := unparen(expr).(type) {
	case *ast.IndexExpr:
		return []ast.Expr{typed.Index}
	case *ast.IndexListExpr:
		return typed.Indices
	default:
		return nil
	}
}

func rowTypeArgument(info *types.Info, args []ast.Expr, queryPackage string) *types.Named {
	for _, arg := range args {
		t := info.TypeOf(arg)
		if t == nil {
			continue
		}
		t = types.Unalias(t)
		if _, ok := t.(*types.TypeParam); ok {
			continue
		}
		named, ok := t.(*types.Named)
		if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
			continue
		}
		if named.Obj().Pkg().Path() != queryPackage {
			continue
		}
		if _, ok := named.Underlying().(*types.Struct); !ok {
			continue
		}
		return named
	}
	return nil
}

func implementsNarsilcTransformer(
	queryPkg *types.Package,
	row *types.Named,
	entity *types.Named,
) bool {
	if queryPkg == nil || row == nil || entity == nil {
		return false
	}
	var narsilc *types.Package
	for _, imported := range queryPkg.Imports() {
		if imported.Path() == NarsilcImportPath {
			narsilc = imported
			break
		}
	}
	if narsilc == nil {
		return false
	}
	object, ok := narsilc.Scope().Lookup("Transformer").(*types.TypeName)
	if !ok {
		return false
	}
	transformer, ok := object.Type().(*types.Named)
	if !ok {
		return false
	}
	instantiated, err := types.Instantiate(nil, transformer, []types.Type{row}, true)
	if err != nil {
		return false
	}
	contract, ok := instantiated.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	return types.Implements(types.NewPointer(entity), contract)
}

func (s *columnEntityState) status(project *Project) columns.EntityStatus {
	status := columns.EntityStatus{
		Mismatches:  []columns.Mismatch{},
		Unpopulated: []columns.TagNote{},
	}
	if !s.hasAutomatic {
		return status
	}
	status.Checked = true

	structType := s.entity.Underlying().(*types.Struct)
	tagged := taggedFields(structType)
	byColumn := make(map[string][]taggedField, len(tagged))
	for _, field := range tagged {
		byColumn[field.column] = append(byColumn[field.column], field)
	}

	for _, duplicate := range duplicateTaggedFields(tagged) {
		file, line := project.fieldPosition(duplicate.field)
		message := fmt.Sprintf(
			"duplicate andurel:%q tag on %s: %s and %s; narsilc rejects duplicate exported tags",
			duplicate.column,
			s.entity.Obj().Name(),
			byColumn[duplicate.column][0].field.Name(),
			duplicate.field.Name(),
		)
		status.Mismatches = append(status.Mismatches, columns.Mismatch{
			Kind:    columns.KindDuplicateTag,
			Column:  duplicate.column,
			Field:   duplicate.field.Name(),
			File:    file,
			Line:    line,
			Message: message,
		})
	}

	missing := make(map[string][]string)
	missingFields := make(map[string]*types.Var)
	returned := make(map[string]bool)
	for _, mapping := range s.sortedMappings() {
		if mapping.transformer {
			continue
		}
		for _, column := range mapping.columns {
			returned[column] = true
			if len(byColumn[column]) > 0 {
				status.Mismatches = appendTypeDifferences(
					status.Mismatches,
					project,
					s.entity,
					mapping,
					column,
					byColumn[column][0],
				)
				continue
			}
			missing[column] = appendUnique(missing[column], mapping.query)
			if missingFields[column] == nil {
				missingFields[column] = missingColumnField(structType, mapping.row, column)
			}
		}
	}

	missingColumns := make([]string, 0, len(missing))
	for column := range missing {
		missingColumns = append(missingColumns, column)
	}
	sort.Strings(missingColumns)
	for _, column := range missingColumns {
		field := missingFields[column]
		file, line := project.typeOrFieldPosition(s.entity, field)
		status.Mismatches = append(status.Mismatches, columns.Mismatch{
			Kind:    columns.KindMissingColumn,
			Column:  column,
			Field:   fieldName(field),
			File:    file,
			Line:    line,
			Message: missingColumnMessage(s.entity.Obj().Name(), column, field, missing[column]),
		})
	}

	if !s.hasTransformer {
		for _, field := range tagged {
			if returned[field.column] {
				continue
			}
			file, line := project.fieldPosition(field.field)
			status.Unpopulated = append(status.Unpopulated, columns.TagNote{
				Column: field.column,
				Field:  field.field.Name(),
				File:   file,
				Line:   line,
			})
		}
	}
	sort.SliceStable(status.Mismatches, func(i, j int) bool {
		left, right := status.Mismatches[i], status.Mismatches[j]
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		if left.Column != right.Column {
			return left.Column < right.Column
		}
		return left.Field < right.Field
	})
	sort.SliceStable(status.Unpopulated, func(i, j int) bool {
		if status.Unpopulated[i].Column != status.Unpopulated[j].Column {
			return status.Unpopulated[i].Column < status.Unpopulated[j].Column
		}
		return status.Unpopulated[i].Field < status.Unpopulated[j].Field
	})
	return status
}

func (s *columnEntityState) sortedMappings() []queryColumnMapping {
	mappings := make([]queryColumnMapping, 0, len(s.mappings))
	for _, mapping := range s.mappings {
		mappings = append(mappings, mapping)
	}
	sort.SliceStable(mappings, func(i, j int) bool {
		if mappings[i].query != mappings[j].query {
			return mappings[i].query < mappings[j].query
		}
		if len(mappings[i].columns) != len(mappings[j].columns) {
			return len(mappings[i].columns) < len(mappings[j].columns)
		}
		return strings.Join(mappings[i].columns, "\x00") <
			strings.Join(mappings[j].columns, "\x00")
	})
	return mappings
}

func appendTypeDifferences(
	mismatches []columns.Mismatch,
	project *Project,
	entity *types.Named,
	mapping queryColumnMapping,
	column string,
	field taggedField,
) []columns.Mismatch {
	rowField := taggedFieldByColumn(mapping.row, column)
	if rowField == nil {
		return mismatches
	}
	if types.Identical(rowField.Type(), field.field.Type()) {
		return mismatches
	}
	expected := project.typeString(rowField.Type())
	actual := project.typeString(field.field.Type())
	for _, existing := range mismatches {
		if existing.Kind == columns.KindTypeDifference &&
			existing.Column == column &&
			existing.Field == field.field.Name() &&
			existing.ExpectedType == expected &&
			existing.ActualType == actual {
			return mismatches
		}
	}
	file, line := project.fieldPosition(field.field)
	message := fmt.Sprintf(
		"note: column %q is %s on %s and %s on %s; pgx may convert between them",
		column,
		expected,
		mapping.row.Obj().Name(),
		actual,
		entity.Obj().Name(),
	)
	return append(mismatches, columns.Mismatch{
		Kind:         columns.KindTypeDifference,
		Column:       column,
		Field:        field.field.Name(),
		File:         file,
		Line:         line,
		Message:      message,
		ExpectedType: expected,
		ActualType:   actual,
	})
}

func taggedFields(structType *types.Struct) []taggedField {
	fields := make([]taggedField, 0, structType.NumFields())
	for i := range structType.NumFields() {
		field := structType.Field(i)
		if !field.Exported() {
			continue
		}
		column := columns.AndurelTag(structType.Tag(i))
		if column == "" {
			continue
		}
		fields = append(fields, taggedField{field: field, column: column})
	}
	return fields
}

func duplicateTaggedFields(fields []taggedField) []taggedField {
	seen := make(map[string]bool, len(fields))
	duplicates := make([]taggedField, 0)
	for _, field := range fields {
		if seen[field.column] {
			duplicates = append(duplicates, field)
			continue
		}
		seen[field.column] = true
	}
	return duplicates
}

func taggedFieldByColumn(named *types.Named, column string) *types.Var {
	if named == nil {
		return nil
	}
	structType, ok := named.Underlying().(*types.Struct)
	if !ok {
		return nil
	}
	for i := range structType.NumFields() {
		field := structType.Field(i)
		if !field.Exported() {
			continue
		}
		if columns.AndurelTag(structType.Tag(i)) == column {
			return field
		}
	}
	return nil
}

func missingColumnField(
	structType *types.Struct,
	row *types.Named,
	column string,
) *types.Var {
	rowField := taggedFieldByColumn(row, column)
	if rowField == nil {
		return nil
	}
	// The tag drifted, but the Row field name still points at the entity field
	// a human should fix. Fall back to the entity type when it is gone.
	for field := range structType.Fields() {
		if field.Name() == rowField.Name() {
			return field
		}
	}
	return nil
}

func fieldName(field *types.Var) string {
	if field == nil {
		return ""
	}
	return field.Name()
}

func missingColumnMessage(
	entity string,
	column string,
	field *types.Var,
	queries []string,
) string {
	message := fmt.Sprintf(
		"query column %q has no exported field with andurel:%q on %s",
		column,
		column,
		entity,
	)
	if field != nil {
		message += fmt.Sprintf("; field %s is not mapped to it", field.Name())
	}
	message += " (returned by " + strings.Join(queries, ", ") + ")"
	return message
}

func appendUnique(values []string, value string) []string {
	if slices.Contains(values, value) {
		return values
	}
	return append(values, value)
}

func (p *Project) fieldPosition(field *types.Var) (string, int) {
	if field == nil {
		return "", 0
	}
	return p.positionAt(field.Pos())
}

func (p *Project) typeOrFieldPosition(named *types.Named, field *types.Var) (string, int) {
	if field != nil {
		return p.fieldPosition(field)
	}
	if named == nil || named.Obj() == nil {
		return "", 0
	}
	return p.positionAt(named.Obj().Pos())
}

func (p *Project) positionAt(pos token.Pos) (string, int) {
	if !pos.IsValid() {
		return "", 0
	}
	position := p.Fset.PositionFor(pos, true)
	return relativeFile(p.RootDir, position.Filename), position.Line
}

func (p *Project) typeString(t types.Type) string {
	return types.TypeString(t, func(pkg *types.Package) string {
		if pkg == nil {
			return ""
		}
		return pkg.Name()
	})
}

func namedIdentity(named *types.Named) string {
	if named == nil || named.Obj() == nil {
		return ""
	}
	if named.Obj().Pkg() == nil {
		return named.Obj().Name()
	}
	return named.Obj().Pkg().Path() + "." + named.Obj().Name()
}

func positionFor(project *Project, file string, line int) token.Pos {
	if project == nil || file == "" || line <= 0 {
		return token.NoPos
	}
	absolute := filepath.Join(project.RootDir, filepath.FromSlash(file))
	for _, pkg := range project.Packages {
		for _, source := range pkg.Syntax {
			position := project.Fset.PositionFor(source.Pos(), false)
			if !sameFile(project.RootDir, position.Filename, absolute) {
				continue
			}
			tokenFile := project.Fset.File(source.Pos())
			if tokenFile == nil || line > tokenFile.LineCount() {
				continue
			}
			return tokenFile.LineStart(line)
		}
	}
	return token.NoPos
}

func sameFile(rootDir, filename, absolute string) bool {
	if filename == "" {
		return false
	}
	if !filepath.IsAbs(filename) {
		filename = filepath.Join(rootDir, filename)
	}
	return filepath.Clean(filename) == filepath.Clean(absolute)
}
