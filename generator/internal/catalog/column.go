package catalog

import (
	"github.com/mbvlabs/andurel/generator/internal/validation"
)

type Column struct {
	Name         string
	DataType     string
	IsNullable   bool
	IsArray      bool
	Length       *int32
	Precision    *int32
	Scale        *int32
	DefaultVal   *string
	CreatedBy    string // migration file that added this column
	ModifiedBy   string // migration file that last modified this column
	IsPrimaryKey bool
	IsUnique     bool
}

func NewColumn(name, dataType string) *Column {
	return &Column{
		Name:       name,
		DataType:   dataType,
		IsNullable: true, // default to nullable unless specified otherwise
	}
}

func (c *Column) SetNotNull() *Column {
	c.IsNullable = false
	return c
}

func (c *Column) SetPrimaryKey() *Column {
	c.IsPrimaryKey = true
	c.IsNullable = false // Primary keys are never nullable
	return c
}

func (c *Column) SetUnique() *Column {
	c.IsUnique = true
	return c
}

func (c *Column) SetDefault(defaultValue string) *Column {
	c.DefaultVal = &defaultValue
	return c
}

func (c *Column) SetLength(length int32) *Column {
	c.Length = &length
	return c
}

func (c *Column) SetPrecisionScale(precision, scale int32) *Column {
	c.Precision = &precision
	c.Scale = &scale
	return c
}

func (c *Column) SetArray() *Column {
	c.IsArray = true
	return c
}

func (c *Column) SetCreatedBy(migrationFile string) *Column {
	c.CreatedBy = migrationFile
	return c
}

func (c *Column) SetModifiedBy(migrationFile string) *Column {
	c.ModifiedBy = migrationFile
	return c
}

func (c *Column) Clone() *Column {
	clone := &Column{
		Name:         c.Name,
		DataType:     c.DataType,
		IsNullable:   c.IsNullable,
		IsArray:      c.IsArray,
		CreatedBy:    c.CreatedBy,
		ModifiedBy:   c.ModifiedBy,
		IsPrimaryKey: c.IsPrimaryKey,
		IsUnique:     c.IsUnique,
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

	return clone
}

func (c *Column) ValidatePrimaryKeyDatatype(databaseType, migrationFile string) error {
	if c.IsPrimaryKey {
		return validation.ValidatePrimaryKeyDatatype(c.DataType, databaseType, migrationFile, c.Name)
	}
	return nil
}
