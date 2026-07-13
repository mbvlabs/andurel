package views

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestViewDataNullTypeHelpers(t *testing.T) {
	types := map[string]string{
		"sql.NullString":  "string",
		"bun.NullString":  "string",
		"sql.NullBool":    "bool",
		"bun.NullBool":    "bool",
		"sql.NullInt16":   "int16",
		"sql.NullInt32":   "int32",
		"bun.NullInt32":   "int32",
		"sql.NullInt64":   "int64",
		"bun.NullInt64":   "int64",
		"sql.NullFloat64": "float64",
		"bun.NullFloat64": "float64",
		"sql.NullTime":    "time.Time",
		"bun.NullTime":    "time.Time",
		"*uuid.UUID":      "uuid.UUID",
		"string":          "string",
	}
	for input, want := range types {
		if got := resolveViewBaseType(input); got != want {
			t.Fatalf("resolveViewBaseType(%q) = %q, want %q", input, got, want)
		}
	}

	fields := []ViewField{
		{Name: "Name", GoType: "sql.NullString"},
		{Name: "Active", GoType: "bun.NullBool"},
		{Name: "Small", GoType: "sql.NullInt16"},
		{Name: "Count", GoType: "bun.NullInt32"},
		{Name: "Total", GoType: "sql.NullInt64"},
		{Name: "Score", GoType: "bun.NullFloat64"},
		{Name: "When", GoType: "sql.NullTime"},
		{Name: "Raw", GoType: "json.RawMessage"},
		{Name: "ID", GoType: "uuid.UUID"},
	}
	if !hasNullFields(fields) || hasNullFields([]ViewField{{GoType: "string"}}) {
		t.Fatal("null field detection failed")
	}
	for _, field := range fields[:7] {
		if !isNullType(field.GoType) || viewDataValue(field, "entity."+field.Name) == "entity."+field.Name {
			t.Fatalf("null field was not converted: %#v", field)
		}
	}
	if got := viewDataValue(fields[7], "entity.Raw"); got != "entity.Raw" {
		t.Fatalf("plain field conversion = %q", got)
	}

	imports := viewDataImports(fields)
	for _, want := range []string{`"encoding/json"`, `"time"`, `"github.com/google/uuid"`} {
		if !strings.Contains(imports, want) {
			t.Fatalf("view data imports missing %q: %q", want, imports)
		}
	}
	if got := viewDataImports([]ViewField{{GoType: "string"}}); got != "" {
		t.Fatalf("plain fields generated imports: %q", got)
	}

	view := &GeneratedView{NamespacePascal: "Admin", ResourceName: "Product", EntityName: "ProductEntity", Fields: fields[:2]}
	definition := viewDataDefinition(view)
	for _, want := range []string{"type AdminProductData struct", "func newAdminProductData", "Name string", "Active bool"} {
		if !strings.Contains(definition, want) {
			t.Fatalf("view definition missing %q:\n%s", want, definition)
		}
	}
	if got := viewDataDefinition(&GeneratedView{Fields: []ViewField{{GoType: "string"}}}); got != "" {
		t.Fatalf("plain view generated DTO: %q", got)
	}
}

func TestInertiaReactAndViewReferenceHelpers(t *testing.T) {
	if !inertiaUsesForm("Create") || !inertiaUsesForm("Edit") || inertiaUsesForm("Show") {
		t.Fatal("inertia form component classification failed")
	}
	if !inertiaNeedsItem("Index") || !inertiaNeedsItem("Show") || !inertiaNeedsItem("Edit") || inertiaNeedsItem("Create") {
		t.Fatal("inertia item component classification failed")
	}

	fields := []struct {
		field       ViewField
		fieldType   string
		createValue string
		inputValue  string
	}{
		{field: ViewField{Name: "Active", GoFormType: "bool"}, fieldType: "boolean", createValue: "false", inputValue: "event.currentTarget.checked"},
		{field: ViewField{Name: "Count", GoFormType: "int64"}, fieldType: "number", createValue: "0", inputValue: "Number(event.currentTarget.value)"},
		{field: ViewField{Name: "Name", GoFormType: "string"}, fieldType: "string", createValue: "''", inputValue: "event.currentTarget.value"},
	}
	for _, test := range fields {
		if got := inertiaReactFieldType(test.field); got != test.fieldType {
			t.Fatalf("field type = %q, want %q", got, test.fieldType)
		}
		if got := inertiaReactCreateValue(test.field); got != test.createValue {
			t.Fatalf("create value = %q, want %q", got, test.createValue)
		}
		if got := inertiaReactInputValue(test.field); got != test.inputValue {
			t.Fatalf("input value = %q, want %q", got, test.inputValue)
		}
	}
	if got := inertiaReactEditValue(fields[0].field); got != "Boolean(item.Active)" {
		t.Fatalf("boolean edit value = %q", got)
	}
	if got := inertiaReactEditValue(fields[1].field); got != "Number(item.Count ?? 0)" {
		t.Fatalf("number edit value = %q", got)
	}
	date := ViewField{Name: "DueAt", GoFormType: "string", InputType: "date"}
	if got := inertiaReactEditValue(date); !strings.HasSuffix(got, ".slice(0, 10)") {
		t.Fatalf("date edit value = %q", got)
	}
	if got := inertiaReactDisplay(fields[0].field, "row"); got != "row.Active ? 'Yes' : 'No'" {
		t.Fatalf("boolean display = %q", got)
	}
	if got := inertiaReactDisplay(fields[2].field, "row"); got != "row.Name" {
		t.Fatalf("string display = %q", got)
	}

	if got := viewDataRef("Admin", "Product", "entity", false); got != "entity" {
		t.Fatalf("plain data ref = %q", got)
	}
	if got := viewDataRef("Admin", "Product", "entity", true); got != "newAdminProductData(entity)" {
		t.Fatalf("DTO data ref = %q", got)
	}
	if got := viewDataRowRef("", "Product", "row", true); got != "rowData" {
		t.Fatalf("root row ref = %q", got)
	}
	if got := viewDataRowRef("Admin", "Product", "row", true); got != "adminProductData" {
		t.Fatalf("namespaced row ref = %q", got)
	}
	if got := viewDataLoopAssignment("", "Product", "row", false); got != "{" {
		t.Fatalf("plain loop assignment = %q", got)
	}
}

func TestResourceViewActionDiscoveryAndAdapterHelpers(t *testing.T) {
	if inertiaViewTemplatePrefix("react") != "inertia_react_" || inertiaViewTemplatePrefix("vue") != "inertia_vue_" {
		t.Fatal("template prefix selection failed")
	}
	if inertiaViewExtension("react") != ".tsx" || inertiaViewExtension("vue") != ".vue" {
		t.Fatal("view extension selection failed")
	}
	if got := namespacePrefix("Admin/API"); got == "" {
		t.Fatal("namespace prefix should not be empty")
	}
	if got := mergeResourceViewActions([]string{"Index", "custom"}, []string{"SHOW", "index", "invalid"}); !reflect.DeepEqual(got, []string{"index", "show"}) {
		t.Fatalf("merged view actions = %#v", got)
	}
	if got := mergeResourceViewActions([]string{"index"}, nil); got != nil {
		t.Fatalf("empty requested view actions = %#v", got)
	}

	path := filepath.Join(t.TempDir(), "products.templ")
	content := `package views

type AdminProductIndex struct{}
type AdminProductShow struct{}
type AdminProductNew struct{}
type AdminProductEdit struct{}
var _ = routes.AdminProductCreate
var _ = routes.AdminProductUpdate
var _ = routes.AdminProductDestroy
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write view: %v", err)
	}
	actions, err := existingResourceViewActions(path, "Product", "admin")
	if err != nil || !reflect.DeepEqual(actions, resourceViewActions) {
		t.Fatalf("existing view actions = %#v, %v", actions, err)
	}
	if _, err := existingResourceViewActions(filepath.Join(t.TempDir(), "missing.templ"), "Product", ""); err == nil {
		t.Fatal("expected missing view error")
	}
}
