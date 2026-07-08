package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateControllerHelperFunctions(t *testing.T) {
	if got := apiNamespace(""); got != "api" {
		t.Fatalf("apiNamespace empty = %q", got)
	}
	if got := apiNamespace("v1"); got != "api/v1" {
		t.Fatalf("apiNamespace nested = %q", got)
	}

	actions := []string{"index", "Index", "export", "show", "export"}
	if got := crudControllerActions(actions); strings.Join(got, ",") != "index,show" {
		t.Fatalf("crudControllerActions = %#v", got)
	}
	if got := nonCRUDControllerActions(actions); strings.Join(got, ",") != "export,export" {
		t.Fatalf("nonCRUDControllerActions = %#v", got)
	}
	if !isCRUDControllerAction("DESTROY") || isCRUDControllerAction("export") {
		t.Fatalf("CRUD action classifier failed")
	}
	if !controllerMethodExists("func (p Products) Export(etx echo.Context) error", "Export") {
		t.Fatalf("expected old echo.Context signature to be detected")
	}

	for _, rendered := range []string{
		actionControllerMethod("p", "Products", "admin", "Product", "Export"),
		actionControllerMethodAPI("p", "Products", "Product", "Export"),
		actionControllerMethodInertia("p", "Products", "admin", "Product", "Export"),
		customRegisterRoutesMethod("p", "Products", "admin", "Product", []string{"export"}),
		customRouteRegistrationBlock("p", "admin", "Product", "Export"),
		actionViewComponent("Product", "Admin", "Export"),
		actionInertiaViewComponent("react", "Product", "Export"),
		actionInertiaViewComponent("vue", "Product", "Export"),
	} {
		if !strings.Contains(rendered, "Export") {
			t.Fatalf("rendered helper missing Export:\n%s", rendered)
		}
	}
	if inertiaActionViewExtension("react") != ".tsx" || inertiaActionViewExtension("vue") != ".vue" {
		t.Fatalf("unexpected inertia view extensions")
	}
	if namespacePrefix("admin/users") != "admin_users_" {
		t.Fatalf("namespacePrefix returned %q", namespacePrefix("admin/users"))
	}
}

func TestEnsureCustomRegisterRoutesBranches(t *testing.T) {
	noRegister := "package controllers\n\ntype Products struct{}\n"
	got := ensureCustomRegisterRoutes(noRegister, "p", "", "Product", []string{"export"})
	if !strings.Contains(got, "RegisterRoutes") || !strings.Contains(got, "ProductExport") {
		t.Fatalf("expected RegisterRoutes to be appended:\n%s", got)
	}

	withNeedle := `package controllers

func (p Products) RegisterRoutes(r *router.Router) error {
	var errs []error
	return errors.Join(errs...)
}
`
	got = ensureCustomRegisterRoutes(withNeedle, "p", "", "Product", []string{"export"})
	if !strings.Contains(got, "Handler: p.Export") || !strings.Contains(got, "return errors.Join") {
		t.Fatalf("expected route registration before return:\n%s", got)
	}

	again := ensureCustomRegisterRoutes(got, "p", "", "Product", []string{"export"})
	if again != got {
		t.Fatalf("expected duplicate route registration to be skipped")
	}

	noNeedle := strings.Replace(withNeedle, "return errors.Join(errs...)", "return nil", 1)
	got = ensureCustomRegisterRoutes(noNeedle, "p", "", "Product", []string{"archive"})
	if !strings.Contains(got, "Handler: p.Archive") {
		t.Fatalf("expected route registration appended without needle:\n%s", got)
	}
}

func TestGenerateActionViewFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/app\n")
	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(original)
	})

	if err := generateActionViewFile("Product", "admin", "products", "example.com/app", "admin_products", []string{"export"}); err != nil {
		t.Fatalf("generateActionViewFile create: %v", err)
	}
	viewPath := filepath.Join(root, "views", "admin_products_resource.templ")
	assertCLITestFileContains(t, root, filepath.ToSlash(viewPath[len(root)+1:]), "templ AdminProductExport()")

	if err := generateActionViewFile("Product", "admin", "products", "example.com/app", "admin_products", []string{"export", "archive"}); err != nil {
		t.Fatalf("generateActionViewFile append: %v", err)
	}
	content, err := os.ReadFile(viewPath)
	if err != nil {
		t.Fatalf("read view: %v", err)
	}
	if strings.Count(string(content), "AdminProductExport") != 1 || !strings.Contains(string(content), "AdminProductArchive") {
		t.Fatalf("unexpected appended view content:\n%s", string(content))
	}

	if err := generateActionInertiaViewFile("Product", "admin", "products", []string{"export"}, "react"); err != nil {
		t.Fatalf("generate react inertia view: %v", err)
	}
	assertCLITestFileContains(t, root, filepath.Join("resources", "js", "Pages", "Admin", "Product", "Export.tsx"), "Head title=\"Product Export\"")

	if err := generateActionInertiaViewFile("Product", "", "products", []string{"show"}, "vue"); err != nil {
		t.Fatalf("generate vue inertia view: %v", err)
	}
	assertCLITestFileContains(t, root, filepath.Join("resources", "js", "Pages", "Product", "Show.vue"), "<template>")
}
