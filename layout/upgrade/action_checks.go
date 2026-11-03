package upgrade

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/semver"
)

const (
	legacyEmailModulePath = "github.com/mbvlabs/andurel/pkg/email"
	postmandModulePath    = "github.com/mbvlabs/andurel/pkg/postmand"
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
var manualActionDetectors = []manualActionDetector{
	{
		id:          templRootActionID,
		version:     rcOneBreakingVersion,
		inertiaOnly: true,
		detect: func(files projectSources) (ActionStatus, string) {
			for _, rel := range []string{"resources/root.templ", "resources/root_templ.go"} {
				if files.exists(rel) {
					return ActionNotApplied, "legacy compiled templ root remains at " + rel
				}
			}
			if files.exists("resources/root.html") && files.exists("resources/root.go") {
				return ActionApplied, "found resources/root.html and resources/root.go"
			}
			return ActionNotApplicable, "project has no legacy compiled templ root"
		},
	},
	{
		id:      postmandMigrationActionID,
		version: rcOneBreakingVersion,
		detect: func(files projectSources) (ActionStatus, string) {
			goMod, hasGoMod := files.read("go.mod")
			if hasGoMod && strings.Contains(goMod, legacyEmailModulePath) {
				return ActionNotApplied, "go.mod still requires " + legacyEmailModulePath
			}
			if where, found := files.firstContaining(
				[]string{"."},
				[]string{".go"},
				legacyEmailModulePath,
			); found {
				return ActionNotApplied, "legacy pkg/email import remains in " + where
			}
			if hasGoMod && strings.Contains(goMod, postmandModulePath) {
				return ActionApplied, "go.mod requires " + postmandModulePath
			}
			return ActionNotApplicable, "project does not use pkg/email"
		},
	},
}

// filterManualActionsForProject keeps only notices whose application-owned
// change is actually present in the project. A notice for a legacy templ root
// or pkg/email disappears once the application has migrated, so an upgrade of
// a fresh rc.1 project reports no stale actions. Actions without a detector are
// kept: a future notice must opt out explicitly, not vanish silently.
func filterManualActionsForProject(root string, actions []ManualAction) []ManualAction {
	files := projectSources{root: root}
	detectors := make(map[string]manualActionDetector, len(manualActionDetectors))
	for _, detector := range manualActionDetectors {
		detectors[detector.id] = detector
	}

	filtered := make([]ManualAction, 0, len(actions))
	for _, action := range actions {
		detector, ok := detectors[action.ID]
		if !ok {
			filtered = append(filtered, action)
			continue
		}
		if status, _ := detector.detect(files); status == ActionNotApplied {
			filtered = append(filtered, action)
		}
	}
	return filtered
}

// mergeManualActions appends extra notices that base does not already carry.
// Matching by stable ID keeps the version-gated order and prevents a project
// that crosses rc.1 from printing the same action twice.
func mergeManualActions(base, extra []ManualAction) []ManualAction {
	merged := slices.Clone(base)
	for _, action := range extra {
		if slices.ContainsFunc(merged, func(existing ManualAction) bool {
			return existing.ID == action.ID
		}) {
			continue
		}
		merged = append(merged, action)
	}
	return merged
}

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

func (s projectSources) read(rel string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(s.root, filepath.FromSlash(rel)))
	if err != nil {
		return "", false
	}
	return string(data), true
}

func (s projectSources) exists(rel string) bool {
	_, err := os.Stat(filepath.Join(s.root, filepath.FromSlash(rel)))
	return err == nil
}

// firstContaining walks roots and returns the project-relative path of the
// first matching file. A root may be a directory or a single file.
func (s projectSources) firstContaining(
	roots []string,
	extensions []string,
	markers ...string,
) (string, bool) {
	for _, root := range roots {
		path := filepath.Join(s.root, filepath.FromSlash(root))
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			if !hasExtension(root, extensions) {
				continue
			}
			if text, ok := s.read(root); ok && containsMarker(text, markers...) {
				return root, true
			}
			continue
		}

		found := ""
		_ = filepath.WalkDir(path, func(candidate string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if entry.IsDir() {
				switch entry.Name() {
				case ".git", "node_modules", "vendor", "assets":
					return filepath.SkipDir
				}
				return nil
			}
			if !hasExtension(entry.Name(), extensions) {
				return nil
			}
			data, readErr := os.ReadFile(candidate)
			if readErr != nil {
				return nil
			}
			if !containsMarker(string(data), markers...) {
				return nil
			}
			rel, relErr := filepath.Rel(s.root, candidate)
			if relErr != nil {
				found = candidate
			} else {
				found = filepath.ToSlash(rel)
			}
			return filepath.SkipAll
		})
		if found != "" {
			return found, true
		}
	}
	return "", false
}

func containsMarker(text string, markers ...string) bool {
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func hasExtension(name string, extensions []string) bool {
	for _, extension := range extensions {
		if strings.HasSuffix(name, extension) {
			return true
		}
	}
	return false
}
