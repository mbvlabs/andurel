package upgrade

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// upgradeE2ESourceVersions are the released CLIs the upgrade test scaffolds
// with. Upgrade notes only exist between proper releases, so a source must
// upgrade to upgradeE2ETargetVersion without manual changes.
var upgradeE2ESourceVersions = []string{
	"v2.0.0-beta.4",
}

const upgradeE2ETargetVersion = "v2.0.0-beta.5"

// TestUpgradeToHEAD scaffolds with each released CLI in
// upgradeE2ESourceVersions, upgrades the result with this checkout, and
// requires the result to build and vet cleanly without manual actions.
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

	for _, source := range upgradeE2ESourceVersions {
		t.Run(source, func(t *testing.T) {
			testUpgradeToHEAD(t, source, localPkg)
		})
	}
}

func testUpgradeToHEAD(t *testing.T, source string, localPkg bool) {
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
	if len(report.ManualActions) != 0 {
		t.Fatalf("upgrade %s to %s reported manual actions: %#v", source, upgradeE2ETargetVersion, report.ManualActions)
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
