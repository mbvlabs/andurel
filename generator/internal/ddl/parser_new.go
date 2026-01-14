package ddl

import (
	"strings"
)

// StripComments removes SQL comments from the input string.
// It handles both single-line comments (-- ...) and block comments (/* ... */).
// It preserves comment-like patterns inside quoted strings.
func StripComments(sql string) string {
	var result strings.Builder
	result.Grow(len(sql))

	i := 0
	for i < len(sql) {
		// Check for single-quoted string
		if sql[i] == '\'' {
			result.WriteByte(sql[i])
			i++
			for i < len(sql) {
				result.WriteByte(sql[i])
				if sql[i] == '\'' {
					// Check for escaped quote ('')
					if i+1 < len(sql) && sql[i+1] == '\'' {
						i++
						result.WriteByte(sql[i])
					} else {
						break
					}
				}
				i++
			}
			i++
			continue
		}

		// Check for double-quoted identifier
		if sql[i] == '"' {
			result.WriteByte(sql[i])
			i++
			for i < len(sql) {
				result.WriteByte(sql[i])
				if sql[i] == '"' {
					// Check for escaped quote ("")
					if i+1 < len(sql) && sql[i+1] == '"' {
						i++
						result.WriteByte(sql[i])
					} else {
						break
					}
				}
				i++
			}
			i++
			continue
		}

		// Check for single-line comment (--)
		if i+1 < len(sql) && sql[i] == '-' && sql[i+1] == '-' {
			// Skip until end of line
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
			// Keep the newline to preserve line structure
			if i < len(sql) {
				result.WriteByte('\n')
				i++
			}
			continue
		}

		// Check for block comment (/* ... */)
		if i+1 < len(sql) && sql[i] == '/' && sql[i+1] == '*' {
			i += 2
			// Skip until end of block comment
			for i+1 < len(sql) {
				if sql[i] == '*' && sql[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			// Add a space to prevent tokens from merging
			result.WriteByte(' ')
			continue
		}

		// Regular character
		result.WriteByte(sql[i])
		i++
	}

	return result.String()
}

// NewDDLParser creates a parser using the visitor pattern
type DDLParser struct {
	createTableParser  *CreateTableParser
	alterTableParser   *AlterTableParser
	dropTableParser    *DropTableParser
	createIndexParser  *CreateIndexParser
	dropIndexParser    *DropIndexParser
	createSchemaParser *CreateSchemaParser
	dropSchemaParser   *DropSchemaParser
	createEnumParser   *CreateEnumParser
	dropEnumParser     *DropEnumParser
}

func NewDDLParser() *DDLParser {
	return &DDLParser{
		createTableParser:  NewCreateTableParser(),
		alterTableParser:   NewAlterTableParser(),
		dropTableParser:    NewDropTableParser(),
		createIndexParser:  NewCreateIndexParser(),
		dropIndexParser:    NewDropIndexParser(),
		createSchemaParser: NewCreateSchemaParser(),
		dropSchemaParser:   NewDropSchemaParser(),
		createEnumParser:   NewCreateEnumParser(),
		dropEnumParser:     NewDropEnumParser(),
	}
}

func (p *DDLParser) Parse(sql, migrationFile string, databaseType string) (Statement, error) {
	// Strip SQL comments before parsing
	sql = StripComments(sql)
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return nil, nil
	}

	sqlLower := strings.ToLower(sql)

	switch {
	case strings.HasPrefix(sqlLower, "create table"):
		return p.createTableParser.Parse(sql, migrationFile, databaseType)
	case strings.HasPrefix(sqlLower, "alter table"):
		return p.alterTableParser.Parse(sql, migrationFile, databaseType)
	case strings.HasPrefix(sqlLower, "drop table"):
		return p.dropTableParser.Parse(sql)
	case strings.HasPrefix(sqlLower, "create index") || strings.HasPrefix(sqlLower, "create unique index"):
		return p.createIndexParser.Parse(sql)
	case strings.HasPrefix(sqlLower, "drop index"):
		return p.dropIndexParser.Parse(sql)
	case strings.HasPrefix(sqlLower, "create schema"):
		return p.createSchemaParser.Parse(sql)
	case strings.HasPrefix(sqlLower, "drop schema"):
		return p.dropSchemaParser.Parse(sql)
	case strings.HasPrefix(sqlLower, "create type"):
		return p.createEnumParser.Parse(sql)
	case strings.HasPrefix(sqlLower, "drop type"):
		return p.dropEnumParser.Parse(sql)
	default:
		return &UnknownStatement{Raw: sql}, nil
	}
}
