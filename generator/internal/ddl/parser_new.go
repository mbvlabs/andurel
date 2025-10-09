package ddl

import (
	"strings"
)

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
