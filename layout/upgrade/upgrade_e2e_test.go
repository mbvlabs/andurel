package upgrade

import (
	"bytes"
	"context"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/mbvlabs/andurel/v2/layout"
	layouttemplates "github.com/mbvlabs/andurel/v2/layout/templates"
)

// upgradeE2ESourceVersions are the released CLIs the upgrade test scaffolds
// with. Keep the previous pre-release and the oldest supported one.
var upgradeE2ESourceVersions = []string{
	"v2.0.0-alpha.4",
	"v2.0.0-alpha.5",
	"v2.0.0-beta.1",
	"v2.0.0-beta.2",
	"v2.0.0-beta.3",
}

const upgradeE2ETargetVersion = "v2.0.0-beta.4"

// TestUpgradeToHEAD scaffolds with each released CLI in
// upgradeE2ESourceVersions, upgrades the result with this checkout, applies
// every manual action the upgrade reports, and requires the result to build and
// vet cleanly.
//
// The test downloads the released CLI and modules, so it only runs in nightly
// and release-readiness. Set ANDUREL_UPGRADE_E2E=1 to enable it.
//
// Set ANDUREL_UPGRADE_E2E_LOCAL_PKG=1 to replace every pkg/* module with this
// checkout after the upgrade, the way `just local-replace` does. Use it on a
// branch whose package changes are not published yet; the default variant
// stays on the published pins for nightly and release-readiness.
func TestUpgradeToHEAD(t *testing.T) {
	if os.Getenv("ANDUREL_UPGRADE_E2E") != "1" {
		t.Skip("upgrade end-to-end test requires ANDUREL_UPGRADE_E2E=1")
	}
	localPkg := os.Getenv("ANDUREL_UPGRADE_E2E_LOCAL_PKG") == "1"
	andurelBinary := buildUpgradeE2EAndurel(t)

	for _, source := range upgradeE2ESourceVersions {
		t.Run(source, func(t *testing.T) {
			testUpgradeToHEAD(t, source, localPkg, andurelBinary)
		})
	}
}

func testUpgradeToHEAD(t *testing.T, source string, localPkg bool, andurelBinary string) {
	parent := t.TempDir()
	appName := "upgradee2e"
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	scaffold := exec.CommandContext(
		ctx,
		"go",
		"run",
		"github.com/mbvlabs/andurel/v2@"+source,
		"new",
		appName,
	)
	scaffold.Dir = parent
	scaffold.Env = append(os.Environ(), "GOWORK=off")
	if output, err := scaffold.CombinedOutput(); err != nil {
		t.Fatalf("scaffold with %s: %v\n%s", source, err, output)
	}

	root := filepath.Join(parent, appName)
	gitRun(t, root, "init")
	gitRun(t, root, "config", "user.email", "upgrade-e2e@example.com")
	gitRun(t, root, "config", "user.name", "Upgrade E2E")
	commitUpgradeTree(t, root, "scaffold from "+source)

	upgrader, err := NewUpgrader(
		root,
		UpgradeOptions{TargetVersion: upgradeE2ETargetVersion},
	)
	if err != nil {
		t.Fatal(err)
	}
	report, err := upgrader.Execute()
	if err != nil {
		t.Fatalf("upgrade %s to %s: %v", source, upgradeE2ETargetVersion, err)
	}

	for _, action := range report.ManualActions {
		t.Logf("applying manual action %s", action.ID)
		if err := applyUpgradeE2EManualAction(t, root, andurelBinary, action); err != nil {
			t.Fatalf(
				"manual action %q (%s) was not applied: %v",
				action.ID,
				action.Title,
				err,
			)
		}
	}

	if localPkg {
		replaceWithLocalPackages(t, root)
	}

	runUpgradeE2EGo(t, root, "mod", "tidy")
	runUpgradeE2EGo(t, root, "build", "./...")
	runUpgradeE2EGo(t, root, "vet", "./...")
}

// replaceWithLocalPackages points every pkg/* module in the upgraded project at
// this checkout, mirroring `just local-replace`.
func replaceWithLocalPackages(t *testing.T, root string) {
	t.Helper()

	framework := localCheckoutRoot(t)
	moduleFiles, err := filepath.Glob(filepath.Join(framework, "pkg", "*", "go.mod"))
	if err != nil {
		t.Fatalf("find pkg modules: %v", err)
	}
	if len(moduleFiles) == 0 {
		t.Fatalf("no pkg modules under %s", framework)
	}
	sort.Strings(moduleFiles)

	for _, moduleFile := range moduleFiles {
		moduleDir := filepath.Dir(moduleFile)
		module := filepath.Base(moduleDir)
		replacement := "github.com/mbvlabs/andurel/pkg/" + module + "=" + moduleDir
		runUpgradeE2EGo(t, root, "mod", "edit", "-replace", replacement)
	}
}

// localCheckoutRoot returns the repository root for this checkout, which owns
// the pkg/* modules.
func localCheckoutRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve the upgrade e2e test file")
	}

	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

func applyUpgradeE2EManualAction(
	t *testing.T,
	root string,
	andurelBinary string,
	action ManualAction,
) error {
	t.Helper()

	switch action.ID {
	case "public-home-v2.0.0-beta.4":
		return applyUpgradeE2EPublicHome(root)
	case "configcheck-v2.0.0-beta.4":
		return applyUpgradeE2EConfigCheck(root)
	case "runtime-readiness-v2.0.0-beta.4":
		return applyUpgradeE2ERuntimeReadiness(root)
	case "inertia-ssr-default-v2.0.0-beta.4":
		return applyUpgradeE2EInertiaSSRDefault(root)
	case "routes-host-scope-v2.0.0-beta.4":
		return applyUpgradeE2ERoutesHostScope(t, root, andurelBinary)
	case "package-api-v2.0.0-alpha.5":
		return applyUpgradeE2EPackageAPI(root)
	case "storage-storagetest-v2.0.0-alpha.5":
		return applyUpgradeE2EStorageTestCluster(root)
	case "inertia-vite-fullurl-v2.0.0":
		return applyUpgradeE2EInertiaFullURL(root)
	case "routing-hosts-v2.0.0-alpha.5":
		return applyUpgradeE2ERoutingHosts(root)
	case "session-revocation-v2.0.0-alpha.5":
		return applyUpgradeE2ESessionRevocation(root)
	case "email-smtp-v2.0.0-alpha.5":
		return applyUpgradeE2EEmailSMTP(root)
	case "validation-validate-v2.0.0-alpha.5":
		return applyUpgradeE2EValidation(root)
	case "email-sender-v2.0.0-beta.2":
		return applyUpgradeE2EEmailSender(root)
	case "vite-ssr-bundle-v2.0.0-beta.2":
		return applyUpgradeE2EViteSSRBundle(root)
	case "root-title-ssr-v2.0.0-beta.2":
		return applyUpgradeE2ERootTitle(root)
	case "cors-host-origins-v2.0.0-beta.3":
		return applyUpgradeE2ECorsHostOrigins(root)
	case "csrf-asset-skip-v2.0.0-beta.3":
		return applyUpgradeE2ECSRFAssetSkip(root)
	case "method-not-allowed-page-v2.0.0-beta.3":
		return applyUpgradeE2EMethodNotAllowedPage(root)
	case "request-host-middleware-v2.0.0-beta.3":
		// Advisory: the action points at the new anduin.RequestHost API and the
		// scaffold has no hand-written host matching to replace.
		return nil
	default:
		return fmt.Errorf(
			"no automated e2e implementation for manual action %q (%s)",
			action.ID,
			action.Title,
		)
	}
}

// buildUpgradeE2EAndurel builds this checkout's CLI once per test run. The
// The upgrade target is this checkout, so route regeneration must use the local
// generator instead of a published release.
func buildUpgradeE2EAndurel(t *testing.T) string {
	t.Helper()

	framework := localCheckoutRoot(t)
	binary := filepath.Join(t.TempDir(), "andurel")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	command := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	command.Dir = framework
	// Build the CLI with the same standalone module resolution as CI. The CLI
	// does not import the candidate Inertia module; generated apps receive local
	// replacements before dependency resolution in the development variant.
	command.Env = append(os.Environ(), "GOWORK=off")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build local andurel CLI: %v\n%s", err, output)
	}

	return binary
}

func runUpgradeE2EAndurel(t *testing.T, root, binary string, args ...string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	command := exec.CommandContext(ctx, binary, args...)
	command.Dir = root
	command.Env = append(os.Environ(), "GOWORK=off")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("andurel %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func renderUpgradeE2ETemplate(name, modulePath string) ([]byte, error) {
	content, err := layouttemplates.Files.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("read template %s: %w", name, err)
	}

	parsed, err := template.New(name).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}

	var rendered bytes.Buffer
	if err := parsed.Execute(&rendered, struct{ ModuleName string }{ModuleName: modulePath}); err != nil {
		return nil, fmt.Errorf("render template %s: %w", name, err)
	}

	return rendered.Bytes(), nil
}

// applyUpgradeE2EPublicHome follows the public-home-v2.0.0-beta.4
// instructions: the generated welcome page is public, while private routes
// keep middleware.AuthOnly.
func applyUpgradeE2EPublicHome(root string) error {
	path := filepath.Join(root, "internal", "runtime", "runtime.go")
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	text := string(content)
	const protected = "handlers.NewPagesHandler(renderer).Register(router, middleware.AuthOnly(users))"
	const public = "handlers.NewPagesHandler(renderer).Register(router)"
	switch {
	case strings.Contains(text, protected):
		text = strings.Replace(text, protected, public, 1)
	case strings.Contains(text, public):
		// Already public.
	case strings.Contains(text, "handlers.NewPages(renderer).Register(router)"):
		// The alpha.4 generated Pages handler is public already.
	default:
		return fmt.Errorf("public-home-v2.0.0-beta.4: no generated home registration in %s", path)
	}

	if text != string(content) {
		if err := writeGofmtFile(path, []byte(text)); err != nil {
			return err
		}
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if strings.Contains(string(updated), protected) {
		return fmt.Errorf("public-home-v2.0.0-beta.4: home page is still behind AuthOnly in %s", path)
	}
	if !strings.Contains(string(updated), public) &&
		!strings.Contains(string(updated), "handlers.NewPages(renderer).Register(router)") {
		return fmt.Errorf("public-home-v2.0.0-beta.4: no public home registration in %s", path)
	}

	return nil
}

// applyUpgradeE2EConfigCheck follows the configcheck-v2.0.0-beta.4
// instructions: create the read-only helper from the current scaffold.
func applyUpgradeE2EConfigCheck(root string) error {
	modulePath, err := upgradeE2EModulePath(root)
	if err != nil {
		return err
	}

	content, err := renderUpgradeE2ETemplate("internal_configcheck_main.tmpl", modulePath)
	if err != nil {
		return err
	}

	path := filepath.Join(root, "internal", "configcheck", "main.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := writeGofmtFile(path, content); err != nil {
		return err
	}

	written, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(written)
	for _, want := range []string{
		"env.Load()",
		"config.New()",
		"storage.NewPostgres(ctx, cfg, storage.WithoutOpenTelemetry())",
		"db.Health(ctx)",
		"json.NewEncoder(os.Stdout)",
		"redactSecret",
	} {
		if !strings.Contains(text, want) {
			return fmt.Errorf(
				"configcheck-v2.0.0-beta.4: generated helper is missing %q",
				want,
			)
		}
	}
	if strings.Contains(text, "{{.") {
		return fmt.Errorf("configcheck-v2.0.0-beta.4: helper has an unresolved placeholder")
	}

	return nil
}

// applyUpgradeE2ERuntimeReadiness follows the
// runtime-readiness-v2.0.0-beta.4 instructions: database plus renderer
// readiness, /api/ready, and a handler that writes the 503 body itself.
func applyUpgradeE2ERuntimeReadiness(root string) error {
	runtimePath := filepath.Join(root, "internal", "runtime", "runtime.go")
	content, err := os.ReadFile(runtimePath)
	if err != nil {
		return err
	}

	text := string(content)
	const oldRegistration = "handlers.NewAPIHandler(db).Register(router)"
	const newRegistration = "handlers.NewAPIHandler(db, newReadiness(db, renderer)).Register(router)"
	if !strings.Contains(text, newRegistration) {
		if !strings.Contains(text, oldRegistration) {
			return fmt.Errorf(
				"runtime-readiness-v2.0.0-beta.4: no NewAPIHandler registration in %s",
				runtimePath,
			)
		}
		text = strings.Replace(text, oldRegistration, newRegistration, 1)
	}

	if !strings.Contains(text, "func newReadiness(") {
		const marker = "func (a *App) Start("
		index := strings.Index(text, marker)
		if index < 0 {
			return fmt.Errorf(
				"runtime-readiness-v2.0.0-beta.4: no App.Start in %s",
				runtimePath,
			)
		}
		text = text[:index] + readinessE2EImplementation + "\n" + text[index:]
	}
	if !strings.Contains(text, "\"errors\"") {
		return fmt.Errorf(
			"runtime-readiness-v2.0.0-beta.4: %s does not import errors",
			runtimePath,
		)
	}

	if err := writeGofmtFile(runtimePath, []byte(text)); err != nil {
		return err
	}

	updated, err := os.ReadFile(runtimePath)
	if err != nil {
		return err
	}
	updatedText := string(updated)
	if strings.Contains(updatedText, oldRegistration) {
		return fmt.Errorf(
			"runtime-readiness-v2.0.0-beta.4: runtime still registers only health in %s",
			runtimePath,
		)
	}
	for _, want := range []string{
		newRegistration,
		"type readiness struct {",
		"func (err readinessError) ReadinessReason() string",
		"r.renderer.Health(ctx)",
	} {
		if !strings.Contains(updatedText, want) {
			return fmt.Errorf(
				"runtime-readiness-v2.0.0-beta.4: runtime is missing %q",
				want,
			)
		}
	}

	if err := applyUpgradeE2EReadyRoute(root); err != nil {
		return err
	}

	return applyUpgradeE2EAPIHandler(root)
}

func applyUpgradeE2EReadyRoute(root string) error {
	path := filepath.Join(root, "routes", "api.go")
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	text := string(content)
	if !strings.Contains(text, "var Health =") {
		return fmt.Errorf(
			"runtime-readiness-v2.0.0-beta.4: no Health route in %s",
			path,
		)
	}
	if strings.Contains(text, "var Ready =") {
		if !strings.Contains(text, "\"/ready\"") || !strings.Contains(text, "\"api.ready\"") {
			return fmt.Errorf(
				"runtime-readiness-v2.0.0-beta.4: existing Ready route in %s is not /api/ready",
				path,
			)
		}
		return nil
	}

	content = append(content, []byte("\n"+readyRouteE2EImplementation)...)
	if err := writeGofmtFile(path, content); err != nil {
		return err
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !strings.Contains(string(updated), "\"/ready\"") ||
		!strings.Contains(string(updated), "\"api.ready\"") {
		return fmt.Errorf(
			"runtime-readiness-v2.0.0-beta.4: Ready route was not added to %s",
			path,
		)
	}

	return nil
}

func applyUpgradeE2EAPIHandler(root string) error {
	path := filepath.Join(root, "handlers", "api.go")
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !strings.Contains(string(content), "func NewAPIHandler(") {
		return fmt.Errorf(
			"runtime-readiness-v2.0.0-beta.4: no API handler in %s",
			path,
		)
	}

	modulePath, err := upgradeE2EModulePath(root)
	if err != nil {
		return err
	}
	rendered, err := renderUpgradeE2ETemplate("handlers_api.tmpl", modulePath)
	if err != nil {
		return err
	}
	if err := writeGofmtFile(path, rendered); err != nil {
		return err
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(updated)
	for _, want := range []string{
		"readinessHandler",
		"routes.Ready.Pattern()",
		`"status": "unready"`,
		"ReadinessReason() string",
	} {
		if !strings.Contains(text, want) {
			return fmt.Errorf(
				"runtime-readiness-v2.0.0-beta.4: API handler is missing %q",
				want,
			)
		}
	}

	return nil
}

// applyUpgradeE2EInertiaSSRDefault follows the
// inertia-ssr-default-v2.0.0-beta.4 instructions: the renderer-wide
// production default, the themed fallback error page, and no accidental
// client-rendering override.
func applyUpgradeE2EInertiaSSRDefault(root string) error {
	runtimePath := filepath.Join(root, "internal", "runtime", "runtime.go")
	content, err := os.ReadFile(runtimePath)
	if err != nil {
		return err
	}

	text := string(content)
	const ssrDefault = "inertia.WithSSRDefault(cfg.App.IsProduction()),"
	if !strings.Contains(text, ssrDefault) {
		const marker = "inertia.WithEnvironment(cfg.App.Environment),"
		index := strings.Index(text, marker)
		if index < 0 {
			return fmt.Errorf(
				"inertia-ssr-default-v2.0.0-beta.4: no renderer environment option in %s",
				runtimePath,
			)
		}
		insertAt := index + len(marker)
		text = text[:insertAt] + "\n\t\t" + ssrDefault + text[insertAt:]
	}

	if err := writeGofmtFile(runtimePath, []byte(text)); err != nil {
		return err
	}

	updated, err := os.ReadFile(runtimePath)
	if err != nil {
		return err
	}
	if !strings.Contains(string(updated), ssrDefault) {
		return fmt.Errorf(
			"inertia-ssr-default-v2.0.0-beta.4: renderer default was not added to %s",
			runtimePath,
		)
	}

	if err := applyUpgradeE2EErrorFallback(root); err != nil {
		return err
	}

	return applyUpgradeE2ESSRFailFastEnv(root)
}

func applyUpgradeE2EErrorFallback(root string) error {
	path := filepath.Join(root, "handlers", "errors.go")
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	text := string(content)
	const fallback = `FallbackComponent: "Errors/InternalError"`
	if strings.Contains(text, fallback) {
		if !strings.Contains(text, "inertia.NewErrorHandler(") {
			return fmt.Errorf(
				"inertia-ssr-default-v2.0.0-beta.4: fallback is not on NewErrorHandler in %s",
				path,
			)
		}
		return nil
	}

	if strings.Contains(text, "inertia.ErrorHandler(renderer,") {
		replacement := "inertia.NewErrorHandler(\n\t\trenderer,\n" +
			"\t\tinertia.ErrorHandlerOptions{" + fallback + "},"
		text = strings.Replace(text, "inertia.ErrorHandler(renderer,", replacement, 1)
		if err := writeGofmtFile(path, []byte(text)); err != nil {
			return err
		}
	} else if strings.Contains(text, "func MapError(renderer *inertia.Renderer) anduin.ErrorHandler {") {
		// The alpha.4 scaffold still has a handwritten switch. Adopt the
		// current scaffold handler so unmapped 5xx responses get the themed
		// fallback while error mappings stay client-rendered.
		modulePath, err := upgradeE2EModulePath(root)
		if err != nil {
			return err
		}
		rendered, err := renderUpgradeE2ETemplate("handlers_errors.tmpl", modulePath)
		if err != nil {
			return err
		}
		if err := writeGofmtFile(path, rendered); err != nil {
			return err
		}
	} else {
		return fmt.Errorf(
			"inertia-ssr-default-v2.0.0-beta.4: no error handler in %s",
			path,
		)
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !strings.Contains(string(updated), fallback) ||
		!strings.Contains(string(updated), "inertia.NewErrorHandler(") {
		return fmt.Errorf(
			"inertia-ssr-default-v2.0.0-beta.4: themed fallback was not configured in %s",
			path,
		)
	}

	return nil
}

func applyUpgradeE2ESSRFailFastEnv(root string) error {
	path := filepath.Join(root, ".env.example")
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	changed := false
	for index, line := range lines {
		if strings.TrimSpace(line) == "INERTIA_SSR_FAIL_FAST=false" {
			lines[index] = "# INERTIA_SSR_FAIL_FAST=false"
			changed = true
		}
	}
	if !changed {
		return nil
	}

	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return err
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for line := range strings.SplitSeq(string(updated), "\n") {
		if strings.TrimSpace(line) == "INERTIA_SSR_FAIL_FAST=false" {
			return fmt.Errorf(
				"inertia-ssr-default-v2.0.0-beta.4: %s still forces client-rendering fallback",
				path,
			)
		}
	}

	return nil
}

// applyUpgradeE2ERoutesHostScope follows the
// routes-host-scope-v2.0.0-beta.4 instructions: regenerate routes.ts with the
// local candidate CLI and adopt the per-render SSR host scope.
func applyUpgradeE2ERoutesHostScope(t *testing.T, root, andurelBinary string) error {
	t.Helper()

	if andurelBinary == "" {
		return fmt.Errorf("routes-host-scope-v2.0.0-beta.4: local andurel binary is missing")
	}
	if err := applyUpgradeE2EAlphaFourRouteNames(root); err != nil {
		return err
	}
	runUpgradeE2EAndurel(t, root, andurelBinary, "sync", "routes")

	if err := verifyUpgradeE2ERoutesJS(root); err != nil {
		return err
	}
	if err := verifyUpgradeE2EAppRouteHosts(root); err != nil {
		return err
	}

	return applyUpgradeE2ESSRRouteHosts(root)
}

// applyUpgradeE2EAlphaFourRouteNames repairs the one duplicate route name in
// the alpha.4 generated routes. The current scaffold names PasswordUpdate
// users.update_user_password; alpha.4 reused users.user_password, and the
// beta.4 generator rejects the duplicate before it writes routes.ts.
func applyUpgradeE2EAlphaFourRouteNames(root string) error {
	path := filepath.Join(root, "routes", "users.go")
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	text := string(content)
	const duplicate = `"users.user_password"`
	if strings.Count(text, duplicate) < 2 {
		return nil
	}

	first := strings.Index(text, duplicate)
	relative := strings.Index(text[first+len(duplicate):], duplicate)
	if relative < 0 {
		return fmt.Errorf(
			"routes-host-scope-v2.0.0-beta.4: could not locate the duplicate route name in %s",
			path,
		)
	}
	second := first + len(duplicate) + relative
	text = text[:second] + `"users.update_user_password"` + text[second+len(duplicate):]

	if err := writeGofmtFile(path, []byte(text)); err != nil {
		return err
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if got := strings.Count(string(updated), duplicate); got != 1 {
		return fmt.Errorf(
			"routes-host-scope-v2.0.0-beta.4: %s still has %d users.user_password route names",
			path,
			got,
		)
	}

	return nil
}

func verifyUpgradeE2ERoutesJS(root string) error {
	path := filepath.Join(root, "resources", "js", "routes.ts")
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	text := string(content)
	for _, want := range []string{
		"configureRouteHostScope",
		"withRouteHosts",
		"route hosts are not configured yet",
	} {
		if !strings.Contains(text, want) {
			return fmt.Errorf(
				"routes-host-scope-v2.0.0-beta.4: regenerated %s is missing %q",
				path,
				want,
			)
		}
	}
	if strings.Contains(text, "let origins: HostOrigins = {}") {
		return fmt.Errorf(
			"routes-host-scope-v2.0.0-beta.4: %s still has the silent empty-origin fallback",
			path,
		)
	}

	return rejectUpgradeE2EModuleScopeRouteCache(path, text)
}

func verifyUpgradeE2EAppRouteHosts(root string) error {
	path, err := upgradeE2EAppEntry(root)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	text := string(content)
	if got := strings.Count(text, "configureRouteHosts("); got != 1 {
		return fmt.Errorf(
			"routes-host-scope-v2.0.0-beta.4: %s has %d boot configurations, want 1",
			path,
			got,
		)
	}

	return rejectUpgradeE2EModuleScopeRouteCache(path, text)
}

func applyUpgradeE2ESSRRouteHosts(root string) error {
	lock, err := layout.ReadLockFile(root)
	if err != nil {
		return err
	}
	if lock.ScaffoldConfig == nil {
		return fmt.Errorf("routes-host-scope-v2.0.0-beta.4: lock file has no scaffold config")
	}

	var templateName string
	var target string
	switch lock.ScaffoldConfig.Inertia {
	case "react":
		templateName = "inertia_react_assets_ssr.tmpl"
		target = filepath.Join(root, "resources", "js", "ssr.tsx")
	case "vue":
		templateName = "inertia_assets_ssr.tmpl"
		target = filepath.Join(root, "resources", "js", "ssr.ts")
	case "svelte":
		templateName = "inertia_svelte_assets_ssr.tmpl"
		target = filepath.Join(root, "resources", "js", "ssr.ts")
	default:
		return fmt.Errorf(
			"routes-host-scope-v2.0.0-beta.4: unsupported Inertia adapter %q",
			lock.ScaffoldConfig.Inertia,
		)
	}

	content, err := os.ReadFile(target)
	if err != nil {
		return err
	}
	text := string(content)
	if strings.Contains(text, "configureRouteHostScope(") &&
		strings.Contains(text, "withRouteHosts(") {
		return rejectUpgradeE2EModuleScopeRouteCache(target, text)
	}
	if !strings.Contains(text, "configureRouteHosts(") {
		return fmt.Errorf(
			"routes-host-scope-v2.0.0-beta.4: no route host boot configuration in %s",
			target,
		)
	}

	rendered, err := layouttemplates.Files.ReadFile(templateName)
	if err != nil {
		return err
	}
	if err := os.WriteFile(target, rendered, 0o644); err != nil {
		return err
	}

	updated, err := os.ReadFile(target)
	if err != nil {
		return err
	}
	updatedText := string(updated)
	for _, want := range []string{"configureRouteHostScope(", "withRouteHosts("} {
		if !strings.Contains(updatedText, want) {
			return fmt.Errorf(
				"routes-host-scope-v2.0.0-beta.4: %s is missing %q",
				target,
				want,
			)
		}
	}

	return rejectUpgradeE2EModuleScopeRouteCache(target, updatedText)
}

func upgradeE2EAppEntry(root string) (string, error) {
	for _, name := range []string{"app.tsx", "app.ts", "app.jsx", "app.js"} {
		path := filepath.Join(root, "resources", "js", name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("routes-host-scope-v2.0.0-beta.4: no browser entry under resources/js")
}

func rejectUpgradeE2EModuleScopeRouteCache(path, content string) error {
	for index, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if line == trimmed &&
			strings.HasPrefix(trimmed, "const ") &&
			strings.Contains(trimmed, "routes.") {
			return fmt.Errorf(
				"routes-host-scope-v2.0.0-beta.4: %s:%d caches a route helper at module scope; call it at render time or use .path()",
				path,
				index+1,
			)
		}
	}

	return nil
}

const readinessE2EImplementation = `// readiness reports whether the process can serve traffic. Liveness stays
// separate: the database must answer, and the renderer decides whether a
// required SSR sidecar must answer too. The renderer treats a disabled
// renderer-wide default, an explicit CSR fallback, and an unsupported
// transport as not gating readiness, so this type only maps its typed reason.
type readiness struct {
	database storage.Connection
	renderer *inertia.Renderer
}

func newReadiness(db storage.Connection, renderer *inertia.Renderer) readiness {
	return readiness{database: db, renderer: renderer}
}

// readinessError carries a stable, secret-free reason for /api/ready. The
// handler reads the reason through ReadinessReason instead of matching error
// text.
type readinessError struct {
	reason string
}

func (err readinessError) Error() string { return err.reason }

func (err readinessError) ReadinessReason() string { return err.reason }

// Health returns the first reason the process is not ready. The reasons are
// stable and carry no configuration values, so readiness output cannot leak a
// connection string.
func (r readiness) Health(ctx context.Context) error {
	if err := r.database.Health(ctx); err != nil {
		return readinessError{reason: "database_unavailable"}
	}
	if r.renderer == nil {
		return nil
	}
	if err := r.renderer.Health(ctx); err != nil {
		reason := "unavailable"
		var ssrErr *inertia.ReadinessError
		if errors.As(err, &ssrErr) && ssrErr.Status != "" {
			reason = string(ssrErr.Status)
		}

		return readinessError{reason: reason}
	}

	return nil
}
`

const readyRouteE2EImplementation = `// Ready is the readiness probe: it answers 200 only when the database answers
// and, outside development, the Inertia SSR sidecar is healthy. Liveness stays
// on Health so orchestration can tell "running" from "ready".
var Ready = routing.NewSimpleRoute(
	"/ready",
	"api.ready",
	APIPrefix,
)
`

// applyUpgradeE2EPackageAPI follows the package-api-v2.0.0-alpha.5
// instructions against internal/runtime, which is application-owned.
func applyUpgradeE2EPackageAPI(root string) error {
	path := filepath.Join(root, "internal", "runtime", "runtime.go")
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	replacements := []struct {
		description string
		old         string
		new         string
	}{
		{
			description: "anduin.NewServer drops the environment and shutdowner arguments",
			old: "\tsrv := anduin.NewServer(\n\t\tctx,\n\t\tcfg.HTTP.Host,\n" +
				"\t\tcfg.HTTP.Port,\n\t\tcfg.App.Environment,\n\t\thandler,\n\t\tnil,\n",
			new: "\tsrv := anduin.NewServer(\n\t\tctx,\n\t\tcfg.HTTP.Host,\n" +
				"\t\tcfg.HTTP.Port,\n\t\thandler,\n",
		},
		{
			description: "Server.Start takes no arguments",
			old:         "return a.Server.Start(ctx, a.Config.App.Environment)",
			new:         "return a.Server.Start()",
		},
		{
			description: "inertia.NewRenderer takes inertia.RendererConfig",
			old: "\treturn inertia.NewRenderer(\n\t\tcfg.Inertia.ContainerID,\n" +
				"\t\troutes.ViteBuild.FullURL(),\n\t\tcfg.Inertia.EntryPoint,\n" +
				"\t\tcfg.Inertia.ViteDevURL,\n\t\tcfg.Inertia.SSRURL,\n" +
				"\t\tcfg.Inertia.SSRRequestTimeout,\n\t\tcfg.Inertia.SSRMaxResponseBytes,\n",
			new: "\treturn inertia.NewRenderer(\n\t\tinertia.RendererConfig{\n" +
				"\t\t\tContainerID:         cfg.Inertia.ContainerID,\n" +
				"\t\t\tBuildPathURL:        routes.ViteBuild.FullURL(),\n" +
				"\t\t\tEntryPoint:          cfg.Inertia.EntryPoint,\n" +
				"\t\t\tViteDevURL:          cfg.Inertia.ViteDevURL,\n" +
				"\t\t\tSSRURL:              cfg.Inertia.SSRURL,\n" +
				"\t\t\tSSRTimeout:          cfg.Inertia.SSRRequestTimeout,\n" +
				"\t\t\tSSRMaxResponseBytes: cfg.Inertia.SSRMaxResponseBytes,\n" +
				"\t\t},\n",
		},
		{
			description: "cache.Store.Close runs on shutdown",
			old: "\t\tclosers: []func(context.Context) error{\n" +
				"\t\t\tfunc(context.Context) error { return db.Close() },\n" +
				"\t\t\ttel.Shutdown,\n\t\t},",
			new: "\t\tclosers: []func(context.Context) error{\n" +
				"\t\t\tfunc(context.Context) error { return db.Close() },\n" +
				"\t\t\tfunc(context.Context) error { return cacheStore.Close() },\n" +
				"\t\t\ttel.Shutdown,\n\t\t},",
		},
	}

	for _, replacement := range replacements {
		if !strings.Contains(string(content), replacement.old) {
			return fmt.Errorf(
				"%s: expected snippet not found in %s",
				replacement.description,
				path,
			)
		}
		content = []byte(strings.Replace(string(content), replacement.old, replacement.new, 1))
	}
	if err := writeGofmtFile(path, content); err != nil {
		return err
	}

	return applyUpgradeE2ESSRRuntime(root)
}

// applyUpgradeE2ESSRRuntime follows the same action's NewSSRRuntime change in
// cmd/ssr, another application-owned entrypoint.
func applyUpgradeE2ESSRRuntime(root string) error {
	path := filepath.Join(root, "cmd", "ssr", "main.go")
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	text := string(content)
	old := "\truntime, err := inertia.NewSSRRuntime(\n\t\tcfg.SSRRuntime,\n" +
		"\t\tcfg.SSRBundle,\n\t\tcfg.SSRListen,\n\t\tcfg.SSRStartupTimeout,\n" +
		"\t\tcfg.SSRMinimumMajor,\n\t\tcfg.SSRRequestTimeout,\n\t\tassets.Files,\n\t)\n"
	if !strings.Contains(text, old) {
		return nil
	}
	new := "\truntime, err := inertia.NewSSRRuntime(inertia.SSRRuntimeConfig{\n" +
		"\t\tExecutable:     cfg.SSRRuntime,\n" +
		"\t\tBundlePath:     cfg.SSRBundle,\n" +
		"\t\tListenURL:      cfg.SSRListen,\n" +
		"\t\tStartupTimeout: cfg.SSRStartupTimeout,\n" +
		"\t\tMinimumMajor:   cfg.SSRMinimumMajor,\n" +
		"\t\tHealthTimeout:  cfg.SSRRequestTimeout,\n" +
		"\t\tBundleFS:       assets.Files,\n" +
		"\t})\n"
	return writeGofmtFile(path, []byte(strings.Replace(text, old, new, 1)))
}

// applyUpgradeE2EStorageTestCluster follows the
// storage-storagetest-v2.0.0-alpha.5 instructions. The alpha.4 scaffold has no
// test-cluster helper references, so this is normally a no-op.
func applyUpgradeE2EStorageTestCluster(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(content)
		updated := strings.ReplaceAll(text, "storage.TestCluster", "storagetest.Cluster")
		updated = strings.ReplaceAll(updated, "storage.NewTestCluster", "storagetest.NewCluster")
		if updated == text {
			return nil
		}
		return fmt.Errorf(
			"%s uses storage test-cluster helpers; update its imports by hand",
			path,
		)
	})
}

type upgradeE2EReplacement struct {
	description string
	old         string
	new         string
	all         bool
}

func upgradeE2EModulePath(root string) (string, error) {
	content, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}

	for line := range strings.SplitSeq(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}

	return "", fmt.Errorf("no module line in %s/go.mod", root)
}

func applyUpgradeE2EReplacements(path string, replacements []upgradeE2EReplacement) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	text := string(content)
	for _, replacement := range replacements {
		if !strings.Contains(text, replacement.old) {
			return fmt.Errorf(
				"%s: expected snippet not found in %s",
				replacement.description,
				path,
			)
		}
		if replacement.all {
			text = strings.ReplaceAll(text, replacement.old, replacement.new)
		} else {
			text = strings.Replace(text, replacement.old, replacement.new, 1)
		}
	}

	if !strings.HasSuffix(path, ".go") {
		return os.WriteFile(path, []byte(text), 0o644)
	}

	return writeGofmtFile(path, []byte(text))
}

// applyUpgradeE2ERoutingHosts follows the routing-hosts-v2.0.0-alpha.5
// instructions. It runs after the package API action, which rewrites
// newRenderer to the RendererConfig form.
func applyUpgradeE2ERoutingHosts(root string) error {
	runtimePath := filepath.Join(root, "internal", "runtime", "runtime.go")
	if err := applyUpgradeE2EReplacements(runtimePath, []upgradeE2EReplacement{
		{
			description: "import the routing package",
			old:         "\t\"github.com/mbvlabs/andurel/pkg/kiks\"\n",
			new:         "\t\"github.com/mbvlabs/andurel/pkg/kiks\"\n\t\"github.com/mbvlabs/andurel/pkg/routing\"\n",
		},
		{
			description: "build the host registry in New",
			old:         "func New(ctx context.Context, cfg config.Config) (*App, error) {\n\ttel, err := newTelemetry",
			new: "func New(ctx context.Context, cfg config.Config) (*App, error) {\n" +
				"\thosts, err := routing.NewHosts(cfg.App.Hosts)\n" +
				"\tif err != nil {\n\t\treturn nil, fmt.Errorf(\"runtime: configure hosts: %w\", err)\n\t}\n\n" +
				"\ttel, err := newTelemetry",
		},
		{
			description: "pass hosts to services.NewIdentity",
			old:         "\tidentity := services.NewIdentity(transactor, users, tokens, queueInsert, cfg.Auth, cfg.Mail)",
			new: "\tidentity := services.NewIdentity(\n\t\ttransactor,\n\t\tusers,\n\t\ttokens,\n" +
				"\t\tqueueInsert,\n\t\tcfg.Auth,\n\t\tcfg.Mail,\n\t\thosts,\n\t)",
		},
		{
			description: "pass hosts to newRenderer",
			old:         "\trenderer, err := newRenderer(cfg)",
			new:         "\trenderer, err := newRenderer(cfg, hosts)",
		},
		{
			description: "install the hosts registry on the router",
			old:         "\trouter := anduin.NewRouter(\n\t\tanduin.WithErrorHandler(handlers.MapError(renderer)),",
			new:         "\trouter := anduin.NewRouter(\n\t\tanduin.WithHosts(hosts),\n\t\tanduin.WithErrorHandler(handlers.MapError(renderer)),",
		},
		{
			description: "newRenderer takes the hosts registry",
			old:         "func newRenderer(cfg config.Config) (*inertia.Renderer, error) {",
			new:         "func newRenderer(cfg config.Config, hosts *routing.Hosts) (*inertia.Renderer, error) {",
		},
		{
			description: "the Inertia build URL takes the hosts registry",
			old:         "routes.ViteBuild.FullURL()",
			new:         "routes.ViteBuild.FullURL(hosts)",
		},
	}); err != nil {
		return err
	}

	queuePath := filepath.Join(root, "internal", "runtime", "queue.go")
	if err := applyUpgradeE2EReplacements(queuePath, []upgradeE2EReplacement{
		{
			description: "import fmt",
			old:         "\t\"context\"\n\t\"errors\"\n\t\"slices\"",
			new:         "\t\"context\"\n\t\"errors\"\n\t\"fmt\"\n\t\"slices\"",
		},
		{
			description: "import the routing package",
			old:         "\t\"github.com/mbvlabs/andurel/pkg/storage\"\n)",
			new:         "\t\"github.com/mbvlabs/andurel/pkg/routing\"\n\t\"github.com/mbvlabs/andurel/pkg/storage\"\n)",
		},
		{
			description: "build the host registry in NewQueue",
			old:         "func NewQueue(ctx context.Context, cfg config.Config) (*Queue, error) {\n\ttel, err := newTelemetry",
			new: "func NewQueue(ctx context.Context, cfg config.Config) (*Queue, error) {\n" +
				"\thosts, err := routing.NewHosts(cfg.App.Hosts)\n" +
				"\tif err != nil {\n\t\treturn nil, fmt.Errorf(\"runtime: configure hosts: %w\", err)\n\t}\n\n" +
				"\ttel, err := newTelemetry",
		},
		{
			description: "pass hosts to services.NewIdentity",
			old:         "\t\tcfg.Auth,\n\t\tcfg.Mail,\n\t)",
			new:         "\t\tcfg.Auth,\n\t\tcfg.Mail,\n\t\thosts,\n\t)",
		},
	}); err != nil {
		return err
	}

	identityPath := filepath.Join(root, "services", "identity.go")
	if err := applyUpgradeE2EReplacements(identityPath, []upgradeE2EReplacement{
		{
			description: "import the routing package",
			old:         "\t\"github.com/mbvlabs/andurel/pkg/storage\"\n)",
			new:         "\t\"github.com/mbvlabs/andurel/pkg/routing\"\n\t\"github.com/mbvlabs/andurel/pkg/storage\"\n)",
		},
		{
			description: "store the hosts registry on Identity",
			old:         "\tdefaultSenderSignature string\n}",
			new:         "\tdefaultSenderSignature string\n\thosts                  *routing.Hosts\n}",
		},
		{
			description: "NewIdentity takes the hosts registry",
			old:         "\tmailCfg config.Mail,\n) Identity {",
			new:         "\tmailCfg config.Mail,\n\thosts *routing.Hosts,\n) Identity {",
		},
		{
			description: "assign the hosts registry",
			old:         "\t\tdefaultSenderSignature: mailCfg.DefaultSenderSignature,\n\t}",
			new:         "\t\tdefaultSenderSignature: mailCfg.DefaultSenderSignature,\n\t\thosts:                  hosts,\n\t}",
		},
	}); err != nil {
		return err
	}

	resetPath := filepath.Join(root, "services", "reset_password.go")
	if err := applyUpgradeE2EReplacements(resetPath, []upgradeE2EReplacement{
		{
			description: "the reset URL takes the hosts registry",
			old:         "routes.PasswordEdit.FullURL(token)",
			new:         "routes.PasswordEdit.FullURL(i.hosts, token)",
		},
	}); err != nil {
		return err
	}

	return applyUpgradeE2EReplacements(filepath.Join(root, "config", "app.go"), []upgradeE2EReplacement{
		{
			description: "remove the package-level ConfigureHosts call",
			old: "\tif err := routing.ConfigureHosts(cfg.Hosts); err != nil {\n" +
				"\t\treturn App{}, fmt.Errorf(\"config: app: %w\", err)\n\t}\n\n\treturn cfg, nil",
			new: "\treturn cfg, nil",
		},
	})
}

// applyUpgradeE2ESessionRevocation follows the
// session-revocation-v2.0.0-alpha.5 instructions: migration, queries, domain,
// cookie and AuthOnly.
func applyUpgradeE2ESessionRevocation(root string) error {
	migration := `-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN session_version INTEGER NOT NULL DEFAULT 1;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN session_version;
-- +goose StatementEnd
`
	migrationPath := filepath.Join(
		root,
		"migrations",
		"20261026000000_add_session_version_to_users.sql",
	)
	if err := os.WriteFile(migrationPath, []byte(migration), 0o644); err != nil {
		return err
	}

	if err := applyUpgradeE2EReplacements(filepath.Join(root, "domains", "user.sql"), []upgradeE2EReplacement{
		{
			description: "add session_version to CreateUser",
			old:         "\tpassword,\n\tis_admin\n) VALUES (\n\t$1, $2, $3, $4, $5, $6, $7\n)",
			new:         "\tpassword,\n\tis_admin,\n\tsession_version\n) VALUES (\n\t$1, $2, $3, $4, $5, $6, $7, $8\n)",
		},
		{
			description: "add session_version to UpdateUser",
			old:         "\tis_admin = $5,\n\tupdated_at = $6\nWHERE id = $1\nRETURNING *;",
			new:         "\tis_admin = $5,\n\tsession_version = $6,\n\tupdated_at = $7\nWHERE id = $1\nRETURNING *;",
		},
		{
			description: "add the BumpSessionVersion query",
			old:         "-- name: DeleteUser :exec\nDELETE FROM users\nWHERE id = $1;\n",
			new: "-- name: DeleteUser :exec\nDELETE FROM users\nWHERE id = $1;\n\n" +
				"-- name: BumpSessionVersion :exec\nUPDATE users\nSET session_version = session_version + 1\nWHERE id = $1;\n",
		},
	}); err != nil {
		return err
	}

	if err := applyUpgradeE2EReplacements(
		filepath.Join(root, "domains", "internal", "queries", "models.go"),
		[]upgradeE2EReplacement{
			{
				description: "add SessionVersion to UserRow",
				old:         "\tIsAdmin          bool               `andurel:\"is_admin\"`\n}",
				new:         "\tIsAdmin          bool               `andurel:\"is_admin\"`\n\tSessionVersion   int32              `andurel:\"session_version\"`\n}",
			},
		},
	); err != nil {
		return err
	}

	columnsOld := `[]string{"id", "created_at", "updated_at", "email", "email_validated_at", "password", "is_admin"}`
	columnsNew := `[]string{"id", "created_at", "updated_at", "email", "email_validated_at", "password", "is_admin", "session_version"}`
	if err := applyUpgradeE2EReplacements(
		filepath.Join(root, "domains", "internal", "queries", "user.sql.go"),
		[]upgradeE2EReplacement{
			{
				description: "add the generated BumpSessionVersion query",
				old:         "const countUsers = `-- name: CountUsers :one",
				new: "const bumpSessionVersion = `-- name: BumpSessionVersion :exec\n" +
					"UPDATE users\nSET session_version = session_version + 1\nWHERE id = $1\n`\n\n" +
					"func (q *Queries) BumpSessionVersion(ctx context.Context, id uuid.UUID) error {\n" +
					"\t_, err := q.db.Exec(ctx, bumpSessionVersion, id)\n\treturn err\n}\n\n" +
					"const countUsers = `-- name: CountUsers :one",
			},
			{
				description: "add session_version to the generated CreateUser SQL",
				old: "\tpassword,\n\tis_admin\n) VALUES (\n\t$1, $2, $3, $4, $5, $6, $7\n)\n" +
					"RETURNING id, created_at, updated_at, email, email_validated_at, password, is_admin",
				new: "\tpassword,\n\tis_admin,\n\tsession_version\n) VALUES (\n\t$1, $2, $3, $4, $5, $6, $7, $8\n)\n" +
					"RETURNING id, created_at, updated_at, email, email_validated_at, password, is_admin, session_version",
			},
			{
				description: "add SessionVersion to CreateUserParams",
				old:         "\tPassword         []byte\n\tIsAdmin          bool\n}\n\nfunc (q *Queries) CreateUser",
				new:         "\tPassword         []byte\n\tIsAdmin          bool\n\tSessionVersion   int32\n}\n\nfunc (q *Queries) CreateUser",
			},
			{
				description: "pass SessionVersion to CreateUser",
				old:         "\t\targ.IsAdmin,\n\t)\n\treturn narsilc.Read[T, UserRow]",
				new:         "\t\targ.IsAdmin,\n\t\targ.SessionVersion,\n\t)\n\treturn narsilc.Read[T, UserRow]",
			},
			{
				description: "select session_version in generated reads",
				old:         ", is_admin\nFROM users",
				new:         ", is_admin, session_version\nFROM users",
				all:         true,
			},
			{
				description: "read session_version in generated read helpers",
				old:         columnsOld,
				new:         columnsNew,
				all:         true,
			},
			{
				description: "add session_version to the generated UpdateUser SQL",
				old: "\tis_admin = $5,\n\tupdated_at = $6\nWHERE id = $1\n" +
					"RETURNING id, created_at, updated_at, email, email_validated_at, password, is_admin",
				new: "\tis_admin = $5,\n\tsession_version = $6,\n\tupdated_at = $7\nWHERE id = $1\n" +
					"RETURNING id, created_at, updated_at, email, email_validated_at, password, is_admin, session_version",
			},
			{
				description: "add SessionVersion to UpdateUserParams",
				old:         "\tPassword         []byte\n\tIsAdmin          bool\n\tUpdatedAt        pgtype.Timestamptz",
				new:         "\tPassword         []byte\n\tIsAdmin          bool\n\tSessionVersion   int32\n\tUpdatedAt        pgtype.Timestamptz",
			},
			{
				description: "pass SessionVersion to UpdateUser",
				old:         "\t\targ.IsAdmin,\n\t\targ.UpdatedAt,",
				new:         "\t\targ.IsAdmin,\n\t\targ.SessionVersion,\n\t\targ.UpdatedAt,",
			},
		},
	); err != nil {
		return err
	}

	if err := applyUpgradeE2EReplacements(filepath.Join(root, "domains", "user.go"), []upgradeE2EReplacement{
		{
			description: "add SessionVersion to User",
			old:         "\tIsAdmin          bool               `andurel:\"is_admin\"`\n}",
			new:         "\tIsAdmin          bool               `andurel:\"is_admin\"`\n\tSessionVersion   int32              `andurel:\"session_version\"`\n}",
		},
		{
			description: "default new users to session version 1",
			old:         "\t\tIsAdmin:          false,\n\t}",
			new:         "\t\tIsAdmin:          false,\n\t\tSessionVersion:   1,\n\t}",
		},
		{
			description: "pass SessionVersion on create",
			old:         "\t\tIsAdmin:          entity.IsAdmin,\n\t})",
			new:         "\t\tIsAdmin:          entity.IsAdmin,\n\t\tSessionVersion:   entity.SessionVersion,\n\t})",
		},
		{
			description: "add SessionVersion to UpdateUserData",
			old:         "\tPassword         []byte\n\tIsAdmin          bool\n}",
			new:         "\tPassword         []byte\n\tIsAdmin          bool\n\tSessionVersion   int32\n}",
		},
		{
			description: "carry SessionVersion through Update",
			old:         "\t\tIsAdmin:          data.IsAdmin,\n\t}",
			new:         "\t\tIsAdmin:          data.IsAdmin,\n\t\tSessionVersion:   data.SessionVersion,\n\t}",
		},
		{
			description: "pass SessionVersion to UpdateUser",
			old:         "\t\tIsAdmin:          entity.IsAdmin,\n\t\tUpdatedAt:        entity.UpdatedAt,",
			new:         "\t\tIsAdmin:          entity.IsAdmin,\n\t\tSessionVersion:   entity.SessionVersion,\n\t\tUpdatedAt:        entity.UpdatedAt,",
		},
		{
			description: "bump the session version when the password changes",
			old: "	return u.Update(ctx, UpdateUserData{\n" +
				"\t\tID:               current.ID,\n\t\tEmail:            current.Email,\n" +
				"\t\tEmailValidatedAt: current.EmailValidatedAt,\n\t\tPassword:         []byte(hashedPassword),\n" +
				"\t\tIsAdmin:          current.IsAdmin,\n\t})\n}",
			new: "	updated, err := u.Update(ctx, UpdateUserData{\n" +
				"\t\tID:               current.ID,\n\t\tEmail:            current.Email,\n" +
				"\t\tEmailValidatedAt: current.EmailValidatedAt,\n\t\tPassword:         []byte(hashedPassword),\n" +
				"\t\tIsAdmin:          current.IsAdmin,\n\t\tSessionVersion:   current.SessionVersion,\n\t})\n" +
				"\tif err != nil {\n\t\treturn User{}, err\n\t}\n\n" +
				"\tif err := u.BumpSessionVersion(ctx, id); err != nil {\n\t\treturn User{}, err\n\t}\n" +
				"\tupdated.SessionVersion++\n\n\treturn updated, nil\n}\n\n" +
				"func (u Users) BumpSessionVersion(ctx context.Context, id uuid.UUID) error {\n" +
				"\treturn u.queries.BumpSessionVersion(ctx, id)\n}",
		},
	}); err != nil {
		return err
	}

	if err := applyUpgradeE2EReplacements(filepath.Join(root, "middleware", "cookies.go"), []upgradeE2EReplacement{
		{
			description: "replace IsAdmin with SessionVersion in the cookie",
			old:         "\tIsAdmin         bool   `json:\"is_admin,omitempty\"`",
			new:         "\tSessionVersion  int32  `json:\"session_version,omitempty\"`",
		},
	}); err != nil {
		return err
	}

	if err := applyUpgradeE2EReplacements(filepath.Join(root, "handlers", "auth.go"), []upgradeE2EReplacement{
		{
			description: "store the session version instead of IsAdmin",
			old:         "\t\tIsAdmin:         entity.IsAdmin,",
			new:         "\t\tSessionVersion:  entity.SessionVersion,",
			all:         true,
		},
	}); err != nil {
		return err
	}

	modulePath, err := upgradeE2EModulePath(root)
	if err != nil {
		return err
	}

	return applyUpgradeE2EReplacements(filepath.Join(root, "middleware", "auth.go"), []upgradeE2EReplacement{
		{
			description: "import errors and uuid for AuthOnly",
			old:         "\t\"context\"\n\t\"net/http\"\n\t\"strconv\"\n\t\"time\"\n",
			new:         "\t\"context\"\n\t\"errors\"\n\t\"net/http\"\n\t\"strconv\"\n\t\"time\"\n\t\"uuid\"\n",
		},
		{
			description: "import domains for AuthOnly",
			old:         "\t\"" + modulePath + "/routes\"",
			new:         "\t\"" + modulePath + "/domains\"\n\t\"" + modulePath + "/routes\"",
		},
		{
			description: "replace AuthOnly with the database-backed version",
			old: "func AuthOnly() anduin.Middleware {\n" +
				"\treturn func(next anduin.Handler) anduin.Handler {\n" +
				"\t\treturn func(ctx context.Context) error {\n" +
				"\t\t\tapp, err := kiks.Get[*App](ctx)\n" +
				"\t\t\tif err != nil {\n\t\t\t\treturn err\n\t\t\t}\n" +
				"\t\t\tif app != nil && app.IsAuthenticated {\n\t\t\t\treturn next(ctx)\n\t\t\t}\n\n" +
				"\t\t\treturn anduin.Redirect(ctx, routes.SessionNew.URL(), http.StatusSeeOther)\n" +
				"\t\t}\n\t}\n}",
			new: authOnlyE2EImplementation(),
		},
	})
}

func authOnlyE2EImplementation() string {
	return "func AuthOnly(users domains.Users) anduin.Middleware {\n" +
		"\treturn func(next anduin.Handler) anduin.Handler {\n" +
		"\t\treturn func(ctx context.Context) error {\n" +
		"\t\t\tapp, err := kiks.Get[*App](ctx)\n" +
		"\t\t\tif err != nil {\n\t\t\t\treturn err\n\t\t\t}\n" +
		"\t\t\tif app == nil || !app.IsAuthenticated {\n\t\t\t\treturn rejectSession(ctx)\n\t\t\t}\n\n" +
		"\t\t\tuserID, err := uuid.Parse(app.UserID)\n" +
		"\t\t\tif err != nil {\n\t\t\t\treturn rejectSession(ctx)\n\t\t\t}\n\n" +
		"\t\t\tuser, err := users.Find(ctx, userID)\n" +
		"\t\t\tif err != nil {\n" +
		"\t\t\t\tif errors.Is(err, domains.ErrNotFound) {\n\t\t\t\t\treturn rejectSession(ctx)\n\t\t\t\t}\n\n" +
		"\t\t\t\treturn err\n\t\t\t}\n\n" +
		"\t\t\tif user.SessionVersion != app.SessionVersion {\n\t\t\t\treturn rejectSession(ctx)\n\t\t\t}\n\n" +
		"\t\t\treturn next(WithViewer(ctx, user))\n\t\t}\n\t}\n}\n\n" +
		"func rejectSession(ctx context.Context) error {\n" +
		"\tif err := kiks.Destroy[*App](ctx); err != nil {\n\t\treturn err\n\t}\n\n" +
		"\treturn anduin.Redirect(ctx, routes.SessionNew.URL(), http.StatusSeeOther)\n}\n\n" +
		"type Viewer struct {\n\tID      uuid.UUID\n\tIsAdmin bool\n}\n\ntype viewerKey struct{}\n\n" +
		"func WithViewer(ctx context.Context, user domains.User) context.Context {\n" +
		"\treturn context.WithValue(ctx, viewerKey{}, Viewer{ID: user.ID, IsAdmin: user.IsAdmin})\n}\n\n" +
		"func ViewerFrom(ctx context.Context) (Viewer, bool) {\n" +
		"\tviewer, ok := ctx.Value(viewerKey{}).(Viewer)\n\n\treturn viewer, ok\n}"
}

// applyUpgradeE2EEmailSender follows the email-sender-v2.0.0-beta.2
// instructions against config/email.go.
func applyUpgradeE2EEmailSender(root string) error {
	path := filepath.Join(root, "config", "email.go")
	if err := applyUpgradeE2EReplacements(path, []upgradeE2EReplacement{
		{
			description: "import net and net/mail",
			old:         "import (\n\t\"fmt\"\n\t\"strings\"",
			new:         "import (\n\t\"fmt\"\n\t\"net\"\n\t\"net/mail\"\n\t\"strings\"",
		},
		{
			description: "derive the default sender without the port",
			old:         "\"noreply@\"+app.Domain",
			new:         "\"noreply@\"+senderHost(app.Domain)",
		},
		{
			description: "validate the sender address",
			old: "\tif strings.TrimSpace(cfg.DefaultSenderSignature) == \"\" {\n" +
				"\t\treturn Mail{}, fmt.Errorf(\"config: mail: DEFAULT_SENDER_SIGNATURE must not be empty\")\n\t}",
			new: "\tsender := strings.TrimSpace(cfg.DefaultSenderSignature)\n" +
				"\taddress, err := mail.ParseAddress(sender)\n" +
				"\tif err != nil || address.Name != \"\" || address.Address != sender {\n" +
				"\t\treturn Mail{}, fmt.Errorf(\"config: mail: DEFAULT_SENDER_SIGNATURE %q must be a bare email address\", sender)\n" +
				"\t}\n" +
				"\tcfg.DefaultSenderSignature = sender",
		},
	}); err != nil {
		return err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content = append(content, []byte("\n// senderHost strips the port from the app domain. HOST_PRIMARY carries a port\n"+
		"// and an email address cannot contain one.\n"+
		"func senderHost(domain string) string {\n"+
		"\thost, _, err := net.SplitHostPort(domain)\n"+
		"\tif err == nil && host != \"\" {\n\t\treturn host\n\t}\n\n"+
		"\treturn domain\n}\n")...)

	return writeGofmtFile(path, content)
}

// applyUpgradeE2EViteSSRBundle follows the vite-ssr-bundle-v2.0.0-beta.2
// instructions against vite.config.ts.
func applyUpgradeE2EViteSSRBundle(root string) error {
	return applyUpgradeE2EReplacements(filepath.Join(root, "vite.config.ts"), []upgradeE2EReplacement{
		{
			description: "bundle SSR dependencies for the production build",
			old:         "  base: command === 'build' ? './' : '/assets/dist/',\n",
			new: "  base: command === 'build' ? './' : '/assets/dist/',\n" +
				"  ssr: command === 'build' ? { noExternal: true } : undefined,\n",
		},
	})
}

// applyUpgradeE2ERootTitle follows the root-title-ssr-v2.0.0-beta.2
// instructions against resources/root.templ.
func applyUpgradeE2ERootTitle(root string) error {
	return applyUpgradeE2EReplacements(filepath.Join(root, "resources", "root.templ"), []upgradeE2EReplacement{
		{
			description: "render the fallback title only without SSR",
			old:         "\t\t\t<title>{ data.ProjectName }</title>\n",
			new: "\t\t\tif data.SSR == nil {\n" +
				"\t\t\t\t<title>{ data.ProjectName }</title>\n" +
				"\t\t\t}\n",
		},
	})
}

// applyUpgradeE2ECorsHostOrigins follows the
// cors-host-origins-v2.0.0-beta.3 instructions against internal/runtime and
// handlers/assets.go. The alpha.4 scaffold declares the CORS policy through a
// helper; alpha.5 and later build it inline in New.
func applyUpgradeE2ECorsHostOrigins(root string) error {
	runtimePath := filepath.Join(root, "internal", "runtime", "runtime.go")
	content, err := os.ReadFile(runtimePath)
	if err != nil {
		return err
	}

	switch {
	case strings.Contains(string(content), "func corsConfig(cfg config.Config) anduin.CORSConfig {"):
		if err := applyUpgradeE2EReplacements(runtimePath, []upgradeE2EReplacement{
			{
				description: "trust only explicit CORS origins",
				old: "\t\tAllowOrigins: mergedOrigins(\n" +
					"\t\t\tcfg.App.Origins(),\n" +
					"\t\t\tcfg.HTTP.CORSAllowedOrigins,\n" +
					"\t\t),",
				new: "\t\tAllowOrigins: cfg.HTTP.CORSAllowedOrigins,\n" +
					"\t\tSkipper:      isViteAssetRequest,",
			},
		}); err != nil {
			return err
		}
	case strings.Contains(string(content), "cors := anduin.DefaultCORS("):
		if err := applyUpgradeE2EReplacements(runtimePath, []upgradeE2EReplacement{
			{
				description: "trust only explicit CORS origins",
				old: "\tcors := anduin.DefaultCORS(\n" +
					"\t\tanduin.MergeOrigins(cfg.App.Origins(), cfg.HTTP.CORSAllowedOrigins)...,\n" +
					"\t)\n",
				new: "\tcors := anduin.DefaultCORS(cfg.HTTP.CORSAllowedOrigins...)\n" +
					"\tcors.Skipper = isViteAssetRequest\n",
			},
			{
				description: "import strings for the asset path helper",
				old:         "\t\"fmt\"\n",
				new:         "\t\"fmt\"\n\t\"strings\"\n",
			},
		}); err != nil {
			return err
		}
	default:
		return fmt.Errorf("cors-host-origins-v2.0.0-beta.3: no CORS policy in %s", runtimePath)
	}

	if err := applyUpgradeE2EViteAssetCORS(root); err != nil {
		return err
	}

	content, err = os.ReadFile(runtimePath)
	if err != nil {
		return err
	}
	content = append(content, []byte(corsAssetHelpers)...)

	return writeGofmtFile(runtimePath, content)
}

const corsAssetHelpers = `
// viteAssetPrefix is the Vite build route subtree without its wildcard. The
// static files are public: they load from a secondary host without the
// visitor's session, so neither credentialed CORS nor a CSRF cookie belongs on
// them.
var viteAssetPrefix = strings.TrimSuffix(routes.ViteBuild.Pattern(), "/*")

func isViteAssetRequest(ctx context.Context) bool {
	request := anduin.Request(ctx)

	return request != nil && isViteAssetPath(request.URL.Path)
}

func isViteAssetPath(path string) bool {
	return strings.HasPrefix(path, viteAssetPrefix+"/")
}
`

// applyUpgradeE2EViteAssetCORS registers the Vite build route with its own
// wildcard, uncredentialed CORS policy.
func applyUpgradeE2EViteAssetCORS(root string) error {
	path := filepath.Join(root, "handlers", "assets.go")
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(content)

	const assetsHandlerRegistration = "\t// Vite build files are public: a secondary-host page loads them without the\n" +
		"\t// visitor's session, so this CORS response is wildcard and uncredentialed.\n" +
		"\trt.Get(routes.ViteBuild.Pattern(), a.dist, anduin.CORS(anduin.CORSConfig{\n" +
		"\t\tAllowOrigins:     []string{\"*\"},\n" +
		"\t\tAllowMethods:     []string{http.MethodGet, http.MethodHead},\n" +
		"\t\tAllowCredentials: false,\n" +
		"\t}))"

	switch {
	case strings.Contains(text, "func (a Assets) Register(rt *anduin.Router) {"):
		legacyRegistration := strings.Replace(
			assetsHandlerRegistration,
			"routes.ViteBuild.Pattern(), a.dist",
			"routes.ViteBuild.Path(), a.ViteBuild",
			1,
		)
		return applyUpgradeE2EReplacements(path, []upgradeE2EReplacement{
			{
				description: "serve Vite assets with public CORS",
				old:         "\trt.Get(routes.ViteBuild.Path(), a.ViteBuild)",
				new:         legacyRegistration,
			},
		})
	case strings.Contains(text, "func (a AssetsHandler) Register(rt *anduin.Router) {"):
		registration := assetsHandlerRegistration
		old := "\trt.Get(routes.ViteBuild.Pattern(), a.dist)"
		if strings.Contains(text, "\trt.Get(routes.ViteBuild.Path(), a.dist)") {
			old = "\trt.Get(routes.ViteBuild.Path(), a.dist)"
			registration = strings.Replace(
				assetsHandlerRegistration,
				"routes.ViteBuild.Pattern()",
				"routes.ViteBuild.Path()",
				1,
			)
		}
		return applyUpgradeE2EReplacements(path, []upgradeE2EReplacement{
			{
				description: "import net/http for the CORS methods",
				old:         "\t\"fmt\"\n\t\"io/fs\"\n",
				new:         "\t\"fmt\"\n\t\"io/fs\"\n\t\"net/http\"\n",
			},
			{
				description: "serve Vite assets with public CORS",
				old:         old,
				new:         registration,
			},
		})
	default:
		return fmt.Errorf("cors-host-origins-v2.0.0-beta.3: no asset route in %s", path)
	}
}

// applyUpgradeE2ECSRFAssetSkip follows the
// csrf-asset-skip-v2.0.0-beta.3 instructions against the CSRF skipper in
// internal/runtime/runtime.go.
func applyUpgradeE2ECSRFAssetSkip(root string) error {
	runtimePath := filepath.Join(root, "internal", "runtime", "runtime.go")
	content, err := os.ReadFile(runtimePath)
	if err != nil {
		return err
	}
	text := string(content)

	switch {
	case strings.Contains(text, "func csrfSkipper(cfg config.Config) func(context.Context) bool {"):
		if err := applyUpgradeE2EReplacements(runtimePath, []upgradeE2EReplacement{
			{
				description: "keep the CSRF cookie off safe asset requests",
				old: "func csrfSkipper(cfg config.Config) func(context.Context) bool {\n" +
					"\treturn func(ctx context.Context) bool {\n" +
					"\t\trequest := anduin.Request(ctx)\n",
				new: "func csrfSkipper(cfg config.Config) func(context.Context) bool {\n" +
					"\treturn func(ctx context.Context) bool {\n" +
					"\t\tif isSafeViteAssetRequest(ctx) {\n" +
					"\t\t\treturn true\n" +
					"\t\t}\n\n" +
					"\t\trequest := anduin.Request(ctx)\n",
			},
		}); err != nil {
			return err
		}
	case strings.Contains(text, "csrf.Skipper = anduin.SkipCSRFForBearer("):
		if err := applyUpgradeE2EReplacements(runtimePath, []upgradeE2EReplacement{
			{
				description: "keep the CSRF cookie off safe asset requests",
				old:         "\tcsrf.Skipper = anduin.SkipCSRFForBearer(routes.APIPrefix, cfg.Session.Name)\n",
				new: "\tbearerSkipper := anduin.SkipCSRFForBearer(routes.APIPrefix, cfg.Session.Name)\n" +
					"\tcsrf.Skipper = func(ctx context.Context) bool {\n" +
					"\t\treturn bearerSkipper(ctx) || isSafeViteAssetRequest(ctx)\n" +
					"\t}\n",
			},
		}); err != nil {
			return err
		}
	default:
		return fmt.Errorf("csrf-asset-skip-v2.0.0-beta.3: no CSRF skipper in %s", runtimePath)
	}

	content, err = os.ReadFile(runtimePath)
	if err != nil {
		return err
	}
	content = append(content, []byte(csrfAssetHelper)...)

	return writeGofmtFile(runtimePath, content)
}

const csrfAssetHelper = `
// isSafeViteAssetRequest limits the CSRF skip to safe methods. An unsafe request
// to a static path still carries an Origin and stays protected.
func isSafeViteAssetRequest(ctx context.Context) bool {
	request := anduin.Request(ctx)
	if request == nil || !isViteAssetPath(request.URL.Path) {
		return false
	}

	switch request.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}
`

// applyUpgradeE2EMethodNotAllowedPage follows the
// method-not-allowed-page-v2.0.0-beta.3 instructions against handlers/errors.go.
// The alpha.4 scaffold keeps a hand-written MapError switch; alpha.5 and later
// use the shared inertia error handler.
func applyUpgradeE2EMethodNotAllowedPage(root string) error {
	path := filepath.Join(root, "handlers", "errors.go")
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(content)

	switch {
	case strings.Contains(text, "\t\tcase errors.Is(err, domains.ErrNotFound):"):
		return applyUpgradeE2EReplacements(path, []upgradeE2EReplacement{
			{
				description: "render the themed not-found page for method mismatches",
				old:         "\t\tcase errors.Is(err, domains.ErrNotFound):\n",
				new: "\t\tcase errors.Is(err, anduin.ErrMethodNotAllowed):\n" +
					"\t\t\treturn renderer.Page(ctx, \"Errors/NotFound\", inertia.Props{}).\n" +
					"\t\t\t\tStatus(http.StatusMethodNotAllowed).\n" +
					"\t\t\t\tRender()\n\n" +
					"\t\tcase errors.Is(err, domains.ErrNotFound):\n",
			},
		})
	case strings.Contains(text, "inertia.MapError(domains.ErrNotFound, http.StatusNotFound, \"Errors/NotFound\"),"):
		return applyUpgradeE2EReplacements(path, []upgradeE2EReplacement{
			{
				description: "render the themed not-found page for method mismatches",
				old:         "\t\tinertia.MapError(domains.ErrNotFound, http.StatusNotFound, \"Errors/NotFound\"),\n",
				new: "\t\tinertia.MapError(anduin.ErrMethodNotAllowed, http.StatusMethodNotAllowed, \"Errors/NotFound\"),\n" +
					"\t\tinertia.MapError(domains.ErrNotFound, http.StatusNotFound, \"Errors/NotFound\"),\n",
			},
		})
	default:
		return fmt.Errorf("method-not-allowed-page-v2.0.0-beta.3: no MapError mapping in %s", path)
	}
}

func applyUpgradeE2EEmailSMTP(root string) error {
	if err := applyUpgradeE2EReplacements(filepath.Join(root, "config", "email.go"), []upgradeE2EReplacement{
		{
			description: "add the SMTP driver constants",
			old:         "\tMailpitDriver MailDriver = \"mailpit\"\n\n\tDefaultMailpitHost = \"0.0.0.0\"\n\tDefaultMailpitPort = \"1025\"",
			new: "\tMailpitDriver MailDriver = \"mailpit\"\n\tSMTPDriver    MailDriver = \"smtp\"\n\n" +
				"\tDefaultMailpitHost = \"0.0.0.0\"\n\tDefaultMailpitPort = \"1025\"\n" +
				"\tDefaultSMTPPort    = \"587\"\n\tDefaultSMTPTLS     = \"starttls\"",
		},
		{
			description: "add the SMTP fields to Mail",
			old:         "\tHost                   string\n\tPort                   string\n}",
			new: "\tHost                   string\n\tPort                   string\n" +
				"\tSMTPHost               string\n\tSMTPPort               string\n" +
				"\tSMTPUsername           string\n\tSMTPPassword           string\n\tSMTPTLS                string\n}",
		},
		{
			description: "read the SMTP environment",
			old:         "\t\tPort:                   env.String(\"MAILPIT_PORT\", DefaultMailpitPort),\n\t}",
			new: "\t\tPort:                   env.String(\"MAILPIT_PORT\", DefaultMailpitPort),\n" +
				"\t\tSMTPHost:               env.String(\"SMTP_HOST\", \"\"),\n" +
				"\t\tSMTPPort:               env.String(\"SMTP_PORT\", DefaultSMTPPort),\n" +
				"\t\tSMTPUsername:           env.String(\"SMTP_USERNAME\", \"\"),\n" +
				"\t\tSMTPPassword:           env.String(\"SMTP_PASSWORD\", \"\"),\n" +
				"\t\tSMTPTLS:                env.String(\"SMTP_TLS\", DefaultSMTPTLS),\n\t}",
		},
	}); err != nil {
		return err
	}

	oldSenders := "func newEmailSenders(\n\tcfg config.Mail,\n" +
		") (email.TransactionalSender, email.MarketingSender, error) {\n" +
		"\tswitch cfg.Driver {\n\tcase config.MailpitDriver:\n" +
		"\t\tclient, err := email.NewMailpit(email.MailpitConfig{\n" +
		"\t\t\tHost: cfg.Host,\n\t\t\tPort: cfg.Port,\n\t\t})\n" +
		"\t\tif err != nil {\n\t\t\treturn nil, nil, fmt.Errorf(\"create Mailpit client: %w\", err)\n\t\t}\n\n" +
		"\t\treturn client, client, nil\n\tdefault:\n" +
		"\t\treturn nil, nil, fmt.Errorf(\"unsupported email provider %q\", cfg.Driver)\n\t}\n}"
	newSenders := "func newEmailSenders(\n\tcfg config.Mail,\n\tapp config.App,\n" +
		") (email.TransactionalSender, email.MarketingSender, error) {\n" +
		"\tif err := validateEmailConfig(cfg, app); err != nil {\n\t\treturn nil, nil, err\n\t}\n\n" +
		"\tswitch cfg.Driver {\n\tcase config.MailpitDriver:\n" +
		"\t\tclient, err := email.NewMailpit(email.MailpitConfig{\n" +
		"\t\t\tHost: cfg.Host,\n\t\t\tPort: cfg.Port,\n\t\t})\n" +
		"\t\tif err != nil {\n\t\t\treturn nil, nil, fmt.Errorf(\"create Mailpit client: %w\", err)\n\t\t}\n\n" +
		"\t\treturn client, client, nil\n\tcase config.SMTPDriver:\n" +
		"\t\tclient, err := email.NewSMTP(email.SMTPConfig{\n" +
		"\t\t\tHost:     cfg.SMTPHost,\n\t\t\tPort:     cfg.SMTPPort,\n" +
		"\t\t\tUsername: cfg.SMTPUsername,\n\t\t\tPassword: cfg.SMTPPassword,\n" +
		"\t\t\tTLS:      email.SMTPTLSMode(cfg.SMTPTLS),\n\t\t})\n" +
		"\t\tif err != nil {\n\t\t\treturn nil, nil, fmt.Errorf(\"create SMTP sender: %w\", err)\n\t\t}\n\n" +
		"\t\treturn client, client, nil\n\tdefault:\n" +
		"\t\treturn nil, nil, fmt.Errorf(\"unsupported email provider %q\", cfg.Driver)\n\t}\n}\n\n" +
		"func validateEmailConfig(cfg config.Mail, app config.App) error {\n\tswitch cfg.Driver {\n" +
		"\tcase config.MailpitDriver:\n\t\tif app.IsProduction() {\n" +
		"\t\t\treturn fmt.Errorf(\n\t\t\t\t\"email provider %q is for local development; set EMAIL_PROVIDER=%s in production\",\n" +
		"\t\t\t\tconfig.MailpitDriver,\n\t\t\t\tconfig.SMTPDriver,\n\t\t\t)\n\t\t}\n" +
		"\tcase config.SMTPDriver:\n\tdefault:\n" +
		"\t\treturn fmt.Errorf(\"unsupported email provider %q\", cfg.Driver)\n\t}\n\n\treturn nil\n}"

	runtimePath := filepath.Join(root, "internal", "runtime", "runtime.go")
	if err := applyUpgradeE2EReplacements(runtimePath, []upgradeE2EReplacement{
		{
			description: "newEmailSenders takes config.App and supports SMTP",
			old:         oldSenders,
			new:         newSenders,
		},
	}); err != nil {
		return err
	}

	return applyUpgradeE2EReplacements(filepath.Join(root, "internal", "runtime", "queue.go"), []upgradeE2EReplacement{
		{
			description: "pass config.App to newEmailSenders",
			old:         "newEmailSenders(cfg.Mail)",
			new:         "newEmailSenders(cfg.Mail, cfg.App)",
		},
	})
}

func applyUpgradeE2EValidation(root string) error {
	for _, name := range []string{"user.go", "token.go"} {
		path := filepath.Join(root, "domains", name)
		if err := applyUpgradeE2EReplacements(path, []upgradeE2EReplacement{
			{
				description: "call the domain Validate method",
				old:         "if err := validation.Validate(&entity); err != nil {",
				new:         "if err := entity.Validate(); err != nil {",
			},
		}); err != nil {
			return err
		}
	}

	return nil
}

func applyUpgradeE2EInertiaFullURL(root string) error {
	path := filepath.Join(root, "internal", "runtime", "runtime.go")
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated := strings.ReplaceAll(
		string(content),
		"routes.ViteBuild.Path()",
		"routes.ViteBuild.FullURL()",
	)
	if updated == string(content) {
		return nil
	}
	return writeGofmtFile(path, []byte(updated))
}

func TestApplyUpgradeE2EPublicHome(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name: "protected generated home becomes public",
			body: "package runtime\n\nfunc New() {\n" +
				"\thandlers.NewPagesHandler(renderer).Register(router, middleware.AuthOnly(users))\n}\n",
		},
		{
			name: "already public generated home",
			body: "package runtime\n\nfunc New() {\n" +
				"\thandlers.NewPagesHandler(renderer).Register(router)\n}\n",
		},
		{
			name: "alpha.4 pages handler is already public",
			body: "package runtime\n\nfunc New() {\n" +
				"\thandlers.NewPages(renderer).Register(router)\n}\n",
		},
		{
			name:    "unrecognized home registration fails",
			body:    "package runtime\n\nfunc New() {}\n",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			path := writeUpgradeE2EFixture(
				t,
				root,
				filepath.Join("internal", "runtime", "runtime.go"),
				test.body,
			)

			err := applyUpgradeE2EPublicHome(root)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected an error for an unrecognized home registration")
				}
				return
			}
			if err != nil {
				t.Fatalf("apply public home: %v", err)
			}

			updated, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(updated), "Register(router, middleware.AuthOnly(users))") {
				t.Fatalf("home page is still protected:\n%s", updated)
			}
			if !strings.Contains(string(updated), "handlers.NewPages") {
				t.Fatalf("home page registration disappeared:\n%s", updated)
			}
		})
	}
}

func TestApplyUpgradeE2EConfigCheck(t *testing.T) {
	root := t.TempDir()
	writeUpgradeE2EFixture(t, root, "go.mod", "module example.com/app\n\ngo 1.27.3\n")

	if err := applyUpgradeE2EConfigCheck(root); err != nil {
		t.Fatalf("apply configcheck: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(root, "internal", "configcheck", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, "example.com/app") {
		t.Fatalf("configcheck does not use the project module path:\n%s", text)
	}
	if strings.Contains(text, "{{.") {
		t.Fatalf("configcheck has an unresolved template placeholder:\n%s", text)
	}
}

func TestApplyUpgradeE2ERuntimeReadiness(t *testing.T) {
	root := t.TempDir()
	writeUpgradeE2EFixture(t, root, "go.mod", "module example.com/app\n\ngo 1.27.3\n")
	writeUpgradeE2EFixture(
		t,
		root,
		filepath.Join("internal", "runtime", "runtime.go"),
		"package runtime\n\nimport (\n\t\"context\"\n\t\"errors\"\n)\n\n"+
			"func New() {\n\thandlers.NewAPIHandler(db).Register(router)\n}\n\n"+
			"func (a *App) Start() error { return nil }\n",
	)
	writeUpgradeE2EFixture(
		t,
		root,
		filepath.Join("handlers", "api.go"),
		"package handlers\n\nfunc NewAPIHandler(health interface{}) {}\n",
	)
	writeUpgradeE2EFixture(
		t,
		root,
		filepath.Join("routes", "api.go"),
		"package routes\n\nvar Health = \"health\"\n",
	)

	if err := applyUpgradeE2ERuntimeReadiness(root); err != nil {
		t.Fatalf("apply runtime readiness: %v", err)
	}

	runtimeContent, err := os.ReadFile(filepath.Join(root, "internal", "runtime", "runtime.go"))
	if err != nil {
		t.Fatal(err)
	}
	runtimeText := string(runtimeContent)
	for _, want := range []string{
		"handlers.NewAPIHandler(db, newReadiness(db, renderer)).Register(router)",
		"type readiness struct {",
		"ReadinessReason() string",
	} {
		if !strings.Contains(runtimeText, want) {
			t.Fatalf("runtime readiness is missing %q:\n%s", want, runtimeText)
		}
	}

	routesContent, err := os.ReadFile(filepath.Join(root, "routes", "api.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(routesContent), "\"/ready\"") {
		t.Fatalf("routes/api.go is missing /ready:\n%s", routesContent)
	}

	handlerContent, err := os.ReadFile(filepath.Join(root, "handlers", "api.go"))
	if err != nil {
		t.Fatal(err)
	}
	handlerText := string(handlerContent)
	for _, want := range []string{"readinessHandler", `"status": "unready"`, "routes.Ready.Pattern()"} {
		if !strings.Contains(handlerText, want) {
			t.Fatalf("API handler is missing %q:\n%s", want, handlerText)
		}
	}
}

func TestApplyUpgradeE2ERuntimeReadinessRequiresRegistration(t *testing.T) {
	root := t.TempDir()
	writeUpgradeE2EFixture(
		t,
		root,
		filepath.Join("internal", "runtime", "runtime.go"),
		"package runtime\n\nfunc (a *App) Start() error { return nil }\n",
	)

	if err := applyUpgradeE2ERuntimeReadiness(root); err == nil {
		t.Fatal("expected an error when the runtime has no NewAPIHandler registration")
	}
}

func TestApplyUpgradeE2EInertiaSSRDefault(t *testing.T) {
	root := t.TempDir()
	writeUpgradeE2EFixture(t, root, "go.mod", "module example.com/app\n\ngo 1.27.3\n")
	writeUpgradeE2EFixture(
		t,
		root,
		filepath.Join("internal", "runtime", "runtime.go"),
		"package runtime\n\nfunc NewRenderer() {\n\t_ = inertia.NewRenderer(\n"+
			"\t\tinertia.RendererConfig{},\n"+
			"\t\tinertia.WithEnvironment(cfg.App.Environment),\n\t)\n}\n",
	)
	writeUpgradeE2EFixture(
		t,
		root,
		filepath.Join("handlers", "errors.go"),
		"package handlers\n\nfunc MapError(renderer *inertia.Renderer) {\n"+
			"\t_ = inertia.ErrorHandler(renderer,\n"+
			"\t\tinertia.MapError(nil, 404, \"Errors/NotFound\"),\n\t)\n}\n",
	)
	writeUpgradeE2EFixture(
		t,
		root,
		".env.example",
		"INERTIA_SSR_FAIL_FAST=false\n",
	)

	if err := applyUpgradeE2EInertiaSSRDefault(root); err != nil {
		t.Fatalf("apply SSR default: %v", err)
	}

	runtimeContent, err := os.ReadFile(filepath.Join(root, "internal", "runtime", "runtime.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(
		string(runtimeContent),
		"inertia.WithSSRDefault(cfg.App.IsProduction()),",
	) {
		t.Fatalf("renderer default was not added:\n%s", runtimeContent)
	}

	errorsContent, err := os.ReadFile(filepath.Join(root, "handlers", "errors.go"))
	if err != nil {
		t.Fatal(err)
	}
	errorsText := string(errorsContent)
	for _, want := range []string{
		"inertia.NewErrorHandler(",
		`FallbackComponent: "Errors/InternalError"`,
	} {
		if !strings.Contains(errorsText, want) {
			t.Fatalf("error handler is missing %q:\n%s", want, errorsText)
		}
	}

	envContent, err := os.ReadFile(filepath.Join(root, ".env.example"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(envContent), "\nINERTIA_SSR_FAIL_FAST=false\n") ||
		strings.HasPrefix(string(envContent), "INERTIA_SSR_FAIL_FAST=false") {
		t.Fatalf(".env.example still forces client-rendering fallback:\n%s", envContent)
	}
}

func TestApplyUpgradeE2EInertiaSSRDefaultRequiresRendererOptions(t *testing.T) {
	root := t.TempDir()
	writeUpgradeE2EFixture(
		t,
		root,
		filepath.Join("internal", "runtime", "runtime.go"),
		"package runtime\n\nfunc NewRenderer() {}\n",
	)

	if err := applyUpgradeE2EInertiaSSRDefault(root); err == nil {
		t.Fatal("expected an error when the renderer has no environment option")
	}
}

func TestApplyUpgradeE2EAlphaFourRouteNames(t *testing.T) {
	root := t.TempDir()
	path := writeUpgradeE2EFixture(
		t,
		root,
		filepath.Join("routes", "users.go"),
		"package routes\n\nvar PasswordCreate = \"users.user_password\"\n"+
			"var PasswordUpdate = \"users.user_password\"\n",
	)

	if err := applyUpgradeE2EAlphaFourRouteNames(root); err != nil {
		t.Fatalf("repair alpha.4 route names: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if strings.Count(text, `"users.user_password"`) != 1 ||
		!strings.Contains(text, `"users.update_user_password"`) {
		t.Fatalf("duplicate route name was not repaired:\n%s", text)
	}
}

func TestVerifyUpgradeE2ERoutesJSRejectsOldShape(t *testing.T) {
	root := t.TempDir()
	writeUpgradeE2EFixture(
		t,
		root,
		filepath.Join("resources", "js", "routes.ts"),
		"type HostOrigins = Record<string, string>\n\nlet origins: HostOrigins = {}\n",
	)

	if err := verifyUpgradeE2ERoutesJS(root); err == nil {
		t.Fatal("expected the old silent-origin routes.ts to be rejected")
	}

	writeUpgradeE2EFixture(
		t,
		root,
		filepath.Join("resources", "js", "routes.ts"),
		"export function configureRouteHostScope() {}\n"+
			"export function withRouteHosts() {}\n"+
			"throw new Error('routes: route hosts are not configured yet')\n",
	)
	if err := verifyUpgradeE2ERoutesJS(root); err != nil {
		t.Fatalf("expected the regenerated routes.ts to pass: %v", err)
	}
}

func TestRejectUpgradeE2EModuleScopeRouteCache(t *testing.T) {
	if err := rejectUpgradeE2EModuleScopeRouteCache(
		"app.tsx",
		"const href = routes.homePage()\n",
	); err == nil {
		t.Fatal("expected a module-scope route helper cache to be rejected")
	}

	if err := rejectUpgradeE2EModuleScopeRouteCache(
		"app.tsx",
		"function render() {\n\tconst href = routes.homePage()\n\treturn href\n}\n",
	); err != nil {
		t.Fatalf("render-time route helper call should pass: %v", err)
	}
}

func writeUpgradeE2EFixture(t *testing.T, root, relativePath, content string) string {
	t.Helper()

	path := filepath.Join(root, relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	return path
}

func writeGofmtFile(path string, content []byte) error {
	formatted, err := format.Source(content)
	if err != nil {
		return fmt.Errorf("format %s: %w", path, err)
	}
	return os.WriteFile(path, formatted, 0o644)
}

func runUpgradeE2EGo(t *testing.T, dir string, args ...string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	command := exec.CommandContext(ctx, "go", args...)
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}
