package ddl

import (
	"fmt"
	"regexp"
	"strings"
)

// DropTableParser handles DROP TABLE statements
type DropTableParser struct{}

func NewDropTableParser() *DropTableParser {
	return &DropTableParser{}
}

func (p *DropTableParser) Parse(sql string) (*DropTableStatement, error) {
	dropRegex, err := regexp.Compile(
		`(?i)drop\s+table(?:\s+if\s+exists)?\s+(?:(\w+)\.)?(\w+)`,
	)
	if err != nil {
		return nil, err
	}
	matches := dropRegex.FindStringSubmatch(sql)

	if len(matches) < 3 {
		return nil, fmt.Errorf("invalid DROP TABLE syntax: %s", sql)
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

func NewCreateIndexParser() *CreateIndexParser {
	return &CreateIndexParser{}
}

func (p *CreateIndexParser) Parse(sql string) (*CreateIndexStatement, error) {
	return &CreateIndexStatement{
		Raw: sql,
	}, nil
}

// DropIndexParser handles DROP INDEX statements
type DropIndexParser struct{}

func NewDropIndexParser() *DropIndexParser {
	return &DropIndexParser{}
}

func (p *DropIndexParser) Parse(sql string) (*DropIndexStatement, error) {
	return &DropIndexStatement{
		Raw: sql,
	}, nil
}

// CreateSchemaParser handles CREATE SCHEMA statements
type CreateSchemaParser struct{}

func NewCreateSchemaParser() *CreateSchemaParser {
	return &CreateSchemaParser{}
}

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

func NewDropSchemaParser() *DropSchemaParser {
	return &DropSchemaParser{}
}

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

func NewCreateEnumParser() *CreateEnumParser {
	return &CreateEnumParser{}
}

func (p *CreateEnumParser) Parse(sql string) (*CreateEnumStatement, error) {
	enumRegex, err := regexp.Compile(`(?i)create\s+type\s+(?:(\w+)\.)?(\w+)\s+as\s+enum`)
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

	return &CreateEnumStatement{
		Raw:        sql,
		SchemaName: schemaName,
		EnumName:   enumName,
	}, nil
}

// DropEnumParser handles DROP TYPE (enum) statements
type DropEnumParser struct{}

func NewDropEnumParser() *DropEnumParser {
	return &DropEnumParser{}
}

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
