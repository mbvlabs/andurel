package ddl

import (
	"context"
	"fmt"
	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"log/slog"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func ApplyDDL(
	catalog *catalog.Catalog,
	stmt string,
	migrationFile string,
	databaseType string,
) error {
	ddlStmt, err := ParseDDLStatement(stmt, migrationFile, databaseType)
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
		return applyCreateTable(catalog, ddlStmt, migrationFile, databaseType)
	case AlterTable:
		return applyAlterTable(catalog, ddlStmt, migrationFile, databaseType)
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
	databaseType string,
) error {
	table, err := parseCreateTableToTable(stmt.Raw, migrationFile, databaseType)
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

// parseCreateTable parses CREATE TABLE statements (legacy compatibility)
func parseCreateTable(sql, migrationFile string, databaseType string) (*DDLStatement, error) {
	createTableRegex, err := regexp.Compile(
		`(?is)create\s+table(\s+if\s+not\s+exists)?\s+(?:(\w+)\.)?(\w+)\s*\(\s*(.*?)\s*\)`,
	)
	if err != nil {
		return nil, err
	}

	matches := createTableRegex.FindStringSubmatch(sql)
	if len(matches) < 5 {
		return nil, fmt.Errorf("invalid CREATE TABLE syntax: %s", sql)
	}

	ifNotExists := matches[1] != ""
	schemaName := matches[2]
	tableName := matches[3]

	return &DDLStatement{
		Type:        CreateTable,
		SchemaName:  schemaName,
		TableName:   tableName,
		IfNotExists: ifNotExists,
		Raw:         sql,
	}, nil
}

func parseCreateTableToTable(
	sql, migrationFile string,
	databaseType string,
) (*catalog.Table, error) {
	ddlStmt, err := ParseDDLStatement(sql, migrationFile, databaseType)
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

	columns, err := parseColumnDefinitions(columnDefs, migrationFile, databaseType)
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
	databaseType string,
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
		return applyMultipleAlterOperations(catalog, stmt, migrationFile, databaseType)
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
	databaseType string,
) error {
	operations, ok := stmt.ColumnChanges["operations"].([]string)
	if !ok {
		return fmt.Errorf(
			"invalid multiple operations data in ALTER TABLE statement",
		)
	}

	for _, operation := range operations {
		tempSQL := fmt.Sprintf("ALTER TABLE %s %s", stmt.TableName, operation)

		individualStmt, err := ParseDDLStatement(tempSQL, migrationFile, databaseType)
		if err != nil {
			return fmt.Errorf(
				"failed to parse individual ALTER operation '%s': %w",
				operation,
				err,
			)
		}

		if err := applyAlterTable(catalog, individualStmt, migrationFile, databaseType); err != nil {
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

// Legacy functions for backward compatibility
// TODO: Remove these once all code is updated to use the new visitor pattern

func parseColumnDefinitions(
	columnDefs, migrationFile string,
	databaseType string,
) ([]*catalog.Column, error) {
	parser := NewCreateTableParser()
	return parser.parseColumnDefinitions(columnDefs, migrationFile, databaseType)
}

func parseDataType(
	typeStr string,
) (dataType string, length *int32, precision *int32, scale *int32) {
	typeStrLower := strings.ToLower(typeStr)

	if strings.Contains(typeStrLower, "timestamp with time zone") {
		return "timestamp with time zone", nil, nil, nil
	}
	if strings.Contains(typeStrLower, "timestamp without time zone") {
		return "timestamp without time zone", nil, nil, nil
	}

	// Handle varchar(n)
	varcharRegex := regexp.MustCompile(`varchar\((\d+)\)`)
	if matches := varcharRegex.FindStringSubmatch(typeStrLower); len(matches) > 1 {
		if n, err := strconv.Atoi(matches[1]); err == nil {
			length := int32(n)
			return "varchar", &length, nil, nil
		}
	}

	// Handle char(n)
	charRegex := regexp.MustCompile(`char\((\d+)\)`)
	if matches := charRegex.FindStringSubmatch(typeStrLower); len(matches) > 1 {
		if n, err := strconv.Atoi(matches[1]); err == nil {
			length := int32(n)
			return "char", &length, nil, nil
		}
	}

	// Handle decimal(p,s)
	decimalRegex := regexp.MustCompile(`decimal\((\d+),(\d+)\)`)
	if matches := decimalRegex.FindStringSubmatch(typeStrLower); len(matches) > 2 {
		if p, err1 := strconv.Atoi(matches[1]); err1 == nil {
			if s, err2 := strconv.Atoi(matches[2]); err2 == nil {
				precision := int32(p)
				scale := int32(s)
				return "decimal", nil, &precision, &scale
			}
		}
	}

	// Handle numeric(p,s)
	numericRegex := regexp.MustCompile(`numeric\((\d+),(\d+)\)`)
	if matches := numericRegex.FindStringSubmatch(typeStrLower); len(matches) > 2 {
		if p, err1 := strconv.Atoi(matches[1]); err1 == nil {
			if s, err2 := strconv.Atoi(matches[2]); err2 == nil {
				precision := int32(p)
				scale := int32(s)
				return "numeric", nil, &precision, &scale
			}
		}
	}

	// Simple types without parameters
	switch typeStrLower {
	case "integer", "int", "int4":
		return "integer", nil, nil, nil
	case "bigint", "int8":
		return "bigint", nil, nil, nil
	case "smallint", "int2":
		return "smallint", nil, nil, nil
	case "serial":
		return "serial", nil, nil, nil
	case "bigserial":
		return "bigserial", nil, nil, nil
	case "text":
		return "text", nil, nil, nil
	case "boolean", "bool":
		return "boolean", nil, nil, nil
	case "date":
		return "date", nil, nil, nil
	case "time":
		return "time", nil, nil, nil
	case "timestamp":
		return "timestamp", nil, nil, nil
	case "real", "float4":
		return "real", nil, nil, nil
	case "double precision", "float8":
		return "double precision", nil, nil, nil
	case "uuid":
		return "uuid", nil, nil, nil
	case "json":
		return "json", nil, nil, nil
	case "jsonb":
		return "jsonb", nil, nil, nil
	default:
		// Return as-is for unknown types
		return strings.TrimSpace(typeStr), nil, nil, nil
	}
}

// validatePrimaryKeyDatatype validates primary key data types based on database
func validatePrimaryKeyDatatype(dataType, databaseType, migrationFile, columnName string) error {
	normalizedDataType := strings.ToLower(dataType)

	switch databaseType {
	case "postgresql":
		if normalizedDataType != "uuid" {
			return fmt.Errorf(`Primary key validation failed in migration '%s':
Column '%s' has datatype '%s' but PostgreSQL primary keys must use 'uuid'.

To fix this, change:
  %s %s PRIMARY KEY
to:
  %s UUID PRIMARY KEY`,
				filepath.Base(migrationFile), columnName, dataType,
				columnName, dataType, columnName)
		}
	case "sqlite":
		if normalizedDataType != "text" {
			return fmt.Errorf(`Primary key validation failed in migration '%s':
Column '%s' has datatype '%s' but SQLite primary keys must use 'text'.

To fix this, change:
  %s %s PRIMARY KEY
to:
  %s TEXT PRIMARY KEY`,
				filepath.Base(migrationFile), columnName, dataType,
				columnName, dataType, columnName)
		}
	default:
		return fmt.Errorf("unsupported database type for primary key validation: %s", databaseType)
	}

	return nil
}
