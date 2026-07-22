package ddl

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/generator/internal/validation"
)

// CreateTableParser handles CREATE TABLE statements
type CreateTableParser struct{}

// NewCreateTableParser creates a new create table parser.
func NewCreateTableParser() *CreateTableParser {
	return &CreateTableParser{}
}

// Parse performs the parse operation.
func (p *CreateTableParser) Parse(
	sql, migrationFile string,
	databaseType string,
) (*CreateTableStatement, error) {
	matches := parseCreateTableMatches(sql)
	if len(matches) < 5 {
		return nil, unsupportedStatement(sql, "CREATE TABLE requires one unquoted table name and an explicit column list")
	}

	ifNotExists := matches[1] != ""
	schemaName := matches[2]
	tableName := matches[3]
	columnDefs := matches[4]
	if strings.ContainsAny(schemaName+tableName, `"'`) {
		return nil, unsupportedStatement(sql, "quoted table identifiers are not supported")
	}

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
	var allowedValueConstraints []struct {
		column string
		values []string
	}
	var foreignKeys []struct {
		column           string
		referencedTable  string
		referencedColumn string
	}

	defs := p.splitColumnDefinitions(columnDefs)
	if len(defs) == 0 {
		return nil, unsupportedStatement(columnDefs, "CREATE TABLE must contain at least one column definition")
	}
	seenColumns := map[string]struct{}{}
	primaryKeyDefinitions := 0

	for _, def := range defs {
		def = strings.TrimSpace(def)
		if def == "" {
			continue
		}

		defLower := strings.ToLower(def)

		if strings.HasPrefix(defLower, "primary key") {
			primaryKeyDefinitions++
			if primaryKeyDefinitions > 1 {
				return nil, unsupportedStatement(def, "multiple table-level PRIMARY KEY definitions are ambiguous")
			}
			pkCols, ok := parsePrimaryKeyColumns(def)
			if !ok || len(pkCols) == 0 {
				return nil, unsupportedStatement(def, "table-level PRIMARY KEY must name one or more columns")
			}
			for _, col := range pkCols {
				name := strings.TrimSpace(col)
				if name == "" {
					return nil, unsupportedStatement(def, "table-level PRIMARY KEY contains an empty column name")
				}
				primaryKeyColumns = append(primaryKeyColumns, name)
			}
			continue
		}

		if strings.HasPrefix(defLower, "foreign key") || strings.Contains(defLower, "foreign key") {
			// Parse table-level FOREIGN KEY constraint
			// Format: FOREIGN KEY (column) REFERENCES table(column)
			// Also handles: CONSTRAINT name FOREIGN KEY (column) REFERENCES table(column)
			matches, ok := parseTableLevelForeignKey(def)
			if !ok {
				return nil, unsupportedStatement(def, "table-level FOREIGN KEY must name one local and one referenced column")
			}
			foreignKeys = append(foreignKeys, struct {
				column           string
				referencedTable  string
				referencedColumn string
			}{
				column:           matches[1],
				referencedTable:  matches[2],
				referencedColumn: matches[3],
			})
			continue
		}

		if strings.HasPrefix(defLower, "check") ||
			(strings.HasPrefix(defLower, "constraint") && strings.Contains(defLower, " check")) {
			column, values := parseCheckInValues(def)
			if len(values) > 0 {
				allowedValueConstraints = append(allowedValueConstraints, struct {
					column string
					values []string
				}{column: column, values: values})
			}
			continue
		}

		if isTableConstraintDefinition(defLower) {
			continue
		}
		if strings.HasPrefix(defLower, "constraint") {
			if strings.Contains(defLower, " unique ") || strings.Contains(defLower, " check ") {
				continue
			}
			return nil, unsupportedStatement(def, "only named FOREIGN KEY, UNIQUE, and CHECK table constraints are supported")
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
			normalizedName := strings.ToLower(col.Name)
			if _, exists := seenColumns[normalizedName]; exists {
				return nil, unsupportedStatement(def, "duplicate column definition for "+col.Name)
			}
			seenColumns[normalizedName] = struct{}{}
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

	for _, constraint := range allowedValueConstraints {
		matched := false
		for _, col := range columns {
			if strings.EqualFold(col.Name, constraint.column) {
				col.SetAllowedValues(constraint.values...)
				matched = true
				break
			}
		}
		if !matched {
			return nil, unsupportedStatement(columnDefs, "CHECK constraint references unknown column "+constraint.column)
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
	if strings.ContainsAny(columnName, `"'`) {
		return nil, unsupportedStatement(def, "quoted column identifiers are not supported")
	}

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

	switch strings.ToLower(dataType) {
	case "serial", "bigserial":
		col.SetAutoIncrement()
	}

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
	if allowedValues := parseInlineCheckValues(def, columnName); len(allowedValues) > 0 {
		col.SetAllowedValues(allowedValues...)
	}

	// Parse inline REFERENCES clause:
	// REFERENCES table(column) or REFERENCES table
	if referencedTable, referencedColumn, ok := parseInlineReference(def); ok {
		col.SetForeignKey(referencedTable, referencedColumn)
	}

	return col, nil
}

func parseInlineCheckValues(def, columnName string) []string {
	checkedColumn, values := parseCheckInValues(def)
	if !strings.EqualFold(checkedColumn, columnName) {
		return nil
	}
	return values
}

func parseCheckInValues(def string) (string, []string) {
	constraint := regexp.MustCompile(`(?i)\bcheck\s*\(\s*(\w+)\s+in\s*\(([^)]*)\)\s*\)`).FindStringSubmatch(def)
	if len(constraint) != 3 {
		return "", nil
	}
	matches := regexp.MustCompile(`'((?:''|[^'])*)'`).FindAllStringSubmatch(constraint[2], -1)
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		values = append(values, strings.ReplaceAll(match[1], "''", "'"))
	}
	return constraint[1], values
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
	if before, after, ok := strings.Cut(namePart, "."); ok {
		schemaName = strings.TrimSpace(before)
		tableName = strings.TrimSpace(after)
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
