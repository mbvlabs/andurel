package upgrade

import (
	"golang.org/x/mod/semver"
)

// ActionStatus is the static detector's verdict for one manual upgrade action.
type ActionStatus string

const (
	// ActionApplied means the project contains the action's expected change.
	ActionApplied ActionStatus = "applied"
	// ActionNotApplied means the action applies to this project but its change
	// was not detected. The project may still hold an equivalent application
	// fix; the action instructions say when that counts.
	ActionNotApplied ActionStatus = "not_applied"
	// ActionNotApplicable means the action does not concern this project: it
	// is for a newer version, it is Inertia-only on a non-Inertia project, or
	// the generated code it changes is not present.
	ActionNotApplicable ActionStatus = "not_applicable"
)

// ActionCheck is one manual upgrade action's detected state in a project. ID is
// the stable action identifier printed by andurel upgrade and
// inspect upgrade-notes. Detail names the file and marker the detector used.
type ActionCheck struct {
	ID      string       `json:"id"`
	Title   string       `json:"title"`
	Version string       `json:"version"`
	Status  ActionStatus `json:"status"`
	Detail  string       `json:"detail,omitempty"`
}

// manualActionDetector statically checks one action's application-owned change.
// The detectors are deliberately conservative: a missing marker is reported as
// not_applied, and the action's "already fixed this yourself" paragraph tells
// the operator how to confirm an equivalent implementation.
type manualActionDetector struct {
	id          string
	version     string
	inertiaOnly bool
	detect      func(projectSources) (ActionStatus, string)
}

// manualActionDetectors is ordered like the upgrade notes. Every action that
// manualActionsForUpgrade can emit has an entry; TestManualActionDetectorsCoverEveryAction
// fails when a new action is added without one.
var manualActionDetectors = []manualActionDetector{}

// CheckManualActions statically checks the manual actions that apply to a
// project at projectVersion, plus actions between projectVersion and
// cliVersion, which are reported not_applicable until the upgrade. Inertia-only
// actions on a non-Inertia project are not_applicable. An empty or invalid
// project version returns nil: doctor already reports the lock-file problem.
func CheckManualActions(
	rootDir string,
	projectVersion string,
	cliVersion string,
	inertia bool,
) []ActionCheck {
	project, ok := canonicalUpgradeVersion(projectVersion)
	if !ok {
		return nil
	}
	target := project
	if current, ok := canonicalUpgradeVersion(cliVersion); ok && semver.Compare(current, target) > 0 {
		target = current
	}

	actions, err := manualActionsForUpgrade("v2.0.0-alpha.0", target, true)
	if err != nil {
		return nil
	}
	titles := make(map[string]ManualAction, len(actions))
	for _, action := range actions {
		titles[action.ID] = action
	}

	files := projectSources{root: rootDir}
	checks := make([]ActionCheck, 0, len(manualActionDetectors))
	for _, detector := range manualActionDetectors {
		action, ok := titles[detector.id]
		if !ok {
			continue
		}
		check := ActionCheck{
			ID:      detector.id,
			Title:   action.Title,
			Version: detector.version,
		}
		switch {
		case semver.Compare(detector.version, project) > 0:
			check.Status = ActionNotApplicable
			check.Detail = "requires " + detector.version + "; run andurel upgrade"
		case detector.inertiaOnly && !inertia:
			check.Status = ActionNotApplicable
			check.Detail = "project does not use Inertia"
		default:
			check.Status, check.Detail = detector.detect(files)
		}
		checks = append(checks, check)
	}
	return checks
}

// projectSources is the project tree the static detectors read. Detectors are
// read-only and tolerate missing files: an absent file is simply no marker.
type projectSources struct {
	root string
}
