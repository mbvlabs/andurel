package ddl

import (
	"context"
	"fmt"
	"log/slog"
	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"path/filepath"
)

func ApplyDDL(
	catalog *catalog.Catalog,
	stmt string,
	migrationFile string,
) error {
	ddlStmt, err := ParseDDLStatement(stmt, migrationFile)
	if err != nil {
		return fmt.Errorf(
			"failed to parse DDL statement in %s: %w",
			filepath.Base(migrationFile),
			err,
		)
	}

	if ddlStmt == nil {
		return nil
	}

	switch ddlStmt.Type {
	case CreateTable:
		return applyCreateTable(catalog, ddlStmt, migrationFile)
	case AlterTable:
		return applyAlterTable(catalog, ddlStmt, migrationFile)
	case DropTable:
		return applyDropTable(catalog, ddlStmt)
	case Unknown:
		slog.WarnContext(
			context.Background(),
			"Unknown DDL statement type in %s: %s",
			filepath.Base(migrationFile),
			ddlStmt.Raw,
		)
		return nil
	case CreateEnum, DropEnum, CreateSchema, DropSchema:
		return nil
	default:
		return fmt.Errorf(
			"unsupported DDL statement type: %v in %s",
			ddlStmt.Type,
			filepath.Base(migrationFile),
		)
	}
}

func applyCreateTable(
	catalog *catalog.Catalog,
	stmt *DDLStatement,
	migrationFile string,
) error {
	table, err := parseCreateTableToTable(stmt.Raw, migrationFile)
	if err != nil {
		return fmt.Errorf("failed to parse CREATE TABLE statement: %w", err)
	}

	schemaName := stmt.SchemaName
	if schemaName == "" {
		schemaName = catalog.DefaultSchema
	}

	if _, err := catalog.GetSchema(schemaName); err != nil {
		if _, createErr := catalog.CreateSchema(schemaName); createErr != nil {
			return fmt.Errorf(
				"failed to create schema %s: %w",
				schemaName,
				createErr,
			)
		}
	}

	if stmt.IfNotExists {
		if _, err := catalog.GetTable(schemaName, table.Name); err == nil {
			return nil
		}
	}

	return catalog.AddTable(schemaName, table)
}

func parseCreateTableToTable(
	sql, migrationFile string,
) (*catalog.Table, error) {
	ddlStmt, err := ParseDDLStatement(sql, migrationFile)
	if err != nil {
		return nil, err
	}

	if ddlStmt.Type != CreateTable {
		return nil, fmt.Errorf("expected CREATE TABLE statement")
	}

	table := catalog.NewTable(ddlStmt.SchemaName, ddlStmt.TableName).
		SetCreatedBy(migrationFile)

	columnDefs, err := extractColumnDefinitions(sql)
	if err != nil {
		return nil, fmt.Errorf("failed to extract column definitions: %w", err)
	}

	columns, err := parseColumnDefinitions(columnDefs, migrationFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse column definitions: %w", err)
	}

	for _, col := range columns {
		if err := table.AddColumn(col); err != nil {
			return nil, fmt.Errorf("failed to add column %s: %w", col.Name, err)
		}
	}

	return table, nil
}

func extractColumnDefinitions(sql string) (string, error) {
	start := -1
	end := -1
	parenLevel := 0

	for i, char := range sql {
		if char == '(' {
			if start == -1 {
				start = i + 1
			}
			parenLevel++
		} else if char == ')' {
			parenLevel--
			if parenLevel == 0 {
				end = i
				break
			}
		}
	}

	if start == -1 || end == -1 {
		return "", fmt.Errorf(
			"could not find column definitions in CREATE TABLE statement",
		)
	}

	return sql[start:end], nil
}

func applyAlterTable(
	catalog *catalog.Catalog,
	stmt *DDLStatement,
	migrationFile string,
) error {
	schemaName := stmt.SchemaName
	if schemaName == "" {
		schemaName = catalog.DefaultSchema
	}

	table, err := catalog.GetTable(schemaName, stmt.TableName)
	if err != nil {
		return fmt.Errorf(
			"table %s.%s not found: %w",
			schemaName,
			stmt.TableName,
			err,
		)
	}

	switch stmt.AlterOperation {
	case "ADD_COLUMN":
		return applyAddColumn(table, stmt.ColumnDef)
	case "DROP_COLUMN":
		return applyDropColumn(table, stmt.ColumnName)
	case "ALTER_COLUMN":
		return applyAlterColumn(
			table,
			stmt.ColumnName,
			stmt.ColumnChanges,
			migrationFile,
		)
	case "RENAME_COLUMN":
		return applyRenameColumn(table, stmt.ColumnName, stmt.NewColumnName)
	case "RENAME_TABLE":
		return applyRenameTable(
			catalog,
			schemaName,
			stmt.TableName,
			stmt.NewTableName,
		)
	case "MULTIPLE_OPERATIONS":
		return applyMultipleAlterOperations(catalog, stmt, migrationFile)
	default:
		return nil
	}
}

func applyAddColumn(table *catalog.Table, column *catalog.Column) error {
	return table.AddColumn(column)
}

func applyDropColumn(table *catalog.Table, columnName string) error {
	return table.DropColumn(columnName)
}

func applyAlterColumn(
	table *catalog.Table,
	columnName string,
	changes map[string]any,
	migrationFile string,
) error {
	column, err := table.GetColumn(columnName)
	if err != nil {
		return err
	}

	newColumn := column.Clone()
	newColumn.SetModifiedBy(migrationFile)

	for changeType, value := range changes {
		switch changeType {
		case "type":
			if typeStr, ok := value.(string); ok {
				dataType, length, precision, scale := parseDataType(typeStr)
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

func applyRenameColumn(table *catalog.Table, oldName, newName string) error {
	return table.RenameColumn(oldName, newName)
}

func applyRenameTable(
	catalog *catalog.Catalog,
	schemaName, oldName, newName string,
) error {
	table, err := catalog.GetTable(schemaName, oldName)
	if err != nil {
		return err
	}

	newTable := table.Clone()
	newTable.Name = newName

	if err := catalog.DropTable(schemaName, oldName); err != nil {
		return err
	}

	return catalog.AddTable(schemaName, newTable)
}

func applyMultipleAlterOperations(
	catalog *catalog.Catalog,
	stmt *DDLStatement,
	migrationFile string,
) error {
	operations, ok := stmt.ColumnChanges["operations"].([]string)
	if !ok {
		return fmt.Errorf(
			"invalid multiple operations data in ALTER TABLE statement",
		)
	}

	for _, operation := range operations {
		tempSQL := fmt.Sprintf("ALTER TABLE %s %s", stmt.TableName, operation)

		individualStmt, err := ParseDDLStatement(tempSQL, migrationFile)
		if err != nil {
			return fmt.Errorf(
				"failed to parse individual ALTER operation '%s': %w",
				operation,
				err,
			)
		}

		if err := applyAlterTable(catalog, individualStmt, migrationFile); err != nil {
			return fmt.Errorf(
				"failed to apply ALTER operation '%s': %w",
				operation,
				err,
			)
		}
	}

	return nil
}

func applyDropTable(
	catalog *catalog.Catalog,
	stmt *DDLStatement,
) error {
	schemaName := stmt.SchemaName
	if schemaName == "" {
		schemaName = catalog.DefaultSchema
	}

	return catalog.DropTable(schemaName, stmt.TableName)
}
