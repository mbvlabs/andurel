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
	assertFileNotContains(
		t,
		projectDir,
		"resources/js/Layouts/Layout.tsx",
		"export const panelClass",
	)
	assertFileContains(
		t,
		projectDir,
		"resources/js/routes.ts",
		"sessionCreate: () => '/users/sign-in'",
	)
	assertFileContains(
		t,
		projectDir,
		"resources/js/Pages/Auth/Login.tsx",
		"import Layout from '@/Layouts/Layout'",
	)
	assertFileContains(
		t,
		projectDir,
		"resources/js/Pages/Auth/Login.tsx",
		"import { routes } from '@/routes'",
	)
	assertFileContains(
		t,
		projectDir,
		"resources/js/Pages/Auth/Login.tsx",
		"form.post(routes.sessionCreate())",
	)
	assertFileContains(
		t,
		projectDir,
		"resources/js/Pages/Auth/Login.tsx",
		"function submit(event: SubmitEvent)",
	)
	assertFileContains(
		t,
		projectDir,
		"resources/js/Pages/Auth/Login.tsx",
		`className="w-full max-w-md border border-[#2f3a37] bg-[#101414]/90 shadow-sm shadow-black/40"`,
	)
	assertFileNotContains(t, projectDir, "resources/js/Pages/Auth/Login.tsx", "panelClass")
	assertFileNotContains(t, projectDir, "resources/js/Pages/Auth/Login.tsx", "FormEvent")
	assertFileContains(
		t,
		projectDir,
		"resources/js/Pages/Auth/Registration.tsx",
		"form.post(routes.registrationCreate())",
	)
	assertFileContains(
		t,
		projectDir,
		"resources/js/Pages/Auth/ResetPassword.tsx",
		"form.put(routes.passwordUpdate())",
	)
	assertFileContains(
		t,
		projectDir,
		"resources/js/Pages/Auth/ResetPasswordRequest.tsx",
		"form.post(routes.passwordCreate())",
	)
	assertFileContains(
		t,
		projectDir,
		"resources/js/Pages/Auth/ConfirmEmail.tsx",
		"form.post(routes.confirmationCreate())",
	)
	assertFileContains(t, projectDir, "resources/js/Pages/Errors/BadRequest.tsx", "Bad request")
	assertFileContains(
		t,
		projectDir,
		"resources/js/Pages/Errors/InternalError.tsx",
		"Something went wrong.",
	)
	assertFileContains(t, projectDir, "resources/js/Pages/Errors/NotFound.tsx", "Not found")
	assertFileContains(t, projectDir, "views/bad_request.templ", "templ BadRequest()")
	assertFileContains(t, projectDir, "views/internal_error.templ", "templ InternalError()")
	assertFileContains(t, projectDir, "views/not_found.templ", "templ NotFound()")
	assertFileContains(t, projectDir, "views/welcome.templ", "type Welcome struct{}")
	assertFileContains(t, projectDir, "package.json", "@vitejs/plugin-react")
	assertFileContains(t, projectDir, "vite.config.ts", "resources/js/app.tsx")
	assertFileContains(t, projectDir, "tsconfig.json", "resources/js/**/*.tsx")
	assertFileContains(t, projectDir, "tsconfig.json", `"types": ["vite/client", "node"]`)
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
	assertFileContains(t, projectDir, "cmd/app/main.go", `inertiaModule,`)
	assertFileNotContains(t, projectDir, "cmd/app/main.go", "internal/inertia")
	assertFileContains(t, projectDir, "cmd/app/main.go", "func newInertia(")
	assertFileContains(t, projectDir, "cmd/app/main.go", "cfg config.InertiaCfg")
	assertFileContains(t, projectDir, "cmd/app/main.go", "appCfg config.AppCfg")
	assertFileContains(t, projectDir, "cmd/app/main.go", "(*inertia.Renderer, error)")
	assertFileContains(t, projectDir, "config/config.go", "NewInertiaCfg,")
	assertFileContains(t, projectDir, "config/inertia.go", "env.Parse(&inertiaCfg)")
	assertFileContains(
		t,
		projectDir,
		"config/inertia.go",
		"func (c InertiaCfg) GetRoot() inertia.RootFunc",
	)
	assertFileContains(
		t,
		projectDir,
		"config/inertia.go",
		"DefaultInertiaBuildPathURL = routes.ViteBuild.Path()",
	)
	assertFileContains(
		t,
		projectDir,
		"config/inertia.go",
		"Root:                DefaultInertiaRoot",
	)
	assertFileContains(
		t,
		projectDir,
		"config/inertia.go",
		"BuildPathURL:        DefaultInertiaBuildPathURL",
	)
	assertFileContains(t, projectDir, "config/inertia.go", "return c.Root")
	assertFileContains(t, projectDir, "config/inertia.go", "return c.EntryPoint")
	assertFileNotContains(t, projectDir, "config/inertia.go", `env:"-"`)
	assertFileContains(t, projectDir, "cmd/app/main.go", `inertia.WithRoot(cfg.GetRoot())`)
	assertFileContains(
		t,
		projectDir,
		"cmd/app/main.go",
		`inertia.WithEntryPoint(cfg.GetEntryPoint())`,
	)
	assertFileNotContains(t, projectDir, "cmd/app/main.go", "views.Root")
	assertFileContains(t, projectDir, "cmd/app/main.go", `OnStart: renderer.Start`)
	assertFileContains(
		t,
		projectDir,
		"cmd/app/main.go",
		`inertia.WithProjectName(appCfg.GetProjectName())`,
	)
	assertFileMissing(t, projectDir, "application/metadata.go")
	assertFileContains(t, projectDir, "config/app.go", `func (c AppCfg) GetBaseURL() string`)
	assertFileContains(t, projectDir, "views/root.templ", `templ Root(data inertia.RootData)`)
	assertFileContains(t, projectDir, "views/root.templ", `@templ.Raw(string(data.ViteHead))`)
	assertFileContains(t, projectDir, "views/root_templ.go", `func Root(`)
	for _, removed := range []string{"render.go", "render_test.go", "root.templ", "root_templ.go", "vite.go"} {
		assertFileMissing(t, projectDir, filepath.Join("internal/inertia", removed))
	}
	assertFileMissing(t, projectDir, "assets/inertia/root.go.html")
	assertFileMissing(t, projectDir, "views/root.go.html")
	assertFileContains(t, projectDir, "router/router.go", "renderer *inertia.Renderer")
	assertFileContains(t, projectDir, "router/router.go", "renderer.Middleware()")
	assertFileContains(t, projectDir, "router/router.go", "renderer.SetReflashHandler")
	assertFileContains(t, projectDir, "router/cookies/flash.go", "func (s *Session) Reflash(")
	assertFileContains(
		t,
		projectDir,
		"controllers/sessions.go",
		`"github.com/mbvlabs/andurel/pkg/inertia"`,
	)
	assertFileContains(t, projectDir, "controllers/sessions.go", "renderer *inertia.Renderer")
	assertFileContains(
		t,
		projectDir,
		"controllers/sessions.go",
		`s.renderer.Page(etx, "Auth/Login"`,
	)
	assertFileContains(t, projectDir, "go.mod", "github.com/mbvlabs/andurel/pkg/hypermedia v0.2.1")
	assertFileContains(t, projectDir, "go.mod", "github.com/mbvlabs/andurel/pkg/inertia v0.1.1")
	assertFileContains(t, projectDir, "router/appctx/appctx.go", "func WithFlashes(")
	assertFileContains(t, projectDir, "router/middleware/middleware.go", "appctx.WithFlashes(")
	assertFileNotContains(t, projectDir, "go.mod", "github.com/mbvlabs/andurel v")
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

	inertiaModule, err := filepath.Abs(filepath.Join("..", "pkg", "inertia"))
	if err != nil {
		t.Fatalf("resolve standalone Inertia module: %v", err)
	}
	cmd := exec.Command(
		"go",
		"mod",
		"edit",
		"-replace=github.com/mbvlabs/andurel/pkg/inertia="+inertiaModule,
	)
	cmd.Dir = projectDir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("configure standalone Inertia module: %v\n%s", err, output)
	}

	storageModule, err := filepath.Abs(filepath.Join("..", "pkg", "storage"))
	if err != nil {
		t.Fatalf("resolve standalone storage module: %v", err)
	}
	cmd = exec.Command(
		"go",
		"mod",
		"edit",
		"-replace=github.com/mbvlabs/andurel/pkg/storage="+storageModule,
	)
	cmd.Dir = projectDir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("configure standalone storage module: %v\n%s", err, output)
	}

	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = projectDir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tidy generated Inertia module: %v\n%s", err, output)
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

	assertFileContains(t, projectDir, "tsconfig.json", `"types": ["vite/client", "node"]`)
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
