// Package goldentest is a thin harness for CLI-driven golden tests.
// Determinism belongs in the product (andurel_golden build tag) and fixtures;
// this package never normalizes, scrubs, or reformats captured bytes.
package goldentest

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sebdah/goldie/v2"
)

var (
	andurelBin string
	toolBinDir string
	repoRoot   string
)

// SetBinary records the prebuilt andurel CLI path for RunCLI.
func SetBinary(path string) {
	andurelBin = path
}

// SetToolBinDir records the directory of pinned formatter tools.
// RunCLI sets ANDUREL_TOOL_BIN and prepends it to PATH.
func SetToolBinDir(dir string) {
	toolBinDir = dir
}

// SetRepoRoot records the repository root (directory containing go.mod).
func SetRepoRoot(dir string) {
	repoRoot = dir
}

// Binary returns the prebuilt andurel CLI path.
func Binary() string {
	return andurelBin
}

// ToolBinDir returns the pinned tool bin directory.
func ToolBinDir() string {
	return toolBinDir
}

// RepoRoot returns the repository root.
func RepoRoot() string {
	if repoRoot != "" {
		return repoRoot
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	// internal/goldentest -> repo root
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// FixturesDir returns testdata/fixtures under the repo root.
func FixturesDir() string {
	return filepath.Join(RepoRoot(), "testdata", "fixtures")
}

// GoldenDir returns testdata/golden under the repo root.
func GoldenDir() string {
	return filepath.Join(RepoRoot(), "testdata", "golden")
}

// CopyFixture copies testdata/fixtures/<name> into a fresh temp directory and
// returns the project path.
func CopyFixture(t testing.TB, name string) string {
	t.Helper()

	src := filepath.Join(FixturesDir(), name)
	info, err := os.Stat(src)
	if err != nil {
		t.Fatalf("fixture %q: %v", name, err)
	}
	if !info.IsDir() {
		t.Fatalf("fixture %q is not a directory", name)
	}

	dst := t.TempDir()
	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copy fixture %q: %v", name, err)
	}
	return dst
}

// RunCLI executes the prebuilt andurel binary with args in dir.
// Non-zero exit fails the test and dumps stdout/stderr.
func RunCLI(t testing.TB, dir string, args ...string) {
	t.Helper()

	out, exitCode := RunCLIExit(t, dir, args...)
	if exitCode != 0 {
		t.Fatalf("andurel %v exited %d\n%s", args, exitCode, out)
	}
}

// RunCLIExit runs the CLI and returns combined stdout/stderr plus the process
// exit code. Output bytes are returned raw — never scrubbed or reformatted.
// Non-exit failures (missing binary, start error) fail the test.
func RunCLIExit(t testing.TB, dir string, args ...string) (output []byte, exitCode int) {
	t.Helper()

	if andurelBin == "" {
		t.Fatal("andurel binary not set; call goldentest.SetBinary from TestMain")
	}

	cmd := exec.Command(andurelBin, args...)
	cmd.Dir = dir
	cmd.Env = cliEnv()

	out, err := cmd.CombinedOutput()
	if err == nil {
		return out, 0
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return out, exitErr.ExitCode()
	}
	t.Fatalf("andurel %v failed to start: %v\n%s", args, err, out)
	return nil, -1
}

// AssertFileBytesEquals fails if projectDir/relPath bytes differ from want.
func AssertFileBytesEquals(t testing.TB, projectDir, relPath string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(relPath)))
	if err != nil {
		t.Fatalf("read %s: %v", relPath, err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s changed unexpectedly\nwant %d bytes, got %d bytes", relPath, len(want), len(got))
	}
}

// NewGoldie returns a goldie assertor rooted at testdata/golden.
func NewGoldie(t testing.TB) *goldie.Goldie {
	t.Helper()
	return goldie.New(t, goldie.WithFixtureDir(GoldenDir()))
}

// AssertFile reads projectDir/relPath and goldie-asserts the raw bytes under
// goldenName (slash-separated path under testdata/golden).
func AssertFile(t testing.TB, g *goldie.Goldie, goldenName, projectDir, relPath string) {
	t.Helper()

	got, err := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(relPath)))
	if err != nil {
		t.Fatalf("read %s: %v", relPath, err)
	}
	g.Assert(t, goldenName, got)
}

// AssertFiles asserts each relPath under goldenPrefix/<relPath>.
func AssertFiles(t testing.TB, g *goldie.Goldie, goldenPrefix, projectDir string, relPaths []string) {
	t.Helper()
	for _, relPath := range relPaths {
		goldenName := filepath.ToSlash(filepath.Join(goldenPrefix, filepath.FromSlash(relPath)))
		AssertFile(t, g, goldenName, projectDir, relPath)
	}
}

// AssertMissing fails if projectDir/relPath exists.
func AssertMissing(t testing.TB, projectDir, relPath string) {
	t.Helper()
	path := filepath.Join(projectDir, filepath.FromSlash(relPath))
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected %s not to exist", relPath)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", relPath, err)
	}
}

// CopyMigrations copies SQL files from generator/testdata/migrations/<name>
// into projectDir/migrations/.
func CopyMigrations(t testing.TB, projectDir, name string) {
	t.Helper()

	src := filepath.Join(RepoRoot(), "generator", "testdata", "migrations", name)
	info, err := os.Stat(src)
	if err != nil {
		t.Fatalf("migrations %q: %v", name, err)
	}
	if !info.IsDir() {
		t.Fatalf("migrations %q is not a directory", name)
	}

	dst := filepath.Join(projectDir, "migrations")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("mkdir migrations: %v", err)
	}
	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copy migrations %q: %v", name, err)
	}
}

func cliEnv() []string {
	env := os.Environ()
	if toolBinDir == "" {
		return env
	}

	path := toolBinDir
	if existing := os.Getenv("PATH"); existing != "" {
		path = toolBinDir + string(os.PathListSeparator) + existing
	}

	filtered := make([]string, 0, len(env)+2)
	for _, entry := range env {
		switch {
		case strings.HasPrefix(entry, "PATH="):
			continue
		case strings.HasPrefix(entry, "ANDUREL_TOOL_BIN="):
			continue
		}
		filtered = append(filtered, entry)
	}
	filtered = append(filtered,
		"PATH="+path,
		"ANDUREL_TOOL_BIN="+toolBinDir,
	)
	return filtered
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

// RequireBinary fails fast if TestMain did not install the CLI.
func RequireBinary(t testing.TB) {
	t.Helper()
	if andurelBin == "" {
		t.Fatal("andurel binary not set")
	}
	if _, err := os.Stat(andurelBin); err != nil {
		t.Fatalf("andurel binary missing: %v", err)
	}
}

// BuildTaggedCLI builds andurel with -tags andurel_golden into dir/andurel.
// Intended for TestMain only (humans/CI run go test; agents must not).
func BuildTaggedCLI(projectRoot, outDir string) (binPath string, err error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	binPath = filepath.Join(outDir, "andurel")
	cmd := exec.Command("go", "build", "-tags", "andurel_golden", "-o", binPath, ".")
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go build -tags andurel_golden: %w\n%s", err, out)
	}
	return binPath, nil
}

// InstallPinnedTools installs templ/golines/goimports at the versions from
// layout/versions into gobin.
func InstallPinnedTools(gobin string, tools []struct {
	Module  string
	Version string
}) error {
	if err := os.MkdirAll(gobin, 0o755); err != nil {
		return err
	}
	for _, tool := range tools {
		cmd := exec.Command("go", "install", tool.Module+"@"+tool.Version)
		cmd.Env = append(os.Environ(), "GOBIN="+gobin)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("go install %s@%s: %w\n%s", tool.Module, tool.Version, err, out)
		}
	}
	return nil
}
