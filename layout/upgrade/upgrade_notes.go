package upgrade

import (
	"strings"

	"golang.org/x/mod/semver"
)

// inertiaViteFullURLVersion is the first released V2 pre-release that shipped
// the absolute Vite build URL for Inertia (#805). Projects already on or past
// it get no note.
const inertiaViteFullURLVersion = "v2.0.0-alpha.3"

// packageAPIVersion is the first release whose scaffold pins the package APIs
// that take config structs and drop unused arguments.
const packageAPIVersion = "v2.0.0-alpha.5"

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
		instructions.WriteString("gofmt -w internal/runtime/app.go\n")
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

	return actions, nil
}

func packageAPIInstructions(inertia bool) string {
	var instructions strings.Builder
	instructions.WriteString(
		"The upgrade pins pkg/anduin v0.2.0, pkg/cache v0.2.0, pkg/inertia v0.8.0 and pkg/storage v0.10.0. " +
			"internal/runtime is application-owned, so update it by hand:\n\n",
	)
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
			"4. inertia.NewRenderer takes inertia.RendererConfig and inertia.NewSSRRuntime takes inertia.SSRRuntimeConfig instead of positional arguments:\n\n```go\n" +
				"inertia.NewRenderer(inertia.RendererConfig{\n\tContainerID:  cfg.Inertia.ContainerID,\n\tBuildPathURL: routes.ViteBuild.FullURL(),\n\t// ...\n})\n```\n\n",
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
