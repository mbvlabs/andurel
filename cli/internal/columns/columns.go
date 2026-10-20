// Package columns holds the small, shared representations behind the
// entity-column check and the read-only introspection commands.
//
// The tag and []string helpers mirror what narsilc does at runtime: the
// andurel tag is read literally (not as JSON with comma options), and a query
// column list is a string slice literal or a local variable holding one.
package columns

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"strconv"
)

// Mismatch kinds reported by the entity-column check. Missing columns and
// duplicate tags are runtime failures; a type difference is a note only.
const (
	KindMissingColumn  = "missing-column"
	KindDuplicateTag   = "duplicate-tag"
	KindTypeDifference = "type-difference"
)

// Mismatch is one column finding for a domain entity. File and Line point at
// the entity field when one exists, otherwise at the entity type. ExpectedType
// and ActualType are only populated for type-difference notes.
type Mismatch struct {
	Kind         string `json:"kind"`
	Column       string `json:"column,omitempty"`
	Field        string `json:"field,omitempty"`
	File         string `json:"file,omitempty"`
	Line         int    `json:"line,omitempty"`
	Message      string `json:"message"`
	ExpectedType string `json:"expected_type,omitempty"`
	ActualType   string `json:"actual_type,omitempty"`
}

// TagNote is a tagged entity field that no automatic query mapping populated.
// It is a note only: narsilc leaves extra tags at their zero value.
type TagNote struct {
	Column string `json:"column"`
	Field  string `json:"field"`
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
}

// EntityStatus is the automatic mapping result for one entity. Checked is
// false when no query mapping applied or every mapping is user code through a
// narsilc Transformer; callers expose that as columns_ok: null.
type EntityStatus struct {
	Checked     bool
	Mismatches  []Mismatch
	Unpopulated []TagNote
}

// AndurelTag returns the full andurel tag value, or "" when the field is not
// mapped. It intentionally matches reflect.StructTag.Get, which is what
// narsilc uses: an empty or "-" tag is ignored, and no comma-separated option
// syntax is stripped.
func AndurelTag(raw string) string {
	tag := reflect.StructTag(raw).Get("andurel")
	if tag == "" || tag == "-" {
		return ""
	}
	return tag
}

// ResolveStringSlice returns the []string literal assigned to expr. It accepts
// a composite literal directly or an identifier assigned earlier in body to
// one, which is how narsilc :many builders keep their column list.
//
// Each step follows an assignment that ends before the current use, so the
// walk always moves backwards through body and terminates.
func ResolveStringSlice(body *ast.BlockStmt, expr ast.Expr) ([]string, error) {
	expr = unparen(expr)
	if values, ok := stringSliceLiteral(expr); ok {
		return values, nil
	}

	ident, ok := expr.(*ast.Ident)
	if !ok {
		return nil, fmt.Errorf("columns are not a []string literal or local variable")
	}

	var assigned ast.Expr
	ast.Inspect(body, func(node ast.Node) bool {
		switch stmt := node.(type) {
		case *ast.AssignStmt:
			// Only statements that finish before the use count; the
			// statement containing expr (cols = cols) must not resolve to
			// itself.
			if stmt.End() > expr.Pos() {
				return true
			}
			for i, lhs := range stmt.Lhs {
				name, ok := lhs.(*ast.Ident)
				if !ok || name.Name != ident.Name {
					continue
				}
				switch {
				case len(stmt.Rhs) == len(stmt.Lhs):
					assigned = stmt.Rhs[i]
				case len(stmt.Rhs) == 1:
					assigned = stmt.Rhs[0]
				}
			}
		case *ast.ValueSpec:
			if stmt.End() > expr.Pos() {
				return true
			}
			for i, name := range stmt.Names {
				if name.Name != ident.Name {
					continue
				}
				switch {
				case len(stmt.Values) == len(stmt.Names):
					assigned = stmt.Values[i]
				case len(stmt.Values) == 1:
					assigned = stmt.Values[0]
				}
			}
		}
		return true
	})
	if assigned == nil {
		return nil, fmt.Errorf("no []string assignment for %s before its use", ident.Name)
	}
	return ResolveStringSlice(body, assigned)
}

func stringSliceLiteral(expr ast.Expr) ([]string, bool) {
	literal, ok := expr.(*ast.CompositeLit)
	if !ok {
		return nil, false
	}
	array, ok := literal.Type.(*ast.ArrayType)
	if !ok {
		return nil, false
	}
	element, ok := array.Elt.(*ast.Ident)
	if !ok || element.Name != "string" {
		return nil, false
	}

	values := make([]string, 0, len(literal.Elts))
	for _, element := range literal.Elts {
		value, ok := element.(*ast.BasicLit)
		if !ok || value.Kind != token.STRING {
			return nil, false
		}
		unquoted, err := strconv.Unquote(value.Value)
		if err != nil {
			return nil, false
		}
		values = append(values, unquoted)
	}
	return values, true
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
