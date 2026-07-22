package ddl

import (
	"regexp"
	"strings"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
)

// DropTableParser handles DROP TABLE statements
type DropTableParser struct{}

// NewDropTableParser creates a new drop table parser.
func NewDropTableParser() *DropTableParser {
	return &DropTableParser{}
}

// Parse performs the parse operation.
func (p *DropTableParser) Parse(sql string) (*DropTableStatement, error) {
	dropRegex, err := regexp.Compile(
		`(?i)^drop\s+table(?:\s+if\s+exists)?\s+(?:(\w+)\.)?(\w+)\s*;?\s*$`,
	)
	if err != nil {
		return nil, err
	}
	matches := dropRegex.FindStringSubmatch(sql)

	if len(matches) < 3 {
		return nil, unsupportedStatement(sql, "DROP TABLE supports one unquoted table name without CASCADE")
	}

	schemaName := matches[1]
	tableName := matches[2]
	ifExists := strings.Contains(strings.ToLower(sql), "if exists")

	return &DropTableStatement{
		Raw:        sql,
		SchemaName: schemaName,
		TableName:  tableName,
		IfExists:   ifExists,
	}, nil
}

// CreateIndexParser handles CREATE INDEX statements
type CreateIndexParser struct{}

// NewCreateIndexParser creates a new create index parser.
func NewCreateIndexParser() *CreateIndexParser {
	return &CreateIndexParser{}
}

// Parse performs the parse operation.
func (p *CreateIndexParser) Parse(sql string) (*CreateIndexStatement, error) {
	return &CreateIndexStatement{
		Raw: sql,
	}, nil
}

// DropIndexParser handles DROP INDEX statements
type DropIndexParser struct{}

// NewDropIndexParser creates a new drop index parser.
func NewDropIndexParser() *DropIndexParser {
	return &DropIndexParser{}
}

// Parse performs the parse operation.
func (p *DropIndexParser) Parse(sql string) (*DropIndexStatement, error) {
	return &DropIndexStatement{
		Raw: sql,
	}, nil
}

// CreateSchemaParser handles CREATE SCHEMA statements
type CreateSchemaParser struct{}

// NewCreateSchemaParser creates a new create schema parser.
func NewCreateSchemaParser() *CreateSchemaParser {
	return &CreateSchemaParser{}
}

// Parse performs the parse operation.
func (p *CreateSchemaParser) Parse(sql string) (*CreateSchemaStatement, error) {
	schemaRegex, err := regexp.Compile(`(?i)create\s+schema\s+(?:if\s+not\s+exists\s+)?(\w+)`)
	if err != nil {
		return nil, err
	}
	matches := schemaRegex.FindStringSubmatch(sql)

	schemaName := ""
	if len(matches) > 1 {
		schemaName = matches[1]
	}

	return &CreateSchemaStatement{
		Raw:        sql,
		SchemaName: schemaName,
	}, nil
}

// DropSchemaParser handles DROP SCHEMA statements
type DropSchemaParser struct{}

// NewDropSchemaParser creates a new drop schema parser.
func NewDropSchemaParser() *DropSchemaParser {
	return &DropSchemaParser{}
}

// Parse performs the parse operation.
func (p *DropSchemaParser) Parse(sql string) (*DropSchemaStatement, error) {
	schemaRegex, err := regexp.Compile(`(?i)drop\s+schema\s+(?:if\s+exists\s+)?(\w+)`)
	if err != nil {
		return nil, err
	}
	matches := schemaRegex.FindStringSubmatch(sql)

	schemaName := ""
	if len(matches) > 1 {
		schemaName = matches[1]
	}

	return &DropSchemaStatement{
		Raw:        sql,
		SchemaName: schemaName,
	}, nil
}

// CreateEnumParser handles CREATE TYPE (enum) statements
type CreateEnumParser struct{}

// NewCreateEnumParser creates a new create enum parser.
func NewCreateEnumParser() *CreateEnumParser {
	return &CreateEnumParser{}
}

// Parse performs the parse operation.
func (p *CreateEnumParser) Parse(sql string) (*CreateEnumStatement, error) {
	enumRegex, err := regexp.Compile(`(?is)create\s+type\s+(?:(\w+)\.)?(\w+)\s+as\s+enum\s*\((.*)\)`)
	if err != nil {
		return nil, err
	}
	matches := enumRegex.FindStringSubmatch(sql)

	schemaName := ""
	enumName := ""
	if len(matches) > 2 {
		schemaName = matches[1]
		enumName = matches[2]
	}
	values := make([]string, 0)
	if len(matches) > 3 {
		valueRegex := regexp.MustCompile(`'((?:''|[^'])*)'`)
		for _, match := range valueRegex.FindAllStringSubmatch(matches[3], -1) {
			values = append(values, strings.ReplaceAll(match[1], "''", "'"))
		}
	}

	return &CreateEnumStatement{
		Raw:        sql,
		SchemaName: schemaName,
		EnumName:   enumName,
		EnumDef:    &catalog.Enum{Name: enumName, Values: values},
	}, nil
}

// DropEnumParser handles DROP TYPE (enum) statements
type DropEnumParser struct{}

// NewDropEnumParser creates a new drop enum parser.
func NewDropEnumParser() *DropEnumParser {
	return &DropEnumParser{}
}

// Parse performs the parse operation.
func (p *DropEnumParser) Parse(sql string) (*DropEnumStatement, error) {
	enumRegex, err := regexp.Compile(`(?i)drop\s+type\s+(?:if\s+exists\s+)?(?:(\w+)\.)?(\w+)`)
	if err != nil {
		return nil, err
	}
	matches := enumRegex.FindStringSubmatch(sql)

	schemaName := ""
	enumName := ""
	if len(matches) > 2 {
		schemaName = matches[1]
		enumName = matches[2]
	}

	return &DropEnumStatement{
		Raw:        sql,
		SchemaName: schemaName,
		EnumName:   enumName,
	}, nil
}
