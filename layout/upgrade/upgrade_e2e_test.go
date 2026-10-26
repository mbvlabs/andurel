package upgrade

import (
	"context"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	upgradeE2ESourceVersion = "v2.0.0-alpha.4"
	upgradeE2ETargetVersion = "v2.0.0-alpha.5"
)

// TestUpgradeAlpha4ToHEAD scaffolds with the released v2.0.0-alpha.4 CLI,
// upgrades the result with this checkout, applies every manual action the
// upgrade reports, and requires the result to build and vet cleanly.
//
// The test downloads the released CLI and modules, so it only runs in nightly
// and release-readiness. Set ANDUREL_UPGRADE_E2E=1 to enable it.
func TestUpgradeAlpha4ToHEAD(t *testing.T) {
	if os.Getenv("ANDUREL_UPGRADE_E2E") != "1" {
		t.Skip("upgrade end-to-end test requires ANDUREL_UPGRADE_E2E=1")
	}

	parent := t.TempDir()
	appName := "upgradee2e"
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	scaffold := exec.CommandContext(
		ctx,
		"go",
		"run",
		"github.com/mbvlabs/andurel/v2@"+upgradeE2ESourceVersion,
		"new",
		appName,
	)
	scaffold.Dir = parent
	scaffold.Env = append(os.Environ(), "GOWORK=off")
	if output, err := scaffold.CombinedOutput(); err != nil {
		t.Fatalf("scaffold with %s: %v\n%s", upgradeE2ESourceVersion, err, output)
	}

	root := filepath.Join(parent, appName)
	gitRun(t, root, "init")
	gitRun(t, root, "config", "user.email", "upgrade-e2e@example.com")
	gitRun(t, root, "config", "user.name", "Upgrade E2E")
	commitUpgradeTree(t, root, "scaffold from "+upgradeE2ESourceVersion)

	upgrader, err := NewUpgrader(
		root,
		UpgradeOptions{TargetVersion: upgradeE2ETargetVersion},
	)
	if err != nil {
		t.Fatal(err)
	}
	report, err := upgrader.Execute()
	if err != nil {
		t.Fatalf("upgrade %s to %s: %v", upgradeE2ESourceVersion, upgradeE2ETargetVersion, err)
	}

	for _, action := range report.ManualActions {
		if err := applyUpgradeE2EManualAction(root, action); err != nil {
			t.Fatalf(
				"manual action %q (%s) was not applied: %v",
				action.ID,
				action.Title,
				err,
			)
		}
	}

	runUpgradeE2EGo(t, root, "mod", "tidy")
	runUpgradeE2EGo(t, root, "build", "./...")
	runUpgradeE2EGo(t, root, "vet", "./...")
}

func applyUpgradeE2EManualAction(root string, action ManualAction) error {
	switch action.ID {
	case "package-api-v2.0.0-alpha.5":
		return applyUpgradeE2EPackageAPI(root)
	case "storage-storagetest-v2.0.0-alpha.5":
		return applyUpgradeE2EStorageTestCluster(root)
	case "inertia-vite-fullurl-v2.0.0":
		return applyUpgradeE2EInertiaFullURL(root)
	default:
		return fmt.Errorf(
			"no automated e2e implementation for manual action %q (%s)",
			action.ID,
			action.Title,
		)
	}
}

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
