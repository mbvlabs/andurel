package catalog

import (
	"fmt"
	"sync"
)

type Catalog struct {
	DefaultSchema string
	Schemas       map[string]*Schema
	mutex         sync.RWMutex
}

type Schema struct {
	Name   string
	Tables map[string]*Table
	Enums  map[string]*Enum
}

type Enum struct {
	Name      string
	Values    []string
	CreatedBy string
}

func NewCatalog(defaultSchema string) *Catalog {
	catalog := &Catalog{
		DefaultSchema: defaultSchema,
		Schemas:       make(map[string]*Schema),
	}

	catalog.Schemas[defaultSchema] = &Schema{
		Name:   defaultSchema,
		Tables: make(map[string]*Table),
		Enums:  make(map[string]*Enum),
	}

	return catalog
}

func (c *Catalog) GetSchema(name string) (*Schema, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if name == "" {
		name = c.DefaultSchema
	}

	schema, exists := c.Schemas[name]
	if !exists {
		return nil, fmt.Errorf("schema %s not found", name)
	}

	return schema, nil
}

func (c *Catalog) CreateSchema(name string) (*Schema, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, exists := c.Schemas[name]; exists {
		return nil, fmt.Errorf("schema %s already exists", name)
	}

	schema := &Schema{
		Name:   name,
		Tables: make(map[string]*Table),
		Enums:  make(map[string]*Enum),
	}

	c.Schemas[name] = schema
	return schema, nil
}

func (c *Catalog) GetTable(schemaName, tableName string) (*Table, error) {
	schema, err := c.GetSchema(schemaName)
	if err != nil {
		return nil, err
	}

	table, exists := schema.Tables[tableName]
	if !exists {
		return nil, fmt.Errorf(
			"table %s not found in schema %s",
			tableName,
			schemaName,
		)
	}

	return table, nil
}

func (c *Catalog) AddTable(schemaName string, table *Table) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if schemaName == "" {
		schemaName = c.DefaultSchema
	}

	schema, exists := c.Schemas[schemaName]
	if !exists {
		return fmt.Errorf("schema %s not found", schemaName)
	}

	if _, exists := schema.Tables[table.Name]; exists {
		return fmt.Errorf(
			"table %s already exists in schema %s",
			table.Name,
			schemaName,
		)
	}

	table.Schema = schemaName
	schema.Tables[table.Name] = table
	return nil
}

func (c *Catalog) DropTable(schemaName, tableName string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if schemaName == "" {
		schemaName = c.DefaultSchema
	}

	schema, exists := c.Schemas[schemaName]
	if !exists {
		return fmt.Errorf("schema %s not found", schemaName)
	}

	if _, exists := schema.Tables[tableName]; !exists {
		return fmt.Errorf(
			"table %s not found in schema %s",
			tableName,
			schemaName,
		)
	}

	delete(schema.Tables, tableName)
	return nil
}

func (c *Catalog) RenameTable(schemaName, oldName, newName string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if schemaName == "" {
		schemaName = c.DefaultSchema
	}

	schema, exists := c.Schemas[schemaName]
	if !exists {
		return fmt.Errorf("schema %s not found", schemaName)
	}

	table, exists := schema.Tables[oldName]
	if !exists {
		return fmt.Errorf(
			"table %s not found in schema %s",
			oldName,
			schemaName,
		)
	}

	if _, exists := schema.Tables[newName]; exists {
		return fmt.Errorf(
			"table %s already exists in schema %s",
			newName,
			schemaName,
		)
	}

	table.Name = newName
	schema.Tables[newName] = table
	delete(schema.Tables, oldName)

	return nil
}

type TableAlteration struct {
	Type      AlterationType
	Column    *Column
	OldName   string
	NewName   string
	IndexName string
	IndexDef  *Index
}

type AlterationType int

const (
	AddColumn AlterationType = iota
	DropColumn
	ModifyColumn
	RenameColumn
	AddIndex
	DropIndex
)

func (c *Catalog) AlterTable(
	schemaName, tableName string,
	alteration TableAlteration,
) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	table, err := c.getTableUnsafe(schemaName, tableName)
	if err != nil {
		return err
	}

	switch alteration.Type {
	case AddColumn:
		if alteration.Column == nil {
			return fmt.Errorf(
				"column definition required for ADD COLUMN operation",
			)
		}
		return table.AddColumn(alteration.Column)

	case DropColumn:
		if alteration.OldName == "" {
			return fmt.Errorf("column name required for DROP COLUMN operation")
		}
		return table.DropColumn(alteration.OldName)

	case ModifyColumn:
		if alteration.Column == nil {
			return fmt.Errorf(
				"column definition required for MODIFY COLUMN operation",
			)
		}
		return table.ModifyColumn(alteration.Column.Name, alteration.Column)

	case RenameColumn:
		if alteration.OldName == "" || alteration.NewName == "" {
			return fmt.Errorf(
				"old and new column names required for RENAME COLUMN operation",
			)
		}
		return table.RenameColumn(alteration.OldName, alteration.NewName)

	case AddIndex:
		if alteration.IndexDef == nil {
			return fmt.Errorf(
				"index definition required for ADD INDEX operation",
			)
		}
		return table.AddIndex(alteration.IndexDef)

	case DropIndex:
		if alteration.IndexName == "" {
			return fmt.Errorf("index name required for DROP INDEX operation")
		}
		return table.DropIndex(alteration.IndexName)

	default:
		return fmt.Errorf("unknown alteration type: %d", alteration.Type)
	}
}

func (c *Catalog) getTableUnsafe(schemaName, tableName string) (*Table, error) {
	if schemaName == "" {
		schemaName = c.DefaultSchema
	}

	schema, exists := c.Schemas[schemaName]
	if !exists {
		return nil, fmt.Errorf("schema %s not found", schemaName)
	}

	table, exists := schema.Tables[tableName]
	if !exists {
		return nil, fmt.Errorf(
			"table %s not found in schema %s",
			tableName,
			schemaName,
		)
	}

	return table, nil
}

func (c *Catalog) ListTables(schemaName string) ([]*Table, error) {
	schema, err := c.GetSchema(schemaName)
	if err != nil {
		return nil, err
	}

	var tables []*Table
	for _, table := range schema.Tables {
		tables = append(tables, table)
	}

	return tables, nil
}

func (c *Catalog) AddEnum(schemaName string, enum *Enum) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if schemaName == "" {
		schemaName = c.DefaultSchema
	}

	schema, exists := c.Schemas[schemaName]
	if !exists {
		return fmt.Errorf("schema %s not found", schemaName)
	}

	if _, exists := schema.Enums[enum.Name]; exists {
		return fmt.Errorf(
			"enum %s already exists in schema %s",
			enum.Name,
			schemaName,
		)
	}

	schema.Enums[enum.Name] = enum
	return nil
}

// GetRelatedTables returns tables that have foreign key relationships with the given table
func (c *Catalog) GetRelatedTables(tableName string) ([]*Table, error) {
	schema, err := c.GetSchema("")
	if err != nil {
		return nil, err
	}

	var relatedTables []*Table
	tableSet := make(map[string]bool)

	// Find tables that reference this table (one-to-many from this table's perspective)
	for _, table := range schema.Tables {
		for _, fk := range table.ForeignKeys {
			if fk.ReferencedTable == tableName {
				if !tableSet[table.Name] {
					relatedTables = append(relatedTables, table)
					tableSet[table.Name] = true
				}
			}
		}
	}

	// Find tables that this table references (many-to-one from this table's perspective)
	targetTable, err := c.GetTable("", tableName)
	if err != nil {
		return nil, err
	}

	for _, fk := range targetTable.ForeignKeys {
		if !tableSet[fk.ReferencedTable] {
			if refTable, err := c.GetTable("", fk.ReferencedTable); err == nil {
				relatedTables = append(relatedTables, refTable)
				tableSet[fk.ReferencedTable] = true
			}
		}
	}

	return relatedTables, nil
}

// GetRelationshipGraph builds a complete relationship graph for all tables in the catalog
func (c *Catalog) GetRelationshipGraph() (*RelationshipGraph, error) {
	schema, err := c.GetSchema("")
	if err != nil {
		return nil, err
	}

	graph := NewRelationshipGraph()

	// Add all tables to the graph
	for _, table := range schema.Tables {
		graph.AddTable(table)
	}

	// Build relations for each table
	for tableName := range schema.Tables {
		relations := c.discoverRelationsForTable(tableName, schema)
		for _, relation := range relations {
			graph.AddRelation(tableName, relation)
		}
	}

	return graph, nil
}

// discoverRelationsForTable discovers all relations for a specific table
func (c *Catalog) discoverRelationsForTable(tableName string, schema *Schema) []*Relation {
	var relations []*Relation
	table := schema.Tables[tableName]

	// One-to-Many and Many-to-One relations from foreign keys
	for _, fk := range table.ForeignKeys {
		// This is a many-to-one relation (this table has FK to another)
		relation := &Relation{
			Type:            ManyToOne,
			FromTable:       tableName,
			FromColumn:      fk.Column,
			ToTable:         fk.ReferencedTable,
			ToColumn:        fk.ReferencedColumn,
			ForeignKey:      fk,
			IsSelfReference: fk.ReferencedTable == tableName,
		}
		relations = append(relations, relation)
	}

	// Find one-to-many relations (other tables reference this table)
	for _, otherTable := range schema.Tables {
		if otherTable.Name == tableName {
			continue
		}

		for _, fk := range otherTable.ForeignKeys {
			if fk.ReferencedTable == tableName {
				// This is a one-to-many relation (other table has FK to this table)
				relation := &Relation{
					Type:            OneToMany,
					FromTable:       tableName,
					FromColumn:      fk.ReferencedColumn,
					ToTable:         otherTable.Name,
					ToColumn:        fk.Column,
					ForeignKey:      fk,
					IsSelfReference: otherTable.Name == tableName,
				}
				relations = append(relations, relation)
			}
		}
	}

	// TODO: Detect many-to-many relations through junction tables
	// This would look for tables with exactly 2 foreign keys that reference different tables

	return relations
}
