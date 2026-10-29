package goldentest

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mbvlabs/andurel/v2/layout/versions"
)

// CandidateInertiaModulePath is the standalone module whose pin may be ahead of
// the published tags while a release is still being assembled. It is only
// served from this checkout when a golden run explicitly opts in with
// ANDUREL_GOLDEN_LOCAL_PKG=1.
const CandidateInertiaModulePath = "github.com/mbvlabs/andurel/pkg/inertia"

const (
	// LocalPkgEnv opts a golden run into resolving CandidateInertiaModulePath
	// from this checkout through a file module proxy. It is development-only:
	// the bytes served are not the published module, so a run with this flag set
	// is not release-shaped proof.
	LocalPkgEnv = "ANDUREL_GOLDEN_LOCAL_PKG"
	// ReleaseEnv opts a golden run into release-shaped module resolution:
	// public proxy, public checksum database, and isolated fresh caches that
	// cannot be satisfied by a developer's local module cache.
	ReleaseEnv = "ANDUREL_GOLDEN_RELEASE"

	publicGoProxy = "https://proxy.golang.org,direct"
	publicGoSumDB = "sum.golang.org"
)

// GoldenMode selects how golden tests resolve the standalone pkg/* pins in
// layout/versions.
type GoldenMode int

const (
	// GoldenModePublished resolves the pins from the public Go proxy. It is the
	// default: a normal golden run never substitutes local package source.
	GoldenModePublished GoldenMode = iota
	// GoldenModeLocalPkg serves this checkout's pkg/inertia source as the pinned
	// version through a file module proxy. Development only.
	GoldenModeLocalPkg
	// GoldenModeRelease resolves the pins from the public proxy and checksum
	// database with isolated fresh caches. Release-shaped proof.
	GoldenModeRelease
)

// String returns the mode name used in test output and errors.
func (mode GoldenMode) String() string {
	switch mode {
	case GoldenModeLocalPkg:
		return "local-pkg"
	case GoldenModeRelease:
		return "release"
	default:
		return "published"
	}
}

// GoldenModeFromEnv reads the golden module-resolution mode from env.
//
// The local proxy and release modes are mutually exclusive: one substitutes
// this checkout for a pin, the other proves the published bytes. Setting both
// is a configuration error, not a preference order.
func GoldenModeFromEnv(env []string) (GoldenMode, error) {
	localPkg, err := envFlag(env, LocalPkgEnv)
	if err != nil {
		return GoldenModePublished, err
	}
	release, err := envFlag(env, ReleaseEnv)
	if err != nil {
		return GoldenModePublished, err
	}
	switch {
	case localPkg && release:
		return GoldenModePublished, fmt.Errorf(
			"%s=1 and %s=1 are mutually exclusive",
			LocalPkgEnv,
			ReleaseEnv,
		)
	case localPkg:
		return GoldenModeLocalPkg, nil
	case release:
		return GoldenModeRelease, nil
	default:
		return GoldenModePublished, nil
	}
}

func envFlag(env []string, key string) (bool, error) {
	for _, entry := range env {
		name, value, ok := strings.Cut(entry, "=")
		if !ok || name != key {
			continue
		}
		switch value {
		case "1":
			return true, nil
		case "", "0":
			return false, nil
		default:
			return false, fmt.Errorf(
				"%s=%q: use 1 to enable or 0/empty to disable",
				key,
				value,
			)
		}
	}
	return false, nil
}

type goldenEnvState struct {
	mode       GoldenMode
	root       string
	proxyDir   string
	modCache   string
	buildCache string
	tempDir    string
}

var (
	goldenEnvMu sync.Mutex
	goldenEnv   *goldenEnvState
)

// GoldenGoEnv returns env configured for the golden mode selected by env.
//
// Published (default) and release modes force the public Go proxy and checksum
// database and strip checksum-bypass settings (GOPRIVATE, GONOSUMDB, ...). The
// release mode additionally owns fresh GOMODCACHE, GOCACHE, and GOTMPDIR
// directories so a cached copy of the pinned module cannot satisfy the run.
//
// Local-pkg mode prepends a file proxy built from this checkout and disables
// checksum verification for it, with the same isolated caches. It must never be
// used to claim release-shaped proof.
func GoldenGoEnv(env []string) ([]string, error) {
	mode, err := GoldenModeFromEnv(env)
	if err != nil {
		return nil, err
	}
	state, err := ensureGoldenEnv(mode)
	if err != nil {
		return nil, err
	}

	filtered := stripGoldenEnv(env)
	filtered = append(filtered, "GOWORK=off")
	switch mode {
	case GoldenModeLocalPkg:
		filtered = append(filtered,
			"GOPROXY=file://"+filepath.ToSlash(state.proxyDir)+","+publicGoProxy,
			"GOSUMDB=off",
			"GOMODCACHE="+state.modCache,
			"GOCACHE="+state.buildCache,
			"GOTMPDIR="+state.tempDir,
			"GOENV=off",
		)
	case GoldenModeRelease:
		filtered = append(filtered,
			"GOPROXY="+publicGoProxy,
			"GOSUMDB="+publicGoSumDB,
			"GOMODCACHE="+state.modCache,
			"GOCACHE="+state.buildCache,
			"GOTMPDIR="+state.tempDir,
			"GOENV=off",
		)
	default:
		filtered = append(filtered,
			"GOPROXY="+publicGoProxy,
			"GOSUMDB="+publicGoSumDB,
		)
	}
	return filtered, nil
}

// CleanupGoldenGoEnv removes the temporary proxy and caches owned by the current
// golden mode. It is safe to call more than once and resets the mode so a later
// GoldenGoEnv call can select a different mode.
func CleanupGoldenGoEnv() {
	goldenEnvMu.Lock()
	defer goldenEnvMu.Unlock()

	if goldenEnv == nil {
		return
	}
	if goldenEnv.root != "" {
		_ = os.RemoveAll(goldenEnv.root)
	}
	goldenEnv = nil
}

func ensureGoldenEnv(mode GoldenMode) (*goldenEnvState, error) {
	goldenEnvMu.Lock()
	defer goldenEnvMu.Unlock()

	if goldenEnv != nil {
		if goldenEnv.mode != mode {
			return nil, fmt.Errorf(
				"golden Go environment already initialized for %s; cannot switch to %s",
				goldenEnv.mode,
				mode,
			)
		}
		return goldenEnv, nil
	}

	state := &goldenEnvState{mode: mode}
	if mode == GoldenModePublished {
		goldenEnv = state
		return state, nil
	}

	root, err := os.MkdirTemp("", "andurel-golden-env-*")
	if err != nil {
		return nil, fmt.Errorf("create golden environment dir: %w", err)
	}
	state.root = root
	state.modCache = filepath.Join(root, "modcache")
	state.buildCache = filepath.Join(root, "gocache")
	state.tempDir = filepath.Join(root, "tmp")
	for _, dir := range []string{state.modCache, state.buildCache, state.tempDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			_ = os.RemoveAll(root)
			return nil, fmt.Errorf("create golden Go cache %s: %w", dir, err)
		}
	}

	if mode == GoldenModeLocalPkg {
		state.proxyDir = filepath.Join(root, "proxy")
		if err := os.MkdirAll(state.proxyDir, 0o755); err != nil {
			_ = os.RemoveAll(root)
			return nil, fmt.Errorf("create local module proxy: %w", err)
		}
		if err := writeCandidateModule(
			state.proxyDir,
			RepoRoot(),
			CandidateInertiaModulePath,
			versions.Inertia,
		); err != nil {
			_ = os.RemoveAll(root)
			return nil, err
		}
	}

	goldenEnv = state
	return state, nil
}

// stripGoldenEnv removes module-resolution settings so the selected mode is the
// only source of proxy, checksum, workspace, and cache configuration.
func stripGoldenEnv(env []string) []string {
	filtered := make([]string, 0, len(env)+8)
	for _, entry := range env {
		name, _, ok := strings.Cut(entry, "=")
		if !ok {
			filtered = append(filtered, entry)
			continue
		}
		switch name {
		case "GOWORK",
			"GOPROXY",
			"GOSUMDB",
			"GOMODCACHE",
			"GOCACHE",
			"GOTMPDIR",
			"GOENV",
			"GOPRIVATE",
			"GONOPROXY",
			"GONOSUMDB",
			"GONOSUMCHECK",
			LocalPkgEnv,
			ReleaseEnv:
			continue
		default:
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func writeCandidateModule(destination, repoRoot, modulePath, version string) error {
	if version == "" {
		return fmt.Errorf("candidate version for %s is empty", modulePath)
	}
	if repoRoot == "" {
		return fmt.Errorf("repository root is not set")
	}
	sourceDir := filepath.Join(repoRoot, filepath.FromSlash(strings.TrimPrefix(modulePath, "github.com/mbvlabs/andurel/")))
	if _, err := os.Stat(filepath.Join(sourceDir, "go.mod")); err != nil {
		return fmt.Errorf("candidate module source %s: %w", sourceDir, err)
	}

	escaped := modulePath
	versionDir := filepath.Join(destination, filepath.FromSlash(escaped), "@v")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		return fmt.Errorf("create proxy version dir: %w", err)
	}

	goMod, err := os.ReadFile(filepath.Join(sourceDir, "go.mod"))
	if err != nil {
		return fmt.Errorf("read candidate go.mod: %w", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, version+".mod"), goMod, 0o644); err != nil {
		return fmt.Errorf("write candidate go.mod: %w", err)
	}
	info, err := json.Marshal(struct {
		Version string    `json:"Version"`
		Time    time.Time `json:"Time"`
	}{Version: version, Time: time.Now().UTC()})
	if err != nil {
		return fmt.Errorf("encode candidate info: %w", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, version+".info"), info, 0o644); err != nil {
		return fmt.Errorf("write candidate info: %w", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "list"), []byte(version+"\n"), 0o644); err != nil {
		return fmt.Errorf("write candidate list: %w", err)
	}
	if err := writeCandidateZip(
		filepath.Join(versionDir, version+".zip"),
		sourceDir,
		modulePath+"@"+version,
	); err != nil {
		return err
	}
	return nil
}

func writeCandidateZip(destination, sourceDir, prefix string) error {
	file, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("create candidate zip: %w", err)
	}
	defer func() { _ = file.Close() }()

	archive := zip.NewWriter(file)
	walkErr := filepath.WalkDir(sourceDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		entryWriter, err := archive.Create(prefix + "/" + filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		source, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = source.Close() }()
		if _, err := io.Copy(entryWriter, source); err != nil {
			return err
		}
		return nil
	})
	if walkErr != nil {
		_ = archive.Close()
		return fmt.Errorf("zip candidate module: %w", walkErr)
	}
	if err := archive.Close(); err != nil {
		return fmt.Errorf("close candidate zip: %w", err)
	}
	return nil
}
