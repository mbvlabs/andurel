// Command sqlcgen-smoke is a temporary, hand-driven smoke driver for the
// andurel sqlc plugin. It constructs a synthetic GenerateRequest and prints
// each emitted file to stdout so we can eyeball whether the templates and
// type mapping produce compile-ready Go.
//
// Not part of the build artifact — delete once we have proper test coverage.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/sqlc-dev/plugin-sdk-go/plugin"

	"github.com/mbvlabs/andurel/internal/sqlcgen"
)

func main() {
	idCol := &plugin.Column{
		Name:    "id",
		NotNull: true,
		Type:    &plugin.Identifier{Name: "uuid"},
	}
	nameCol := &plugin.Column{
		Name:    "name",
		NotNull: true,
		Type:    &plugin.Identifier{Name: "text"},
	}

	mkQuery := func(name, cmd string, params []*plugin.Parameter, cols []*plugin.Column) *plugin.Query {
		return &plugin.Query{
			Name:     name,
			Cmd:      cmd,
			Comments: []string{"model: Server"},
			Params:   params,
			Columns:  cols,
		}
	}

	allCols := []*plugin.Column{idCol, nameCol}

	req := &plugin.GenerateRequest{
		Settings: &plugin.Settings{Codegen: &plugin.Codegen{Options: []byte(`{}`)}},
		Queries: []*plugin.Query{
			mkQuery("QueryServerByID", ":one", []*plugin.Parameter{{Number: 1, Column: idCol}}, allCols),
			mkQuery("QueryServers", ":many", nil, allCols),
			mkQuery("InsertServer", ":one", []*plugin.Parameter{
				{Number: 1, Column: idCol},
				{Number: 2, Column: nameCol},
			}, allCols),
			mkQuery("UpdateServer", ":one", []*plugin.Parameter{
				{Number: 1, Column: idCol},
				{Number: 2, Column: nameCol},
			}, allCols),
			mkQuery("DeleteServer", ":exec", []*plugin.Parameter{{Number: 1, Column: idCol}}, nil),
			mkQuery("UpsertServer", ":one", []*plugin.Parameter{
				{Number: 1, Column: idCol},
				{Number: 2, Column: nameCol},
			}, allCols),
			mkQuery("QueryServerBySlug", ":one", []*plugin.Parameter{
				{Number: 1, Column: &plugin.Column{Name: "slug", NotNull: true, Type: &plugin.Identifier{Name: "text"}}},
			}, allCols),
		},
	}

	resp, err := sqlcgen.Generate(context.Background(), req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "generate:", err)
		os.Exit(1)
	}
	for _, f := range resp.GetFiles() {
		fmt.Printf("// === %s ===\n%s\n", f.GetName(), f.GetContents())
	}
}
