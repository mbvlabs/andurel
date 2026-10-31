package patterns

import (
	"go/parser"
	"go/token"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/internal/columns"
	layouttemplates "github.com/mbvlabs/andurel/v2/layout/templates"
)

func readTemplate(t *testing.T, name string) []byte {
	t.Helper()
	data, err := fs.ReadFile(layouttemplates.Files, name)
	if err != nil {
		t.Fatalf("read template %s: %v", name, err)
	}
	return data
}

func manifestFile(t *testing.T, manifest Manifest, path string) ManifestFile {
	t.Helper()
	for _, file := range manifest.Files {
		if file.Path == path {
			return file
		}
	}
	t.Fatalf("manifest has no %s: %#v", path, manifest.Files)
	return ManifestFile{}
}

func parsedStruct(t *testing.T, name, source, typeName string) []columns.StructField {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), name, source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v\n%s", name, err, source)
	}
	fields, ok := columns.StructFields(file, typeName)
	if !ok {
		t.Fatalf("%s declares no struct %s", name, typeName)
	}
	return fields
}

// TestRenderedEntityMatchesScaffold renders each scaffold entity from the
// scaffold's own models.go and query templates and requires the struct (fields,
// tags, types) to equal the hand-written domain entity in the scaffold
// templates. The query files supply the Create params that decide each
// timestamp's nullability.
func TestRenderedEntityMatchesScaffold(t *testing.T) {
	models := readTemplate(t, "domains_internal_queries_models.tmpl")

	for _, tc := range []struct {
		entity, table, query, template, path string
	}{
		{"User", "users", "domains_internal_queries_user.tmpl", "domains_user.tmpl", "domains/user.go"},
		{"Token", "tokens", "domains_internal_queries_token.tmpl", "domains_token.tmpl", "domains/token.go"},
	} {
		t.Run(tc.entity, func(t *testing.T) {
			project := Project{
				Module: "app",
				Models: models,
				Queries: map[string][]byte{
					"domains/internal/queries/" + tc.table + ".sql.go": readTemplate(t, tc.query),
				},
			}
			manifest, err := ManifestForProject(tc.entity, tc.table, project)
			if err != nil {
				t.Fatal(err)
			}
			if !manifest.RowFound {
				t.Fatalf("%sRow not found in the models template", tc.entity)
			}

			rendered := parsedStruct(t, tc.path, manifestFile(t, manifest, tc.path).Content, tc.entity)
			scaffold := parsedStruct(t, tc.template, string(readTemplate(t, tc.template)), tc.entity)
			if !slices.Equal(rendered, scaffold) {
				t.Fatalf("rendered %s = %#v\nscaffold %#v", tc.entity, rendered, scaffold)
			}
		})
	}
}

func TestRowManifestRendersDomainFiles(t *testing.T) {
	manifest, err := ManifestForProject("BlogPost", "posts", blogPostProject("example.com/app", blogPostCreateParams))
	if err != nil {
		t.Fatal(err)
	}
	if !manifest.RowFound || manifest.Row != "BlogPostRow" || len(manifest.Columns) != 6 {
		t.Fatalf("unexpected manifest header: %#v", manifest)
	}
	for _, file := range manifest.Files {
		if strings.HasPrefix(file.Path, "migrations/") {
			t.Errorf("row exists but the manifest still creates a migration: %s", file.Path)
		}
	}

	goFile := manifestFile(t, manifest, "domains/blog_post.go").Content
	if _, err := parser.ParseFile(token.NewFileSet(), "blog_post.go", goFile, 0); err != nil {
		t.Fatalf("rendered Go does not parse: %v\n%s", err, goFile)
	}
	for _, want := range []string{
		"\t\"time\"\n\t\"uuid\"\n\n\t\"example.com/app/domains/internal/queries\"\n",
		"func NewBlogPosts(db storage.Connection) BlogPosts {",
		"func (b BlogPosts) InTx(tx storage.Tx) BlogPosts {",
		"// andurel:table posts\ntype BlogPost struct {",
		"\tUpdatedAt time.Time `andurel:\"updated_at\"`",
		"builder := validation.NewBuilder()",
		`// TODO: rules for Title, e.g. builder.Required("title", b.Title)`,
		`// TODO: rules for URLSlug, e.g. builder.Required("urlSlug", b.URLSlug)`,
	} {
		if !strings.Contains(goFile, want) {
			t.Errorf("rendered Go is missing %q\n%s", want, goFile)
		}
	}
	// Every timestamp is NOT NULL in the fixture's Create params, so the entity
	// needs no pgx import, no nullable helper, and no Transform: narsilc scans the
	// columns into the tagged fields directly.
	if strings.Contains(goFile, "pgtype") || strings.Contains(goFile, "NullableTime") ||
		strings.Contains(goFile, "Transform") {
		t.Errorf("rendered Go references pgtype, a helper or Transform for NOT NULL timestamps only\n%s", goFile)
	}
	// Only non-pointer string fields get a TODO.
	if strings.Contains(goFile, "rules for Summary") || strings.Contains(goFile, "rules for Body") {
		t.Errorf("TODO rendered for a pointer or pgtype field\n%s", goFile)
	}

	sql := manifestFile(t, manifest, "domains/blog_post.sql").Content
	for _, want := range []string{
		"-- name: GetBlogPost :one\nSELECT *\nFROM posts\nWHERE id = $1\nLIMIT 1;\n",
		"-- name: ListBlogPosts :many\n-- @order id\n",
		"-- name: CountBlogPosts :one\n",
		"INSERT INTO posts (\n\tid,\n\tcreated_at,\n\tupdated_at,\n\ttitle,\n\turl_slug,\n\tsummary\n) VALUES (\n\t$1, $2, $3, $4, $5, $6\n)\nRETURNING *;\n",
		"SET\n\ttitle = $2,\n\turl_slug = $3,\n\tsummary = $4,\n\tupdated_at = $5\nWHERE id = $1\nRETURNING *;\n",
		"-- name: DeleteBlogPost :exec\nDELETE FROM posts\nWHERE id = $1;\n",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("rendered SQL is missing %q\n%s", want, sql)
		}
	}
}

const blogPostModels = `// Code generated by narsilc. DO NOT EDIT.

package queries

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"uuid"
)

type BlogPostRow struct {
	ID        uuid.UUID          ` + "`andurel:\"id\"`" + `
	CreatedAt time.Time          ` + "`andurel:\"created_at\"`" + `
	UpdatedAt pgtype.Timestamptz ` + "`andurel:\"updated_at\"`" + `
	Title     string             ` + "`andurel:\"title\"`" + `
	URLSlug   string             ` + "`andurel:\"url_slug\"`" + `
	Summary   *string            ` + "`andurel:\"summary\"`" + `
}
`

// blogPostCreateParams is the generated Create params for BlogPost. Its
// timestamps are NOT NULL: CreatedAt and UpdatedAt are time.Time.
const blogPostCreateParams = `package queries

import (
	"time"

	"uuid"
)

type CreateBlogPostParams struct {
	ID        uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
	Title     string
	URLSlug   string
	Summary   *string
}
`

// blogPostProject returns the project context for BlogPost with the given
// generated query file, which holds the Create params.
func blogPostProject(module, queryFile string) Project {
	project := Project{Module: module, Models: []byte(blogPostModels)}
	if queryFile != "" {
		project.Queries = map[string][]byte{
			"domains/internal/queries/blog_post.sql.go": []byte(queryFile),
		}
	}
	return project
}

// TestRenderedTimestampNullability pins the timestamp mapping the generator
// emits: a NOT NULL pgtype timestamp becomes time.Time, a nullable one becomes
// *time.Time with a nullable helper, and a missing Create params type falls back
// to nullable with a note the reader can see.
func TestRenderedTimestampNullability(t *testing.T) {
	nullableParams := strings.Replace(blogPostCreateParams, "UpdatedAt time.Time", "UpdatedAt pgtype.Timestamptz", 1)
	nullableParams = strings.Replace(nullableParams, "import (\n\t\"time\"\n\n\t\"uuid\"\n)", "import (\n\t\"time\"\n\n\t\"github.com/jackc/pgx/v5/pgtype\"\n\t\"uuid\"\n)", 1)

	for _, tc := range []struct {
		name    string
		project Project
		want    []string
		notWant []string
	}{
		{
			name:    "not null",
			project: blogPostProject("example.com/app", blogPostCreateParams),
			want: []string{
				"\tUpdatedAt time.Time `andurel:\"updated_at\"`",
			},
			notWant: []string{"NullableTime", "pgtype", "unknown", "UpdatedAt *time.Time", "Transform"},
		},
		{
			name:    "nullable",
			project: blogPostProject("example.com/app", nullableParams),
			want: []string{
				"\tUpdatedAt *time.Time `andurel:\"updated_at\"`",
				"// blogPostNullableTime converts a nullable entity time into the row's pgtype for",
				"func blogPostNullableTime(value *time.Time) pgtype.Timestamptz {",
				"\"github.com/jackc/pgx/v5/pgtype\"",
			},
			notWant: []string{"unknown", "Transform"},
		},
		{
			name:    "no create params",
			project: blogPostProject("example.com/app", ""),
			want: []string{
				"\tUpdatedAt *time.Time `andurel:\"updated_at\"`",
				"// UpdatedAt is treated as nullable (*time.Time) because queries.CreateBlogPostParams",
				"// does not give its nullability. If the column is NOT NULL, change it to",
				"// time.Time and drop the nullable helper. Run andurel sync queries first,",
				"func blogPostNullableTime(value *time.Time) pgtype.Timestamptz {",
			},
		},
		{
			// An unparseable file in the queries package must not stop the render,
			// and the params are still found in the readable file.
			name: "broken unrelated queries file",
			project: Project{
				Module: "example.com/app",
				Models: []byte(blogPostModels),
				Queries: map[string][]byte{
					"domains/internal/queries/aaa_broken.sql.go": []byte("package queries\n\nfunc ("),
					"domains/internal/queries/blog_post.sql.go":  []byte(blogPostCreateParams),
				},
			},
			want:    []string{"\tUpdatedAt time.Time `andurel:\"updated_at\"`"},
			notWant: []string{"unknown", "NullableTime", "pgtype"},
		},
		{
			// The params are declared in a file that sorts after an unrelated one.
			name: "create params in a later file",
			project: Project{
				Module: "example.com/app",
				Models: []byte(blogPostModels),
				Queries: map[string][]byte{
					"domains/internal/queries/a_other.sql.go":     []byte("package queries\n\ntype Other struct{ X int }\n"),
					"domains/internal/queries/z_blog_post.sql.go": []byte(nullableParams),
				},
			},
			want: []string{
				"\tUpdatedAt *time.Time `andurel:\"updated_at\"`",
				"func blogPostNullableTime(value *time.Time) pgtype.Timestamptz {",
			},
			notWant: []string{"unknown"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest, err := ManifestForProject("BlogPost", "posts", tc.project)
			if err != nil {
				t.Fatal(err)
			}
			goFile := manifestFile(t, manifest, "domains/blog_post.go").Content
			if _, err := parser.ParseFile(token.NewFileSet(), "blog_post.go", goFile, 0); err != nil {
				t.Fatalf("rendered Go does not parse: %v\n%s", err, goFile)
			}
			for _, want := range tc.want {
				if !strings.Contains(goFile, want) {
					t.Errorf("rendered Go is missing %q\n%s", want, goFile)
				}
			}
			for _, notWant := range tc.notWant {
				if strings.Contains(goFile, notWant) {
					t.Errorf("rendered Go contains %q\n%s", notWant, goFile)
				}
			}
		})
	}
}

func TestManifestWithoutRowRendersMigration(t *testing.T) {
	for name, project := range map[string]Project{
		"no models":  {Module: "app"},
		"row absent": {Module: "app", Models: []byte(blogPostModels)},
	} {
		t.Run(name, func(t *testing.T) {
			manifest, err := ManifestForProject("Gadget", "", project)
			if err != nil {
				t.Fatal(err)
			}
			if manifest.RowFound || manifest.Row != "GadgetRow" || manifest.Table != "gadgets" {
				t.Fatalf("unexpected manifest header: %#v", manifest)
			}
			migration := manifest.Files[0]
			if migration.Path != "migrations/<next>_create_gadgets_table.sql" || migration.Language != "sql" {
				t.Fatalf("first file is not the migration: %#v", migration)
			}
			for _, want := range []string{"-- +goose Up\ncreate table gadgets (\n", "-- +goose Down\ndrop table if exists gadgets;\n"} {
				if !strings.Contains(migration.Content, want) {
					t.Errorf("migration skeleton is missing %q\n%s", want, migration.Content)
				}
			}
			for _, file := range manifest.Files[1:] {
				if file.Content != "" {
					t.Errorf("%s has content without a row", file.Path)
				}
			}
		})
	}
}

func TestManifestForProjectRejectsBrokenModels(t *testing.T) {
	_, err := ManifestForProject("User", "users", Project{Models: []byte("package queries\ntype UserRow struct {")})
	if err == nil || !strings.Contains(err.Error(), ModelsPath) {
		t.Fatalf("err = %v, want a parse error naming %s", err, ModelsPath)
	}
}

func TestLowerCamel(t *testing.T) {
	tests := map[string]string{
		"Email":            "email",
		"EmailValidatedAt": "emailValidatedAt",
		"URLSlug":          "urlSlug",
		"ID":               "id",
		"name":             "name",
	}
	for in, want := range tests {
		if got := lowerCamel(in); got != want {
			t.Errorf("lowerCamel(%q) = %q, want %q", in, got, want)
		}
	}
}
