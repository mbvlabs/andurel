package columns

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"
)

func TestAndurelTag(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: `andurel:"email"`, want: "email"},
		{raw: `json:"email" andurel:"email_address"`, want: "email_address"},
		{raw: `andurel:"email,omitempty"`, want: "email,omitempty"},
		{raw: `andurel:"-"`, want: ""},
		{raw: `andurel:""`, want: ""},
		{raw: `json:"email"`, want: ""},
		{raw: ``, want: ""},
	}
	for _, tt := range tests {
		if got := AndurelTag(tt.raw); got != tt.want {
			t.Errorf("AndurelTag(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

// parseUse parses body as a function body and returns it with the argument
// of the last use(...) call, which stands in for the narsilc column argument.
func parseUse(t *testing.T, body string) (*ast.BlockStmt, ast.Expr) {
	t.Helper()
	file, err := parser.ParseFile(
		token.NewFileSet(),
		"q.go",
		"package q\n\nfunc f() {\n"+body+"\n}\n",
		0,
	)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	decl := file.Decls[0].(*ast.FuncDecl)
	var arg ast.Expr
	ast.Inspect(decl.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if ok {
			if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "use" {
				arg = call.Args[0]
			}
		}
		return true
	})
	if arg == nil {
		t.Fatal("body has no use(...) call")
	}
	return decl.Body, arg
}

func TestResolveStringSlice(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "inline literal",
			body: `use([]string{"id", "email"})`,
			want: []string{"id", "email"},
		},
		{
			name: "parenthesized literal",
			body: `use(([]string{"id"}))`,
			want: []string{"id"},
		},
		{
			name: "short variable",
			body: "cols := []string{\"id\", `created_at`}\nuse(cols)",
			want: []string{"id", "created_at"},
		},
		{
			name: "var declaration",
			body: "var cols = []string{\"id\"}\nuse(cols)",
			want: []string{"id"},
		},
		{
			name: "multi assignment",
			body: "a, cols := 1, []string{\"name\"}\n_ = a\nuse(cols)",
			want: []string{"name"},
		},
		{
			name: "variable alias",
			body: "base := []string{\"id\"}\ncols := base\nuse(cols)",
			want: []string{"id"},
		},
		{
			name: "last assignment before use wins",
			body: "cols := []string{\"old\"}\ncols = []string{\"new\"}\nuse(cols)\ncols = []string{\"later\"}",
			want: []string{"new"},
		},
		{
			name: "self assignment resolves to the earlier value",
			body: "cols := []string{\"id\"}\ncols = cols\nuse(cols)",
			want: []string{"id"},
		},
		{
			name: "empty literal",
			body: `use([]string{})`,
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, arg := parseUse(t, tt.body)
			got, err := ResolveStringSlice(body, arg)
			if err != nil {
				t.Fatalf("ResolveStringSlice: %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveStringSliceRejectsUnresolvable(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "non-literal element",
			body: "name := \"id\"\nuse([]string{name})",
			want: "not a []string literal",
		},
		{
			name: "wrong element type",
			body: `use([]any{"id"})`,
			want: "not a []string literal",
		},
		{
			name: "function call",
			body: `use(columns())`,
			want: "not a []string literal",
		},
		{
			name: "assigned only after use",
			body: "var cols []string\nuse(cols)\ncols = []string{\"id\"}",
			want: "no []string assignment for cols",
		},
		{
			name: "undeclared",
			body: `use(cols)`,
			want: "no []string assignment for cols",
		},
		{
			name: "self assignment without an earlier value",
			body: "var cols []string\ncols = cols\nuse(cols)",
			want: "no []string assignment for cols",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, arg := parseUse(t, tt.body)
			got, err := ResolveStringSlice(body, arg)
			if err == nil {
				t.Fatalf("ResolveStringSlice = %q, want error containing %q", got, tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}
