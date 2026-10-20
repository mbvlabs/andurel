package upgrade

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
)

// CodeUnsupportedUpgrade is the stable error code for a project that cannot be
// upgraded automatically.
const CodeUnsupportedUpgrade = "unsupported_upgrade"

// UnsupportedUpgradeData is the structured payload attached to an
// UnsupportedUpgradeError.
type UnsupportedUpgradeData struct {
	From           string   `json:"from"`
	To             string   `json:"to"`
	RebuildVersion string   `json:"rebuild_version"`
	LegacyMarkers  []string `json:"legacy_markers"`
}

// UnsupportedUpgradeError reports a project that predates the Go-native
// rebuild and needs a manual port. It is informational: the gate is driven by
// the version, while LegacyMarkers only describe the shape that was detected.
type UnsupportedUpgradeError struct {
	FromVersion    string
	ToVersion      string
	RebuildVersion string
	LegacyMarkers  []string
}

// Error returns the human-readable rejection message.
func (e *UnsupportedUpgradeError) Error() string {
	return fmt.Sprintf(
		"cannot upgrade project from %s to %s: projects created before %s use the "+
			"pre-rebuild Echo/Fx layout and cannot be upgraded automatically",
		e.FromVersion,
		e.ToVersion,
		e.RebuildVersion,
	)
}

// Code returns the stable machine-readable error code.
func (e *UnsupportedUpgradeError) Code() string {
	return CodeUnsupportedUpgrade
}

// Hint tells the caller how to migrate the project by hand. The gate is
// version-driven, so retrying the upgrade cannot succeed while the project
// still carries the pre-rebuild version.
func (e *UnsupportedUpgradeError) Hint() string {
	return "Use a fresh Go-native scaffold from `andurel new` as the migration baseline, " +
		"then port the app by hand to domains/, services/, handlers/ and pkg/anduin, " +
		"guided by `andurel patterns show composition`, `andurel patterns show domain` " +
		"and `andurel patterns show handler`. Upgrading the pre-rebuild project in " +
		"place is not supported."
}

// Data returns the structured detail carried by the CLI error envelope.
func (e *UnsupportedUpgradeError) Data() UnsupportedUpgradeData {
	markers := e.LegacyMarkers
	if markers == nil {
		markers = []string{}
	}
	return UnsupportedUpgradeData{
		From:           e.FromVersion,
		To:             e.ToVersion,
		RebuildVersion: e.RebuildVersion,
		LegacyMarkers:  markers,
	}
}

// validateGoNativeRebuild refuses to upgrade a pre-rebuild v2 project past the
// Go-native rebuild. The boundary is inclusive on the target: an upgrade whose
// destination is goNativeRebuildVersion is refused too. Non-semver versions
// (such as development builds) are not checked.
func validateGoNativeRebuild(projectRoot, projectVersion, targetVersion string) error {
	from, fromOK := canonicalUpgradeVersion(projectVersion)
	to, toOK := canonicalUpgradeVersion(targetVersion)
	rebuild, rebuildOK := canonicalUpgradeVersion(goNativeRebuildVersion)
	if !fromOK || !toOK || !rebuildOK {
		return nil
	}
	if semver.Compare(from, rebuild) >= 0 || semver.Compare(to, rebuild) < 0 {
		return nil
	}
	return &UnsupportedUpgradeError{
		FromVersion:    projectVersion,
		ToVersion:      targetVersion,
		RebuildVersion: goNativeRebuildVersion,
		LegacyMarkers:  detectLegacyMarkers(projectRoot),
	}
}

// detectLegacyMarkers describes the pre-rebuild shape, when present. It is
// informational only and never changes whether the gate applies.
func detectLegacyMarkers(projectRoot string) []string {
	markers := make([]string, 0, 4)
	for _, directory := range []string{"controllers", "router", "models"} {
		info, err := os.Stat(filepath.Join(projectRoot, directory))
		if err == nil && info.IsDir() {
			markers = append(markers, directory+"/")
		}
	}
	if goModRequiresFX(projectRoot) {
		markers = append(markers, "go.uber.org/fx")
	}
	return markers
}

func goModRequiresFX(projectRoot string) bool {
	content, err := os.ReadFile(filepath.Join(projectRoot, "go.mod"))
	if err != nil {
		return false
	}
	file, err := modfile.Parse("go.mod", content, nil)
	if err != nil {
		return false
	}
	for _, requirement := range file.Require {
		if requirement != nil && requirement.Mod.Path == "go.uber.org/fx" {
			return true
		}
	}
	return false
}
