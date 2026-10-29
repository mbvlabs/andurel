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

// CandidateInertiaModulePath is the standalone module whose candidate pin may
// be ahead of the published tags. Golden scaffolds tidy with GOWORK=off, so the
// candidate version must be resolvable.
const CandidateInertiaModulePath = "github.com/mbvlabs/andurel/pkg/inertia"

var (
	candidateProxyOnce sync.Once
	candidateProxyDir  string
	candidateProxyErr  error
)

// CandidateModuleProxy serves this checkout's pkg/inertia source as the
// candidate version pinned in layout/versions through a file-based Go module
// proxy. Development golden scaffolds can then run go mod tidy against
// unreleased candidate APIs without a committed replace directive or a
// machine-specific path.
//
// The proxy also owns dedicated GOMODCACHE, GOCACHE, and GOTMPDIR directories.
// Candidate code must never enter the user's global module or build cache: a
// later published pkg/inertia v0.13.0 has to be fetched and verified from the
// real proxy, not silently satisfied by cached bytes from this checkout.
//
// This is development integration, not release-shaped proof: it deliberately
// substitutes local code for the pinned version. The release gate still
// requires pkg/inertia/v0.13.0 to be tagged and published, and the durable
// acceptance harness runs release mode with no proxy override.
func CandidateModuleProxy() (string, error) {
	candidateProxyOnce.Do(func() {
		repoRoot := RepoRoot()
		if repoRoot == "" {
			candidateProxyErr = fmt.Errorf("repository root is not set")
			return
		}
		dir, err := os.MkdirTemp("", "andurel-candidate-proxy-*")
		if err != nil {
			candidateProxyErr = fmt.Errorf("create candidate proxy dir: %w", err)
			return
		}
		candidateProxyDir = dir
		candidateProxyErr = writeCandidateModule(dir, repoRoot, CandidateInertiaModulePath, versions.Inertia)
	})
	return candidateProxyDir, candidateProxyErr
}

// CleanupCandidateModuleProxy removes the temporary proxy and its isolated Go
// caches. It is safe to call more than once.
func CleanupCandidateModuleProxy() {
	if candidateProxyDir == "" {
		return
	}
	_ = os.RemoveAll(candidateProxyDir)
	candidateProxyDir = ""
}

// CandidateGoEnv returns env with the candidate module proxy in front of the
// public Go proxy, a dedicated module/build cache under the proxy directory,
// and checksum verification disabled for the local file proxy. It returns an
// error instead of silently falling back to the ambient environment: a test
// that cannot build the candidate proxy must fail, not run against a different
// module set.
func CandidateGoEnv(env []string) ([]string, error) {
	proxyDir, err := CandidateModuleProxy()
	if err != nil {
		return nil, err
	}
	moduleCache := filepath.Join(proxyDir, "modcache")
	buildCache := filepath.Join(proxyDir, "gocache")
	tempDir := filepath.Join(proxyDir, "tmp")
	for _, dir := range []string{moduleCache, buildCache, tempDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create candidate Go cache %s: %w", dir, err)
		}
	}

	proxyURL := "file://" + filepath.ToSlash(proxyDir)
	filtered := make([]string, 0, len(env)+8)
	for _, entry := range env {
		switch {
		case strings.HasPrefix(entry, "GOPROXY="),
			strings.HasPrefix(entry, "GOSUMDB="),
			strings.HasPrefix(entry, "GOMODCACHE="),
			strings.HasPrefix(entry, "GOCACHE="),
			strings.HasPrefix(entry, "GOTMPDIR="),
			strings.HasPrefix(entry, "GOENV="),
			strings.HasPrefix(entry, "GONOSUMDB="),
			strings.HasPrefix(entry, "GONOSUMCHECK="):
			continue
		}
		filtered = append(filtered, entry)
	}
	return append(
		filtered,
		"GOPROXY="+proxyURL+",https://proxy.golang.org,direct",
		"GOSUMDB=off",
		"GOMODCACHE="+moduleCache,
		"GOCACHE="+buildCache,
		"GOTMPDIR="+tempDir,
		// Ignore a user-level go env file so proxy and cache isolation cannot be
		// overridden by the developer's machine configuration.
		"GOENV=off",
	), nil
}

func writeCandidateModule(destination, repoRoot, modulePath, version string) error {
	if version == "" {
		return fmt.Errorf("candidate version for %s is empty", modulePath)
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
