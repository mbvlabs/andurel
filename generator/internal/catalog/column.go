package catalog

import (
	"github.com/mbvlabs/andurel/generator/internal/validation"
)

// ForeignKey represents foreign key.
type ForeignKey struct {
	ReferencedTable  string
	ReferencedColumn string
}

// Column represents column.
type Column struct {
	Name            string
	DataType        string
	IsNullable      bool
	IsArray         bool
	Length          *int32
	Precision       *int32
	Scale           *int32
	DefaultVal      *string
	CreatedBy       string // migration file that added this column
	ModifiedBy      string // migration file that last modified this column
	IsPrimaryKey    bool
	IsUnique        bool
	IsAutoIncrement bool
	ForeignKey      *ForeignKey // nil if not a foreign key
}

// NewColumn creates a new column.
func NewColumn(name, dataType string) *Column {
	return &Column{
		Name:       name,
		DataType:   dataType,
		IsNullable: true, // default to nullable unless specified otherwise
	}
}

// SetNotNull sets not null.
func (c *Column) SetNotNull() *Column {
	c.IsNullable = false
	return c
}

// SetPrimaryKey sets primary key.
func (c *Column) SetPrimaryKey() *Column {
	c.IsPrimaryKey = true
	c.IsNullable = false // Primary keys are never nullable
	return c
}

// SetUnique sets unique.
func (c *Column) SetUnique() *Column {
	c.IsUnique = true
	return c
}

// SetAutoIncrement sets auto increment.
func (c *Column) SetAutoIncrement() *Column {
	c.IsAutoIncrement = true
	return c
}

// SetDefault sets default.
func (c *Column) SetDefault(defaultValue string) *Column {
	c.DefaultVal = &defaultValue
	return c
}

// SetLength sets length.
func (c *Column) SetLength(length int32) *Column {
	c.Length = &length
	return c
}

// SetPrecisionScale sets precision scale.
func (c *Column) SetPrecisionScale(precision, scale int32) *Column {
	c.Precision = &precision
	c.Scale = &scale
	return c
}

// SetArray sets array.
func (c *Column) SetArray() *Column {
	c.IsArray = true
	return c
}

// SetForeignKey sets foreign key.
func (c *Column) SetForeignKey(referencedTable, referencedColumn string) *Column {
	c.ForeignKey = &ForeignKey{
		ReferencedTable:  referencedTable,
		ReferencedColumn: referencedColumn,
	}
	return c
}

// SetCreatedBy sets created by.
func (c *Column) SetCreatedBy(migrationFile string) *Column {
	c.CreatedBy = migrationFile
	return c
}

// SetModifiedBy sets modified by.
func (c *Column) SetModifiedBy(migrationFile string) *Column {
	c.ModifiedBy = migrationFile
	return c
}

// Clone performs the clone operation.
func (c *Column) Clone() *Column {
	clone := &Column{
		Name:            c.Name,
		DataType:        c.DataType,
		IsNullable:      c.IsNullable,
		IsArray:         c.IsArray,
		CreatedBy:       c.CreatedBy,
		ModifiedBy:      c.ModifiedBy,
		IsPrimaryKey:    c.IsPrimaryKey,
		IsUnique:        c.IsUnique,
		IsAutoIncrement: c.IsAutoIncrement,
	}

	if c.Length != nil {
		length := *c.Length
		clone.Length = &length
	}

	if c.Precision != nil {
		precision := *c.Precision
		clone.Precision = &precision
	}

	if c.Scale != nil {
		scale := *c.Scale
		clone.Scale = &scale
	}

	if c.DefaultVal != nil {
		defaultVal := *c.DefaultVal
		clone.DefaultVal = &defaultVal
	}

	if c.ForeignKey != nil {
		clone.ForeignKey = &ForeignKey{
			ReferencedTable:  c.ForeignKey.ReferencedTable,
			ReferencedColumn: c.ForeignKey.ReferencedColumn,
		}
	}

	return clone
}

// ValidatePrimaryKeyDatatype performs the validate primary key datatype operation.
func (c *Column) ValidatePrimaryKeyDatatype(databaseType, migrationFile string) error {
	if c.IsPrimaryKey {
		return validation.ValidatePrimaryKeyDatatype(c.DataType, databaseType, migrationFile, c.Name)
	}
	return nil
}
