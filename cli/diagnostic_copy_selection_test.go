package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagnosticCopyToolsCopiesOnlySelectedBinaries(t *testing.T) {
	project := t.TempDir()
	writeTestFile(t, project, "go.mod", "module example.com/app\n")
	writeTestFile(t, project, "resources/root.templ", "package resources\n")
	writeExecutable(t, project, "bin/templ", "#!/bin/sh\necho templ\n")
	writeExecutable(t, project, "bin/tailwindcli", "#!/bin/sh\necho tailwind\n")
	writeExecutable(t, project, "bin/narsilc", "#!/bin/sh\necho narsilc\n")
	writeExecutable(t, project, "bin/usql", "#!/bin/sh\necho usql\n")
	if err := os.Chmod(filepath.Join(project, "bin"), 0o750); err != nil {
		t.Fatal(err)
	}

	err := withDiagnosticProjectCopyTools(
		project,
		[]string{"templ", "tailwindcli"},
		func(copyRoot string) error {
			binInfo, err := os.Lstat(filepath.Join(copyRoot, "bin"))
			if err != nil {
				return err
			}
			if binInfo.Mode()&os.ModeSymlink != 0 || !binInfo.IsDir() {
				t.Fatalf("copied bin mode = %s, want independent directory", binInfo.Mode())
			}
			if got := binInfo.Mode().Perm(); got != 0o750 {
				t.Fatalf("copied bin permissions = %o, want 750", got)
			}
			for _, name := range []string{"templ", "tailwindcli"} {
				copiedPath := filepath.Join(copyRoot, "bin", name)
				info, err := os.Lstat(copiedPath)
				if err != nil {
					return err
				}
				if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
					t.Fatalf("copied %s mode = %s, want regular file", name, info.Mode())
				}
				if got := info.Mode().Perm(); got != 0o755 {
					t.Fatalf("copied %s permissions = %o, want 755", name, got)
				}
				got, err := os.ReadFile(copiedPath)
				if err != nil {
					return err
				}
				want, err := os.ReadFile(filepath.Join(project, "bin", name))
				if err != nil {
					return err
				}
				if !bytes.Equal(got, want) {
					t.Fatalf("copied %s content = %q, want %q", name, got, want)
				}
			}
			for _, name := range []string{"narsilc", "usql"} {
				if _, err := os.Lstat(filepath.Join(copyRoot, "bin", name)); !os.IsNotExist(err) {
					t.Fatalf("unselected %s stat error = %v, want not exist", name, err)
				}
			}
			if _, err := os.Stat(filepath.Join(copyRoot, "resources", "root.templ")); err != nil {
				t.Fatalf("non-bin project file was not copied: %v", err)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDiagnosticCopyToolsWithoutToolsOmitsBin(t *testing.T) {
	project := t.TempDir()
	writeTestFile(t, project, "go.mod", "module example.com/app\n")
	writeTestFile(t, project, "routes/pages.go", "package routes\n")
	if err := os.MkdirAll(filepath.Join(project, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing-tool", filepath.Join(project, "bin", "unused")); err != nil {
		t.Fatal(err)
	}

	for _, tools := range [][]string{nil, {}} {
		err := withDiagnosticProjectCopyTools(project, tools, func(copyRoot string) error {
			if _, err := os.Lstat(filepath.Join(copyRoot, "bin")); !os.IsNotExist(err) {
				t.Fatalf("copied bin stat error = %v, want not exist", err)
			}
			if _, err := os.Stat(filepath.Join(copyRoot, "routes", "pages.go")); err != nil {
				t.Fatalf("non-bin project file was not copied: %v", err)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("copy without tools %v: %v", tools, err)
		}
	}
}

func TestDiagnosticCopyToolsMutationsDoNotReachOriginalTools(t *testing.T) {
	parent := t.TempDir()
	project := filepath.Join(parent, "project")
	external := filepath.Join(parent, "external")
	if err := os.MkdirAll(filepath.Join(project, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeExecutable(t, external, "templ", "#!/bin/sh\necho external templ\n")
	templTarget := filepath.Join(external, "templ")
	if err := os.Symlink(templTarget, filepath.Join(project, "bin", "templ")); err != nil {
		t.Fatal(err)
	}
	writeExecutable(t, project, "bin/tailwindcli", "#!/bin/sh\necho project tailwind\n")

	before, err := os.ReadFile(templTarget)
	if err != nil {
		t.Fatal(err)
	}
	err = withDiagnosticProjectCopyTools(
		project,
		[]string{"templ", "tailwindcli"},
		func(copyRoot string) error {
			copiedTempl := filepath.Join(copyRoot, "bin", "templ")
			info, err := os.Lstat(copiedTempl)
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
				t.Fatalf("copied templ mode = %s, want independent regular file", info.Mode())
			}
			if err := os.WriteFile(copiedTempl, []byte("changed copy\n"), 0o755); err != nil {
				return err
			}
			return os.WriteFile(
				filepath.Join(copyRoot, "bin", "tailwindcli"),
				[]byte("changed tailwind copy\n"),
				0o755,
			)
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	after, err := os.ReadFile(templTarget)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("mutation reached original tool\nbefore=%q\nafter=%q", before, after)
	}
	originalTailwind, err := os.ReadFile(filepath.Join(project, "bin", "tailwindcli"))
	if err != nil {
		t.Fatal(err)
	}
	if string(originalTailwind) != "#!/bin/sh\necho project tailwind\n" {
		t.Fatalf("original tailwind changed: %q", originalTailwind)
	}
}

func TestDiagnosticCopyToolsDereferencesSymlinkedBin(t *testing.T) {
	parent := t.TempDir()
	project := filepath.Join(parent, "project")
	externalBin := filepath.Join(parent, "external", "bin")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	writeExecutable(t, externalBin, "templ", "#!/bin/sh\necho external templ\n")
	writeExecutable(t, externalBin, "unused", "#!/bin/sh\necho external unused\n")
	if err := os.Symlink(externalBin, filepath.Join(project, "bin")); err != nil {
		t.Fatal(err)
	}

	before := snapshotAllTestFiles(t, externalBin)
	err := withDiagnosticProjectCopyTools(project, []string{"templ"}, func(copyRoot string) error {
		copiedBin := filepath.Join(copyRoot, "bin")
		info, err := os.Lstat(copiedBin)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			t.Fatalf("copied bin mode = %s, want independent directory", info.Mode())
		}
		templInfo, err := os.Lstat(filepath.Join(copiedBin, "templ"))
		if err != nil {
			return err
		}
		if templInfo.Mode()&os.ModeSymlink != 0 || !templInfo.Mode().IsRegular() {
			t.Fatalf("copied templ mode = %s, want regular file", templInfo.Mode())
		}
		if _, err := os.Lstat(filepath.Join(copiedBin, "unused")); !os.IsNotExist(err) {
			t.Fatalf("unselected external tool stat error = %v, want not exist", err)
		}
		return os.WriteFile(filepath.Join(copiedBin, "templ"), []byte("changed\n"), 0o755)
	})
	if err != nil {
		t.Fatal(err)
	}
	if after := snapshotAllTestFiles(t, externalBin); !mapsOfBytesEqual(before, after) {
		t.Fatalf("mutation reached symlinked bin target\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestDiagnosticCopyToolsSkipsUnselectedBrokenLinks(t *testing.T) {
	project := t.TempDir()
	writeExecutable(t, project, "bin/templ", "#!/bin/sh\nexit 0\n")
	if err := os.Symlink("missing-target", filepath.Join(project, "bin", "unused")); err != nil {
		t.Fatal(err)
	}

	err := withDiagnosticProjectCopyTools(project, []string{"templ"}, func(copyRoot string) error {
		if _, err := os.Stat(filepath.Join(copyRoot, "bin", "templ")); err != nil {
			return err
		}
		if _, err := os.Lstat(filepath.Join(copyRoot, "bin", "unused")); !os.IsNotExist(err) {
			t.Fatalf("unselected broken link stat error = %v, want not exist", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDiagnosticCopyToolsRejectsBrokenSelectedLinksAndCycles(t *testing.T) {
	t.Run("broken selected link", func(t *testing.T) {
		project := t.TempDir()
		if err := os.MkdirAll(filepath.Join(project, "bin"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("missing-target", filepath.Join(project, "bin", "templ")); err != nil {
			t.Fatal(err)
		}
		err := withDiagnosticProjectCopyTools(project, []string{"templ"}, func(string) error {
			return nil
		})
		if err == nil || !strings.Contains(err.Error(), "templ") {
			t.Fatalf("broken selected link error = %v", err)
		}
	})

	t.Run("selected tool cycle", func(t *testing.T) {
		project := t.TempDir()
		if err := os.MkdirAll(filepath.Join(project, "bin"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(".", filepath.Join(project, "bin", "templ")); err != nil {
			t.Fatal(err)
		}
		err := withDiagnosticProjectCopyTools(project, []string{"templ"}, func(string) error {
			return nil
		})
		if err == nil || !strings.Contains(err.Error(), "cycle") {
			t.Fatalf("selected tool cycle error = %v", err)
		}
	})
}

func TestDiagnosticCopyFullModeStillCopiesEveryBinary(t *testing.T) {
	project := t.TempDir()
	writeExecutable(t, project, "bin/templ", "#!/bin/sh\nexit 0\n")
	writeExecutable(t, project, "bin/unused", "#!/bin/sh\nexit 0\n")

	err := withDiagnosticProjectCopy(project, func(copyRoot string) error {
		for _, name := range []string{"templ", "unused"} {
			if _, err := os.Stat(filepath.Join(copyRoot, "bin", name)); err != nil {
				t.Fatalf("full copy omitted %s: %v", name, err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
