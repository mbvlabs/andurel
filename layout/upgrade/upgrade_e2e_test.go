package upgrade

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mbvlabs/andurel/v2/layout"
	"golang.org/x/mod/modfile"
)

const (
	// upgradeE2ETargetVersion is the historical released target for the
	// beta.4 regression case, not the release candidate.
	upgradeE2ETargetVersion = "v2.0.0-beta.5"

	// The candidate target is handled by this checkout, not a released CLI.
	upgradeE2ECandidateVersion = "v2.0.0-rc.2"
)

// upgradeE2ECases covers the existing released beta.4 -> beta.5 regression and
// the supported beta.5/beta.7/rc.1 -> HEAD candidate paths.
var upgradeE2ECases = []struct {
	name            string
	source          string
	target          string
	verifyCandidate bool
	legacy          bool
}{
	{
		name:   "v2.0.0-beta.4-to-v2.0.0-beta.5",
		source: "v2.0.0-beta.4",
		target: upgradeE2ETargetVersion,
		legacy: true,
	},
	{
		name:            "v2.0.0-beta.5-to-v2.0.0-rc.2",
		source:          "v2.0.0-beta.5",
		target:          upgradeE2ECandidateVersion,
		verifyCandidate: true,
		legacy:          true,
	},
	{
		name:            "v2.0.0-beta.7-to-v2.0.0-rc.2",
		source:          "v2.0.0-beta.7",
		target:          upgradeE2ECandidateVersion,
		verifyCandidate: true,
		legacy:          true,
	},
	{
		name:            "v2.0.0-rc.1-to-v2.0.0-rc.2",
		source:          "v2.0.0-rc.1",
		target:          upgradeE2ECandidateVersion,
		verifyCandidate: true,
	},
}

// TestUpgradeToHEAD scaffolds with each released CLI in upgradeE2ECases,
// upgrades with this checkout, and builds and vets the result. Legacy beta
// cases apply the documented rc.1 root and email migrations; the rc.1 case
// retains its existing wiring and application-owned Vite configuration.
// Candidate cases also check manifest metadata and current package pins.
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

	for _, tc := range upgradeE2ECases {
		t.Run(tc.name, func(t *testing.T) {
			testUpgradeToTarget(t, tc.source, tc.target, localPkg, tc.verifyCandidate, tc.legacy)
		})
	}
}

func testUpgradeToTarget(
	t *testing.T,
	source string,
	target string,
	localPkg bool,
	verifyCandidate bool,
	legacy bool,
) {
	t.Helper()

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
	unchanged := map[string]string{}
	if !legacy {
		for _, rel := range []string{"resources/root.html", "resources/root.go", "emails/render.go", "vite.config.ts"} {
			unchanged[rel] = string(mustReadProjectFile(t, root, rel))
		}
		if !strings.Contains(unchanged["resources/root.go"], "html/template") ||
			!strings.Contains(unchanged["emails/render.go"], postmandImportPath) {
			t.Fatal("rc.1 scaffold does not have the expected root and email wiring")
		}
	}
	gitRun(t, root, "init")
	gitRun(t, root, "config", "user.email", "upgrade-e2e@example.com")
	gitRun(t, root, "config", "user.name", "Upgrade E2E")
	commitUpgradeTree(t, root, "scaffold from "+source)

	upgrader, err := NewUpgrader(
		root,
		UpgradeOptions{TargetVersion: target},
	)
	if err != nil {
		t.Fatal(err)
	}
	report, err := upgrader.Execute()
	if err != nil {
		t.Fatalf("upgrade %s to %s: %v", source, target, err)
	}
	if report.FromVersion != source {
		t.Fatalf("upgrade reported from version %q, want %q", report.FromVersion, source)
	}
	if report.ToVersion != target {
		t.Fatalf("upgrade reported target version %q, want %q", report.ToVersion, target)
	}
	// rc.1 makes two deliberate application-owned breaking changes. The
	// upgrade pins this CLI's pkg/inertia and pkg/storage even when the target
	// is an earlier pre-release, so every legacy beta shape now needs both
	// migrations. The upgrade reports them and never rewrites the root document
	// or email code, so this test applies the documented migrations itself.
	var wantActions []string
	if legacy {
		wantActions = []string{templRootActionID, postmandMigrationActionID}
	}
	if got := actionIDs(report.ManualActions); !slices.Equal(got, wantActions) {
		t.Fatalf("upgrade %s to %s manual actions = %#v, want %#v", source, target, got, wantActions)
	}

	for rel, before := range unchanged {
		if after := string(mustReadProjectFile(t, root, rel)); after != before {
			t.Errorf("upgrade rewrote application-owned %s", rel)
		}
	}
	if !legacy && !strings.Contains(string(mustReadProjectFile(t, root, "app/runtime.go")), "inertia.WithRoot(resources.Root)") {
		t.Error("upgrade lost the existing root wiring")
	}
	if legacy {
		assertLegacyTemplRootAwaitingMigration(t, root)
		// rc.1 deliberately breaks compiled templ roots. Upgrades never rewrite
		// this application-owned document, so every upgraded beta project applies
		// the documented manual migration before building. The current package
		// pins are what make the old compiled root fail, so this holds for the
		// historical beta target too.
		migrateLegacyTemplRoot(t, root)
		// rc.1 removes pkg/email. Upgrade deliberately does not rewrite
		// application-owned Go code, so the test applies the import rewrite the
		// release notes prescribe before tidy and build.
		runUpgradeE2EGo(t, root, "get", "github.com/mbvlabs/andurel/pkg/postmand@v0.6.2")
		runUpgradeE2EGo(t, root, "get", "github.com/mbvlabs/andurel/pkg/storage@v0.16.1")
		migrateLegacyEmailImports(t, root)
	}
	if localPkg {
		replaceWithLocalPackages(t, root)
	}

	runUpgradeE2EGo(t, root, "mod", "tidy")
	if verifyCandidate {
		assertCandidateProject(t, root, target, localPkg)
	}
	runUpgradeE2EGo(t, root, "build", "./...")
	runUpgradeE2EGo(t, root, "vet", "./...")
}

// assertLegacyTemplRootAwaitingMigration proves the pre-rc.1 upgrade leaves the
// app-owned compiled templ root untouched. The upgrade must not pretend the
// removed templ helpers still exist.
func assertLegacyTemplRootAwaitingMigration(t *testing.T, root string) {
	t.Helper()

	for _, rel := range []string{"resources/root.templ", "resources/root_templ.go"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("upgrade rewrote app-owned root %s: %v", rel, err)
		}
	}
	compiled := string(mustReadProjectFile(t, root, "resources/root_templ.go"))
	if !strings.Contains(compiled, "github.com/a-h/templ") ||
		!strings.Contains(compiled, "inertia.SSRHead") {
		t.Fatalf("expected a legacy compiled templ root, got:\n%s", compiled)
	}
}

// migrateLegacyTemplRoot applies the documented rc.1 root migration: replace
// the templ document with resources/root.html + resources/root.go. The runtime
// call site is unchanged because resources.Root now satisfies WithRoot.
func migrateLegacyTemplRoot(t *testing.T, root string) {
	t.Helper()

	templates := filepath.Join(localCheckoutRoot(t), "layout", "templates")
	writeUpgradeProjectFile(
		t,
		root,
		"resources/root.html",
		mustReadTestFile(t, filepath.Join(templates, "resources_root_html.tmpl")),
	)
	writeUpgradeProjectFile(
		t,
		root,
		"resources/root.go",
		mustReadTestFile(t, filepath.Join(templates, "resources_root_go.tmpl")),
	)
	for _, rel := range []string{"resources/root.templ", "resources/root_templ.go"} {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(rel))); err != nil &&
			!os.IsNotExist(err) {
			t.Fatalf("remove legacy root %s: %v", rel, err)
		}
	}
	dropTool := exec.Command(
		"go",
		"mod",
		"edit",
		"-droptool=github.com/a-h/templ/cmd/templ",
	)
	dropTool.Dir = root
	if err := dropTool.Run(); err != nil {
		// The upgrade already drops the directive; a missing tool is not an error.
		t.Logf("droptool after upgrade: %v", err)
	}
}

func writeUpgradeProjectFile(t *testing.T, root, rel, content string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func mustReadTestFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

// assertCandidateProject independently checks rc.2's required pins after tidy.
func assertCandidateProject(t *testing.T, root string, target string, localPkg bool) {
	t.Helper()

	lock, err := layout.ReadLockFile(root)
	if err != nil {
		t.Fatalf("read upgraded andurel manifest: %v", err)
	}
	if lock.Version != target {
		t.Fatalf("andurel manifest framework version = %q, want %q", lock.Version, target)
	}

	content, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("read upgraded go.mod: %v", err)
	}
	file, err := modfile.Parse("go.mod", content, nil)
	if err != nil {
		t.Fatalf("parse upgraded go.mod: %v", err)
	}

	required := make(map[string]string, len(file.Require))
	for _, requirement := range file.Require {
		if requirement != nil {
			required[requirement.Mod.Path] = requirement.Mod.Version
		}
	}
	for _, pin := range []struct {
		path    string
		version string
	}{
		{
			path:    "github.com/mbvlabs/andurel/pkg/anduin",
			version: "v0.8.1",
		},
		{
			path:    "github.com/mbvlabs/andurel/pkg/inertia",
			version: "v0.15.1",
		},
		{
			path:    "github.com/mbvlabs/andurel/pkg/postmand",
			version: "v0.6.2",
		},
	} {
		if got := required[pin.path]; got != pin.version {
			t.Errorf("go.mod pin %s = %q, want %q", pin.path, got, pin.version)
		}
	}
	if !localPkg && len(file.Replace) != 0 {
		t.Fatalf("go.mod has local replace directives after candidate upgrade: %#v", file.Replace)
	}
}

const (
	legacyEmailImportPath = "github.com/mbvlabs/andurel/pkg/email"
	postmandImportPath    = "github.com/mbvlabs/andurel/pkg/postmand"
)

// migrateLegacyEmailImports applies the documented rc.1 breaking migration to
// application-owned Go files. The upgrade does not rewrite application code;
// this helper mirrors the release-note import rewrite so the candidate E2E
// asserts the post-migration app rather than pretending the removed pkg/email
// types remain compatible with pkg/storage v0.16.0.
func migrateLegacyEmailImports(t *testing.T, root string) {
	t.Helper()

	migrated := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "vendor":
				return filepath.SkipDir
			default:
				return nil
			}
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if !bytes.Contains(content, []byte(legacyEmailImportPath)) {
			return nil
		}

		updated, changed, migrateErr := migrateLegacyEmailFile(path, content)
		if migrateErr != nil {
			return fmt.Errorf("migrate %s: %w", path, migrateErr)
		}
		if !changed {
			return nil
		}
		if writeErr := os.WriteFile(path, updated, 0o644); writeErr != nil {
			return writeErr
		}
		migrated++

		return nil
	})
	if err != nil {
		t.Fatalf("migrate legacy pkg/email imports: %v", err)
	}
	t.Logf("applied documented pkg/email -> pkg/postmand import migration to %d files", migrated)
}

// migrateLegacyEmailFile rewrites an import of pkg/email to pkg/postmand and,
// when the old import was unaliased, the `email.` selectors bound to it. An
// explicitly aliased import keeps its local name, so its selectors do not
// change. Selectors whose identifier resolves to a local declaration are left
// alone even if they are named email.
func migrateLegacyEmailFile(path string, content []byte) ([]byte, bool, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return nil, false, err
	}

	unaliased := false
	found := false
	for _, importSpec := range file.Imports {
		importPath, unquoteErr := strconv.Unquote(importSpec.Path.Value)
		if unquoteErr != nil {
			return nil, false, unquoteErr
		}
		if importPath != legacyEmailImportPath {
			continue
		}

		found = true
		if importSpec.Name == nil {
			unaliased = true
		}
		importSpec.Path.Value = strconv.Quote(postmandImportPath)
	}
	if !found {
		return content, false, nil
	}

	if unaliased {
		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := selector.X.(*ast.Ident)
			if !ok || ident.Name != "email" || ident.Obj != nil {
				return true
			}
			ident.Name = "postmand"

			return true
		})
	}

	var formatted bytes.Buffer
	if err := format.Node(&formatted, fset, file); err != nil {
		return nil, false, err
	}

	return formatted.Bytes(), true, nil
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
