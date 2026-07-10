package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGolinesCheckComparesEveryRelevantGoFileAndFailsOnDifferences(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "clean.go", "package sample\n")
	writeTestFile(t, root, "nested/dirty.go", "package sample\n")
	writeTestFile(t, root, "nested/clean_test.go", "package sample\n")
	writeTestFile(t, root, "testdata/ignored.go", "package ignored\n")
	writeTestFile(t, root, "vendor/ignored.go", "package ignored\n")

	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "golines.log")
	writeExecutable(t, binDir, "golines", `#!/bin/sh
printf '%s\n' "$3" >> "$GOLINES_LOG"
if [ "$3" = nested/dirty.go ]; then
  printf 'package sample\n\n'
else
  /bin/cat "$3"
fi
`)
	t.Setenv("PATH", binDir)
	t.Setenv("GOLINES_LOG", logPath)

	err := runGolines(root, true)
	if err == nil || !strings.Contains(err.Error(), "nested/dirty.go") {
		t.Fatalf("runGolines dirty check = %v", err)
	}
	log, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("read golines log: %v", readErr)
	}
	for _, path := range []string{"clean.go", "nested/dirty.go", "nested/clean_test.go"} {
		if !strings.Contains(string(log), path+"\n") {
			t.Fatalf("golines did not inspect %s:\n%s", path, log)
		}
	}
	for _, path := range []string{"testdata/ignored.go", "vendor/ignored.go"} {
		if strings.Contains(string(log), path) {
			t.Fatalf("golines inspected excluded file %s:\n%s", path, log)
		}
	}
}

func TestGolinesCheckPassesWhenEveryFileMatches(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "clean.go", "package sample\n")
	binDir := t.TempDir()
	writeExecutable(t, binDir, "golines", "#!/bin/sh\n/bin/cat \"$3\"\n")
	t.Setenv("PATH", binDir)
	if err := runGolines(root, true); err != nil {
		t.Fatalf("clean golines check: %v", err)
	}
}

func TestGoFmtCheckReportsDirtyFilesWithoutChangingThem(t *testing.T) {
	root := t.TempDir()
	dirty := "package sample\nfunc value( )int{return 1}\n"
	writeTestFile(t, root, "dirty.go", dirty)
	if err := runGoFmt(root, true); err == nil {
		t.Fatal("expected gofmt check to report dirty file")
	}
	content, err := os.ReadFile(filepath.Join(root, "dirty.go"))
	if err != nil {
		t.Fatalf("read dirty file: %v", err)
	}
	if string(content) != dirty {
		t.Fatal("gofmt check changed the original file")
	}

	writeTestFile(t, root, "dirty.go", "package sample\n\nfunc value() int { return 1 }\n")
	if err := runGoFmt(root, true); err != nil {
		t.Fatalf("clean gofmt check: %v", err)
	}
}

func TestTemplFormatCheckUsesTemporaryCopyAndDetectsChanges(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "views/page.templ", "package views\n")
	writeExecutable(t, root, "bin/templ", "#!/bin/sh\nprintf '// formatted\\n' >> \"$2/page.templ\"\n")
	original, err := os.ReadFile(filepath.Join(root, "views", "page.templ"))
	if err != nil {
		t.Fatalf("read original template: %v", err)
	}
	if err := runTemplFmt(root, true); err == nil || !strings.Contains(err.Error(), "views/page.templ") {
		t.Fatalf("templ check = %v", err)
	}
	after, err := os.ReadFile(filepath.Join(root, "views", "page.templ"))
	if err != nil {
		t.Fatalf("read template after check: %v", err)
	}
	if string(after) != string(original) {
		t.Fatal("templ check changed original template")
	}
}

func TestTemplFormatCheckPassesWhenTemplatesAreUnchanged(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "views/page.templ", "package views\n")
	writeExecutable(t, root, "bin/templ", "#!/bin/sh\nexit 0\n")
	if err := runTemplFmt(root, true); err != nil {
		t.Fatalf("clean templ check: %v", err)
	}
}
