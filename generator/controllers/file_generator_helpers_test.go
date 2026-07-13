package controllers

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMergeControllerSourcesPreservesExistingAndAddsGenerated(t *testing.T) {
	existing := `package controllers

import "fmt"

type Products struct{}

func (p Products) Index() { fmt.Println("custom") }
`
	generated := `package controllers

import (
	"errors"
	"fmt"
)

type Products struct{}
type ProductPayload struct{}

func (p Products) Index() { fmt.Println("generated") }
func (p Products) Show() {}
func Helper() { _ = errors.New("helper") }
`
	merged, err := mergeControllerSources(existing, generated)
	if err != nil {
		t.Fatalf("merge controller sources: %v", err)
	}
	for _, want := range []string{`"fmt"`, `"errors"`, "type ProductPayload struct", "func (p Products) Show()", "func Helper()"} {
		if !strings.Contains(merged, want) {
			t.Fatalf("merged source missing %q:\n%s", want, merged)
		}
	}
	if strings.Contains(merged, `fmt.Println("generated")`) || strings.Count(merged, "type Products struct") != 1 {
		t.Fatalf("existing declarations were replaced or duplicated:\n%s", merged)
	}

	withoutImports, err := mergeControllerSources("package controllers\ntype Products struct{}\n", "package controllers\nimport \"fmt\"\nfunc Helper() { fmt.Println() }\n")
	if err != nil || !strings.Contains(withoutImports, `"fmt"`) {
		t.Fatalf("merge into source without imports: %v\n%s", err, withoutImports)
	}
	if _, err := mergeControllerSources("package controllers\nfunc (", generated); err == nil || !strings.Contains(err.Error(), "existing controller") {
		t.Fatalf("expected existing parse error, got %v", err)
	}
	if _, err := mergeControllerSources(existing, "package controllers\nfunc ("); err == nil || !strings.Contains(err.Error(), "generated controller") {
		t.Fatalf("expected generated parse error, got %v", err)
	}
}

func TestControllerFrontendActionsAndRegistrationHelpers(t *testing.T) {
	if got := detectControllerFrontend(`import "example.com/internal/inertia"`); got != controllerFrontendInertia {
		t.Fatalf("inertia frontend = %q", got)
	}
	if got := detectControllerFrontend(`import "example.com/internal/hypermedia"`); got != controllerFrontendTempl {
		t.Fatalf("templ frontend = %q", got)
	}
	if got := detectControllerFrontend("package controllers"); got != controllerFrontendUnknown {
		t.Fatalf("unknown frontend = %q", got)
	}

	if got := mergeActions([]string{"Index", "show"}, []string{"SHOW", "create"}); !reflect.DeepEqual(got, []string{"index", "show", "create"}) {
		t.Fatalf("merged actions = %#v", got)
	}
	if got := mergeActions([]string{"index"}, nil); got != nil {
		t.Fatalf("empty requested actions = %#v", got)
	}

	method := ensureRegisterRoutes("package controllers\n", "p", "Products", "admin", "Product", []string{"INDEX", "custom"})
	if !strings.Contains(method, "RegisterRoutes") || !strings.Contains(method, "routes.AdminProductIndex.Path()") || strings.Contains(method, "Custom") {
		t.Fatalf("unexpected generated registration method:\n%s", method)
	}

	existing := `package controllers

func (p Products) RegisterRoutes(r *router.Router) error {
	var errs []error
	var err error

	return errors.Join(errs...)
}
`
	updated := ensureRegisterRoutes(existing, "p", "Products", "", "Product", []string{"create", "custom"})
	if strings.Count(updated, "routes.ProductCreate.Path()") != 1 || !strings.Contains(updated, "http.MethodPost") {
		t.Fatalf("registration was not inserted once:\n%s", updated)
	}
	if got := ensureRegisterRoutes(updated, "p", "Products", "", "Product", []string{"create"}); got != updated {
		t.Fatalf("existing registration should be unchanged:\n%s", got)
	}

	withoutReturn := "package controllers\n\nfunc (p Products) RegisterRoutes(r *router.Router) error {}\n"
	appended := ensureRegisterRoutes(withoutReturn, "p", "Products", "", "Product", []string{"destroy"})
	if !strings.Contains(appended, "http.MethodDelete") {
		t.Fatalf("registration fallback was not appended:\n%s", appended)
	}
	if block := routeRegistrationBlock("p", "", "Product", "custom"); block != "" {
		t.Fatalf("custom action generated CRUD route block: %q", block)
	}
}

func TestExistingControllerActionsAndFilterErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "products.go")
	source := `package controllers

type Products struct{}
func NewProducts() Products { return Products{} }
func (p Products) Index() {}
func (p Products) Create() {}
func (p Products) Publish() {}
`
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatalf("write controller: %v", err)
	}
	actions, err := existingControllerActions(path)
	if err != nil || !reflect.DeepEqual(actions, []string{"index", "create"}) {
		t.Fatalf("existing actions = %#v, %v", actions, err)
	}
	if _, err := existingControllerActions(filepath.Join(t.TempDir(), "missing.go")); err == nil {
		t.Fatal("expected missing controller parse error")
	}
	if _, err := filterControllerActions("package controllers\nfunc (", []string{"index"}); err == nil {
		t.Fatal("expected invalid controller source error")
	}
}
