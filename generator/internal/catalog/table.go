package catalog

import (
	"fmt"
)

type Table struct {
	Schema    string
	Name      string
	Columns   []*Column
	Indexes   []*Index
	CreatedBy string // migration file that created this table
}

type Index struct {
	Name      string
	Columns   []string
	IsUnique  bool
	CreatedBy string
}

func NewTable(schema, name string) *Table {
	return &Table{
		Schema:  schema,
		Name:    name,
		Columns: make([]*Column, 0),
		Indexes: make([]*Index, 0),
	}
}

func (t *Table) AddColumn(column *Column) error {
	for _, existingCol := range t.Columns {
		if existingCol.Name == column.Name {
			return fmt.Errorf(
				"column %s already exists in table %s",
				column.Name,
				t.Name,
			)
		}
	}

	t.Columns = append(t.Columns, column)
	return nil
}

func (t *Table) GetColumn(name string) (*Column, error) {
	for _, col := range t.Columns {
		if col.Name == name {
			return col, nil
		}
	}
	return nil, fmt.Errorf("column %s not found in table %s", name, t.Name)
}

func (t *Table) DropColumn(name string) error {
	for i, col := range t.Columns {
		if col.Name == name {
			t.Columns = append(t.Columns[:i], t.Columns[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("column %s not found in table %s", name, t.Name)
}

func (t *Table) ModifyColumn(name string, newColumn *Column) error {
	for i, col := range t.Columns {
		if col.Name == name {
			newColumn.CreatedBy = col.CreatedBy
			t.Columns[i] = newColumn
			return nil
		}
	}
	return fmt.Errorf("column %s not found in table %s", name, t.Name)
}

func (t *Table) RenameColumn(oldName, newName string) error {
	col, err := t.GetColumn(oldName)
	if err != nil {
		return err
	}

	if _, err := t.GetColumn(newName); err == nil {
		return fmt.Errorf(
			"column %s already exists in table %s",
			newName,
			t.Name,
		)
	}

	col.Name = newName
	return nil
}

func (t *Table) AddIndex(index *Index) error {
	for _, existingIdx := range t.Indexes {
		if existingIdx.Name == index.Name {
			return fmt.Errorf(
				"index %s already exists in table %s",
				index.Name,
				t.Name,
			)
		}
	}

	t.Indexes = append(t.Indexes, index)
	return nil
}

func (t *Table) DropIndex(name string) error {
	for i, idx := range t.Indexes {
		if idx.Name == name {
			t.Indexes = append(t.Indexes[:i], t.Indexes[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("index %s not found in table %s", name, t.Name)
}

func (t *Table) GetPrimaryKeyColumns() []*Column {
	var pkColumns []*Column
	for _, col := range t.Columns {
		if col.IsPrimaryKey {
			pkColumns = append(pkColumns, col)
		}
	}
	return pkColumns
}

func (t *Table) SetCreatedBy(migrationFile string) *Table {
	t.CreatedBy = migrationFile
	return t
}

func (t *Table) Clone() *Table {
	clone := &Table{
		Schema:    t.Schema,
		Name:      t.Name,
		CreatedBy: t.CreatedBy,
		Columns:   make([]*Column, len(t.Columns)),
		Indexes:   make([]*Index, len(t.Indexes)),
	}

	for i, col := range t.Columns {
		clone.Columns[i] = col.Clone()
	}

	for i, idx := range t.Indexes {
		clone.Indexes[i] = &Index{
			Name:      idx.Name,
			Columns:   append([]string(nil), idx.Columns...),
			IsUnique:  idx.IsUnique,
			CreatedBy: idx.CreatedBy,
		}
	}

	return clone
}
