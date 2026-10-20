package golden

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/internal/goldentest"
)

type domainManifest struct {
	Entity   string `json:"entity"`
	Table    string `json:"table"`
	Row      string `json:"row"`
	RowFound bool   `json:"row_found"`
	Files    []struct {
		Path     string `json:"path"`
		Language string `json:"language"`
		Content  string `json:"content"`
	} `json:"files"`
}

// patternsShowDomain runs patterns show domain --entity/--table --json in
// project and returns the manifest object as emitted, plus its decoded form.
func patternsShowDomain(t *testing.T, project, entity, table string) (json.RawMessage, domainManifest) {
	t.Helper()

	args := []string{"patterns", "show", "domain", "--entity", entity, "--json"}
	if table != "" {
		args = append(args, "--table", table)
	}
	output, exitCode := goldentest.RunCLIExit(t, project, args...)
	if exitCode != 0 {
		t.Fatalf("andurel %v exited %d\n%s", args, exitCode, output)
	}
	var envelope struct {
		Data struct {
			Manifest json.RawMessage `json:"manifest"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(output), &envelope); err != nil {
		t.Fatalf("decode patterns show JSON: %v\n%s", err, output)
	}
	if len(envelope.Data.Manifest) == 0 {
		t.Fatalf("patterns show %s emitted no manifest\n%s", entity, output)
	}
	var manifest domainManifest
	if err := json.Unmarshal(envelope.Data.Manifest, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	return envelope.Data.Manifest, manifest
}

// TestPatternsShowDomainEntity goldens the per-entity domain manifest for a
// table whose narsilc row exists (widgets, after sync queries) and for one
// that does not (gadgets). The manifest JSON is goldened as emitted, and each
// rendered file content separately so diffs read as code.
func TestPatternsShowDomainEntity(t *testing.T) {
	goldentest.RequireBinary(t)

	project := goldentest.CopyFixture(t, "generate_base")
	goldentest.RunCLI(t, project, "sync", "queries")

	g := goldentest.NewGoldie(t)
	for _, tc := range []struct {
		name, entity, table string
		rowFound            bool
	}{
		{name: "row_exists", entity: "Widget", table: "widgets", rowFound: true},
		{name: "row_missing", entity: "Gadget", table: "gadgets"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, manifest := patternsShowDomain(t, project, tc.entity, tc.table)
			if manifest.RowFound != tc.rowFound {
				t.Fatalf("row_found = %v, want %v", manifest.RowFound, tc.rowFound)
			}

			var indented bytes.Buffer
			if err := json.Indent(&indented, raw, "", "  "); err != nil {
				t.Fatalf("indent manifest: %v", err)
			}
			indented.WriteByte('\n')
			prefix := "patterns/domain/" + tc.name
			g.Assert(t, prefix+"/manifest.json", indented.Bytes())

			rendered := 0
			for _, file := range manifest.Files {
				if file.Content == "" {
					continue
				}
				rendered++
				name := strings.NewReplacer("<", "", ">", "").Replace(file.Path)
				g.Assert(t, prefix+"/"+name, []byte(file.Content))
			}
			if rendered == 0 {
				t.Fatal("manifest rendered no file content")
			}
		})
	}
}

// TestPatternsShowDomainMatchesScaffoldUser is the W6 acceptance path on a
// fresh scaffold: the User entity rendered from queries.UserRow has the same
// fields, andurel tags and types as the scaffold's domains/user.go.
func TestPatternsShowDomainMatchesScaffoldUser(t *testing.T) {
	goldentest.RequireBinary(t)
	goldentest.RequireFullScaffold(t)

	parent := t.TempDir()
	goldentest.RunCLIFull(t, parent, "new", "app")
	project := filepath.Join(parent, "app")

	_, manifest := patternsShowDomain(t, project, "User", "users")
	if !manifest.RowFound {
		t.Fatal("fresh scaffold: queries.UserRow not found")
	}
	var rendered string
	for _, file := range manifest.Files {
		if file.Path == "domains/user.go" {
			rendered = file.Content
		}
	}
	if rendered == "" {
		t.Fatal("manifest has no rendered domains/user.go")
	}

	scaffold, err := os.ReadFile(filepath.Join(project, "domains", "user.go"))
	if err != nil {
		t.Fatalf("read domains/user.go: %v", err)
	}
	got := entityFields(t, "rendered domains/user.go", rendered, "User")
	want := entityFields(t, "domains/user.go", string(scaffold), "User")
	if len(want) == 0 {
		t.Fatal("scaffold User has no fields")
	}
	if !slices.Equal(got, want) {
		t.Fatalf("rendered User fields differ from the scaffold\n got: %v\nwant: %v", got, want)
	}
}

// entityField is one struct field as written: name, type and andurel tag.
type entityField struct {
	Name, Type, Tag string
}

func entityFields(t *testing.T, label, src, typeName string) []entityField {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), label, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", label, err)
	}
	var fields []entityField
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		spec, ok := node.(*ast.TypeSpec)
		if !ok || spec.Name.Name != typeName {
			return true
		}
		structType, ok := spec.Type.(*ast.StructType)
		if !ok {
			return false
		}
		found = true
		for _, field := range structType.Fields.List {
			tag := ""
			if field.Tag != nil {
				raw, err := strconv.Unquote(field.Tag.Value)
				if err != nil {
					t.Fatalf("%s: unquote tag %s: %v", label, field.Tag.Value, err)
				}
				tag = reflect.StructTag(raw).Get("andurel")
			}
			for _, name := range field.Names {
				fields = append(fields, entityField{Name: name.Name, Type: types.ExprString(field.Type), Tag: tag})
			}
		}
		return false
	})
	if !found {
		t.Fatalf("%s declares no struct %s", label, typeName)
	}
	return fields
}
