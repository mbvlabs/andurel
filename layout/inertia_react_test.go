package layout

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldReactInertiaAssets(t *testing.T) {
	projectDir := t.TempDir()

	if err := Scaffold(projectDir, "testapp", "postgresql", "test", nil, "react", ""); err != nil {
		t.Fatalf("scaffold react inertia project: %v", err)
	}

	assertFileContains(t, projectDir, "resources/js/app.tsx", "@inertiajs/react")
	assertFileContains(t, projectDir, "resources/js/Layouts/Layout.tsx", "children")
	assertFileContains(t, projectDir, "resources/js/routes.ts", "sessionCreate: () => '/users/sign-in'")
	assertFileContains(t, projectDir, "resources/js/Pages/Auth/Login.tsx", "form.post(routes.sessionCreate())")
	assertFileContains(t, projectDir, "resources/js/Pages/Auth/Login.tsx", "function submit(event: SubmitEvent)")
	assertFileNotContains(t, projectDir, "resources/js/Pages/Auth/Login.tsx", "FormEvent")
	assertFileContains(t, projectDir, "resources/js/Pages/Auth/Registration.tsx", "form.post(routes.registrationCreate())")
	assertFileContains(t, projectDir, "resources/js/Pages/Auth/ResetPassword.tsx", "form.put(routes.passwordUpdate())")
	assertFileContains(t, projectDir, "resources/js/Pages/Auth/ResetPasswordRequest.tsx", "form.post(routes.passwordCreate())")
	assertFileContains(t, projectDir, "resources/js/Pages/Auth/ConfirmEmail.tsx", "form.post(routes.confirmationCreate())")
	assertFileContains(t, projectDir, "resources/js/Pages/Errors/BadRequest.tsx", "Bad request")
	assertFileContains(t, projectDir, "resources/js/Pages/Errors/InternalError.tsx", "Something went wrong.")
	assertFileContains(t, projectDir, "resources/js/Pages/Errors/NotFound.tsx", "Not found")
	assertFileContains(t, projectDir, "views/bad_request.templ", "templ BadRequest()")
	assertFileContains(t, projectDir, "views/internal_error.templ", "templ InternalError()")
	assertFileContains(t, projectDir, "views/not_found.templ", "templ NotFound()")
	assertFileContains(t, projectDir, "views/welcome.templ", "type Welcome struct{}")
	assertFileContains(t, projectDir, "package.json", "@vitejs/plugin-react")
	assertFileContains(t, projectDir, "vite.config.ts", "resources/js/app.tsx")
	assertFileContains(t, projectDir, "tsconfig.json", "resources/js/**/*.tsx")
	assertFileContains(t, projectDir, "tsconfig.json", `"types": ["vite/client"]`)
	assertFileContains(t, projectDir, "resources/js/app.tsx", "type ResolvedComponent")
	assertFileContains(t, projectDir, "resources/js/app.tsx", "createInertiaApp<PageProps>({")
	assertFileContains(t, projectDir, "resources/js/app.tsx", "type PageModule = {")
	assertFileContains(t, projectDir, "resources/js/app.tsx", "default: ResolvedComponent")
	assertFileContains(t, projectDir, "resources/js/app.tsx", "<App {...props} />")
	assertFileNotContains(t, projectDir, "resources/js/app.tsx", "type InertiaComponent")
	assertFileNotContains(t, projectDir, "resources/js/app.tsx", "App={App as")
	assertFileNotContains(t, projectDir, "resources/js/app.tsx", "props={props as")
	assertFileNotContains(t, projectDir, "resources/js/app.tsx", "ComponentType<any>")
	assertFileNotContains(t, projectDir, "resources/js/app.tsx", "Record<string, any>")
	assertFileContains(t, projectDir, "cmd/app/main.go", "internal/inertia")
	assertFileContains(t, projectDir, "router/router.go", "inertia.Middleware()")
	assertFileContains(t, projectDir, "go.mod", "github.com/romsar/gonertia")
	assertFileMissing(t, projectDir, "resources/js/app.ts")
	assertFileMissing(t, projectDir, "resources/js/Pages/Head.tsx")
	assertFileMissing(t, projectDir, "resources/js/Pages/Layout.tsx")
	assertFileMissing(t, projectDir, "resources/js/Pages/Welcome.tsx")
	assertFileMissing(t, projectDir, "resources/js/Pages/Welcome.vue")
	assertFileMissing(t, projectDir, "views/home.templ")
	assertFileMissing(t, projectDir, "views/login.templ")
	assertFileMissing(t, projectDir, "views/registration.templ")
	assertFileMissing(t, projectDir, "views/reset_password.templ")
	assertFileMissing(t, projectDir, "views/confirm_email.templ")
}

func TestScaffoldVueInertiaTSConfigIncludesViteClientTypes(t *testing.T) {
	projectDir := t.TempDir()

	if err := Scaffold(projectDir, "testapp", "postgresql", "test", nil, "vue", ""); err != nil {
		t.Fatalf("scaffold vue inertia project: %v", err)
	}

	assertFileContains(t, projectDir, "tsconfig.json", `"types": ["vite/client"]`)
}

func assertFileContains(t *testing.T, root, relPath, want string) {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(root, relPath))
	if err != nil {
		t.Fatalf("read %s: %v", relPath, err)
	}
	if !strings.Contains(string(content), want) {
		t.Fatalf("%s does not contain %q", relPath, want)
	}
}

func assertFileMissing(t *testing.T, root, relPath string) {
	t.Helper()

	if _, err := os.Stat(filepath.Join(root, relPath)); err == nil {
		t.Fatalf("%s exists unexpectedly", relPath)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", relPath, err)
	}
}

func assertFileNotContains(t *testing.T, root, relPath, unwanted string) {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(root, relPath))
	if err != nil {
		t.Fatalf("read %s: %v", relPath, err)
	}
	if strings.Contains(string(content), unwanted) {
		t.Fatalf("%s contains %q", relPath, unwanted)
	}
}
