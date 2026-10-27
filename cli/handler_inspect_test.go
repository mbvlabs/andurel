package cli

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/output"
)

func writeHandlerInspectProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/app\n\ngo 1.27.1\n")
	writeTestFile(t, root, "routes/products.go", `package routes

import "github.com/mbvlabs/andurel/pkg/routing"

const ProductPrefix = "/products"

var ProductIndex = routing.NewSimpleRoute("", "products.index", ProductPrefix)

var ProductCreate = routing.NewSimpleRoute("", "products.create", ProductPrefix)

var ProductDestroy = routing.NewSimpleRoute("/archive", "products.destroy", ProductPrefix)
`)
	writeTestFile(t, root, "handlers/products.go", `package handlers

import (
	"context"

	r "example.com/app/routes"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/cache"
	"github.com/mbvlabs/andurel/pkg/inertia"
)

type ProductHandler struct {
	renderer *inertia.Renderer
	cache    cache.Store
}

func NewProductHandler(
	products domains.Products,
	renderer *inertia.Renderer,
	store cache.Store,
) (ProductHandler, error) {
	return ProductHandler{renderer: renderer, cache: store}, nil
}

func (h ProductHandler) Register(rt *anduin.Router) {
	rt.Get(r.ProductIndex.Pattern(), h.Index)
	rt.Post(r.ProductCreate.Path(), h.Create, h.throttle())
	rt.Host("admin").Delete(r.ProductDestroy.Path(), h.Destroy)
	rt.Get("/healthz", h.Index)
	_ = h.cache.Get(context.Background(), "ignored")
}

func (h ProductHandler) Index(ctx context.Context) error   { return nil }
func (h ProductHandler) Create(ctx context.Context) error  { return nil }
func (h ProductHandler) Destroy(ctx context.Context) error { return nil }
`)
	writeTestFile(t, root, "handlers/errors.go", `package handlers

func MapError() {}
`)
	writeTestFile(t, root, "handlers/products_test.go", `package handlers
`)

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})
	return root
}

func TestInspectHandlersReportsConstructorAndRoutes(t *testing.T) {
	writeHandlerInspectProject(t)

	var out bytes.Buffer
	cmd := newHandlersCommand()
	output.RegisterPersistentFlags(cmd)
	cmd.SetOut(&out)
	_ = cmd.PersistentFlags().Set("json", "true")
	if err := cmd.Execute(); err != nil {
		t.Fatalf("inspect handlers: %v", err)
	}

	var envelope struct {
		OK   bool          `json:"ok"`
		Data []projectItem `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v\n%s", err, out.String())
	}
	if !envelope.OK {
		t.Fatalf("inspect handlers failed:\n%s", out.String())
	}

	byPath := map[string]projectItem{}
	for _, item := range envelope.Data {
		byPath[item.Path] = item
	}
	errorsItem, ok := byPath["handlers/errors.go"]
	if !ok {
		t.Fatalf("handlers/errors.go missing from items: %#v", envelope.Data)
	}
	if len(errorsItem.Handlers) != 0 {
		t.Fatalf("handlers/errors.go handlers = %#v, want none", errorsItem.Handlers)
	}
	if strings.Contains(out.String(), `"path":"handlers/errors.go","kind":"handler","handlers"`) {
		t.Fatalf("file without handlers should omit the handlers key:\n%s", out.String())
	}

	products := byPath["handlers/products.go"]
	want := []handlerInfo{{
		Type:        "ProductHandler",
		Constructor: "NewProductHandler",
		Params: []handlerParam{
			{Name: "products", Type: "domains.Products"},
			{Name: "renderer", Type: "*inertia.Renderer"},
			{Name: "store", Type: "cache.Store"},
		},
		ReturnsError: true,
		Routes: []handlerRoute{
			{Variable: "ProductIndex", Name: "products.index", Method: "GET", Path: "/products", Action: "Index", Line: 27},
			{Variable: "ProductCreate", Name: "products.create", Method: "POST", Path: "/products", Action: "Create", Line: 28},
			{Variable: "ProductDestroy", Name: "products.destroy", Method: "DELETE", Path: "/products/archive", Action: "Destroy", Line: 29},
			{Method: "GET", Path: "/healthz", Action: "Index", Line: 30},
		},
	}}
	if !reflect.DeepEqual(products.Handlers, want) {
		got, _ := json.MarshalIndent(products.Handlers, "", "  ")
		t.Fatalf("products handlers =\n%s", got)
	}

	var jq bytes.Buffer
	jqCmd := newHandlersCommand()
	output.RegisterPersistentFlags(jqCmd)
	jqCmd.SetOut(&jq)
	_ = jqCmd.PersistentFlags().Set("jq", ".")
	if err := jqCmd.Execute(); err != nil {
		t.Fatalf("inspect handlers --jq: %v", err)
	}
	var projected []projectItem
	if err := json.Unmarshal(jq.Bytes(), &projected); err != nil {
		t.Fatalf("decode --jq output: %v\n%s", err, jq.String())
	}
	if len(projected) != len(envelope.Data) {
		t.Fatalf("--jq . items = %d, want %d", len(projected), len(envelope.Data))
	}

	var ids bytes.Buffer
	idsCmd := newHandlersCommand()
	output.RegisterPersistentFlags(idsCmd)
	idsCmd.SetOut(&ids)
	_ = idsCmd.PersistentFlags().Set("ids-only", "true")
	if err := idsCmd.Execute(); err != nil {
		t.Fatalf("inspect handlers --ids-only: %v", err)
	}
	if got, want := ids.String(), "errors\nproducts\nproducts_test\n"; got != want {
		t.Fatalf("--ids-only = %q, want %q", got, want)
	}
}

func TestInspectHandlersHumanOutput(t *testing.T) {
	writeHandlerInspectProject(t)

	var human bytes.Buffer
	cmd := newHandlersCommand()
	cmd.SetOut(&human)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("inspect handlers human: %v", err)
	}
	text := human.String()
	for _, needle := range []string{
		"Handlers (1)",
		"ProductHandler  NewProductHandler(products domains.Products, renderer *inertia.Renderer, store cache.Store)  handlers/products.go",
		"products.destroy",
		"/healthz",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("human output missing %q:\n%s", needle, text)
		}
	}
}

func TestInspectHandlersWithoutHandlerTypes(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/app\n\ngo 1.27.1\n")
	writeTestFile(t, root, "handlers/errors.go", "package handlers\n\nfunc MapError() {}\n")
	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	var human bytes.Buffer
	cmd := newHandlersCommand()
	cmd.SetOut(&human)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("inspect handlers human: %v", err)
	}
	if !strings.Contains(human.String(), "No handlers found (1 handler files).") {
		t.Fatalf("human output = %q", human.String())
	}
}
