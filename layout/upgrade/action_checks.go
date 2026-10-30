package upgrade

import (
	"os"
	"path/filepath"
	"strings"

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
var manualActionDetectors = []manualActionDetector{
	{
		id:          "inertia-vite-fullurl-v2.0.0",
		version:     inertiaViteFullURLVersion,
		inertiaOnly: true,
		detect: func(files projectSources) (ActionStatus, string) {
			return markerStatus(files, goRoots, "ViteBuild.FullURL(")
		},
	},
	{
		id:      "package-api-v2.0.0-alpha.5",
		version: packageAPIVersion,
		detect: func(files projectSources) (ActionStatus, string) {
			if where, found := files.firstContaining(goRoots, goExtensions, "storage.RunInTransaction("); found {
				return ActionNotApplied, "still calls storage.RunInTransaction in " + where
			}
			return markerStatus(files, goRoots, "anduin.NewServer(")
		},
	},
	{
		id:      "storage-storagetest-v2.0.0-alpha.5",
		version: storageTestClusterVersion,
		detect: func(files projectSources) (ActionStatus, string) {
			for _, marker := range []string{"storage.TestCluster", "storage.NewTestCluster"} {
				if where, found := files.firstContaining(goRoots, goExtensions, marker); found {
					return ActionNotApplied, "still uses " + marker + " in " + where
				}
			}
			return ActionApplied, "no storage.TestCluster references remain"
		},
	},
	{
		id:      "routing-hosts-v2.0.0-alpha.5",
		version: packageAPIVersion,
		detect: func(files projectSources) (ActionStatus, string) {
			return markerStatus(files, goRoots, "routing.NewHosts(")
		},
	},
	{
		id:      "session-revocation-v2.0.0-alpha.5",
		version: packageAPIVersion,
		detect: func(files projectSources) (ActionStatus, string) {
			return markerStatus(files, sqlRoots, "session_version")
		},
	},
	{
		id:      "email-smtp-v2.0.0-alpha.5",
		version: packageAPIVersion,
		detect: func(files projectSources) (ActionStatus, string) {
			return markerStatus(files, configRoots, "SMTPHost")
		},
	},
	{
		id:      "validation-validate-v2.0.0-alpha.5",
		version: packageAPIVersion,
		detect: func(files projectSources) (ActionStatus, string) {
			if !files.dirExists("domains") {
				return ActionNotApplicable, "project has no domains package"
			}
			if where, found := files.firstContaining(domainRoots, goExtensions, "validation.Validate("); found {
				return ActionNotApplied, "still calls validation.Validate in " + where
			}
			return ActionApplied, "no validation.Validate call remains under domains"
		},
	},
	{
		id:      "email-sender-v2.0.0-beta.2",
		version: betaTwoFixesVersion,
		detect: func(files projectSources) (ActionStatus, string) {
			return markerStatus(files, configRoots, "senderHost(")
		},
	},
	{
		id:          "vite-ssr-bundle-v2.0.0-beta.2",
		version:     betaTwoFixesVersion,
		inertiaOnly: true,
		detect: func(files projectSources) (ActionStatus, string) {
			return markerStatus(files, []string{"vite.config.ts", "vite.config.js", "vite.config.mts"}, "noExternal: true")
		},
	},
	{
		id:          "root-title-ssr-v2.0.0-beta.2",
		version:     betaTwoFixesVersion,
		inertiaOnly: true,
		detect: func(files projectSources) (ActionStatus, string) {
			return markerStatus(files, []string{"resources/root.templ"}, "data.SSR == nil")
		},
	},
	{
		id:      "cors-host-origins-v2.0.0-beta.3",
		version: betaThreeFixesVersion,
		detect: func(files projectSources) (ActionStatus, string) {
			return markerStatus(files, goRoots, "isViteAssetRequest")
		},
	},
	{
		id:      "csrf-asset-skip-v2.0.0-beta.3",
		version: betaThreeFixesVersion,
		detect: func(files projectSources) (ActionStatus, string) {
			return markerStatus(files, goRoots, "isSafeViteAssetRequest")
		},
	},
	{
		id:      "method-not-allowed-page-v2.0.0-beta.3",
		version: betaThreeFixesVersion,
		detect: func(files projectSources) (ActionStatus, string) {
			return markerStatus(files, handlerRoots, "ErrMethodNotAllowed")
		},
	},
	{
		id:      "request-host-middleware-v2.0.0-beta.3",
		version: betaThreeFixesVersion,
		detect: func(files projectSources) (ActionStatus, string) {
			return markerStatus(files, goRoots, "RequestHost(ctx)")
		},
	},
	{
		id:      "public-home-v2.0.0-beta.4",
		version: betaFourFixesVersion,
		detect: func(files projectSources) (ActionStatus, string) {
			if where, found := files.firstContaining(
				goRoots,
				goExtensions,
				"NewPagesHandler(renderer).Register(router, middleware.AuthOnly",
				"NewPages(renderer).Register(router, middleware.AuthOnly",
			); found {
				return ActionNotApplied, "welcome page is still behind AuthOnly in " + where
			}
			if where, found := files.firstContaining(
				goRoots,
				goExtensions,
				"NewPagesHandler(renderer).Register(router)",
				"NewPages(renderer).Register(router)",
			); found {
				return ActionApplied, "public welcome registration found in " + where
			}
			return ActionNotApplicable, "project has no generated Pages registration"
		},
	},
	{
		id:      "configcheck-v2.0.0-beta.4",
		version: betaFourFixesVersion,
		detect: func(files projectSources) (ActionStatus, string) {
			for _, path := range []string{"internal/configcheck/main.go", "cmd/configcheck/main.go"} {
				if files.exists(path) {
					return ActionApplied, "found " + path
				}
			}
			return ActionNotApplied, "no internal/configcheck/main.go or cmd/configcheck/main.go"
		},
	},
	{
		id:      "runtime-readiness-v2.0.0-beta.4",
		version: betaFourFixesVersion,
		detect: func(files projectSources) (ActionStatus, string) {
			return markerStatus(files, append(append([]string{}, routeRoots...), handlerRoots...), "ReadinessReason")
		},
	},
	{
		id:          "inertia-ssr-default-v2.0.0-beta.4",
		version:     betaFourFixesVersion,
		inertiaOnly: true,
		detect: func(files projectSources) (ActionStatus, string) {
			return markerStatus(files, goRoots, "WithSSRDefault(")
		},
	},
	{
		id:          "routes-host-scope-v2.0.0-beta.4",
		version:     betaFourFixesVersion,
		inertiaOnly: true,
		detect: func(files projectSources) (ActionStatus, string) {
			return markerStatus(files, []string{"resources/js"}, "configureRouteHostScope")
		},
	},
}

var (
	goRoots       = []string{"internal/runtime", "app", "cmd", "handlers", "middleware", "services", "routes", "config"}
	goExtensions  = []string{".go"}
	allExtensions = []string{
		".go", ".ts", ".tsx", ".js", ".jsx", ".mts", ".templ", ".sql", ".yaml", ".yml",
	}
	handlerRoots = []string{"handlers"}
	configRoots  = []string{"config"}
	routeRoots   = []string{"routes"}
	domainRoots  = []string{"domains"}
	sqlRoots     = []string{"domains", "migrations"}
)

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

// projectSources reads project files for the static detectors. Detectors are
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

func (s projectSources) dirExists(rel string) bool {
	info, err := os.Stat(filepath.Join(s.root, filepath.FromSlash(rel)))
	return err == nil && info.IsDir()
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
				case "node_modules", "vendor", ".git", "assets":
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

func markerStatus(files projectSources, roots []string, markers ...string) (ActionStatus, string) {
	if where, found := files.firstContaining(roots, allExtensions, markers...); found {
		return ActionApplied, "found " + markers[0] + " in " + where
	}
	return ActionNotApplied, "no " + markers[0] + " marker under " + strings.Join(roots, ", ")
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
