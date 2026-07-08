package ddl

import "github.com/mbvlabs/andurel/generator/internal/catalog"

// StatementType represents the type of DDL statement
type StatementType int

const (
	// CreateTable is a constant value for create table.
	CreateTable StatementType = iota
	// AlterTable is a constant value for alter table.
	AlterTable
	// DropTable is a constant value for drop table.
	DropTable
	// CreateIndex is a constant value for create index.
	CreateIndex
	// DropIndex is a constant value for drop index.
	DropIndex
	// CreateSchema is a constant value for create schema.
	CreateSchema
	// DropSchema is a constant value for drop schema.
	DropSchema
	// CreateEnum is a constant value for create enum.
	CreateEnum
	// DropEnum is a constant value for drop enum.
	DropEnum
	// Unknown is a constant value for unknown.
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

// Accept performs the accept operation.
func (s *CreateTableStatement) Accept(visitor DDLVisitor) error {
	return visitor.VisitCreateTable(s)
}

// GetRaw returns raw.
func (s *CreateTableStatement) GetRaw() string {
	return s.Raw
}

// GetType returns type.
func (s *CreateTableStatement) GetType() StatementType {
	return CreateTable
}

// AlterTableStatement represents alter table statement.
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

// Accept performs the accept operation.
func (s *AlterTableStatement) Accept(visitor DDLVisitor) error {
	return visitor.VisitAlterTable(s)
}

// GetRaw returns raw.
func (s *AlterTableStatement) GetRaw() string {
	return s.Raw
}

// GetType returns type.
func (s *AlterTableStatement) GetType() StatementType {
	return AlterTable
}

// DropTableStatement represents drop table statement.
type DropTableStatement struct {
	Raw        string
	SchemaName string
	TableName  string
	IfExists   bool
}

// Accept performs the accept operation.
func (s *DropTableStatement) Accept(visitor DDLVisitor) error {
	return visitor.VisitDropTable(s)
}

// GetRaw returns raw.
func (s *DropTableStatement) GetRaw() string {
	return s.Raw
}

// GetType returns type.
func (s *DropTableStatement) GetType() StatementType {
	return DropTable
}

// CreateIndexStatement represents create index statement.
type CreateIndexStatement struct {
	Raw string
}

// Accept performs the accept operation.
func (s *CreateIndexStatement) Accept(visitor DDLVisitor) error {
	return visitor.VisitCreateIndex(s)
}

// GetRaw returns raw.
func (s *CreateIndexStatement) GetRaw() string {
	return s.Raw
}

// GetType returns type.
func (s *CreateIndexStatement) GetType() StatementType {
	return CreateIndex
}

// DropIndexStatement represents drop index statement.
type DropIndexStatement struct {
	Raw string
}

// Accept performs the accept operation.
func (s *DropIndexStatement) Accept(visitor DDLVisitor) error {
	return visitor.VisitDropIndex(s)
}

// GetRaw returns raw.
func (s *DropIndexStatement) GetRaw() string {
	return s.Raw
}

// GetType returns type.
func (s *DropIndexStatement) GetType() StatementType {
	return DropIndex
}

// CreateSchemaStatement represents create schema statement.
type CreateSchemaStatement struct {
	Raw        string
	SchemaName string
}

// Accept performs the accept operation.
func (s *CreateSchemaStatement) Accept(visitor DDLVisitor) error {
	return visitor.VisitCreateSchema(s)
}

// GetRaw returns raw.
func (s *CreateSchemaStatement) GetRaw() string {
	return s.Raw
}

// GetType returns type.
func (s *CreateSchemaStatement) GetType() StatementType {
	return CreateSchema
}

// DropSchemaStatement represents drop schema statement.
type DropSchemaStatement struct {
	Raw        string
	SchemaName string
}

// Accept performs the accept operation.
func (s *DropSchemaStatement) Accept(visitor DDLVisitor) error {
	return visitor.VisitDropSchema(s)
}

// GetRaw returns raw.
func (s *DropSchemaStatement) GetRaw() string {
	return s.Raw
}

// GetType returns type.
func (s *DropSchemaStatement) GetType() StatementType {
	return DropSchema
}

// CreateEnumStatement represents create enum statement.
type CreateEnumStatement struct {
	Raw        string
	SchemaName string
	EnumName   string
	EnumDef    *catalog.Enum
}

// Accept performs the accept operation.
func (s *CreateEnumStatement) Accept(visitor DDLVisitor) error {
	return visitor.VisitCreateEnum(s)
}

// GetRaw returns raw.
func (s *CreateEnumStatement) GetRaw() string {
	return s.Raw
}

// GetType returns type.
func (s *CreateEnumStatement) GetType() StatementType {
	return CreateEnum
}

// DropEnumStatement represents drop enum statement.
type DropEnumStatement struct {
	Raw        string
	SchemaName string
	EnumName   string
}

// Accept performs the accept operation.
func (s *DropEnumStatement) Accept(visitor DDLVisitor) error {
	return visitor.VisitDropEnum(s)
}

// GetRaw returns raw.
func (s *DropEnumStatement) GetRaw() string {
	return s.Raw
}

// GetType returns type.
func (s *DropEnumStatement) GetType() StatementType {
	return DropEnum
}

// UnknownStatement represents unknown statement.
type UnknownStatement struct {
	Raw string
}

// Accept performs the accept operation.
func (s *UnknownStatement) Accept(visitor DDLVisitor) error {
	return nil
}

// GetRaw returns raw.
func (s *UnknownStatement) GetRaw() string {
	return s.Raw
}

// GetType returns type.
func (s *UnknownStatement) GetType() StatementType {
	return Unknown
}
