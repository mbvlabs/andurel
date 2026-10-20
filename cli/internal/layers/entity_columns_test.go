package layers

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/internal/columns"
)

const entityColumnsNarsilcStub = `package narsilc

type Scanner interface {
	Scan(dest ...any) error
}

type Transformer[Row any] interface {
	Transform(Row) error
}

func Scan[T any](s Scanner, columns []string) (T, error) {
	var zero T
	return zero, nil
}

func Read[T, Row any](s Scanner, columns []string) (T, error) {
	return Scan[T](s, columns)
}

func FromRow[T, Row any](row Row) (T, error) {
	var zero T
	_ = row
	return zero, nil
}
`

const entityColumnsQueries = `package queries

import (
	"context"

	"github.com/mbvlabs/narsilc"
)

type Queries struct{}

type UserRow struct {
	ID       string §andurel:"id"§
	Email    string §andurel:"email"§
	Nickname string §andurel:"nickname"§
}

func (q *Queries) GetUser[T any](ctx context.Context) (T, error) {
	var scanner narsilc.Scanner
	return narsilc.Read[T, UserRow](scanner, []string{"id", "email"})
}

func (q *Queries) ListUsers[T any](ctx context.Context) *ListUsersBuilder[T] {
	return &ListUsersBuilder[T]{}
}

type ListUsersBuilder[T any] struct{}

func (b *ListUsersBuilder[T]) All() ([]T, error) {
	var rows narsilc.Scanner
	var probe T
	_, mapRow := any(&probe).(narsilc.Transformer[UserRow])
	cols := []string{"id", "email"}
	var item T
	var err error
	if mapRow {
		var scanned UserRow
		scanned, err = narsilc.Scan[UserRow](rows, cols)
		if err == nil {
			item, err = narsilc.FromRow[T, UserRow](scanned)
		}
	} else {
		item, err = narsilc.Scan[T](rows, cols)
	}
	_ = item
	return nil, err
}
`

const entityColumnsDomains = `package domains

import (
	"context"

	"example.com/app/domains/internal/queries"
)

type Users struct {
	queries *queries.Queries
}

type User struct {
	ID       string §andurel:"id"§
	Email    string §andurel:"email"§
	Nickname string §andurel:"nickname"§
}

func (u Users) Find(ctx context.Context) (User, error) {
	return u.queries.GetUser[User](ctx)
}

func (u Users) All(ctx context.Context) ([]User, error) {
	return u.queries.ListUsers[User](ctx).All()
}
`

func writeEntityColumnsProject(t *testing.T, overrides map[string]string) string {
	t.Helper()

	root := t.TempDir()
	files := map[string]string{
		"go.mod": `module example.com/app

go 1.27

require github.com/mbvlabs/narsilc v0.0.0

replace github.com/mbvlabs/narsilc => ./stubs/narsilc
`,
		"stubs/narsilc/go.mod":                "module github.com/mbvlabs/narsilc\n\ngo 1.27\n",
		"stubs/narsilc/narsilc.go":            entityColumnsNarsilcStub,
		"domains/internal/queries/queries.go": entityColumnsQueries,
		"domains/user.go":                     entityColumnsDomains,
	}
	maps.Copy(files, overrides)
	for path, contents := range files {
		writeProjectFile(t, root, path, fixtureSource(contents))
	}
	return root
}

func fixtureSource(source string) string {
	return strings.ReplaceAll(source, "§", "`")
}

func entityColumnFindings(t *testing.T, root string) []Violation {
	t.Helper()
	violations, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	found := make([]Violation, 0)
	for _, violation := range violations {
		if violation.Rule == "entity-columns" {
			found = append(found, violation)
		}
	}
	return found
}

func sourceLine(t *testing.T, root, rel, needle string) int {
	t.Helper()
	content := readProjectFile(t, root, rel)
	index := strings.Index(content, needle)
	if index < 0 {
		t.Fatalf("%s does not contain %q", rel, needle)
	}
	return strings.Count(content[:index], "\n") + 1
}

func readProjectFile(t *testing.T, root, rel string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(content)
}

func TestEntityColumnsReportsMissingColumnAtField(t *testing.T) {
	root := writeEntityColumnsProject(t, map[string]string{
		"domains/user.go": strings.Replace(
			entityColumnsDomains,
			`Email    string §andurel:"email"§`,
			`Email    string §andurel:"emial"§`,
			1,
		),
	})

	findings := entityColumnFindings(t, root)
	if len(findings) != 1 {
		t.Fatalf("findings = %#v, want one deduplicated entity-columns violation", findings)
	}
	finding := findings[0]
	if finding.File != "domains/user.go" {
		t.Fatalf("file = %q, want domains/user.go", finding.File)
	}
	if want := sourceLine(t, root, "domains/user.go", "\tEmail "); finding.Line != want {
		t.Fatalf("line = %d, want Email field line %d (%#v)", finding.Line, want, finding)
	}
	if !strings.Contains(finding.Message, `query column "email"`) {
		t.Fatalf("message = %q, want the missing email column", finding.Message)
	}
	if !strings.Contains(finding.Fix, `andurel:"<column>"`) {
		t.Fatalf("fix = %q, want the tag/sync fix", finding.Fix)
	}

	status := inspectStatus(t, root, "User")
	if !status.Checked || !hasMismatchKind(status, columns.KindMissingColumn) {
		t.Fatalf("status = %#v, want a checked missing-column mismatch", status)
	}
}

func TestEntityColumnsReportsMissingFieldAtType(t *testing.T) {
	root := writeEntityColumnsProject(t, map[string]string{
		"domains/user.go": `package domains

import (
	"context"

	"example.com/app/domains/internal/queries"
)

type Users struct {
	queries *queries.Queries
}

type User struct {
	ID string §andurel:"id"§
}

func (u Users) Find(ctx context.Context) (User, error) {
	return u.queries.GetUser[User](ctx)
}
`,
	})

	findings := entityColumnFindings(t, root)
	if len(findings) != 1 {
		t.Fatalf("findings = %#v, want one entity-columns violation", findings)
	}
	if want := sourceLine(t, root, "domains/user.go", "type User struct"); findings[0].Line != want {
		t.Fatalf("line = %d, want entity type line %d (%#v)", findings[0].Line, want, findings[0])
	}
	if !strings.Contains(findings[0].Message, `query column "email"`) {
		t.Fatalf("message = %q, want missing email", findings[0].Message)
	}
}

func TestEntityColumnsIgnoresUnexportedTaggedField(t *testing.T) {
	root := writeEntityColumnsProject(t, map[string]string{
		"domains/user.go": `package domains

import (
	"context"

	"example.com/app/domains/internal/queries"
)

type Users struct {
	queries *queries.Queries
}

type User struct {
	ID    string §andurel:"id"§
	email string §andurel:"email"§
}

func (u Users) Find(ctx context.Context) (User, error) {
	return u.queries.GetUser[User](ctx)
}
`,
	})

	findings := entityColumnFindings(t, root)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, `query column "email"`) {
		t.Fatalf("findings = %#v, want unexported email to be missing", findings)
	}
}

func TestEntityColumnsReportsDuplicateTags(t *testing.T) {
	root := writeEntityColumnsProject(t, map[string]string{
		"domains/user.go": `package domains

import (
	"context"

	"example.com/app/domains/internal/queries"
)

type Users struct {
	queries *queries.Queries
}

type User struct {
	ID          string §andurel:"id"§
	Email       string §andurel:"email"§
	BackupEmail string §andurel:"email"§
}

func (u Users) Find(ctx context.Context) (User, error) {
	return u.queries.GetUser[User](ctx)
}
`,
	})

	findings := entityColumnFindings(t, root)
	if len(findings) != 1 {
		t.Fatalf("findings = %#v, want one duplicate-tag violation", findings)
	}
	if want := sourceLine(t, root, "domains/user.go", "\tBackupEmail "); findings[0].Line != want {
		t.Fatalf("line = %d, want duplicate field line %d (%#v)", findings[0].Line, want, findings[0])
	}
	if !strings.Contains(findings[0].Message, "duplicate andurel:\"email\" tag") {
		t.Fatalf("message = %q, want duplicate tag", findings[0].Message)
	}

	status := inspectStatus(t, root, "User")
	if !hasMismatchKind(status, columns.KindDuplicateTag) {
		t.Fatalf("status = %#v, want duplicate-tag mismatch", status)
	}
}

func TestEntityColumnsExtraTagsAreNotesOnly(t *testing.T) {
	root := writeEntityColumnsProject(t, nil)

	if findings := entityColumnFindings(t, root); len(findings) != 0 {
		t.Fatalf("extra Nickname tag must not fail verify: %#v", findings)
	}
	status := inspectStatus(t, root, "User")
	if !status.Checked {
		t.Fatalf("status = %#v, want checked", status)
	}
	if len(status.Mismatches) != 0 {
		t.Fatalf("mismatches = %#v, want none", status.Mismatches)
	}
	if len(status.Unpopulated) != 1 ||
		status.Unpopulated[0].Column != "nickname" ||
		status.Unpopulated[0].Field != "Nickname" {
		t.Fatalf("unpopulated = %#v, want nickname", status.Unpopulated)
	}
}

func TestEntityColumnsTypeDifferencesAreNotesOnly(t *testing.T) {
	root := writeEntityColumnsProject(t, map[string]string{
		"domains/user.go": strings.Replace(
			entityColumnsDomains,
			`Email    string §andurel:"email"§`,
			`Email    int §andurel:"email"§`,
			1,
		),
	})

	if findings := entityColumnFindings(t, root); len(findings) != 0 {
		t.Fatalf("type difference must not fail verify: %#v", findings)
	}
	status := inspectStatus(t, root, "User")
	if !status.Checked {
		t.Fatalf("status = %#v, want checked", status)
	}
	if len(status.Mismatches) != 1 || status.Mismatches[0].Kind != columns.KindTypeDifference {
		t.Fatalf("mismatches = %#v, want one type-difference note", status.Mismatches)
	}
	mismatch := status.Mismatches[0]
	if mismatch.ExpectedType != "string" || mismatch.ActualType != "int" {
		t.Fatalf("type note = %#v, want string -> int", mismatch)
	}
	if columnsOK(status) == nil || !*columnsOK(status) {
		t.Fatalf("type difference must keep columns_ok true: %#v", status)
	}
}

func TestEntityColumnsSkipsTransformerPointerReceiver(t *testing.T) {
	root := writeEntityColumnsProject(t, map[string]string{
		"domains/user.go": `package domains

import (
	"context"

	"example.com/app/domains/internal/queries"
)

type Users struct {
	queries *queries.Queries
}

type User struct {
	ID    string §andurel:"id"§
	Email string §andurel:"emial"§
}

func (u *User) Transform(row queries.UserRow) error {
	return nil
}

func (u Users) Find(ctx context.Context) (User, error) {
	return u.queries.GetUser[User](ctx)
}
`,
	})

	if findings := entityColumnFindings(t, root); len(findings) != 0 {
		t.Fatalf("Transformer mapping must not be checked: %#v", findings)
	}
	status := inspectStatus(t, root, "User")
	if status.Checked || len(status.Mismatches) != 0 || len(status.Unpopulated) != 0 {
		t.Fatalf("status = %#v, want unchecked with no claims", status)
	}
}

func TestEntityColumnsUnrelatedTransformerInterfaceDoesNotSkip(t *testing.T) {
	root := writeEntityColumnsProject(t, map[string]string{
		"email/email.go": `package email

type UserRow struct{}

type Transformer[Row any] interface {
	Transform(Row) error
}
`,
		"domains/user.go": `package domains

import (
	"context"

	"example.com/app/domains/internal/queries"
	"example.com/app/email"
)

type Users struct {
	queries *queries.Queries
}

type User struct {
	ID    string §andurel:"id"§
	Email string §andurel:"emial"§
}

func (u *User) Transform(row email.UserRow) error {
	return nil
}

func (u Users) Find(ctx context.Context) (User, error) {
	return u.queries.GetUser[User](ctx)
}
`,
	})

	findings := entityColumnFindings(t, root)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, `query column "email"`) {
		t.Fatalf("an unrelated Transformer interface must not skip: %#v", findings)
	}
}

func TestEntityColumnsTreatsAndurelTagLiterally(t *testing.T) {
	root := writeEntityColumnsProject(t, map[string]string{
		"domains/user.go": strings.Replace(
			entityColumnsDomains,
			`Email    string §andurel:"email"§`,
			`Email    string §andurel:"email,omitempty"§`,
			1,
		),
	})

	findings := entityColumnFindings(t, root)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, `query column "email"`) {
		t.Fatalf(
			"commas are literal in narsilc tags; findings = %#v, want missing email",
			findings,
		)
	}
}

func TestEntityColumnsChecksManyBuilderColumns(t *testing.T) {
	queries := strings.Replace(
		entityColumnsQueries,
		`cols := []string{"id", "email"}`,
		`cols := []string{"id", "email", "nickname"}`,
		1,
	)
	root := writeEntityColumnsProject(t, map[string]string{
		"domains/internal/queries/queries.go": queries,
		"domains/user.go": strings.Replace(
			entityColumnsDomains,
			"\tNickname string §andurel:\"nickname\"§\n",
			"",
			1,
		),
	})

	findings := entityColumnFindings(t, root)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, `query column "nickname"`) {
		t.Fatalf("findings = %#v, want the builder-only nickname column", findings)
	}
	if !strings.Contains(findings[0].Message, "ListUsers") {
		t.Fatalf("message = %q, want the builder query name", findings[0].Message)
	}
}

func TestEntityColumnsHandlesQueryImportAndTypeAliases(t *testing.T) {
	root := writeEntityColumnsProject(t, map[string]string{
		"domains/user.go": `package domains

import (
	"context"

	q "example.com/app/domains/internal/queries"
)

type Users struct {
	queries *q.Queries
}

type User struct {
	ID    string §andurel:"id"§
	Email string §andurel:"emial"§
}

type Member = User

func (u Users) Find(ctx context.Context) (Member, error) {
	return u.queries.GetUser[Member](ctx)
}
`,
	})

	findings := entityColumnFindings(t, root)
	if len(findings) != 1 {
		t.Fatalf("findings = %#v, want one alias violation", findings)
	}
	if want := sourceLine(t, root, "domains/user.go", "\tEmail "); findings[0].Line != want {
		t.Fatalf("line = %d, want underlying User.Email line %d", findings[0].Line, want)
	}
}

func TestEntityColumnsIgnoresReadCallsOutsideGeneratedQueries(t *testing.T) {
	root := writeEntityColumnsProject(t, map[string]string{
		"internal/loader/loader.go": `package loader

import "github.com/mbvlabs/narsilc"

type Row struct {
	ID string §andurel:"id"§
}

func Load[T any]() (T, error) {
	var scanner narsilc.Scanner
	return narsilc.Read[T, Row](scanner, []string{"id", "not_a_domain_column"})
}
`,
		"domains/user.go": `package domains

import (
	"context"

	"example.com/app/domains/internal/queries"
	"example.com/app/internal/loader"
)

type Users struct {
	queries *queries.Queries
}

type User struct {
	ID    string §andurel:"id"§
	Email string §andurel:"email"§
}

func (u Users) Find(ctx context.Context) (User, error) {
	return u.queries.GetUser[User](ctx)
}

func LoadUser() (User, error) {
	return loader.Load[User]()
}
`,
	})

	if findings := entityColumnFindings(t, root); len(findings) != 0 {
		t.Fatalf("narsilc calls outside generated queries must not be app query findings: %#v", findings)
	}
}

func TestEntityColumnsIgnoresNonAppQueryModule(t *testing.T) {
	root := writeEntityColumnsProject(t, map[string]string{
		"go.mod": `module example.com/app

go 1.27

require (
	example.com/other v0.0.0
	github.com/mbvlabs/narsilc v0.0.0
)

replace example.com/other => ./other

replace github.com/mbvlabs/narsilc => ./stubs/narsilc
`,
		"other/go.mod": "module example.com/other\n\ngo 1.27\n",
		"other/externalqueries/queries.go": `package externalqueries

import "context"

type Queries struct{}

func (q Queries) GetUser[T any](ctx context.Context) (T, error) {
	var zero T
	return zero, nil
}
`,
		"domains/user.go": `package domains

import (
	"context"

	ext "example.com/other/externalqueries"
)

type User struct {
	ID    string §andurel:"id"§
	Email string §andurel:"emial"§
}

func (u User) Find(ctx context.Context) (User, error) {
	return ext.Queries{}.GetUser[User](ctx)
}
`,
	})

	if findings := entityColumnFindings(t, root); len(findings) != 0 {
		t.Fatalf("non-app query module must not be treated as app queries: %#v", findings)
	}
}

func TestEntityColumnsDeduplicatesTestVariants(t *testing.T) {
	root := writeEntityColumnsProject(t, map[string]string{
		"domains/user.go": strings.Replace(
			entityColumnsDomains,
			`Email    string §andurel:"email"§`,
			`Email    string §andurel:"emial"§`,
			1,
		),
		"domains/user_test.go": `package domains

import (
	"context"
	"testing"
)

func TestFindUser(t *testing.T) {
	var users Users
	_, _ = users.Find(context.Background())
}
`,
	})

	findings := entityColumnFindings(t, root)
	if len(findings) != 1 {
		t.Fatalf("findings = %#v, want test variants deduplicated", findings)
	}
}

func TestInspectEntityColumnsLeavesUnqueriedEntityUnchecked(t *testing.T) {
	root := writeEntityColumnsProject(t, map[string]string{
		"domains/orphan.go": `package domains

type Orphan struct {
	ID string §andurel:"id"§
}
`,
	})

	statuses, err := InspectEntityColumns(root)
	if err != nil {
		t.Fatalf("InspectEntityColumns: %v", err)
	}
	if _, ok := statuses["Orphan"]; ok {
		t.Fatalf("unqueried Orphan must be unchecked, got %#v", statuses["Orphan"])
	}
	if status := statuses["User"]; !status.Checked {
		t.Fatalf("User status = %#v, want checked", status)
	}
}

func TestInspectEntityColumnsReportsLoadFailure(t *testing.T) {
	root := writeEntityColumnsProject(t, map[string]string{
		"main.go": "package main\n\nfunc main() {\n\tvar value int = \"bad\"\n\t_ = value\n}\n",
	})

	_, err := InspectEntityColumns(root)
	if err == nil {
		t.Fatal("InspectEntityColumns should fail on an ill-typed project")
	}
	if !strings.Contains(err.Error(), "load project for entity-column check") ||
		!strings.Contains(err.Error(), "type-check project") {
		t.Fatalf("error = %v, want actionable load/type-check failure", err)
	}
}

func inspectStatus(t *testing.T, root, name string) columns.EntityStatus {
	t.Helper()
	statuses, err := InspectEntityColumns(root)
	if err != nil {
		t.Fatalf("InspectEntityColumns: %v", err)
	}
	status, ok := statuses[name]
	if !ok {
		t.Fatalf("InspectEntityColumns has no status for %s: %#v", name, statuses)
	}
	return status
}

func hasMismatchKind(status columns.EntityStatus, kind string) bool {
	for _, mismatch := range status.Mismatches {
		if mismatch.Kind == kind {
			return true
		}
	}
	return false
}

func columnsOK(status columns.EntityStatus) *bool {
	if !status.Checked {
		return nil
	}
	ok := true
	for _, mismatch := range status.Mismatches {
		switch mismatch.Kind {
		case columns.KindMissingColumn, columns.KindDuplicateTag:
			ok = false
		}
	}
	return &ok
}
