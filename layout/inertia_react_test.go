package layout

import (
	"os"
	"os/exec"
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
	assertFileNotContains(t, projectDir, "resources/js/Layouts/Layout.tsx", "export const panelClass")
	assertFileContains(t, projectDir, "resources/js/routes.ts", "sessionCreate: () => '/users/sign-in'")
	assertFileContains(t, projectDir, "resources/js/Pages/Auth/Login.tsx", "import Layout from '@/Layouts/Layout'")
	assertFileContains(t, projectDir, "resources/js/Pages/Auth/Login.tsx", "import { routes } from '@/routes'")
	assertFileContains(t, projectDir, "resources/js/Pages/Auth/Login.tsx", "form.post(routes.sessionCreate())")
	assertFileContains(t, projectDir, "resources/js/Pages/Auth/Login.tsx", "function submit(event: SubmitEvent)")
	assertFileContains(t, projectDir, "resources/js/Pages/Auth/Login.tsx", `className="w-full max-w-md border border-[#2f3a37] bg-[#101414]/90 shadow-sm shadow-black/40"`)
	assertFileNotContains(t, projectDir, "resources/js/Pages/Auth/Login.tsx", "panelClass")
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
	assertFileContains(t, projectDir, "cmd/app/main.go", `func provideInertia(cfg config.Config) (*inertia.Renderer, *inertia.SSRRuntime, error)`)
	assertFileContains(t, projectDir, "cmd/app/main.go", `fx.Provide(`)
	assertFileContains(t, projectDir, "cmd/app/main.go", `provideInertia,`)
	assertFileContains(t, projectDir, "cmd/app/main.go", `inertia.WithRequestFlash(`)
	assertFileContains(t, projectDir, "internal/inertia/response.go", `type Renderer struct`)
	assertFileContains(t, projectDir, "internal/inertia/render.go", `assetFS fs.FS,`)
	assertFileContains(t, projectDir, "internal/inertia/render.go", `func New(`)
	assertFileContains(t, projectDir, "internal/inertia/render.go", `initVite(assetFS, environment, buildPathURL)`)
	assertFileContains(t, projectDir, "internal/inertia/render.go", `WithRequestProps(`)
	assertFileContains(t, projectDir, "internal/inertia/render.go", `WithRequestFlash(`)
	assertFileContains(t, projectDir, "internal/inertia/middleware.go", `func (renderer *Renderer) Middleware()`)
	assertFileContains(t, projectDir, "internal/inertia/ssr_http.go", `func NewHTTPSSRRenderer(`)
	assertFileNotContains(t, projectDir, "internal/inertia/render.go", "gonertia")
	assertFileNotContains(t, projectDir, "internal/inertia/render.go", "andurel.lock")
	assertFileNotContains(t, projectDir, "internal/inertia/render.go", "github.com/mbvlabs/andurel")
	for _, forbiddenImport := range []string{
		`"testapp/assets"`,
		`"testapp/config"`,
		`"testapp/internal/request"`,
		`"testapp/router/cookies"`,
		`"testapp/router/routes"`,
	} {
		assertFileNotContains(t, projectDir, "internal/inertia/render.go", forbiddenImport)
		assertFileNotContains(t, projectDir, "internal/inertia/vite.go", forbiddenImport)
	}
	assertFileContains(t, projectDir, "internal/inertia/root.templ", `@PageScript(data.ContainerID, data.PageJSON)`)
	assertFileContains(t, projectDir, "internal/inertia/root.templ", `@SSRBody(data.SSR)`)
	assertFileNotContains(t, projectDir, "internal/inertia/root.templ", "andurelinertia")
	assertFileContains(t, projectDir, "internal/inertia/root_templ.go", `func Root(`)
	assertFileMissing(t, projectDir, "assets/inertia/root.go.html")
	assertFileMissing(t, projectDir, "views/root.go.html")
	assertFileContains(t, projectDir, "internal/inertia/render_test.go", "TestRendererPageIncludesRequestFlash")
	assertFileContains(t, projectDir, "router/router.go", "renderer *inertia.Renderer")
	assertFileContains(t, projectDir, "router/router.go", "renderer.Middleware()")
	assertFileContains(t, projectDir, "router/router.go", "renderer.SetReflashHandler")
	assertFileContains(t, projectDir, "router/cookies/flash.go", "func Reflash(")
	assertFileContains(t, projectDir, "controllers/sessions.go", "renderer *inertia.Renderer")
	assertFileContains(t, projectDir, "controllers/sessions.go", `s.renderer.Page(etx, "Auth/Login"`)
	assertFileNotContains(t, projectDir, "go.mod", "github.com/mbvlabs/andurel")
	assertFileNotContains(t, projectDir, "go.mod", "github.com/romsar/gonertia")
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

	cmd := exec.Command("go", "test", "./internal/inertia")
	cmd.Dir = projectDir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOCACHE="+filepath.Join(projectDir, ".gocache"))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated Inertia tests failed: %v\n%s", err, output)
	}

	cmd = exec.Command("go", "vet", "./...")
	cmd.Dir = projectDir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOCACHE="+filepath.Join(projectDir, ".gocache"))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated Inertia project failed go vet: %v\n%s", err, output)
	}

	lock, err := ReadLockFile(projectDir)
	if err != nil {
		t.Fatalf("read react lock: %v", err)
	}
	if lock.ScaffoldConfig == nil || lock.ScaffoldConfig.Inertia != "react" {
		t.Fatalf("unexpected scaffold config: %#v", lock.ScaffoldConfig)
	}
	lockData, err := os.ReadFile(filepath.Join(projectDir, "andurel.lock"))
	if err != nil {
		t.Fatalf("read react lock JSON: %v", err)
	}
	if strings.Contains(string(lockData), "inertiaRoot") {
		t.Fatalf("lock contains removed inertiaRoot field:\n%s", lockData)
	}
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
