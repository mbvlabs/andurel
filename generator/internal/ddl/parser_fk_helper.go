package ddl

import (
	"github.com/mbvlabs/andurel/generator/internal/catalog"
)

// convertForeignKeyConstraintToCatalogFK converts DDL ForeignKeyConstraint to catalog ForeignKey
func convertForeignKeyConstraintToCatalogFK(fk *ForeignKeyConstraint) *catalog.ForeignKey {
	catalogFK := catalog.NewForeignKey(fk.Name, fk.Column, fk.ReferencedTable, fk.ReferencedColumn)
	catalogFK.SetCreatedBy(fk.CreatedBy)
	
	// Convert referential actions
	switch fk.OnDelete {
	case "CASCADE":
		catalogFK.SetOnDelete(catalog.Cascade)
	case "SET NULL":
		catalogFK.SetOnDelete(catalog.SetNull)
	case "RESTRICT":
		catalogFK.SetOnDelete(catalog.Restrict)
	case "SET DEFAULT":
		catalogFK.SetOnDelete(catalog.SetDefault)
	default:
		catalogFK.SetOnDelete(catalog.NoAction)
	}
	
	switch fk.OnUpdate {
	case "CASCADE":
		catalogFK.SetOnUpdate(catalog.Cascade)
	case "SET NULL":
		catalogFK.SetOnUpdate(catalog.SetNull)
	case "RESTRICT":
		catalogFK.SetOnUpdate(catalog.Restrict)
	case "SET DEFAULT":
		catalogFK.SetOnUpdate(catalog.SetDefault)
	default:
		catalogFK.SetOnUpdate(catalog.NoAction)
	}
	
	return catalogFK
}