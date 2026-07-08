package ddl

import (
	"fmt"
	"testing"
)

type recordingVisitor struct {
	visited []string
	err     error
}

func (v *recordingVisitor) visit(name string) error {
	v.visited = append(v.visited, name)
	return v.err
}

func (v *recordingVisitor) VisitCreateTable(*CreateTableStatement) error {
	return v.visit("create_table")
}

func (v *recordingVisitor) VisitAlterTable(*AlterTableStatement) error {
	return v.visit("alter_table")
}

func (v *recordingVisitor) VisitDropTable(*DropTableStatement) error {
	return v.visit("drop_table")
}

func (v *recordingVisitor) VisitCreateIndex(*CreateIndexStatement) error {
	return v.visit("create_index")
}

func (v *recordingVisitor) VisitDropIndex(*DropIndexStatement) error {
	return v.visit("drop_index")
}

func (v *recordingVisitor) VisitCreateSchema(*CreateSchemaStatement) error {
	return v.visit("create_schema")
}

func (v *recordingVisitor) VisitDropSchema(*DropSchemaStatement) error {
	return v.visit("drop_schema")
}

func (v *recordingVisitor) VisitCreateEnum(*CreateEnumStatement) error {
	return v.visit("create_enum")
}

func (v *recordingVisitor) VisitDropEnum(*DropEnumStatement) error {
	return v.visit("drop_enum")
}

func TestStatementAccessorsAndAccept(t *testing.T) {
	tests := []struct {
		name      string
		statement Statement
		wantType  StatementType
		wantVisit string
	}{
		{name: "create table", statement: &CreateTableStatement{Raw: "create table"}, wantType: CreateTable, wantVisit: "create_table"},
		{name: "alter table", statement: &AlterTableStatement{Raw: "alter table"}, wantType: AlterTable, wantVisit: "alter_table"},
		{name: "drop table", statement: &DropTableStatement{Raw: "drop table"}, wantType: DropTable, wantVisit: "drop_table"},
		{name: "create index", statement: &CreateIndexStatement{Raw: "create index"}, wantType: CreateIndex, wantVisit: "create_index"},
		{name: "drop index", statement: &DropIndexStatement{Raw: "drop index"}, wantType: DropIndex, wantVisit: "drop_index"},
		{name: "create schema", statement: &CreateSchemaStatement{Raw: "create schema"}, wantType: CreateSchema, wantVisit: "create_schema"},
		{name: "drop schema", statement: &DropSchemaStatement{Raw: "drop schema"}, wantType: DropSchema, wantVisit: "drop_schema"},
		{name: "create enum", statement: &CreateEnumStatement{Raw: "create enum"}, wantType: CreateEnum, wantVisit: "create_enum"},
		{name: "drop enum", statement: &DropEnumStatement{Raw: "drop enum"}, wantType: DropEnum, wantVisit: "drop_enum"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			visitor := &recordingVisitor{}

			if got := tt.statement.GetRaw(); got != tt.statement.GetRaw() || got == "" {
				t.Fatalf("GetRaw = %q", got)
			}
			if got := tt.statement.GetType(); got != tt.wantType {
				t.Fatalf("GetType = %v, want %v", got, tt.wantType)
			}
			if err := tt.statement.Accept(visitor); err != nil {
				t.Fatalf("Accept returned error: %v", err)
			}
			if len(visitor.visited) != 1 || visitor.visited[0] != tt.wantVisit {
				t.Fatalf("visited = %#v, want %q", visitor.visited, tt.wantVisit)
			}
		})
	}
}

func TestStatementAcceptPropagatesVisitorError(t *testing.T) {
	wantErr := fmt.Errorf("visitor failed")
	visitor := &recordingVisitor{err: wantErr}

	if err := (&CreateTableStatement{Raw: "create table"}).Accept(visitor); err != wantErr {
		t.Fatalf("Accept error = %v, want %v", err, wantErr)
	}
}

func TestUnknownStatementAccessors(t *testing.T) {
	stmt := &UnknownStatement{Raw: "select 1"}
	visitor := &recordingVisitor{err: fmt.Errorf("should not be called")}

	if got := stmt.GetRaw(); got != "select 1" {
		t.Fatalf("GetRaw = %q", got)
	}
	if got := stmt.GetType(); got != Unknown {
		t.Fatalf("GetType = %v, want Unknown", got)
	}
	if err := stmt.Accept(visitor); err != nil {
		t.Fatalf("Unknown Accept should ignore visitor, got %v", err)
	}
	if len(visitor.visited) != 0 {
		t.Fatalf("Unknown Accept should not visit, got %#v", visitor.visited)
	}
}
