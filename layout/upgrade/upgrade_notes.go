package upgrade

import (
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

// goNativeRebuildVersion is the release that replaced Echo and Fx with the
// Go-native domains/services/handlers layout. Projects older than it cannot be
// upgraded automatically; they must be ported by hand.
const goNativeRebuildVersion = "v2.0.0-alpha.4"

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

// manualActionsForUpgrade lists the manual actions between two versions.
// Upgrade notes only exist between proper releases; pre-releases carry none.
func manualActionsForUpgrade(
	fromVersion string,
	toVersion string,
	includeInertiaMigration bool,
) ([]ManualAction, error) {
	return []ManualAction{}, nil
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
