package catalog

type RelationType string

const (
	OneToMany  RelationType = "one_to_many"
	ManyToOne  RelationType = "many_to_one"
	ManyToMany RelationType = "many_to_many"
	OneToOne   RelationType = "one_to_one"
)

type Relation struct {
	Type            RelationType
	FromTable       string
	FromColumn      string
	ToTable         string
	ToColumn        string
	ForeignKey      *ForeignKey
	IsSelfReference bool
	JoinTable       string // for many-to-many relations
	JoinFromColumn  string // for many-to-many relations
	JoinToColumn    string // for many-to-many relations
}

type RelationshipGraph struct {
	Tables    map[string]*Table
	Relations map[string][]*Relation // keyed by table name
}

func NewRelationshipGraph() *RelationshipGraph {
	return &RelationshipGraph{
		Tables:    make(map[string]*Table),
		Relations: make(map[string][]*Relation),
	}
}

func (rg *RelationshipGraph) AddTable(table *Table) {
	rg.Tables[table.Name] = table
}

func (rg *RelationshipGraph) AddRelation(tableName string, relation *Relation) {
	if rg.Relations[tableName] == nil {
		rg.Relations[tableName] = make([]*Relation, 0)
	}
	rg.Relations[tableName] = append(rg.Relations[tableName], relation)
}

func (rg *RelationshipGraph) GetRelations(tableName string) []*Relation {
	return rg.Relations[tableName]
}

func (rg *RelationshipGraph) GetTable(tableName string) *Table {
	return rg.Tables[tableName]
}