package catalog

type ReferentialAction string

const (
	NoAction   ReferentialAction = "NO ACTION"
	Restrict   ReferentialAction = "RESTRICT"
	Cascade    ReferentialAction = "CASCADE"
	SetNull    ReferentialAction = "SET NULL"
	SetDefault ReferentialAction = "SET DEFAULT"
)

type ForeignKey struct {
	Name             string
	Column           string
	ReferencedSchema string
	ReferencedTable  string
	ReferencedColumn string
	OnUpdate         ReferentialAction
	OnDelete         ReferentialAction
	CreatedBy        string // migration file
}

func NewForeignKey(name, column, referencedTable, referencedColumn string) *ForeignKey {
	return &ForeignKey{
		Name:             name,
		Column:           column,
		ReferencedTable:  referencedTable,
		ReferencedColumn: referencedColumn,
		OnUpdate:         NoAction,
		OnDelete:         NoAction,
	}
}

func (fk *ForeignKey) SetReferencedSchema(schema string) *ForeignKey {
	fk.ReferencedSchema = schema
	return fk
}

func (fk *ForeignKey) SetOnUpdate(action ReferentialAction) *ForeignKey {
	fk.OnUpdate = action
	return fk
}

func (fk *ForeignKey) SetOnDelete(action ReferentialAction) *ForeignKey {
	fk.OnDelete = action
	return fk
}

func (fk *ForeignKey) SetCreatedBy(migrationFile string) *ForeignKey {
	fk.CreatedBy = migrationFile
	return fk
}