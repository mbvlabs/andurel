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

	// upgradeE2ECandidateVersion is supplied directly to this checkout's
	// upgrader. There is deliberately no v2.0.0-rc.1 source CLI below because
	// that CLI is not published by this test; the candidate is HEAD code.
	upgradeE2ECandidateVersion = "v2.0.0-rc.1"

	// The rc.1 candidate publishes these framework package pins.
	upgradeE2ECandidateAnduinVersion   = "v0.8.0"
	upgradeE2ECandidateInertiaVersion  = "v0.14.1"
	upgradeE2ECandidatePostmandVersion = "v0.1.0"
)

// upgradeE2ECases covers the existing released beta.4 -> beta.5 regression and
// the supported public beta.5/beta.7 -> v2.0.0-rc.1 candidate paths. Upgrade
// notes intentionally exist only between proper releases, so every
// pre-release-to-pre-release case has no manual actions.
var upgradeE2ECases = []struct {
	name            string
	source          string
	target          string
	verifyCandidate bool
}{
	{
		name:   "v2.0.0-beta.4-to-v2.0.0-beta.5",
		source: "v2.0.0-beta.4",
		target: upgradeE2ETargetVersion,
	},
	{
		name:            "v2.0.0-beta.5-to-v2.0.0-rc.1",
		source:          "v2.0.0-beta.5",
		target:          upgradeE2ECandidateVersion,
		verifyCandidate: true,
	},
	{
		name:            "v2.0.0-beta.7-to-v2.0.0-rc.1",
		source:          "v2.0.0-beta.7",
		target:          upgradeE2ECandidateVersion,
		verifyCandidate: true,
	},
}

// TestUpgradeToHEAD scaffolds with each released CLI in upgradeE2ECases,
// upgrades the result with this checkout, and requires the result to build and
// vet cleanly. Candidate cases first apply the documented rc.1
// pkg/email -> pkg/postmand import migration because the module removal is a
// deliberate pre-rc breaking change. They additionally require the rc.1
// manifest metadata and current published package pins.
//
// The test downloads the released CLI and modules, so it only runs in nightly
// and release-readiness. Set ANDUREL_UPGRADE_E2E=1 to enable it.
//
// Set ANDUREL_UPGRADE_E2E_LOCAL_PKG=1 to replace every pkg/* module with this
// checkout after the upgrade, the way `just local-replace` does. Use it on a
// branch whose package changes are not published yet; the default variant
// stays on the published pins for nightly and release-readiness. Candidate
// cases are release-shaped and are skipped in local-replace mode.
func TestUpgradeToHEAD(t *testing.T) {
	if os.Getenv("ANDUREL_UPGRADE_E2E") != "1" {
		t.Skip("upgrade end-to-end test requires ANDUREL_UPGRADE_E2E=1")
	}
	localPkg := os.Getenv("ANDUREL_UPGRADE_E2E_LOCAL_PKG") == "1"

	for _, tc := range upgradeE2ECases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.verifyCandidate && localPkg {
				t.Skip("candidate paths assert published pins; ANDUREL_UPGRADE_E2E_LOCAL_PKG replaces them")
			}
			testUpgradeToTarget(t, tc.source, tc.target, localPkg, tc.verifyCandidate)
		})
	}
}

func testUpgradeToTarget(
	t *testing.T,
	source string,
	target string,
	localPkg bool,
	verifyCandidate bool,
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
	if len(report.ManualActions) != 0 {
		t.Fatalf("upgrade %s to %s reported manual actions: %#v", source, target, report.ManualActions)
	}

	if verifyCandidate {
		// rc.1 removes pkg/email. Upgrade deliberately does not rewrite
		// application-owned Go code, so the candidate test applies the import
		// rewrite the release notes prescribe before tidy and build.
		migrateLegacyEmailImports(t, root)
	}

	if localPkg {
		replaceWithLocalPackages(t, root)
	}

	runUpgradeE2EGo(t, root, "mod", "tidy")
	if verifyCandidate {
		assertCandidateProject(t, root, target)
	}
	runUpgradeE2EGo(t, root, "build", "./...")
	runUpgradeE2EGo(t, root, "vet", "./...")
}

// assertCandidateProject checks the final candidate metadata after the
// prescribed tidy. The expected pins are literal so a pin drift cannot silently
// make this release-candidate assertion follow the code it is checking.
func assertCandidateProject(t *testing.T, root string, target string) {
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
			version: upgradeE2ECandidateAnduinVersion,
		},
		{
			path:    "github.com/mbvlabs/andurel/pkg/inertia",
			version: upgradeE2ECandidateInertiaVersion,
		},
		{
			path:    "github.com/mbvlabs/andurel/pkg/postmand",
			version: upgradeE2ECandidatePostmandVersion,
		},
	} {
		if got := required[pin.path]; got != pin.version {
			t.Errorf("go.mod pin %s = %q, want %q", pin.path, got, pin.version)
		}
	}
	if len(file.Replace) != 0 {
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
