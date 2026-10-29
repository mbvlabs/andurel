package goldentest

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/layout/versions"
)

func TestGoldenModeFromEnv(t *testing.T) {
	tests := []struct {
		name string
		env  []string
		want GoldenMode
	}{
		{name: "default", want: GoldenModePublished},
		{name: "explicit zero", env: []string{LocalPkgEnv + "=0", ReleaseEnv + "=0"}, want: GoldenModePublished},
		{name: "local pkg", env: []string{LocalPkgEnv + "=1"}, want: GoldenModeLocalPkg},
		{name: "release", env: []string{ReleaseEnv + "=1"}, want: GoldenModeRelease},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := GoldenModeFromEnv(test.env)
			if err != nil {
				t.Fatalf("GoldenModeFromEnv: %v", err)
			}
			if got != test.want {
				t.Fatalf("mode = %s, want %s", got, test.want)
			}
		})
	}
}

func TestGoldenModeFromEnvRejectsConflictsAndInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		env  []string
	}{
		{name: "both modes", env: []string{LocalPkgEnv + "=1", ReleaseEnv + "=1"}},
		{name: "invalid local value", env: []string{LocalPkgEnv + "=yes"}},
		{name: "invalid release value", env: []string{ReleaseEnv + "=true"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := GoldenModeFromEnv(test.env); err == nil {
				t.Fatal("GoldenModeFromEnv succeeded, want error")
			}
		})
	}
}

func TestGoldenGoEnvPublishedUsesPublicProxy(t *testing.T) {
	cleanupGoldenEnv(t)

	env, err := GoldenGoEnv([]string{
		"PATH=/bin",
		"GOWORK=/tmp/workspace",
		"GOPROXY=file:///tmp/stale-proxy",
		"GOSUMDB=off",
		"GOPRIVATE=github.com/mbvlabs/*",
		"GONOSUMDB=github.com/mbvlabs/*",
		"GOMODCACHE=/tmp/stale-modcache",
		LocalPkgEnv + "=0",
	})
	if err != nil {
		t.Fatalf("GoldenGoEnv: %v", err)
	}
	assertSingleEnv(t, env, "GOWORK", "off")
	assertSingleEnv(t, env, "GOPROXY", publicGoProxy)
	assertSingleEnv(t, env, "GOSUMDB", publicGoSumDB)
	assertNoEnv(t, env, "GOPRIVATE")
	assertNoEnv(t, env, "GONOSUMDB")
	assertNoEnv(t, env, "GOMODCACHE")
	assertNoEnv(t, env, "GOCACHE")
	assertNoEnv(t, env, "GOTMPDIR")
	assertNoEnv(t, env, "GOENV")
	assertNoEnv(t, env, LocalPkgEnv)
	assertNoEnv(t, env, ReleaseEnv)
	for _, entry := range env {
		if strings.HasPrefix(entry, "GOPROXY=file://") {
			t.Fatalf("published mode injected a file proxy: %s", entry)
		}
	}
}

func TestGoldenGoEnvLocalPkgServesCheckout(t *testing.T) {
	root := useFakeRepoRoot(t)
	cleanupGoldenEnv(t)

	env, err := GoldenGoEnv([]string{
		"PATH=/bin",
		"GOPROXY=file:///tmp/stale-proxy",
		"GOSUMDB=off",
		LocalPkgEnv + "=1",
	})
	if err != nil {
		t.Fatalf("GoldenGoEnv: %v", err)
	}

	assertSingleEnv(t, env, "GOWORK", "off")
	assertSingleEnv(t, env, "GOSUMDB", "off")
	assertSingleEnv(t, env, "GOENV", "off")
	proxy := envValue(t, env, "GOPROXY")
	fileProxy, publicProxy, ok := strings.Cut(proxy, ",")
	if !ok || !strings.HasPrefix(fileProxy, "file://") {
		t.Fatalf("GOPROXY = %q, want a file proxy followed by the public proxy", proxy)
	}
	if publicProxy != publicGoProxy {
		t.Fatalf("public fallback = %q, want %q", publicProxy, publicGoProxy)
	}
	proxyDir := strings.TrimPrefix(fileProxy, "file://")
	if !strings.HasPrefix(proxyDir, filepath.ToSlash(os.TempDir())) {
		t.Fatalf("proxy dir %q is not under the temp dir", proxyDir)
	}
	if strings.Contains(proxyDir, filepath.ToSlash(root)) {
		t.Fatalf("proxy dir %q must be owned and temporary, not the checkout", proxyDir)
	}

	assertOwnedCache(t, env, "GOMODCACHE")
	assertOwnedCache(t, env, "GOCACHE")
	assertOwnedCache(t, env, "GOTMPDIR")

	versionDir := filepath.Join(
		filepath.FromSlash(proxyDir),
		filepath.FromSlash(CandidateInertiaModulePath),
		"@v",
	)
	for _, name := range []string{versions.Inertia + ".mod", versions.Inertia + ".info", versions.Inertia + ".zip", "list"} {
		if _, err := os.Stat(filepath.Join(versionDir, name)); err != nil {
			t.Fatalf("local proxy missing %s: %v", name, err)
		}
	}

	archive, err := zip.OpenReader(filepath.Join(versionDir, versions.Inertia+".zip"))
	if err != nil {
		t.Fatalf("open local proxy zip: %v", err)
	}
	defer func() { _ = archive.Close() }()
	wantPrefix := CandidateInertiaModulePath + "@" + versions.Inertia + "/go.mod"
	found := false
	for _, file := range archive.File {
		if file.Name == wantPrefix {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("local proxy zip does not contain %s", wantPrefix)
	}
}

func TestGoldenGoEnvReleaseForcesPublicResolution(t *testing.T) {
	cleanupGoldenEnv(t)

	env, err := GoldenGoEnv([]string{
		"PATH=/bin",
		"GOPROXY=file:///tmp/stale-proxy",
		"GOSUMDB=off",
		"GOPRIVATE=github.com/mbvlabs/*",
		ReleaseEnv + "=1",
	})
	if err != nil {
		t.Fatalf("GoldenGoEnv: %v", err)
	}

	assertSingleEnv(t, env, "GOWORK", "off")
	assertSingleEnv(t, env, "GOPROXY", publicGoProxy)
	assertSingleEnv(t, env, "GOSUMDB", publicGoSumDB)
	assertSingleEnv(t, env, "GOENV", "off")
	assertNoEnv(t, env, "GOPRIVATE")
	assertNoEnv(t, env, ReleaseEnv)
	for _, entry := range env {
		if strings.HasPrefix(entry, "GOPROXY=file://") {
			t.Fatalf("release mode injected a file proxy: %s", entry)
		}
	}

	modCache := assertOwnedCache(t, env, "GOMODCACHE")
	assertOwnedCache(t, env, "GOCACHE")
	assertOwnedCache(t, env, "GOTMPDIR")

	entries, err := os.ReadDir(modCache)
	if err != nil {
		t.Fatalf("read release module cache: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("release module cache is not fresh: %d entries", len(entries))
	}
}

func TestGoldenGoEnvRejectsModeSwitchUntilCleanup(t *testing.T) {
	useFakeRepoRoot(t)
	cleanupGoldenEnv(t)

	if _, err := GoldenGoEnv([]string{LocalPkgEnv + "=1"}); err != nil {
		t.Fatalf("local mode: %v", err)
	}
	if _, err := GoldenGoEnv([]string{ReleaseEnv + "=1"}); err == nil {
		t.Fatal("switching modes without cleanup succeeded, want error")
	}

	CleanupGoldenGoEnv()
	if _, err := GoldenGoEnv([]string{ReleaseEnv + "=1"}); err != nil {
		t.Fatalf("release mode after cleanup: %v", err)
	}
}

func TestCleanupGoldenGoEnvRemovesOwnedDirectories(t *testing.T) {
	useFakeRepoRoot(t)
	cleanupGoldenEnv(t)

	env, err := GoldenGoEnv([]string{LocalPkgEnv + "=1"})
	if err != nil {
		t.Fatalf("GoldenGoEnv: %v", err)
	}
	proxyDir := envValue(t, env, "GOPROXY")
	proxyDir, _, ok := strings.Cut(proxyDir, ",")
	if !ok {
		t.Fatalf("GOPROXY has no public fallback: %q", proxyDir)
	}
	proxyDir = strings.TrimPrefix(proxyDir, "file://")
	modCache := envValue(t, env, "GOMODCACHE")

	CleanupGoldenGoEnv()
	for _, dir := range []string{proxyDir, modCache} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("%s still exists after cleanup: %v", dir, err)
		}
	}
	CleanupGoldenGoEnv()
}

func TestCleanupGoldenGoEnvRemovesReleaseCaches(t *testing.T) {
	cleanupGoldenEnv(t)

	env, err := GoldenGoEnv([]string{ReleaseEnv + "=1"})
	if err != nil {
		t.Fatalf("GoldenGoEnv: %v", err)
	}
	modCache := envValue(t, env, "GOMODCACHE")
	buildCache := envValue(t, env, "GOCACHE")

	CleanupGoldenGoEnv()
	for _, dir := range []string{modCache, buildCache} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("%s still exists after cleanup: %v", dir, err)
		}
	}
}

func TestGoldenGoEnvLocalPkgRequiresModuleSource(t *testing.T) {
	oldRoot := repoRoot
	SetRepoRoot(t.TempDir())
	t.Cleanup(func() {
		SetRepoRoot(oldRoot)
		CleanupGoldenGoEnv()
	})
	cleanupGoldenEnv(t)

	if _, err := GoldenGoEnv([]string{LocalPkgEnv + "=1"}); err == nil {
		t.Fatal("GoldenGoEnv succeeded without pkg/inertia source, want error")
	}
}

func cleanupGoldenEnv(t *testing.T) {
	t.Helper()
	CleanupGoldenGoEnv()
	t.Cleanup(CleanupGoldenGoEnv)
}

func useFakeRepoRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	moduleDir := filepath.Join(root, "pkg", "inertia")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("create fake module dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(moduleDir, "go.mod"),
		[]byte("module "+CandidateInertiaModulePath+"\n\ngo 1.27.3\n"),
		0o644,
	); err != nil {
		t.Fatalf("write fake go.mod: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(moduleDir, "renderer.go"),
		[]byte("package inertia\n"),
		0o644,
	); err != nil {
		t.Fatalf("write fake source: %v", err)
	}

	oldRoot := repoRoot
	SetRepoRoot(root)
	t.Cleanup(func() { SetRepoRoot(oldRoot) })
	return root
}

func assertSingleEnv(t *testing.T, env []string, key, want string) {
	t.Helper()
	values := envValues(env, key)
	if len(values) != 1 {
		t.Fatalf("%s has %d values, want 1: %v", key, len(values), values)
	}
	if values[0] != want {
		t.Fatalf("%s = %q, want %q", key, values[0], want)
	}
}

func assertNoEnv(t *testing.T, env []string, key string) {
	t.Helper()
	if values := envValues(env, key); len(values) != 0 {
		t.Fatalf("%s is set, want absent: %v", key, values)
	}
}

func assertOwnedCache(t *testing.T, env []string, key string) string {
	t.Helper()
	value := envValue(t, env, key)
	if !strings.HasPrefix(value, filepath.ToSlash(os.TempDir())) {
		t.Fatalf("%s = %q, want a directory under the temp dir", key, value)
	}
	info, err := os.Stat(value)
	if err != nil {
		t.Fatalf("%s dir: %v", key, err)
	}
	if !info.IsDir() {
		t.Fatalf("%s is not a directory: %s", key, value)
	}
	return value
}

func envValue(t *testing.T, env []string, key string) string {
	t.Helper()
	values := envValues(env, key)
	if len(values) != 1 {
		t.Fatalf("%s has %d values, want 1: %v", key, len(values), values)
	}
	return values[0]
}

func envValues(env []string, key string) []string {
	var values []string
	for _, entry := range env {
		name, value, ok := strings.Cut(entry, "=")
		if ok && name == key {
			values = append(values, value)
		}
	}
	return values
}
