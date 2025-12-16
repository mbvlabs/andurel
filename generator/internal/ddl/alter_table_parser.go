package ddl

import (
	"fmt"
	"regexp"
	"strings"
)

// AlterTableParser handles ALTER TABLE statements
type AlterTableParser struct {
	createTableParser *CreateTableParser
}

func NewAlterTableParser() *AlterTableParser {
	return &AlterTableParser{
		createTableParser: NewCreateTableParser(),
	}
}

func (p *AlterTableParser) Parse(
	sql, migrationFile string,
	databaseType string,
) (*AlterTableStatement, error) {
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

	operationList := p.splitAlterOperations(operations)

	if len(operationList) == 1 {
		return p.parseAlterTableSingleOperation(
			schemaName,
			tableName,
			operationList[0],
			sql,
			migrationFile,
			databaseType,
		)
	}

	stmt := &AlterTableStatement{
		Raw:            sql,
		SchemaName:     schemaName,
		TableName:      tableName,
		AlterOperation: "MULTIPLE_OPERATIONS",
		ColumnChanges:  make(map[string]any),
		Operations:     operationList,
	}

	return stmt, nil
}

func (p *AlterTableParser) parseAlterTableSingleOperation(
	schemaName, tableName, operation, sql, migrationFile string,
	databaseType string,
) (*AlterTableStatement, error) {
	stmt := &AlterTableStatement{
		Raw:        sql,
		SchemaName: schemaName,
		TableName:  tableName,
	}

	operationLower := strings.ToLower(operation)

	switch {
	case strings.HasPrefix(operationLower, "add column"):
		return p.parseAddColumn(stmt, operation, migrationFile, databaseType)
	case strings.HasPrefix(operationLower, "drop column"):
		return p.parseDropColumn(stmt, operation)
	case strings.HasPrefix(operationLower, "alter column"):
		return p.parseAlterColumn(stmt, operation)
	case strings.HasPrefix(operationLower, "rename column"):
		return p.parseRenameColumn(stmt, operation)
	case strings.HasPrefix(operationLower, "rename to"):
		return p.parseRenameTable(stmt, operation)
	case strings.HasPrefix(operationLower, "add constraint"):
		stmt.AlterOperation = "ADD_CONSTRAINT"
		return stmt, nil
	case strings.HasPrefix(operationLower, "drop constraint"):
		stmt.AlterOperation = "DROP_CONSTRAINT"
		return stmt, nil
	default:
		return stmt, nil
	}
}

func (p *AlterTableParser) parseAddColumn(
	stmt *AlterTableStatement,
	operation, migrationFile string,
	databaseType string,
) (*AlterTableStatement, error) {
	addColumnRegex := regexp.MustCompile(
		`(?i)add\s+column\s+(?:if\s+not\s+exists\s+)?(.+)`,
	)
	matches := addColumnRegex.FindStringSubmatch(operation)

	if len(matches) < 2 {
		return nil, fmt.Errorf("invalid ADD COLUMN syntax: %s", operation)
	}

	columnDef := strings.TrimSpace(matches[1])
	column, err := p.createTableParser.parseColumnDefinition(columnDef, migrationFile, databaseType)
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

func (p *AlterTableParser) parseDropColumn(
	stmt *AlterTableStatement,
	operation string,
) (*AlterTableStatement, error) {
	dropColumnRegex := regexp.MustCompile(`(?i)drop\s+column\s+(\w+)`)
	matches := dropColumnRegex.FindStringSubmatch(operation)

	if len(matches) < 2 {
		return nil, fmt.Errorf("invalid DROP COLUMN syntax: %s", operation)
	}

	stmt.AlterOperation = "DROP_COLUMN"
	stmt.ColumnName = matches[1]

	return stmt, nil
}

func (p *AlterTableParser) parseAlterColumn(
	stmt *AlterTableStatement,
	operation string,
) (*AlterTableStatement, error) {
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

func (p *AlterTableParser) parseRenameColumn(
	stmt *AlterTableStatement,
	operation string,
) (*AlterTableStatement, error) {
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

func (p *AlterTableParser) parseRenameTable(
	stmt *AlterTableStatement,
	operation string,
) (*AlterTableStatement, error) {
	renameTableRegex := regexp.MustCompile(`(?i)rename\s+to\s+(\w+)`)
	matches := renameTableRegex.FindStringSubmatch(operation)

	if len(matches) < 2 {
		return nil, fmt.Errorf("invalid RENAME TABLE syntax: %s", operation)
	}

	stmt.AlterOperation = "RENAME_TABLE"
	stmt.NewTableName = matches[1]

	return stmt, nil
}

func (p *AlterTableParser) splitAlterOperations(operations string) []string {
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
