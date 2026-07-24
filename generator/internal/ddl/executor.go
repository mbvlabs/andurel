package ddl

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
)

// ApplyDDL applies d d l.
func ApplyDDL(
	catalog *catalog.Catalog,
	sql string,
	migrationFile string,
	databaseType string,
) error {
	parser := NewDDLParser()
	stmt, err := parser.Parse(sql, migrationFile, databaseType)
	if err != nil {
		return fmt.Errorf(
			"failed to parse DDL statement in %s: %w",
			filepath.Base(migrationFile),
			err,
		)
	}

	if stmt == nil {
		return nil
	}

	// Log unknown statements
	if stmt.GetType() == Unknown {
		if !unknownStatementIsModelNeutral(stmt.GetRaw()) {
			return unsupportedStatement(stmt.GetRaw(), "the statement may change tables or columns used to generate models")
		}
		slog.WarnContext(
			context.Background(),
			"Unknown DDL statement type",
			"file", filepath.Base(migrationFile),
			"sql", stmt.GetRaw(),
		)
		return nil
	}

	// Skip statements we don't process (not errors)
	switch stmt.GetType() {
	case DropEnum, DropSchema:
		if strings.Contains(strings.ToLower(stmt.GetRaw()), "cascade") {
			return unsupportedStatement(stmt.GetRaw(), "CASCADE can remove table columns or tables used to generate models")
		}
		return nil
	case CreateIndex, DropIndex:
		return nil
	}

	// Use visitor pattern
	visitor := NewCatalogVisitor(catalog, migrationFile, databaseType)
	return stmt.Accept(visitor)
}
