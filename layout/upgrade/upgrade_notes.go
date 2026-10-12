package upgrade

import (
	"strings"

	"golang.org/x/mod/semver"
)

// inertiaViteFullURLVersion is the first released V2 pre-release that shipped
// the absolute Vite build URL for Inertia (#805). Projects already on or past
// it get no note.
const inertiaViteFullURLVersion = "v2.0.0-alpha.3"

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
			"inertia.NewRenderer(\n\tcfg.ContainerID,\n\troutes.ViteBuild.FullURL(),\n\tcfg.EntryPoint,\n\t// ...\n)\n",
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

	return actions, nil
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
