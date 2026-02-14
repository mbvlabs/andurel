package ddl

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/generator/internal/validation"
)

// CreateTableParser handles CREATE TABLE statements
type CreateTableParser struct{}

func NewCreateTableParser() *CreateTableParser {
	return &CreateTableParser{}
}

func (p *CreateTableParser) Parse(
	sql, migrationFile string,
	databaseType string,
) (*CreateTableStatement, error) {
	matches := parseCreateTableMatches(sql)
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
	var foreignKeys []struct {
		column           string
		referencedTable  string
		referencedColumn string
	}

	defs := p.splitColumnDefinitions(columnDefs)

	for _, def := range defs {
		def = strings.TrimSpace(def)
		if def == "" {
			continue
		}

		defLower := strings.ToLower(def)

		if strings.HasPrefix(defLower, "primary key") {
			if pkCols, ok := parsePrimaryKeyColumns(def); ok {
				for _, col := range pkCols {
					primaryKeyColumns = append(primaryKeyColumns, strings.TrimSpace(col))
				}
			}
			continue
		}

		if strings.HasPrefix(defLower, "foreign key") || strings.Contains(defLower, "foreign key") {
			// Parse table-level FOREIGN KEY constraint
			// Format: FOREIGN KEY (column) REFERENCES table(column)
			// Also handles: CONSTRAINT name FOREIGN KEY (column) REFERENCES table(column)
			if matches, ok := parseTableLevelForeignKey(def); ok {
				foreignKeys = append(foreignKeys, struct {
					column           string
					referencedTable  string
					referencedColumn string
				}{
					column:           matches[1],
					referencedTable:  matches[2],
					referencedColumn: matches[3],
				})
			}
			continue
		}

		if strings.HasPrefix(defLower, "constraint") || isTableConstraintDefinition(defLower) {
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

	// Apply table-level foreign keys
	for _, fk := range foreignKeys {
		for _, col := range columns {
			if col.Name == fk.column {
				col.SetForeignKey(fk.referencedTable, fk.referencedColumn)
				break
			}
		}
	}

	return columns, nil
}

func (p *CreateTableParser) parseColumnDefinition(
	def, migrationFile string,
	databaseType string,
) (*catalog.Column, error) {
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

	if defaultVal, ok := parseDefaultValue(def); ok {
		col.SetDefault(defaultVal)
	}

	// Parse inline REFERENCES clause:
	// REFERENCES table(column) or REFERENCES table
	if referencedTable, referencedColumn, ok := parseInlineReference(def); ok {
		col.SetForeignKey(referencedTable, referencedColumn)
	}

	return col, nil
}

func (p *CreateTableParser) parseDataType(
	typeStr string,
) (dataType string, length *int32, precision *int32, scale *int32) {
	return ParseDataType(typeStr)
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

func (p *CreateTableParser) validatePrimaryKeyDatatype(
	dataType, databaseType, migrationFile, columnName string,
) error {
	return validation.ValidatePrimaryKeyDatatype(dataType, databaseType, migrationFile, columnName)
}

func parseCreateTableMatches(sql string) []string {
	sqlLower := strings.ToLower(sql)
	createIdx := strings.Index(sqlLower, "create table")
	if createIdx == -1 {
		return nil
	}

	afterCreate := strings.TrimSpace(sql[createIdx+len("create table"):])
	if afterCreate == "" {
		return nil
	}

	ifNotExists := ""
	afterLower := strings.ToLower(afterCreate)
	if strings.HasPrefix(afterLower, "if not exists") {
		ifNotExists = " if not exists"
		afterCreate = strings.TrimSpace(afterCreate[len("if not exists"):])
	}

	openIdx := strings.Index(afterCreate, "(")
	closeIdx := strings.LastIndex(afterCreate, ")")
	if openIdx == -1 || closeIdx == -1 || closeIdx <= openIdx {
		return nil
	}

	namePart := strings.TrimSpace(afterCreate[:openIdx])
	columnDefs := strings.TrimSpace(afterCreate[openIdx+1 : closeIdx])
	if namePart == "" {
		return nil
	}

	schemaName := ""
	tableName := namePart
	if dotIdx := strings.Index(namePart, "."); dotIdx != -1 {
		schemaName = strings.TrimSpace(namePart[:dotIdx])
		tableName = strings.TrimSpace(namePart[dotIdx+1:])
	}

	if tableName == "" {
		return nil
	}

	return []string{"", ifNotExists, schemaName, tableName, columnDefs}
}

func parsePrimaryKeyColumns(def string) ([]string, bool) {
	start := strings.Index(def, "(")
	end := strings.LastIndex(def, ")")
	if start == -1 || end == -1 || end <= start {
		return nil, false
	}
	return strings.Split(def[start+1:end], ","), true
}

func parseTableLevelForeignKey(def string) ([]string, bool) {
	defLower := strings.ToLower(def)
	fkIdx := strings.Index(defLower, "foreign key")
	if fkIdx == -1 {
		return nil, false
	}

	openCol := strings.Index(def[fkIdx:], "(")
	if openCol == -1 {
		return nil, false
	}
	openCol += fkIdx
	closeCol := strings.Index(def[openCol:], ")")
	if closeCol == -1 {
		return nil, false
	}
	closeCol += openCol
	column := strings.TrimSpace(def[openCol+1 : closeCol])

	afterColsLower := strings.ToLower(def[closeCol+1:])
	refIdxRel := strings.Index(afterColsLower, "references")
	if refIdxRel == -1 {
		return nil, false
	}
	refIdx := closeCol + 1 + refIdxRel
	afterRef := strings.TrimSpace(def[refIdx+len("references"):])
	if afterRef == "" {
		return nil, false
	}

	refOpen := strings.Index(afterRef, "(")
	refClose := strings.Index(afterRef, ")")
	if refOpen == -1 || refClose == -1 || refClose <= refOpen {
		return nil, false
	}
	referencedTable := strings.TrimSpace(afterRef[:refOpen])
	referencedColumn := strings.TrimSpace(afterRef[refOpen+1 : refClose])
	if column == "" || referencedTable == "" || referencedColumn == "" {
		return nil, false
	}

	return []string{"", column, referencedTable, referencedColumn}, true
}

func isTableConstraintDefinition(defLower string) bool {
	return strings.HasPrefix(defLower, "unique(") ||
		strings.HasPrefix(defLower, "unique ") ||
		defLower == "unique" ||
		strings.HasPrefix(defLower, "check(") ||
		strings.HasPrefix(defLower, "check ") ||
		defLower == "check"
}

func parseDefaultValue(def string) (string, bool) {
	defLower := strings.ToLower(def)
	defaultIdx := strings.Index(defLower, "default")
	if defaultIdx == -1 {
		return "", false
	}

	value := strings.TrimSpace(def[defaultIdx+len("default"):])
	if value == "" {
		return "", false
	}

	for _, keyword := range []string{" not null", " primary key", " unique", " references ", " check "} {
		if idx := strings.Index(strings.ToLower(value), keyword); idx != -1 {
			value = strings.TrimSpace(value[:idx])
			break
		}
	}

	return value, value != ""
}

func parseInlineReference(def string) (string, string, bool) {
	defLower := strings.ToLower(def)
	refIdx := strings.Index(defLower, "references")
	if refIdx == -1 {
		return "", "", false
	}

	afterRef := strings.TrimSpace(def[refIdx+len("references"):])
	if afterRef == "" {
		return "", "", false
	}

	refOpen := strings.Index(afterRef, "(")
	if refOpen == -1 {
		return strings.TrimSpace(afterRef), "id", true
	}
	refClose := strings.Index(afterRef, ")")
	if refClose == -1 || refClose <= refOpen {
		return "", "", false
	}

	referencedTable := strings.TrimSpace(afterRef[:refOpen])
	referencedColumn := strings.TrimSpace(afterRef[refOpen+1 : refClose])
	if referencedTable == "" || referencedColumn == "" {
		return "", "", false
	}

	return referencedTable, referencedColumn, true
}
