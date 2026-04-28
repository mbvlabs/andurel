package sqlcgen

import (
	"fmt"
	"strings"

	"github.com/sqlc-dev/plugin-sdk-go/plugin"
)

// goImport pairs an import path with an optional alias. An empty alias means
// the package is imported under its base name.
type goImport struct {
	Path  string
	Alias string
}

// goType is the Go-language representation of a SQL column or parameter,
// along with any imports it needs at the call site.
type goType struct {
	Expr    string // e.g. "uuid.UUID", "string", "[]byte", "int64"
	Imports []goImport
}

// mapColumnType translates the column's SQL type into a Go type expression.
//
// This is intentionally small — it only covers the types that arise from
// scaffolded CRUD queries. Anything outside that set returns an error so the
// plugin fails loudly rather than producing silently-broken code; users can
// expand this mapper or scope-out the query under a different model: while
// the broader type system is fleshed out (deferred per spec).
func mapColumnType(col *plugin.Column) (goType, error) {
	if col == nil {
		return goType{}, fmt.Errorf("nil column")
	}
	name := strings.ToLower(col.GetType().GetName())
	notNull := col.GetNotNull()

	switch name {
	case "uuid":
		if !notNull {
			return goType{}, fmt.Errorf("nullable uuid not yet supported")
		}
		return goType{Expr: "uuid.UUID", Imports: []goImport{{Path: "github.com/google/uuid"}}}, nil
	case "text", "varchar", "char", "bpchar", "citext", "name":
		if !notNull {
			return goType{
				Expr:    "pgtype.Text",
				Imports: []goImport{{Path: "github.com/jackc/pgx/v5/pgtype"}},
			}, nil
		}
		return goType{Expr: "string"}, nil
	case "int2", "smallint":
		if !notNull {
			return goType{
				Expr:    "pgtype.Int2",
				Imports: []goImport{{Path: "github.com/jackc/pgx/v5/pgtype"}},
			}, nil
		}
		return goType{Expr: "int16"}, nil
	case "int4", "integer", "serial":
		if !notNull {
			return goType{
				Expr:    "pgtype.Int4",
				Imports: []goImport{{Path: "github.com/jackc/pgx/v5/pgtype"}},
			}, nil
		}
		return goType{Expr: "int32"}, nil
	case "int8", "bigint", "bigserial":
		if !notNull {
			return goType{
				Expr:    "pgtype.Int8",
				Imports: []goImport{{Path: "github.com/jackc/pgx/v5/pgtype"}},
			}, nil
		}
		return goType{Expr: "int64"}, nil
	case "bool", "boolean":
		if !notNull {
			return goType{
				Expr:    "pgtype.Bool",
				Imports: []goImport{{Path: "github.com/jackc/pgx/v5/pgtype"}},
			}, nil
		}
		return goType{Expr: "bool"}, nil
	case "timestamp", "timestamptz":
		return goType{
			Expr:    "pgtype.Timestamptz",
			Imports: []goImport{{Path: "github.com/jackc/pgx/v5/pgtype"}},
		}, nil
	case "date":
		return goType{
			Expr:    "pgtype.Date",
			Imports: []goImport{{Path: "github.com/jackc/pgx/v5/pgtype"}},
		}, nil
	case "bytea":
		return goType{Expr: "[]byte"}, nil
	case "jsonb", "json":
		return goType{Expr: "[]byte"}, nil
	case "float4", "real":
		if !notNull {
			return goType{
				Expr:    "pgtype.Float4",
				Imports: []goImport{{Path: "github.com/jackc/pgx/v5/pgtype"}},
			}, nil
		}
		return goType{Expr: "float32"}, nil
	case "float8", "double precision":
		if !notNull {
			return goType{
				Expr:    "pgtype.Float8",
				Imports: []goImport{{Path: "github.com/jackc/pgx/v5/pgtype"}},
			}, nil
		}
		return goType{Expr: "float64"}, nil
	}

	return goType{}, fmt.Errorf("unsupported column type %q (column %q)", name, col.GetName())
}
