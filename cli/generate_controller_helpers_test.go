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
		actionInertiaViewComponent("svelte", "Product", "Export"),
	} {
		if !strings.Contains(rendered, "Export") {
			t.Fatalf("rendered helper missing Export:\n%s", rendered)
		}
	}
	if inertiaActionViewExtension("react") != ".tsx" || inertiaActionViewExtension("vue") != ".vue" || inertiaActionViewExtension("svelte") != ".svelte" {
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

	if err := generateActionInertiaViewFile("Product", "admin", "products", []string{"archive"}, "svelte"); err != nil {
		t.Fatalf("generate svelte inertia view: %v", err)
	}
	assertCLITestFileContains(t, root, filepath.Join("resources", "js", "Pages", "Admin", "Product", "Archive.svelte"), "<svelte:head>")
	assertCLITestFileContains(t, root, filepath.Join("resources", "js", "Pages", "Admin", "Product", "Archive.svelte"), "<title>Product Archive</title>")
}

func TestGenerateActionControllerFileModesAndAppend(t *testing.T) {
	for _, test := range []struct {
		name    string
		inertia string
		isAPI   bool
		want    string
	}{
		{name: "templ", want: "hypermedia.RenderPage"},
		{name: "react", inertia: "react", want: "inertia.Page"},
		{name: "svelte", inertia: "svelte", want: "inertia.Page"},
		{name: "api", isAPI: true, want: "etx.JSON"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			previous, err := os.Getwd()
			if err != nil {
				t.Fatalf("get working directory: %v", err)
			}
			if err := os.Chdir(root); err != nil {
				t.Fatalf("change working directory: %v", err)
			}
			defer func() {
				if err := os.Chdir(previous); err != nil {
					t.Fatalf("restore working directory: %v", err)
				}
			}()

			controllerPath := filepath.Join("controllers", "admin", "products.go")
			if err := generateActionControllerFile("Product", "admin", "products", "products", "example.com/app", controllerPath, []string{"export"}, test.inertia, test.isAPI); err != nil {
				t.Fatalf("create action controller: %v", err)
			}
			content, err := os.ReadFile(controllerPath)
			if err != nil {
				t.Fatalf("read action controller: %v", err)
			}
			if !strings.Contains(string(content), test.want) || !strings.Contains(string(content), "ProductExport") {
				t.Fatalf("generated controller missing expected content:\n%s", content)
			}

			if err := generateActionControllerFile("Product", "admin", "products", "products", "example.com/app", controllerPath, []string{"export", "archive"}, test.inertia, test.isAPI); err != nil {
				t.Fatalf("append action controller: %v", err)
			}
			content, err = os.ReadFile(controllerPath)
			if err != nil {
				t.Fatalf("read appended controller: %v", err)
			}
			if strings.Count(string(content), "func (p Products) Export") != 1 || !strings.Contains(string(content), "func (p Products) Archive") {
				t.Fatalf("controller actions were duplicated or omitted:\n%s", content)
			}
			if test.isAPI {
				if _, err := os.Stat("views"); !os.IsNotExist(err) {
					t.Fatalf("API generation unexpectedly created views: %v", err)
				}
			}
		})
	}
}

func TestReadModulePathBranches(t *testing.T) {
	root := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	}()

	if _, err := readModulePath(); err == nil || !strings.Contains(err.Error(), "failed to read") {
		t.Fatalf("expected missing go.mod error, got %v", err)
	}
	if err := os.WriteFile("go.mod", []byte("go 1.26\n"), 0o600); err != nil {
		t.Fatalf("write module-less go.mod: %v", err)
	}
	if _, err := readModulePath(); err == nil || !strings.Contains(err.Error(), "module declaration not found") {
		t.Fatalf("expected missing declaration error, got %v", err)
	}
	if err := os.WriteFile("go.mod", []byte("module example.com/app\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if module, err := readModulePath(); err != nil || module != "example.com/app" {
		t.Fatalf("readModulePath() = %q, %v", module, err)
	}
}
