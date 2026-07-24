package upgrade

import (
	"fmt"
	"strings"

	"github.com/mbvlabs/andurel/layout"
	layouttemplates "github.com/mbvlabs/andurel/layout/templates"
	"golang.org/x/mod/semver"
)

const sessionCookieRecoveryVersion = "v1.5.3"

// ManualAction describes an application-owned change that an upgrade cannot
// apply without risking user code.
type ManualAction struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Instructions string `json:"instructions"`
}

func manualActionsForUpgrade(fromVersion, toVersion, modulePath string) ([]ManualAction, error) {
	if !crossesVersion(fromVersion, toVersion, sessionCookieRecoveryVersion) {
		return nil, nil
	}

	sessionSource, err := renderTemplateToBytes(
		"router_cookies_session.tmpl",
		layouttemplates.Files,
		&layout.TemplateData{ModuleName: modulePath},
	)
	if err != nil {
		return nil, fmt.Errorf("render session-cookie recovery instructions: %w", err)
	}

	var instructions strings.Builder
	instructions.WriteString("The router tree is application-owned, so Andurel did not change it automatically.\n")
	instructions.WriteString("If this recovery is already present, no action is required.\n\n")
	instructions.WriteString("1. Create router/cookies/session.go:\n\n```go\n")
	instructions.Write(sessionSource)
	instructions.WriteString("```\n\n")
	instructions.WriteString("2. In router/cookies/cookies.go and router/cookies/flash.go:\n")
	instructions.WriteString("   - Replace calls to session.Get with getSession.\n")
	instructions.WriteString("   - Remove the now-unused github.com/labstack/echo-contrib/v5/session imports.\n\n")
	instructions.WriteString("3. In router/middleware/middleware.go, add this inside ValidateSession after the assets and API bypass and before calling the next handler:\n\n")
	instructions.WriteString("```go\nif err := cookies.RecoverInvalidSessions(c); err != nil {\n\treturn err\n}\n```\n\n")
	instructions.WriteString("4. Add this direct requirement to go.mod:\n\n")
	instructions.WriteString("```go\ngithub.com/gorilla/securecookie v1.1.2\n```\n\n")
	instructions.WriteString("5. Format and verify the migration:\n\n```text\n")
	instructions.WriteString("gofmt -w router/cookies/session.go router/cookies/cookies.go router/cookies/flash.go router/middleware/middleware.go\n")
	instructions.WriteString("go fix ./...\ngo vet ./...\n```\n")

	return []ManualAction{{
		ID:           "session-cookie-recovery-v1.5.3",
		Title:        "Update application-owned session handling",
		Instructions: instructions.String(),
	}}, nil
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
