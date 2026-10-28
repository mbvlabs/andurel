package layout

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	layouttemplates "github.com/mbvlabs/andurel/v2/layout/templates"
)

func TestGeneratedGlobalCORSIsScopedToExplicitOrigins(t *testing.T) {
	assertTemplateContains(t, "runtime_runtime.tmpl",
		"anduin.DefaultCORS(cfg.HTTP.CORSAllowedOrigins...)",
		"cors.Skipper = isViteAssetRequest",
		"bearerSkipper := anduin.SkipCSRFForBearer(routes.APIPrefix, cfg.Session.Name)",
		"return bearerSkipper(ctx) || isSafeViteAssetRequest(ctx)",
		"anduin.MergeOrigins(cfg.App.Origins(), cfg.HTTP.CSRFTrustedOrigins)...",
	)
	assertTemplateMissing(t, "runtime_runtime.tmpl",
		"anduin.MergeOrigins(cfg.App.Origins(), cfg.HTTP.CORSAllowedOrigins)...",
	)
}

func TestGeneratedViteAssetsUsePublicCORS(t *testing.T) {
	assertTemplateContains(t, "handlers_assets.tmpl",
		"rt.Get(routes.ViteBuild.Pattern(), a.dist, anduin.CORS(anduin.CORSConfig{",
		"AllowOrigins:     []string{\"*\"}",
		"AllowCredentials: false",
	)
}

func TestGeneratedMethodNotAllowedRendersNotFoundPage(t *testing.T) {
	assertTemplateContains(t, "handlers_errors.tmpl",
		`inertia.MapError(anduin.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "Errors/NotFound")`,
	)
}

func TestGeneratedDenyFramingUsesRequestHost(t *testing.T) {
	assertTemplateContains(t, "router_middleware_middleware.tmpl",
		"func DenyFraming(hosts ...routing.HostName) anduin.Middleware",
		"anduin.RequestHost(ctx)",
		`anduin.SetHeader(ctx, "X-Frame-Options", "DENY")`,
		`anduin.AddHeader(ctx, "Content-Security-Policy", "frame-ancestors 'none'")`,
		"router.Use(middleware.DenyFraming(config.HostAdmin))",
	)
}

func TestGeneratedRuntimeTestsCoverAssetAndMethodNotAllowedComposition(t *testing.T) {
	assertTemplateContains(t, "runtime_runtime_test.tmpl",
		"func TestGlobalCORSDoesNotTrustHostOrigins(",
		"func TestGlobalCORSAllowsConfiguredOrigin(",
		"func TestViteAssetsUsePublicCORS(",
		"func TestViteAssetsSkipCSRFCookie(",
		"func TestMethodNotAllowedRendersNotFoundPage(",
		"assets/dist has no build output; run pnpm build before this test",
	)
}

// TestGeneratedDenyFramingMiddlewareCompiles renders the middleware template
// into a probe module and checks the policy only fires for the named host. The
// scaffold does not install it, so this is the only compile and behaviour check
// it gets until an application wires it up.
func TestGeneratedDenyFramingMiddlewareCompiles(t *testing.T) {
	root := t.TempDir()
	data := &TemplateData{ModuleName: "testapp", ProjectName: "testapp"}
	if err := renderTemplate(
		root, "router_middleware_middleware.tmpl", "middleware/middleware.go",
		layouttemplates.Files, data,
	); err != nil {
		t.Fatalf("render middleware template: %v", err)
	}

	var module strings.Builder
	if _, err := fmt.Fprintf(&module, `module testapp

go %s
`, goVersion); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"anduin", "routing"} {
		path, err := filepath.Abs(filepath.Join("..", "pkg", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fmt.Fprintf(
			&module,
			"\nrequire github.com/mbvlabs/andurel/pkg/%s v0.0.0\n"+
				"replace github.com/mbvlabs/andurel/pkg/%s => %q\n",
			name, name, filepath.ToSlash(path),
		); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(module.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	probe := `package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"testapp/middleware"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/routing"
)

func main() {
	hosts, err := routing.NewHosts(map[routing.HostName]routing.HostSpec{
		routing.HostPrimary: {Hostname: "primary.example"},
		"admin":            {Hostname: "admin.example"},
	})
	must(err)

	router := anduin.NewRouter(anduin.WithHosts(hosts))
	router.Use(func(next anduin.Handler) anduin.Handler {
		return func(ctx context.Context) error {
			anduin.SetHeader(ctx, "Content-Security-Policy", "default-src 'self'; script-src 'nonce-test'")
			return next(ctx)
		}
	}, middleware.DenyFraming("admin"))
	router.Get("/", func(ctx context.Context) error {
		return anduin.Text(ctx, http.StatusOK, "ok")
	})

	admin := httptest.NewRequest(http.MethodGet, "/", nil)
	admin.Host = "admin.example"
	adminResponse := httptest.NewRecorder()
	router.Handler().ServeHTTP(adminResponse, admin)
	assert(adminResponse.Header().Get("X-Frame-Options") == "DENY", "admin X-Frame-Options")
	policies := adminResponse.Header().Values("Content-Security-Policy")
	assert(len(policies) == 2, "both CSP policies survive")
	assert(policies[0] == "default-src 'self'; script-src 'nonce-test'", "existing CSP preserved")
	assert(policies[1] == "frame-ancestors 'none'", "admin framing policy appended")

	primary := httptest.NewRequest(http.MethodGet, "/", nil)
	primary.Host = "primary.example"
	primaryResponse := httptest.NewRecorder()
	router.Handler().ServeHTTP(primaryResponse, primary)
	assert(primaryResponse.Header().Get("X-Frame-Options") == "", "primary X-Frame-Options")
	primaryPolicies := primaryResponse.Header().Values("Content-Security-Policy")
	assert(len(primaryPolicies) == 1, "primary has only the original CSP")
	assert(primaryPolicies[0] == policies[0], "primary CSP preserved")

	fmt.Println("PASS: DenyFraming sets headers only for the named host")
}

func assert(ok bool, message string) {
	if !ok {
		panic(message)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
`
	if err := os.WriteFile(filepath.Join(root, "probe.go"), []byte(probe), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "run", "-mod=mod", "probe.go")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated DenyFraming probe: %v\n%s", err, output)
	}
}
