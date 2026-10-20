package columns

import (
	"go/ast"
	"go/types"
	"path"
	"regexp"
	"strconv"
)

// StructField is one named field of a struct declared in Go source, such as
// a narsilc <X>Row in domains/internal/queries/models.go or a domain entity.
// Type is the field type as written (pgtype.Timestamptz, []byte, *string) and
// Column is its andurel tag, empty when the field is not mapped.
type StructField struct {
	Name   string `json:"field"`
	Type   string `json:"type"`
	Column string `json:"column"`
}

// StructFields returns the named fields of the struct type called name in
// file, in declaration order. ok is false when file declares no struct type
// with that name. Embedded fields are skipped; narsilc rows have none.
func StructFields(file *ast.File, name string) (fields []StructField, ok bool) {
	for _, decl := range file.Decls {
		for _, spec := range typeSpecs(decl) {
			if spec.Name.Name != name {
				continue
			}
			structType, isStruct := spec.Type.(*ast.StructType)
			if !isStruct {
				return nil, false
			}
			fields = []StructField{}
			for _, field := range structType.Fields.List {
				tag := ""
				if field.Tag != nil {
					if raw, err := strconv.Unquote(field.Tag.Value); err == nil {
						tag = AndurelTag(raw)
					}
				}
				for _, ident := range field.Names {
					fields = append(fields, StructField{
						Name:   ident.Name,
						Type:   types.ExprString(field.Type),
						Column: tag,
					})
				}
			}
			return fields, true
		}
	}
	return nil, false
}

// ImportPaths maps the package name each import of file is referenced by to
// its import path. An unnamed import uses the last path element, skipping a
// trailing major-version element (github.com/jackc/pgx/v5 → pgx).
func ImportPaths(file *ast.File) map[string]string {
	imports := make(map[string]string, len(file.Imports))
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := ImportName(importPath)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == "_" || name == "." {
			continue
		}
		imports[name] = importPath
	}
	return imports
}

var majorVersion = regexp.MustCompile(`^v[0-9]+$`)

// ImportName is the conventional package name for importPath.
func ImportName(importPath string) string {
	name := path.Base(importPath)
	if majorVersion.MatchString(name) {
		if parent := path.Base(path.Dir(importPath)); parent != "." && parent != "/" {
			return parent
		}
	}
	return name
}

func typeSpecs(decl ast.Decl) []*ast.TypeSpec {
	gen, ok := decl.(*ast.GenDecl)
	if !ok {
		return nil
	}
	specs := make([]*ast.TypeSpec, 0, len(gen.Specs))
	for _, spec := range gen.Specs {
		if typeSpec, ok := spec.(*ast.TypeSpec); ok {
			specs = append(specs, typeSpec)
		}
	}
	return specs
}
