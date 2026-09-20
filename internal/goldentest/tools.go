package goldentest

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/layout/cmds"
)

// SeedProjectTools copies pinned binaries from ToolBinDir into projectDir/bin,
// overwriting fixture stubs so CLI lookups of bin/<tool> stay offline.
func SeedProjectTools(t testing.TB, projectDir string) {
	t.Helper()
	if toolBinDir == "" {
		return
	}
	if err := seedProjectTools(projectDir, toolBinDir); err != nil {
		t.Fatalf("seed project tools: %v", err)
	}
}

func seedProjectTools(projectDir, srcDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	dstDir := filepath.Join(projectDir, "bin")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		src := filepath.Join(srcDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if err := copyFile(src, filepath.Join(dstDir, entry.Name()), info.Mode()); err != nil {
			return err
		}
	}
	return nil
}

// InstallLockTools downloads lock-managed binaries (narsilc, tailwindcli, goose)
// into destDir using the same URL templates and SHA-256 digests as andurel.lock.
func InstallLockTools(destDir string, names []string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	platform := goos + "/" + goarch
	expected := layout.GetExpectedTools(nil)
	for _, name := range names {
		tool, ok := expected[name]
		if !ok || tool == nil {
			return fmt.Errorf("unknown lock tool %q", name)
		}
		if tool.Download == nil || tool.Download.URLTemplate == "" {
			return fmt.Errorf("lock tool %q has no download spec", name)
		}
		digest, ok := tool.Download.SHA256[platform]
		if !ok || digest == "" {
			return fmt.Errorf("lock tool %q has no SHA-256 for %s", name, platform)
		}
		dest := filepath.Join(destDir, name)
		if err := cmds.DownloadVerifiedFromURLTemplate(
			name,
			tool.Version,
			tool.Download.URLTemplate,
			tool.Download.Archive,
			tool.Download.BinaryName,
			goos,
			goarch,
			dest,
			digest,
		); err != nil {
			return fmt.Errorf("download %s: %w", name, err)
		}
		_ = os.Chmod(dest, 0o755)
	}
	return nil
}

// RequireFullScaffold skips the test unless ANDUREL_GOLDEN_FULL=1.
func RequireFullScaffold(t testing.TB) {
	t.Helper()
	if os.Getenv("ANDUREL_GOLDEN_FULL") != "1" {
		t.Skip("full scaffold goldens require ANDUREL_GOLDEN_FULL=1")
	}
}

// WriteProjectFile writes path relative to projectDir, creating parents.
func WriteProjectFile(t testing.TB, projectDir, relPath, contents string) {
	t.Helper()
	path := filepath.Join(projectDir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", relPath, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
}
