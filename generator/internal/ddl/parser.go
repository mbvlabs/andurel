package ddl

import (
	"fmt"
	"mbvlabs/andurel/generator/internal/catalog"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type StatementType int

const (
	CreateTable StatementType = iota
	AlterTable
	DropTable
	CreateIndex
	DropIndex
	CreateSchema
	DropSchema
	CreateEnum
	DropEnum
	Unknown
)

type DDLStatement struct {
	Type           StatementType
	SchemaName     string
	TableName      string
	ColumnDef      *catalog.Column
	Operation      string // ADD, DROP, MODIFY, etc.
	Raw            string // original SQL
	IndexDef       *catalog.Index
	EnumDef        *catalog.Enum
	IfNotExists    bool           // for CREATE TABLE IF NOT EXISTS
	AlterOperation string         // ADD_COLUMN, DROP_COLUMN, ALTER_COLUMN, RENAME_COLUMN, RENAME_TABLE
	ColumnName     string         // for column-specific operations
	NewColumnName  string         // for RENAME operations
	NewTableName   string         // for table renames
	ColumnChanges  map[string]any // for ALTER COLUMN operations (type, nullable, default)
}

func ParseDDLStatement(sql, migrationFile string) (*DDLStatement, error) {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return nil, nil
	}

	stmt := &DDLStatement{
		Raw: sql,
	}

	sqlLower := strings.ToLower(sql)

	switch {
	case strings.HasPrefix(sqlLower, "create table"):
		return parseCreateTable(sql, migrationFile)
	case strings.HasPrefix(sqlLower, "alter table"):
		return parseAlterTable(sql, migrationFile)
	case strings.HasPrefix(sqlLower, "drop table"):
		return parseDropTable(sql)
	case strings.HasPrefix(sqlLower, "create index") || strings.HasPrefix(sqlLower, "create unique index"):
		return parseCreateIndex(sql)
	case strings.HasPrefix(sqlLower, "drop index"):
		return parseDropIndex(sql)
	case strings.HasPrefix(sqlLower, "create schema"):
		return parseCreateSchema(sql)
	case strings.HasPrefix(sqlLower, "drop schema"):
		return parseDropSchema(sql)
	case strings.HasPrefix(sqlLower, "create type"):
		return parseCreateEnum(sql)
	case strings.HasPrefix(sqlLower, "drop type"):
		return parseDropEnum(sql)
	default:
		stmt.Type = Unknown
		return stmt, nil
	}
}

func parseCreateTable(sql, migrationFile string) (*DDLStatement, error) {
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
	columnDefs := matches[4]

	table := catalog.NewTable(schemaName, tableName).SetCreatedBy(migrationFile)

	columns, err := parseColumnDefinitions(columnDefs, migrationFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse column definitions: %w", err)
	}

	for _, col := range columns {
		if err := table.AddColumn(col); err != nil {
			return nil, fmt.Errorf("failed to add column %s: %w", col.Name, err)
		}
	}

	return &DDLStatement{
		Type:        CreateTable,
		SchemaName:  schemaName,
		TableName:   tableName,
		Raw:         sql,
		IfNotExists: ifNotExists,
	}, nil
}

func parseColumnDefinitions(
	columnDefs, migrationFile string,
) ([]*catalog.Column, error) {
	var columns []*catalog.Column
	var primaryKeyColumns []string

	defs := splitColumnDefinitions(columnDefs)

	for _, def := range defs {
		def = strings.TrimSpace(def)
		if def == "" {
			continue
		}

		defLower := strings.ToLower(def)

		if strings.HasPrefix(defLower, "primary key") {
			pkRegex := regexp.MustCompile(
				`(?i)primary\s+key\s*\(\s*([^)]+)\s*\)`,
			)
			if matches := pkRegex.FindStringSubmatch(def); len(matches) > 1 {
				pkCols := strings.SplitSeq(matches[1], ",")
				for col := range pkCols {
					primaryKeyColumns = append(
						primaryKeyColumns,
						strings.TrimSpace(col),
					)
				}
			}
			continue
		}

		if strings.HasPrefix(defLower, "foreign key") ||
			strings.HasPrefix(defLower, "constraint") ||
			strings.HasPrefix(defLower, "unique") ||
			strings.HasPrefix(defLower, "check") {
			continue
		}

		col, err := parseColumnDefinition(def, migrationFile)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to parse column definition '%s': %w",
				def,
				err,
			)
		}

		if col != nil {
			columns = append(columns, col)
		}
	}

	for _, col := range columns {
		for _, pkCol := range primaryKeyColumns {
			if col.Name == pkCol {
				col.SetPrimaryKey()
			}
		}
	}

	return columns, nil
}

func parseColumnDefinition(def, migrationFile string) (*catalog.Column, error) {
	parts := strings.Fields(def)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid column definition: %s", def)
	}

	columnName := parts[0]

	constraintKeywords := []string{
		"not",
		"null",
		"primary",
		"key",
		"unique",
		"default",
		"references",
		"check",
	}
	typeEndIndex := len(parts)

	for i := 1; i < len(parts); i++ {
		wordLower := strings.ToLower(parts[i])
		if slices.Contains(constraintKeywords, wordLower) {
			typeEndIndex = i
		}
		if typeEndIndex != len(parts) {
			break
		}
	}

	columnType := strings.Join(parts[1:typeEndIndex], " ")

	dataType, length, precision, scale := parseDataType(columnType)

	col := catalog.NewColumn(columnName, dataType).SetCreatedBy(migrationFile)

	if length != nil {
		col.SetLength(*length)
	}

	if precision != nil && scale != nil {
		col.SetPrecisionScale(*precision, *scale)
	}

	defLower := strings.ToLower(def)

	if strings.Contains(defLower, "not null") {
		col.SetNotNull()
	}

	if strings.Contains(defLower, "primary key") {
		col.SetPrimaryKey()
	}

	if strings.Contains(defLower, "unique") {
		col.SetUnique()
	}

	defaultRegex := regexp.MustCompile(`(?i)default\s+([^,\s]+(?:\s+[^,\s]+)*)`)
	if matches := defaultRegex.FindStringSubmatch(def); len(matches) > 1 {
		col.SetDefault(strings.TrimSpace(matches[1]))
	}

	return col, nil
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
	if strings.Contains(typeStrLower, "time with time zone") {
		return "time with time zone", nil, nil, nil
	}
	if strings.Contains(typeStrLower, "time without time zone") {
		return "time without time zone", nil, nil, nil
	}
	if strings.Contains(typeStrLower, "double precision") {
		return "double precision", nil, nil, nil
	}

	typeRegex := regexp.MustCompile(`^(\w+)(?:\(([^)]+)\))?$`)
	matches := typeRegex.FindStringSubmatch(typeStr)

	if len(matches) < 2 {
		return typeStr, nil, nil, nil
	}

	dataType = strings.ToLower(matches[1])

	if len(matches) > 2 && matches[2] != "" {
		params := strings.Split(matches[2], ",")

		if len(params) == 1 {
			if val, err := strconv.ParseInt(strings.TrimSpace(params[0]), 10, 32); err == nil {
				length32 := int32(val)
				length = &length32
			}
		} else if len(params) == 2 {
			if val, err := strconv.ParseInt(strings.TrimSpace(params[0]), 10, 32); err == nil {
				precision32 := int32(val)
				precision = &precision32
			}
			if val, err := strconv.ParseInt(strings.TrimSpace(params[1]), 10, 32); err == nil {
				scale32 := int32(val)
				scale = &scale32
			}
		}
	}

	return dataType, length, precision, scale
}

func splitColumnDefinitions(defs string) []string {
	var result []string
	var current strings.Builder
	parenLevel := 0
	bracketLevel := 0
	inSingleQuote := false
	inDoubleQuote := false

	for _, char := range defs {
		switch char {
		case '\'':
			if !inDoubleQuote {
				inSingleQuote = !inSingleQuote
			}
			current.WriteRune(char)
		case '"':
			if !inSingleQuote {
				inDoubleQuote = !inDoubleQuote
			}
			current.WriteRune(char)
		case '(':
			if !inSingleQuote && !inDoubleQuote {
				parenLevel++
			}
			current.WriteRune(char)
		case ')':
			if !inSingleQuote && !inDoubleQuote {
				parenLevel--
			}
			current.WriteRune(char)
		case '[':
			if !inSingleQuote && !inDoubleQuote {
				bracketLevel++
			}
			current.WriteRune(char)
		case ']':
			if !inSingleQuote && !inDoubleQuote {
				bracketLevel--
			}
			current.WriteRune(char)
		case ',':
			if parenLevel == 0 && bracketLevel == 0 && !inSingleQuote && !inDoubleQuote {
				result = append(result, current.String())
				current.Reset()
			} else {
				current.WriteRune(char)
			}
		default:
			current.WriteRune(char)
		}
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}

func parseAddColumn(
	stmt *DDLStatement,
	operation, migrationFile string,
) (*DDLStatement, error) {
	addColumnRegex := regexp.MustCompile(
		`(?i)add\s+column\s+(?:if\s+not\s+exists\s+)?(.+)`,
	)
	matches := addColumnRegex.FindStringSubmatch(operation)

	if len(matches) < 2 {
		return nil, fmt.Errorf("invalid ADD COLUMN syntax: %s", operation)
	}

	columnDef := strings.TrimSpace(matches[1])
	column, err := parseColumnDefinition(columnDef, migrationFile)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to parse column definition in ADD COLUMN: %w",
			err,
		)
	}

	stmt.AlterOperation = "ADD_COLUMN"
	stmt.ColumnDef = column
	stmt.ColumnName = column.Name

	return stmt, nil
}

func parseDropColumn(
	stmt *DDLStatement,
	operation string,
) (*DDLStatement, error) {
	dropColumnRegex := regexp.MustCompile(`(?i)drop\s+column\s+(\w+)`)
	matches := dropColumnRegex.FindStringSubmatch(operation)

	if len(matches) < 2 {
		return nil, fmt.Errorf("invalid DROP COLUMN syntax: %s", operation)
	}

	stmt.AlterOperation = "DROP_COLUMN"
	stmt.ColumnName = matches[1]

	return stmt, nil
}

func parseAlterColumn(
	stmt *DDLStatement,
	operation string,
) (*DDLStatement, error) {
	stmt.AlterOperation = "ALTER_COLUMN"
	stmt.ColumnChanges = make(map[string]any)

	alterColumnRegex := regexp.MustCompile(`(?i)alter\s+column\s+(\w+)\s+(.+)`)
	matches := alterColumnRegex.FindStringSubmatch(operation)

	if len(matches) < 3 {
		return nil, fmt.Errorf("invalid ALTER COLUMN syntax: %s", operation)
	}

	stmt.ColumnName = matches[1]
	columnOperation := strings.TrimSpace(matches[2])
	columnOpLower := strings.ToLower(columnOperation)

	switch {
	case strings.HasPrefix(columnOpLower, "type"):
		typeRegex := regexp.MustCompile(`(?i)type\s+(.+)`)
		typeMatches := typeRegex.FindStringSubmatch(columnOperation)
		if len(typeMatches) > 1 {
			stmt.ColumnChanges["type"] = strings.TrimSpace(typeMatches[1])
		}
	case strings.HasPrefix(columnOpLower, "set not null"):
		stmt.ColumnChanges["nullable"] = false
	case strings.HasPrefix(columnOpLower, "drop not null"):
		stmt.ColumnChanges["nullable"] = true
	case strings.HasPrefix(columnOpLower, "set default"):
		defaultRegex := regexp.MustCompile(`(?i)set\s+default\s+(.+)`)
		defaultMatches := defaultRegex.FindStringSubmatch(columnOperation)
		if len(defaultMatches) > 1 {
			stmt.ColumnChanges["default"] = strings.TrimSpace(defaultMatches[1])
		}
	case strings.HasPrefix(columnOpLower, "drop default"):
		stmt.ColumnChanges["drop_default"] = true
	}

	return stmt, nil
}

func parseRenameColumn(
	stmt *DDLStatement,
	operation string,
) (*DDLStatement, error) {
	renameColumnRegex := regexp.MustCompile(
		`(?i)rename\s+column\s+(\w+)\s+to\s+(\w+)`,
	)
	matches := renameColumnRegex.FindStringSubmatch(operation)

	if len(matches) < 3 {
		return nil, fmt.Errorf("invalid RENAME COLUMN syntax: %s", operation)
	}

	stmt.AlterOperation = "RENAME_COLUMN"
	stmt.ColumnName = matches[1]
	stmt.NewColumnName = matches[2]

	return stmt, nil
}

func parseRenameTable(
	stmt *DDLStatement,
	operation string,
) (*DDLStatement, error) {
	renameTableRegex := regexp.MustCompile(`(?i)rename\s+to\s+(\w+)`)
	matches := renameTableRegex.FindStringSubmatch(operation)

	if len(matches) < 2 {
		return nil, fmt.Errorf("invalid RENAME TABLE syntax: %s", operation)
	}

	stmt.AlterOperation = "RENAME_TABLE"
	stmt.NewTableName = matches[1]

	return stmt, nil
}

func parseAlterTable(sql, migrationFile string) (*DDLStatement, error) {
	alterRegex := regexp.MustCompile(
		`(?is)alter\s+table\s+(?:if\s+exists\s+)?(?:(\w+)\.)?(\w+)\s+(.+)`,
	)
	matches := alterRegex.FindStringSubmatch(sql)

	if len(matches) < 4 {
		return nil, fmt.Errorf("invalid ALTER TABLE syntax: %s", sql)
	}

	schemaName := matches[1]
	tableName := matches[2]
	operations := strings.TrimSpace(matches[3])

	operationList := splitAlterOperations(operations)

	if len(operationList) == 1 {
		return parseAlterTableSingleOperation(
			schemaName,
			tableName,
			operationList[0],
			sql,
			migrationFile,
		)
	}

	stmt := &DDLStatement{
		Type:           AlterTable,
		SchemaName:     schemaName,
		TableName:      tableName,
		Operation:      operations,
		Raw:            sql,
		AlterOperation: "MULTIPLE_OPERATIONS",
		ColumnChanges:  make(map[string]any),
	}

	stmt.ColumnChanges["operations"] = operationList

	return stmt, nil
}

func parseAlterTableSingleOperation(
	schemaName, tableName, operation, sql, migrationFile string,
) (*DDLStatement, error) {
	stmt := &DDLStatement{
		Type:       AlterTable,
		SchemaName: schemaName,
		TableName:  tableName,
		Operation:  operation,
		Raw:        sql,
	}

	operationLower := strings.ToLower(operation)

	switch {
	case strings.HasPrefix(operationLower, "add column"):
		return parseAddColumn(stmt, operation, migrationFile)
	case strings.HasPrefix(operationLower, "drop column"):
		return parseDropColumn(stmt, operation)
	case strings.HasPrefix(operationLower, "alter column"):
		return parseAlterColumn(stmt, operation)
	case strings.HasPrefix(operationLower, "rename column"):
		return parseRenameColumn(stmt, operation)
	case strings.HasPrefix(operationLower, "rename to"):
		return parseRenameTable(stmt, operation)
	default:
		return stmt, nil
	}
}

func splitAlterOperations(operations string) []string {
	var result []string
	var current strings.Builder
	parenLevel := 0
	inQuotes := false
	var quoteChar rune

	for _, char := range operations {
		switch char {
		case '\'', '"':
			if !inQuotes {
				inQuotes = true
				quoteChar = char
			} else if char == quoteChar {
				inQuotes = false
				quoteChar = 0
			}
			current.WriteRune(char)
		case '(':
			if !inQuotes {
				parenLevel++
			}
			current.WriteRune(char)
		case ')':
			if !inQuotes {
				parenLevel--
			}
			current.WriteRune(char)
		case ',':
			if !inQuotes && parenLevel == 0 {
				result = append(result, strings.TrimSpace(current.String()))
				current.Reset()
			} else {
				current.WriteRune(char)
			}
		default:
			current.WriteRune(char)
		}
	}

	if current.Len() > 0 {
		result = append(result, strings.TrimSpace(current.String()))
	}

	return result
}

func parseDropTable(sql string) (*DDLStatement, error) {
	dropRegex := regexp.MustCompile(
		`(?i)drop\s+table(?:\s+if\s+exists)?\s+(?:(\w+)\.)?(\w+)`,
	)
	matches := dropRegex.FindStringSubmatch(sql)

	if len(matches) < 3 {
		return nil, fmt.Errorf("invalid DROP TABLE syntax: %s", sql)
	}

	schemaName := matches[1]
	tableName := matches[2]

	return &DDLStatement{
		Type:       DropTable,
		SchemaName: schemaName,
		TableName:  tableName,
		Raw:        sql,
	}, nil
}

func parseCreateIndex(sql string) (*DDLStatement, error) {
	return &DDLStatement{
		Type: CreateIndex,
		Raw:  sql,
	}, nil
}

func parseDropIndex(sql string) (*DDLStatement, error) {
	return &DDLStatement{
		Type: DropIndex,
		Raw:  sql,
	}, nil
}

func parseCreateSchema(sql string) (*DDLStatement, error) {
	return &DDLStatement{
		Type: CreateSchema,
		Raw:  sql,
	}, nil
}

func parseDropSchema(sql string) (*DDLStatement, error) {
	return &DDLStatement{
		Type: DropSchema,
		Raw:  sql,
	}, nil
}

func parseCreateEnum(sql string) (*DDLStatement, error) {
	return &DDLStatement{
		Type: CreateEnum,
		Raw:  sql,
	}, nil
}

func parseDropEnum(sql string) (*DDLStatement, error) {
	return &DDLStatement{
		Type: DropEnum,
		Raw:  sql,
	}, nil
}
