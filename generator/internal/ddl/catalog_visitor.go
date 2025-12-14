package ddl

import (
	"fmt"
	"log/slog"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
)

type CatalogVisitor struct {
	catalog      *catalog.Catalog
	migrationFile string
	databaseType string
}

func NewCatalogVisitor(cat *catalog.Catalog, migrationFile, databaseType string) *CatalogVisitor {
	return &CatalogVisitor{
		catalog:      cat,
		migrationFile: migrationFile,
		databaseType: databaseType,
	}
}

func (v *CatalogVisitor) VisitCreateTable(stmt *CreateTableStatement) error {
	schemaName := stmt.SchemaName
	if schemaName == "" {
		schemaName = v.catalog.DefaultSchema
	}

	if _, err := v.catalog.GetSchema(schemaName); err != nil {
		if _, createErr := v.catalog.CreateSchema(schemaName); createErr != nil {
			return fmt.Errorf("failed to create schema %s: %w", schemaName, createErr)
		}
	}

	if stmt.IfNotExists {
		if _, err := v.catalog.GetTable(schemaName, stmt.TableName); err == nil {
			return nil
		}
	}

	table := catalog.NewTable(schemaName, stmt.TableName).SetCreatedBy(v.migrationFile)

	for _, col := range stmt.Columns {
		if err := table.AddColumn(col); err != nil {
			return fmt.Errorf("failed to add column %s: %w", col.Name, err)
		}
	}

	return v.catalog.AddTable(schemaName, table)
}

func (v *CatalogVisitor) VisitAlterTable(stmt *AlterTableStatement) error {
	schemaName := stmt.SchemaName
	if schemaName == "" {
		schemaName = v.catalog.DefaultSchema
	}

	table, err := v.catalog.GetTable(schemaName, stmt.TableName)
	if err != nil {
		return fmt.Errorf("table %s.%s not found: %w", schemaName, stmt.TableName, err)
	}

	switch stmt.AlterOperation {
	case "ADD_COLUMN":
		return table.AddColumn(stmt.ColumnDef)
	case "DROP_COLUMN":
		return table.DropColumn(stmt.ColumnName)
	case "ALTER_COLUMN":
		return v.applyAlterColumn(table, stmt.ColumnName, stmt.ColumnChanges)
	case "RENAME_COLUMN":
		return table.RenameColumn(stmt.ColumnName, stmt.NewColumnName)
	case "RENAME_TABLE":
		return v.applyRenameTable(schemaName, stmt.TableName, stmt.NewTableName)
	case "MULTIPLE_OPERATIONS":
		// FIXED: Direct access to stmt.Operations - no conversion needed!
		return v.applyMultipleOperations(schemaName, stmt.TableName, stmt.Operations)
	default:
		// Unknown operation, log but don't fail
		slog.Warn("Unknown ALTER TABLE operation", "operation", stmt.AlterOperation)
		return nil
	}
}

func (v *CatalogVisitor) applyAlterColumn(table *catalog.Table, columnName string, changes map[string]any) error {
	column, err := table.GetColumn(columnName)
	if err != nil {
		return err
	}

	newColumn := column.Clone()
	newColumn.SetModifiedBy(v.migrationFile)

	for changeType, value := range changes {
		switch changeType {
		case "type":
			if typeStr, ok := value.(string); ok {
				dataType, length, precision, scale := ParseDataType(typeStr)
				newColumn.DataType = dataType
				if length != nil {
					newColumn.SetLength(*length)
				}
				if precision != nil && scale != nil {
					newColumn.SetPrecisionScale(*precision, *scale)
				}
			}
		case "nullable":
			if nullable, ok := value.(bool); ok {
				newColumn.IsNullable = nullable
			}
		case "default":
			if defaultVal, ok := value.(string); ok {
				newColumn.SetDefault(defaultVal)
			}
		case "drop_default":
			if drop, ok := value.(bool); ok && drop {
				newColumn.DefaultVal = nil
			}
		}
	}

	return table.ModifyColumn(columnName, newColumn)
}

func (v *CatalogVisitor) applyRenameTable(schemaName, oldName, newName string) error {
	table, err := v.catalog.GetTable(schemaName, oldName)
	if err != nil {
		return err
	}

	newTable := table.Clone()
	newTable.Name = newName

	if err := v.catalog.DropTable(schemaName, oldName); err != nil {
		return err
	}

	return v.catalog.AddTable(schemaName, newTable)
}

func (v *CatalogVisitor) applyMultipleOperations(schemaName, tableName string, operations []string) error {
	parser := NewAlterTableParser()

	for _, operation := range operations {
		tempSQL := fmt.Sprintf("ALTER TABLE %s %s", tableName, operation)

		stmt, err := parser.Parse(tempSQL, v.migrationFile, v.databaseType)
		if err != nil {
			return fmt.Errorf("failed to parse individual ALTER operation '%s': %w", operation, err)
		}

		// Recursively visit the individual operation
		if err := v.VisitAlterTable(stmt); err != nil {
			return fmt.Errorf("failed to apply ALTER operation '%s': %w", operation, err)
		}
	}

	return nil
}

func (v *CatalogVisitor) VisitDropTable(stmt *DropTableStatement) error {
	schemaName := stmt.SchemaName
	if schemaName == "" {
		schemaName = v.catalog.DefaultSchema
	}
	return v.catalog.DropTable(schemaName, stmt.TableName)
}

// Stub implementations for operations we don't process
func (v *CatalogVisitor) VisitCreateIndex(stmt *CreateIndexStatement) error {
	return nil
}

func (v *CatalogVisitor) VisitDropIndex(stmt *DropIndexStatement) error {
	return nil
}

func (v *CatalogVisitor) VisitCreateSchema(stmt *CreateSchemaStatement) error {
	return nil
}

func (v *CatalogVisitor) VisitDropSchema(stmt *DropSchemaStatement) error {
	return nil
}

func (v *CatalogVisitor) VisitCreateEnum(stmt *CreateEnumStatement) error {
	return nil
}

func (v *CatalogVisitor) VisitDropEnum(stmt *DropEnumStatement) error {
	return nil
}
