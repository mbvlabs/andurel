package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
)

// domainField is one struct field mapped to a column through its andurel tag.
type domainField struct {
	Name   string `json:"name"`
	Column string `json:"column"`
	Type   string `json:"type"`
}

// domainEntity is one struct in a domain file: its mapped fields, its table,
// and whether the file provides Parse constructors (newtype coverage).
type domainEntity struct {
	Name   string        `json:"name"`
	Table  string        `json:"table,omitempty"`
	Fields []domainField `json:"fields"`
	Parsed bool          `json:"parsed"`
}

// describeDomainFile parses one domains .go file and reports the type→column
// mapping an agent needs for type-driven work: which structs map which fields
// to which columns, and whether Parse constructors exist.
func describeDomainFile(path string) []domainEntity {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return nil
	}

	table := ""
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if name, ok := strings.CutPrefix(comment.Text, "// andurel:table"); ok {
				table = strings.TrimSpace(name)
			}
		}
	}

	parsed := false
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Recv != nil {
			continue
		}
		if strings.HasPrefix(fn.Name.Name, "Parse") {
			parsed = true
			break
		}
	}

	entities := []domainEntity{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			fields := []domainField{}
			for _, field := range structType.Fields.List {
				if field.Tag == nil {
					continue
				}
				column := reflect.StructTag(strings.Trim(field.Tag.Value, "`")).Get("andurel")
				if column == "" || column == "-" {
					continue
				}
				for _, name := range field.Names {
					fields = append(fields, domainField{
						Name:   name.Name,
						Column: column,
						Type:   domainTypeString(field.Type),
					})
				}
			}
			if len(fields) == 0 {
				continue
			}

			entities = append(entities, domainEntity{
				Name:   typeSpec.Name.Name,
				Table:  table,
				Fields: fields,
				Parsed: parsed,
			})
		}
	}

	return entities
}

// domainTypeString renders a field type expression compactly.
func domainTypeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return domainTypeString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + domainTypeString(t.Elt)
	case *ast.StarExpr:
		return "*" + domainTypeString(t.X)
	default:
		return ""
	}
}

// isDomainEntityFile reports whether rel (slash-separated, relative to the
// project root) is a domain file rather than generated output or tests.
// Domains live in one flat package: one file per domain plus errors.go.
func isDomainEntityFile(rel string) bool {
	if !strings.HasPrefix(rel, "domains/") {
		return false
	}
	rest := strings.TrimPrefix(rel, "domains/")
	if strings.HasPrefix(rest, "internal/") {
		return false
	}
	return strings.HasSuffix(rest, ".go") && !strings.HasSuffix(rest, "_test.go")
}
