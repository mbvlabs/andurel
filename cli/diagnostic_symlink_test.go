package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagnosticCopyDereferencesExternalRelativeSymlinks(t *testing.T) {
	parent := t.TempDir()
	project := filepath.Join(parent, "project")
	shared := filepath.Join(parent, "shared")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, shared, "views/page.templ", "package views\n")
	writeTestFile(t, shared, "settings.txt", "external settings\n")
	if err := os.Symlink(filepath.Join("..", "shared", "views"), filepath.Join(project, "views")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "shared", "settings.txt"), filepath.Join(project, "settings.txt")); err != nil {
		t.Fatal(err)
	}

	before := snapshotAllTestFiles(t, shared)
	if err := withDiagnosticProjectCopy(project, func(copyRoot string) error {
		viewsInfo, err := os.Lstat(filepath.Join(copyRoot, "views"))
		if err != nil {
			return err
		}
		if viewsInfo.Mode()&os.ModeSymlink != 0 || !viewsInfo.IsDir() {
			t.Fatalf("copied views mode = %s, want real directory", viewsInfo.Mode())
		}
		settingsInfo, err := os.Lstat(filepath.Join(copyRoot, "settings.txt"))
		if err != nil {
			return err
		}
		if !settingsInfo.Mode().IsRegular() {
			t.Fatalf("copied settings mode = %s, want regular file", settingsInfo.Mode())
		}
		content, err := os.ReadFile(filepath.Join(copyRoot, "views", "page.templ"))
		if err != nil {
			return err
		}
		if !bytes.Equal(content, []byte("package views\n")) {
			t.Fatalf("copied template = %q", content)
		}
		return os.WriteFile(filepath.Join(copyRoot, "views", "page.templ"), []byte("changed copy\n"), 0o644)
	}); err != nil {
		t.Fatal(err)
	}
	if after := snapshotAllTestFiles(t, shared); !mapsOfBytesEqual(before, after) {
		t.Fatalf("diagnostic copy mutation reached shared target\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestDiagnosticCopyDereferencesAbsoluteSymlinkAndSkipsLinkedExclusions(t *testing.T) {
	project := t.TempDir()
	external := t.TempDir()
	writeTestFile(t, external, "data/value.txt", "value\n")
	writeTestFile(t, external, "node_modules/private.txt", "excluded\n")
	if err := os.Symlink(filepath.Join(external, "data"), filepath.Join(project, "data")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(external, "node_modules"), filepath.Join(project, "dependencies")); err != nil {
		t.Fatal(err)
	}
	if err := withDiagnosticProjectCopy(project, func(copyRoot string) error {
		if _, err := os.Stat(filepath.Join(copyRoot, "data", "value.txt")); err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(copyRoot, "dependencies")); !os.IsNotExist(err) {
			t.Fatalf("linked excluded directory stat error = %v, want not exist", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestTemplFormatCheckInspectsExternalSymlinkContentWithoutMutatingIt(t *testing.T) {
	parent := t.TempDir()
	project := filepath.Join(parent, "project")
	shared := filepath.Join(parent, "shared")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, shared, "page.templ", "package views\n")
	writeExecutable(t, project, "bin/templ", "#!/bin/sh\nprintf '// formatted copy\\n' >> \"$2/page.templ\"\n")
	if err := os.Symlink(filepath.Join("..", "shared"), filepath.Join(project, "views")); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(shared, "page.templ"))
	if err != nil {
		t.Fatal(err)
	}
	err = runTemplFmt(project, true)
	if err == nil || !strings.Contains(err.Error(), "views/page.templ") {
		t.Fatalf("templ format check = %v", err)
	}
	after, err := os.ReadFile(filepath.Join(shared, "page.templ"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("templ format check changed external target\nbefore=%q\nafter=%q", before, after)
	}
}

func TestDiagnosticCopyRejectsBrokenLinksAndCycles(t *testing.T) {
	t.Run("broken link", func(t *testing.T) {
		project := t.TempDir()
		if err := os.Symlink("missing-target", filepath.Join(project, "broken")); err != nil {
			t.Fatal(err)
		}
		err := copyDiagnosticProject(project, filepath.Join(t.TempDir(), "copy"))
		if err == nil || !strings.Contains(err.Error(), "broken") {
			t.Fatalf("broken link error = %v", err)
		}
	})

	t.Run("directory cycle", func(t *testing.T) {
		project := t.TempDir()
		writeTestFile(t, project, "nested/value.txt", "value\n")
		if err := os.Symlink("..", filepath.Join(project, "nested", "back")); err != nil {
			t.Fatal(err)
		}
		err := copyDiagnosticProject(project, filepath.Join(t.TempDir(), "copy"))
		if err == nil || !strings.Contains(err.Error(), "cycle") {
			t.Fatalf("cycle error = %v", err)
		}
	})
}

func mapsOfBytesEqual(left, right map[string][]byte) bool {
	if len(left) != len(right) {
		return false
	}
	for path, content := range left {
		if !bytes.Equal(content, right[path]) {
			return false
		}
	}
	return true
}
