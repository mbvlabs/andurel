package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/mbvlabs/andurel/v2/cli/patterns"
)

// domainField is one struct field mapped to a column through its andurel tag.
type domainField struct {
	Name   string `json:"name"`
	Column string `json:"column"`
	Type   string `json:"type"`
}

// domainEntity is one struct in a domain file: its table, its mapped fields,
// whether it has a Validate method, and whether its receiver (the plural
// type, e.g. Users for User) is constructed in internal/runtime.
type domainEntity struct {
	Name      string        `json:"name"`
	Table     string        `json:"table,omitempty"`
	Fields    []domainField `json:"fields"`
	Validates bool          `json:"validates"`
	Receiver  string        `json:"receiver,omitempty"`
	Wired     bool          `json:"wired"`
}

// runtimeConstructedReceivers parses the non-test Go files in runtimeDir and
// returns the receivers constructed there: X for every domains.NewX(...)
// call, respecting the import alias of <module>/domains. A missing directory
// or unparsable file contributes nothing.
func runtimeConstructedReceivers(runtimeDir, module string) map[string]bool {
	wired := map[string]bool{}
	entries, err := os.ReadDir(runtimeDir)
	if err != nil {
		return wired
	}
	domainsImport := module + "/domains"
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(
			token.NewFileSet(),
			filepath.Join(runtimeDir, name),
			nil,
			parser.SkipObjectResolution,
		)
		if err != nil {
			continue
		}
		domainsName := localName(importNames(file), domainsImport)
		if domainsName == "" {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok || pkg.Name != domainsName {
				return true
			}
			if receiver, ok := strings.CutPrefix(selector.Sel.Name, "New"); ok && receiver != "" {
				wired[receiver] = true
			}
			return true
		})
	}
	return wired
}

// describeDomainFile parses one domains .go file and reports the type→column
// mapping an agent needs: which structs map which fields to which columns,
// the table from the `// andurel:table <name>` annotation, whether each
// entity has a Validate method, and whether its receiver is constructed in
// internal/runtime (wired).
func describeDomainFile(path string, wired map[string]bool) []domainEntity {
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

	validates := map[string]bool{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 || fn.Name.Name != "Validate" {
			continue
		}
		recvType := fn.Recv.List[0].Type
		if star, ok := recvType.(*ast.StarExpr); ok {
			recvType = star.X
		}
		if ident, ok := recvType.(*ast.Ident); ok {
			validates[ident.Name] = true
		}
	}

	declared := map[string]bool{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			if typeSpec, ok := spec.(*ast.TypeSpec); ok {
				declared[typeSpec.Name.Name] = true
			}
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

			name := typeSpec.Name.Name
			receiver := patterns.Pluralize(name)
			if !declared[receiver] {
				receiver = ""
			}
			entities = append(entities, domainEntity{
				Name:      name,
				Table:     table,
				Fields:    fields,
				Validates: validates[name],
				Receiver:  receiver,
				Wired:     receiver != "" && wired[receiver],
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
