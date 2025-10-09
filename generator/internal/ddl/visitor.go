package ddl

import "github.com/mbvlabs/andurel/generator/internal/catalog"

// StatementType represents the type of DDL statement
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

// TableVisitor handles table-related DDL operations
type TableVisitor interface {
	VisitCreateTable(stmt *CreateTableStatement) error
	VisitAlterTable(stmt *AlterTableStatement) error
	VisitDropTable(stmt *DropTableStatement) error
}

// IndexVisitor handles index-related DDL operations
type IndexVisitor interface {
	VisitCreateIndex(stmt *CreateIndexStatement) error
	VisitDropIndex(stmt *DropIndexStatement) error
}

// SchemaVisitor handles schema-related DDL operations
type SchemaVisitor interface {
	VisitCreateSchema(stmt *CreateSchemaStatement) error
	VisitDropSchema(stmt *DropSchemaStatement) error
}

// EnumVisitor handles enum-related DDL operations
type EnumVisitor interface {
	VisitCreateEnum(stmt *CreateEnumStatement) error
	VisitDropEnum(stmt *DropEnumStatement) error
}

// DDLVisitor combines all DDL visitor interfaces
type DDLVisitor interface {
	TableVisitor
	IndexVisitor
	SchemaVisitor
	EnumVisitor
}

// Base statement interfaces
type Statement interface {
	Accept(visitor DDLVisitor) error
	GetRaw() string
	GetType() StatementType
}

// Statement implementations
type CreateTableStatement struct {
	Raw         string
	SchemaName  string
	TableName   string
	IfNotExists bool
	Columns     []*catalog.Column
}

func (s *CreateTableStatement) Accept(visitor DDLVisitor) error {
	return visitor.VisitCreateTable(s)
}

func (s *CreateTableStatement) GetRaw() string {
	return s.Raw
}

func (s *CreateTableStatement) GetType() StatementType {
	return CreateTable
}

type AlterTableStatement struct {
	Raw            string
	SchemaName     string
	TableName      string
	AlterOperation string
	ColumnName     string
	NewColumnName  string
	NewTableName   string
	ColumnDef      *catalog.Column
	ColumnChanges  map[string]any
	Operations     []string
}

func (s *AlterTableStatement) Accept(visitor DDLVisitor) error {
	return visitor.VisitAlterTable(s)
}

func (s *AlterTableStatement) GetRaw() string {
	return s.Raw
}

func (s *AlterTableStatement) GetType() StatementType {
	return AlterTable
}

type DropTableStatement struct {
	Raw        string
	SchemaName string
	TableName  string
	IfExists   bool
}

func (s *DropTableStatement) Accept(visitor DDLVisitor) error {
	return visitor.VisitDropTable(s)
}

func (s *DropTableStatement) GetRaw() string {
	return s.Raw
}

func (s *DropTableStatement) GetType() StatementType {
	return DropTable
}

type CreateIndexStatement struct {
	Raw string
}

func (s *CreateIndexStatement) Accept(visitor DDLVisitor) error {
	return visitor.VisitCreateIndex(s)
}

func (s *CreateIndexStatement) GetRaw() string {
	return s.Raw
}

func (s *CreateIndexStatement) GetType() StatementType {
	return CreateIndex
}

type DropIndexStatement struct {
	Raw string
}

func (s *DropIndexStatement) Accept(visitor DDLVisitor) error {
	return visitor.VisitDropIndex(s)
}

func (s *DropIndexStatement) GetRaw() string {
	return s.Raw
}

func (s *DropIndexStatement) GetType() StatementType {
	return DropIndex
}

type CreateSchemaStatement struct {
	Raw        string
	SchemaName string
}

func (s *CreateSchemaStatement) Accept(visitor DDLVisitor) error {
	return visitor.VisitCreateSchema(s)
}

func (s *CreateSchemaStatement) GetRaw() string {
	return s.Raw
}

func (s *CreateSchemaStatement) GetType() StatementType {
	return CreateSchema
}

type DropSchemaStatement struct {
	Raw        string
	SchemaName string
}

func (s *DropSchemaStatement) Accept(visitor DDLVisitor) error {
	return visitor.VisitDropSchema(s)
}

func (s *DropSchemaStatement) GetRaw() string {
	return s.Raw
}

func (s *DropSchemaStatement) GetType() StatementType {
	return DropSchema
}

type CreateEnumStatement struct {
	Raw        string
	SchemaName string
	EnumName   string
	EnumDef    *catalog.Enum
}

func (s *CreateEnumStatement) Accept(visitor DDLVisitor) error {
	return visitor.VisitCreateEnum(s)
}

func (s *CreateEnumStatement) GetRaw() string {
	return s.Raw
}

func (s *CreateEnumStatement) GetType() StatementType {
	return CreateEnum
}

type DropEnumStatement struct {
	Raw        string
	SchemaName string
	EnumName   string
}

func (s *DropEnumStatement) Accept(visitor DDLVisitor) error {
	return visitor.VisitDropEnum(s)
}

func (s *DropEnumStatement) GetRaw() string {
	return s.Raw
}

func (s *DropEnumStatement) GetType() StatementType {
	return DropEnum
}

type UnknownStatement struct {
	Raw string
}

func (s *UnknownStatement) Accept(visitor DDLVisitor) error {
	return nil
}

func (s *UnknownStatement) GetRaw() string {
	return s.Raw
}

func (s *UnknownStatement) GetType() StatementType {
	return Unknown
}

// DDLStatement provides backward compatibility for existing code
// TODO: Remove this once all code is updated to use the new visitor pattern
type DDLStatement struct {
	Type           StatementType
	SchemaName     string
	TableName      string
	ColumnDef      *catalog.Column
	Operation      string
	Raw            string
	IndexDef       *catalog.Index
	EnumDef        *catalog.Enum
	IfNotExists    bool
	AlterOperation string
	ColumnName     string
	NewColumnName  string
	NewTableName   string
	ColumnChanges  map[string]any
}

// ParseDDLStatement provides backward compatibility
// TODO: Remove this once all code is updated to use the new visitor pattern
func ParseDDLStatement(sql, migrationFile string, databaseType string) (*DDLStatement, error) {
	parser := NewDDLParser()
	stmt, err := parser.Parse(sql, migrationFile, databaseType)
	if err != nil {
		return nil, err
	}
	if stmt == nil {
		return nil, nil
	}

	// Convert new Statement interface to legacy DDLStatement
	ddlStmt := &DDLStatement{
		Type: stmt.GetType(),
		Raw:  stmt.GetRaw(),
	}

	// Type-specific conversions
	switch s := stmt.(type) {
	case *CreateTableStatement:
		ddlStmt.SchemaName = s.SchemaName
		ddlStmt.TableName = s.TableName
		ddlStmt.IfNotExists = s.IfNotExists
		ddlStmt.ColumnDef = nil // Columns are handled differently in new system
	case *AlterTableStatement:
		ddlStmt.SchemaName = s.SchemaName
		ddlStmt.TableName = s.TableName
		ddlStmt.AlterOperation = s.AlterOperation
		ddlStmt.ColumnName = s.ColumnName
		ddlStmt.NewColumnName = s.NewColumnName
		ddlStmt.NewTableName = s.NewTableName
		ddlStmt.ColumnDef = s.ColumnDef
		ddlStmt.ColumnChanges = s.ColumnChanges
	case *DropTableStatement:
		ddlStmt.SchemaName = s.SchemaName
		ddlStmt.TableName = s.TableName
		ddlStmt.IfNotExists = s.IfExists
	case *CreateIndexStatement:
		ddlStmt.IndexDef = nil // TODO: Convert from new index format
	case *DropIndexStatement:
		ddlStmt.IndexDef = nil // TODO: Convert from new index format
	case *CreateSchemaStatement:
		ddlStmt.SchemaName = s.SchemaName
	case *DropSchemaStatement:
		ddlStmt.SchemaName = s.SchemaName
	case *CreateEnumStatement:
		ddlStmt.SchemaName = s.SchemaName
		ddlStmt.EnumDef = s.EnumDef
	case *DropEnumStatement:
		ddlStmt.SchemaName = s.SchemaName
	}

	return ddlStmt, nil
}
