package upgrade

import (
	"fmt"
	"strings"

	"github.com/mbvlabs/andurel/v2/layout/versions"
	"golang.org/x/mod/semver"
)

// goNativeRebuildVersion is the release that replaced Echo and Fx with the
// Go-native domains/services/handlers layout. Projects older than it cannot be
// upgraded automatically; they must be ported by hand.
const goNativeRebuildVersion = "v2.0.0-alpha.4"

// rcOneBreakingVersion is the first release candidate. It makes two deliberate
// application-owned breaking changes: the Inertia root moves from templ to
// html/template and pkg/email is replaced by pkg/postmand.
const rcOneBreakingVersion = "v2.0.0-rc.1"

// Stable manual-action IDs. They are also the keys andurel doctor uses.
const (
	templRootActionID         = "templ-root-v2.0.0-rc.1"
	postmandMigrationActionID = "postmand-v2.0.0-rc.1"
)

// ManualAction describes an application-owned change that an upgrade cannot
// apply without risking user code. ID is stable and is also the key andurel
// doctor uses to report the action's static detector state.
type ManualAction struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Instructions string `json:"instructions"`
}

// ManualActionsBetween returns the manual actions an upgrade from fromVersion
// to toVersion prints. It lets a project re-read them after andurel.toml
// already records toVersion.
func ManualActionsBetween(
	fromVersion string,
	toVersion string,
	includeInertiaMigration bool,
) ([]ManualAction, error) {
	from, fromOK := canonicalUpgradeVersion(fromVersion)
	if !fromOK {
		return nil, fmt.Errorf("invalid from version %q", fromVersion)
	}
	to, toOK := canonicalUpgradeVersion(toVersion)
	if !toOK {
		return nil, fmt.Errorf("invalid to version %q", toVersion)
	}
	if semver.Compare(from, to) >= 0 {
		return nil, fmt.Errorf("from version %s must be older than %s", from, to)
	}

	return manualActionsForUpgrade(from, to, includeInertiaMigration)
}

// legacyBreakingManualActions returns the two rc.1 breaking-change notices.
// Callers decide when they apply: the upgrade report filters them against the
// project's actual root document and pkg/email usage, so a project that never
// had the legacy shape gets no notice. The version-gated helper below uses the
// same list for inspect upgrade-notes and doctor.
func legacyBreakingManualActions(includeInertiaMigration bool) []ManualAction {
	actions := make([]ManualAction, 0, 2)
	if includeInertiaMigration {
		actions = append(actions, ManualAction{
			ID:           templRootActionID,
			Title:        "Replace the compiled templ root with resources/root.html",
			Instructions: templRootMigrationInstructions(),
		})
	}
	actions = append(actions, ManualAction{
		ID:           postmandMigrationActionID,
		Title:        "Replace pkg/email with pkg/postmand",
		Instructions: postmandMigrationInstructions(),
	})
	return actions
}

// manualActionsForUpgrade lists the manual actions between two versions.
// Upgrade notes normally only exist between proper releases. rc.1 is the
// approved exception: it deliberately removes compiled templ roots and
// pkg/email before rc, so those breaking changes carry actionable notices.
func manualActionsForUpgrade(
	fromVersion string,
	toVersion string,
	includeInertiaMigration bool,
) ([]ManualAction, error) {
	if crossesVersion(fromVersion, toVersion, rcOneBreakingVersion) {
		return legacyBreakingManualActions(includeInertiaMigration), nil
	}
	return []ManualAction{}, nil
}

// ManualActionsForProject returns the manual actions that still apply to the
// project at root. Unlike ManualActionsBetween, it filters the combined list
// against the project's actual structure, dependencies and imports, so an
// application that already migrated gets no stale migration notice. The
// upgrade plan and inspect upgrade-notes both use this so they agree with each
// other and with doctor's per-action detectors.
func ManualActionsForProject(
	root string,
	fromVersion string,
	toVersion string,
	includeInertiaMigration bool,
) ([]ManualAction, error) {
	actions, err := ManualActionsBetween(fromVersion, toVersion, includeInertiaMigration)
	if err != nil {
		return nil, err
	}
	combined := mergeManualActions(
		actions,
		legacyBreakingManualActions(includeInertiaMigration),
	)
	return filterManualActionsForProject(root, combined), nil
}

func templRootMigrationInstructions() string {
	var instructions strings.Builder
	instructions.WriteString(
		"rc.1 removes templ. pkg/inertia v" + strings.TrimPrefix(versions.Inertia, "v") +
			" renders a standard-library html/template document, so an old compiled templ root no longer builds.\n\n",
	)
	instructions.WriteString(
		"Andurel never rewrites this application-owned document. Replace resources/root.templ and " +
			"resources/root_templ.go with resources/root.html:\n\n```html\n",
	)
	instructions.WriteString(
		"<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">{{if not .SSR}}<title>{{.ProjectName}}</title>{{end}}{{.SSRHead}}{{.ViteHead}}</head><body class=\"bg-background text-foreground\">{{if .SSR}}{{.SSRBody}}{{else}}{{.PageScript}} {{.AppMount}}{{end}}{{.ViteBody}}</body></html>\n",
	)
	instructions.WriteString(
		"```\n\nand resources/root.go:\n\n```go\npackage resources\n\nimport (\n\t\"embed\"\n\t\"html/template\"\n)\n\n//go:embed root.html\nvar files embed.FS\n\nvar Root = template.Must(template.New(\"root.html\").ParseFS(files, \"root.html\"))\n```\n\n",
	)
	instructions.WriteString(
		"Keep config/inertia.go's inertia.WithRoot(resources.Root); the .SSRHead, .ViteHead, .PageScript, .AppMount, .SSRBody and .ViteBody values are pre-rendered safe fragments and must stay unescaped. Delete the two legacy root files, remove github.com/a-h/templ/cmd/templ from the go.mod tool directive, then run:\n\n```text\ngo get github.com/mbvlabs/andurel/pkg/inertia@" + versions.Inertia + "\ngo mod tidy\ngofmt -w .\nandurel doctor\nandurel build\nandurel verify\n```\n",
	)
	return instructions.String()
}

func postmandMigrationInstructions() string {
	var instructions strings.Builder
	instructions.WriteString(
		"rc.1 removes github.com/mbvlabs/andurel/pkg/email. Applications use pkg/postmand instead; there is no " +
			"compatibility adapter and the old data types are intentionally not compatible.\n\n",
	)
	instructions.WriteString(
		"Andurel never rewrites application-owned email code. Upgrade the modules in this order, because storage " +
			"older than v" + strings.TrimPrefix(versions.Storage, "v") +
			" still exposes pkg/email types in its worker API:\n\n```text\ngo get github.com/mbvlabs/andurel/pkg/postmand@" +
			versions.Postmand + "\ngo get github.com/mbvlabs/andurel/pkg/storage@" + versions.Storage + "\n```\n\n",
	)
	instructions.WriteString(
		"Rewrite imports from github.com/mbvlabs/andurel/pkg/email to github.com/mbvlabs/andurel/pkg/postmand and use " +
			"postmand.Transformer, postmand.Compose and postmand.TransactionalData. Remove any pkgemail alias.\n\n",
	)
	instructions.WriteString(
		"Replace the old emails/render.go with the shared template renderer:\n\n```go\npackage emails\n\nimport (\n\t\"embed\"\n\n\t\"github.com/mbvlabs/andurel/pkg/postmand\"\n)\n\n//go:embed templates/*.tmpl\nvar templatesFS embed.FS\n\nvar templates = postmand.MustParseTemplates(templatesFS, \"templates\")\n```\n\n",
	)
	instructions.WriteString(
		"Then replace renderHTML(\"name\", v) with templates.HTML(\"name\", v) and renderText(\"name\", v) with templates.Text(\"name\", v) in each email. Run go mod tidy only after storage is upgraded, then:\n\n```text\ngofmt -w .\ngo fix ./...\ngo vet ./...\nandurel doctor\nandurel build\n```\n",
	)
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
