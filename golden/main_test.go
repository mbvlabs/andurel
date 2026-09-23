package golden

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mbvlabs/andurel/v2/internal/goldentest"
	"github.com/mbvlabs/andurel/v2/layout/versions"
)

func TestMain(m *testing.M) {
	root, err := findRepoRoot()
	if err != nil {
		panic(err)
	}
	goldentest.SetRepoRoot(root)

	tmpDir, err := os.MkdirTemp("", "andurel-golden-*")
	if err != nil {
		panic(fmt.Sprintf("create temp dir: %v", err))
	}

	bin, err := goldentest.BuildTaggedCLI(root, tmpDir)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		panic(err)
	}
	goldentest.SetBinary(bin)

	toolBin := filepath.Join(tmpDir, "bin")
	if err := goldentest.InstallPinnedTools(toolBin, []struct {
		Module  string
		Version string
	}{
		{"github.com/a-h/templ/cmd/templ", versions.Templ},
		{"github.com/segmentio/golines", versions.Golines},
		{"golang.org/x/tools/cmd/goimports", versions.Goimports},
	}); err != nil {
		_ = os.RemoveAll(tmpDir)
		panic(err)
	}
	// Lock-managed binaries (same URL templates + digests as andurel.lock).
	if err := goldentest.InstallLockTools(toolBin, []string{
		"narsilc",
		"tailwindcli",
		"goose",
	}); err != nil {
		_ = os.RemoveAll(tmpDir)
		panic(err)
	}
	goldentest.SetToolBinDir(toolBin)

	code := m.Run()
	_ = os.RemoveAll(tmpDir)
	os.Exit(code)
}

func findRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", wd)
		}
		dir = parent
	}
}
