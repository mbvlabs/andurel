package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mbvlabs/andurel/layout"
)

func TestDoctorDiagnosticsUseTemporaryCopiesWithoutMutatingOriginalProject(t *testing.T) {
	stubLatestAndurelVersion(t, "v1.0.0", nil)
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/doctor\n\ngo 1.26.0\n")
	writeTestFile(t, root, "go.sum", "original sum\n")
	writeTestFile(t, root, "main.go", "package main\n\nfunc main() {}\n")
	writeTestFile(t, root, "views/page.templ", "package views\n")
	writeTestFile(t, root, "views/page_templ.go", "package views\n")
	writeTestFile(t, root, "router/routes/pages.go", `package routes

import "github.com/mbvlabs/andurel/router"

var Home = router.NewRoute("/", "home")
`)
	writeTestFile(t, root, generatedRoutesJSPath, "stale routes\n")
	writeTestFile(t, root, ".git/status-sentinel", "unchanged\n")

	lock := layout.NewAndurelLock("v1.0.0")
	lock.ScaffoldConfig = &layout.ScaffoldConfig{ProjectName: "doctor", Database: "postgresql", Inertia: "react"}
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	writeExecutable(t, root, "bin/templ", "#!/bin/sh\nprintf 'package views\\n// generated in temporary copy\\n' > views/page_templ.go\n")

	fakePath := t.TempDir()
	writeExecutable(t, fakePath, "go", "#!/bin/sh\nif [ \"$1\" = mod ]; then printf '\\nchanged in copy\\n' >> go.mod; printf '\\nchanged in copy\\n' >> go.sum; fi\n")
	t.Setenv("PATH", fakePath)

	tempBase := t.TempDir()
	makeDiagnosticTempDir = func(_ string, pattern string) (string, error) {
		return os.MkdirTemp(tempBase, pattern)
	}
	removeDiagnosticTempDir = os.RemoveAll
	t.Cleanup(func() {
		makeDiagnosticTempDir = os.MkdirTemp
		removeDiagnosticTempDir = os.RemoveAll
	})

	before := snapshotAllTestFiles(t, root)
	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() { findGoModRoot = originalFindGoModRoot })
	if _, err := collectDoctorReport("v1.0.0", true); err != nil {
		t.Fatalf("collect doctor report: %v", err)
	}
	if afterReport := snapshotAllTestFiles(t, root); !reflect.DeepEqual(afterReport, before) {
		t.Fatalf("doctor changed original project\nbefore: %#v\nafter: %#v", before, afterReport)
	}
	if result := checkGoModTidy(root, true); result.status != statusWarn {
		t.Fatalf("tidy check = %#v", result)
	}
	if result := checkTemplGenerate(root, true); result.status != statusFail {
		t.Fatalf("templ check = %#v", result)
	}
	if result := checkRoutesTSGenerate(root, true); result.status != statusFail {
		t.Fatalf("routes check = %#v", result)
	}
	after := snapshotAllTestFiles(t, root)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("doctor diagnostics changed original project\nbefore: %#v\nafter: %#v", before, after)
	}
	assertDirectoryEmpty(t, tempBase)
}

func TestDiagnosticTemporaryCopiesAreCleanedAfterSuccessAndFailure(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/cleanup\n")
	tempBase := t.TempDir()
	makeDiagnosticTempDir = func(_ string, pattern string) (string, error) {
		return os.MkdirTemp(tempBase, pattern)
	}
	removeDiagnosticTempDir = os.RemoveAll
	t.Cleanup(func() {
		makeDiagnosticTempDir = os.MkdirTemp
		removeDiagnosticTempDir = os.RemoveAll
	})

	if err := withDiagnosticProjectCopy(root, func(string) error { return nil }); err != nil {
		t.Fatalf("successful diagnostic copy: %v", err)
	}
	assertDirectoryEmpty(t, tempBase)

	wantErr := errors.New("diagnostic failed")
	err := withDiagnosticProjectCopy(root, func(string) error { return wantErr })
	if !errors.Is(err, wantErr) {
		t.Fatalf("failure = %v, want wrapped diagnostic failure", err)
	}
	assertDirectoryEmpty(t, tempBase)
}

func snapshotAllTestFiles(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = bytes.Clone(content)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot project: %v", err)
	}
	return files
}

func assertDirectoryEmpty(t *testing.T, path string) {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatalf("read temporary base: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary data was not cleaned: %#v", entries)
	}
}
