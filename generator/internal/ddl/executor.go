package ddl

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
)

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
	case CreateEnum, DropEnum, CreateSchema, DropSchema, CreateIndex, DropIndex:
		return nil
	}

	// Use visitor pattern
	visitor := NewCatalogVisitor(catalog, migrationFile, databaseType)
	return stmt.Accept(visitor)
}
