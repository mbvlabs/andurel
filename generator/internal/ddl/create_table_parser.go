package ddl

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
)

// CreateTableParser handles CREATE TABLE statements
type CreateTableParser struct{}

func NewCreateTableParser() *CreateTableParser {
	return &CreateTableParser{}
}

func (p *CreateTableParser) Parse(sql, migrationFile string, databaseType string) (*CreateTableStatement, error) {
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

	columns, err := p.parseColumnDefinitions(columnDefs, migrationFile, databaseType)
	if err != nil {
		return nil, fmt.Errorf("failed to parse column definitions: %w", err)
	}

	return &CreateTableStatement{
		Raw:         sql,
		SchemaName:  schemaName,
		TableName:   tableName,
		IfNotExists: ifNotExists,
		Columns:     columns,
	}, nil
}

func (p *CreateTableParser) parseColumnDefinitions(
	columnDefs, migrationFile string,
	databaseType string,
) ([]*catalog.Column, error) {
	var columns []*catalog.Column
	var primaryKeyColumns []string

	defs := p.splitColumnDefinitions(columnDefs)

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

		col, err := p.parseColumnDefinition(def, migrationFile, databaseType)
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
				if err := p.validatePrimaryKeyDatatype(col.DataType, databaseType, migrationFile, col.Name); err != nil {
					return nil, err
				}
			}
		}
	}

	return columns, nil
}

func (p *CreateTableParser) parseColumnDefinition(def, migrationFile string, databaseType string) (*catalog.Column, error) {
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

	dataType, length, precision, scale := p.parseDataType(columnType)

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
		if err := p.validatePrimaryKeyDatatype(col.DataType, databaseType, migrationFile, col.Name); err != nil {
			return nil, err
		}
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

func (p *CreateTableParser) parseDataType(
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

func (p *CreateTableParser) splitColumnDefinitions(defs string) []string {
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

func (p *CreateTableParser) validatePrimaryKeyDatatype(dataType, databaseType, migrationFile, columnName string) error {
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
	}

	return nil
}
