package upgrade

import (
	"fmt"
	"strings"

	"github.com/mbvlabs/andurel/v2/layout/versions"
	"golang.org/x/mod/semver"
)

// inertiaViteFullURLVersion is the first released V2 pre-release that shipped
// the absolute Vite build URL for Inertia (#805). Projects already on or past
// it get no note.
const inertiaViteFullURLVersion = "v2.0.0-alpha.3"

// packageAPIVersion is the first release whose scaffold pins the package APIs
// that take config structs and drop unused arguments.
const packageAPIVersion = "v2.0.0-alpha.5"

// storageTestClusterVersion is the first release whose scaffold imports the
// test-container helpers from pkg/storage/storagetest.
const storageTestClusterVersion = "v2.0.0-alpha.5"

// goNativeRebuildVersion is the release that replaced Echo and Fx with the
// Go-native domains/services/handlers layout. Projects older than it cannot be
// upgraded automatically; they must be ported by hand.
const goNativeRebuildVersion = "v2.0.0-alpha.4"

// ManualAction describes an application-owned change that an upgrade cannot
// apply without risking user code.
type ManualAction struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Instructions string `json:"instructions"`
}

func manualActionsForUpgrade(
	fromVersion string,
	toVersion string,
	includeInertiaMigration bool,
) ([]ManualAction, error) {
	actions := []ManualAction{}

	if includeInertiaMigration &&
		crossesVersion(fromVersion, toVersion, inertiaViteFullURLVersion) {
		var instructions strings.Builder
		instructions.WriteString(
			"Inertia production tags now need an absolute Vite build URL so secondary-host pages load the entry from the primary origin.\n\n",
		)
		instructions.WriteString(
			"The composition root is application-owned, so Andurel does not rewrite NewInertia automatically.\n\n",
		)
		instructions.WriteString(
			"1. In NewInertia (or the inertia renderer provider), pass routes.ViteBuild.FullURL() instead of Path():\n\n```go\n",
		)
		instructions.WriteString(
			"inertia.NewRenderer(inertia.RendererConfig{\n\tContainerID:  cfg.ContainerID,\n\tBuildPathURL: routes.ViteBuild.FullURL(),\n\t// ...\n})\n",
		)
		instructions.WriteString("```\n\n")
		instructions.WriteString("2. Format and verify the migration:\n\n```text\n")
		instructions.WriteString("gofmt -w internal/runtime\n")
		instructions.WriteString("go fix ./...\ngo vet ./...\n```\n")

		actions = append(actions, ManualAction{
			// The ID predates the alpha.3 gate. It stays unchanged because
			// structured upgrade output exposes it as a stable identifier.
			ID:           "inertia-vite-fullurl-v2.0.0",
			Title:        "Pass ViteBuild.FullURL() to the Inertia renderer",
			Instructions: instructions.String(),
		})
	}

	if crossesVersion(fromVersion, toVersion, packageAPIVersion) {
		actions = append(actions, ManualAction{
			ID:           "package-api-v2.0.0-alpha.5",
			Title:        "Update internal/runtime for the anduin, cache, inertia and storage APIs",
			Instructions: packageAPIInstructions(includeInertiaMigration),
		})
	}

	if crossesVersion(fromVersion, toVersion, storageTestClusterVersion) {
		actions = append(actions, ManualAction{
			ID:           "storage-storagetest-v2.0.0-alpha.5",
			Title:        "Move test-cluster helpers to pkg/storage/storagetest",
			Instructions: storageTestClusterInstructions(),
		})
	}

	return actions, nil
}

func storageTestClusterInstructions() string {
	var instructions strings.Builder
	instructions.WriteString(
		"pkg/storage no longer imports testing or testcontainers. The test-cluster helpers moved to pkg/storage/storagetest with their methods unchanged:\n\n",
	)
	instructions.WriteString(
		"1. In your test files, import the new package and rename the helpers:\n\n```go\n" +
			"import \"github.com/mbvlabs/andurel/pkg/storage/storagetest\"\n\n" +
			"var testCluster *storagetest.Cluster\n\n" +
			"cluster, err := storagetest.NewCluster(context.Background())\n```\n\n",
	)
	instructions.WriteString(
		"2. There is no compatibility alias. Replace `storage.TestCluster` with `storagetest.Cluster` and " +
			"`storage.NewTestCluster` with `storagetest.NewCluster`; `Close` and `NewTestDB` keep their signatures.\n\n",
	)
	instructions.WriteString(fmt.Sprintf(
		"3. The upgrade pins pkg/storage %s. Format and verify:\n\n```text\n"+
			"gofmt -w .\ngo fix ./...\ngo vet ./...\ngolangci-lint run ./...\n```\n",
		versions.Storage,
	))

	return instructions.String()
}

func packageAPIInstructions(inertia bool) string {
	var instructions strings.Builder
	instructions.WriteString(fmt.Sprintf(
		"The upgrade pins pkg/anduin %s, pkg/cache %s, pkg/inertia %s, pkg/kiks %s and pkg/storage %s. "+
			"internal/runtime is application-owned, so update it by hand:\n\n",
		versions.Anduin,
		versions.Cache,
		versions.Inertia,
		versions.Kiks,
		versions.Storage,
	))
	instructions.WriteString(
		"1. anduin.NewServer drops the environment and shutdowner arguments, and Server.Start takes none:\n\n```go\n" +
			"srv := anduin.NewServer(ctx, cfg.HTTP.Host, cfg.HTTP.Port, handler, anduin.WithTimeouts(/* ... */))\n" +
			"err := srv.Start()\n```\n\n",
	)
	instructions.WriteString(
		"2. cache.Store has Close. Close the store on shutdown, after the server stops:\n\n```go\n" +
			"closers = append(closers, func(context.Context) error { return cacheStore.Close() })\n```\n\n",
	)
	instructions.WriteString(
		"3. storage.RunInTransaction is gone. Use storage.AtomicOps with a storage.Transactor.\n\n",
	)
	instructions.WriteString(
		"River v0.40 adds main-line migration 7 (river_notification, drops river_client). " +
			"New projects ship it; existing projects add it with `andurel generate migration add_river_notification_drop_river_client` " +
			"and copy the up and down SQL from River's riverdriver/riverpgxv5/migration/main/007_*.sql.\n\n",
	)
	if inertia {
		instructions.WriteString(
			"4. In internal/runtime, inertia.NewRenderer takes inertia.RendererConfig; in cmd/ssr/main.go, inertia.NewSSRRuntime takes inertia.SSRRuntimeConfig. Both replace positional arguments:\n\n```go\n" +
				"inertia.NewRenderer(inertia.RendererConfig{\n\tContainerID:  cfg.Inertia.ContainerID,\n\tBuildPathURL: routes.ViteBuild.FullURL(),\n\t// ...\n})\n\n" +
				"inertia.NewSSRRuntime(inertia.SSRRuntimeConfig{\n\tExecutable:     cfg.SSRRuntime,\n\tBundlePath:     cfg.SSRBundle,\n\tListenURL:      cfg.SSRListen,\n\tStartupTimeout: cfg.SSRStartupTimeout,\n\tMinimumMajor:   cfg.SSRMinimumMajor,\n\tHealthTimeout:  cfg.SSRRequestTimeout,\n\tBundleFS:       assets.Files,\n})\n```\n\n",
		)
		instructions.WriteString(
			"5. pnpm 10+ blocks esbuild's install script, so `pnpm install` fails. Add pnpm-workspace.yaml:\n\n```yaml\n" +
				"allowBuilds:\n  esbuild: true\nonlyBuiltDependencies:\n  - esbuild\n```\n\n",
		)
		instructions.WriteString(
			"6. Svelte projects: @inertiajs/svelte v3 `useForm` returns a runes object, not a store. " +
				"In resources/js/Pages/Auth/*.svelte replace `$form.` with `form.`.\n\n",
		)
	}
	instructions.WriteString("Then format and verify:\n\n```text\ngofmt -w internal/runtime\ngo fix ./...\ngo vet ./...\n```\n")

	return instructions.String()
}

func crossesVersion(fromVersion, toVersion, boundary string) bool {
	from, fromOK := canonicalUpgradeVersion(fromVersion)
	to, toOK := canonicalUpgradeVersion(toVersion)
	boundary, boundaryOK := canonicalUpgradeVersion(boundary)
	return fromOK && toOK && boundaryOK &&
		semver.Compare(from, boundary) < 0 &&
		semver.Compare(to, boundary) >= 0
}

func canonicalUpgradeVersion(version string) (string, bool) {
	version = strings.TrimSpace(version)
	if version == "" {
		return "", false
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	version = semver.Canonical(version)
	return version, version != ""
}
