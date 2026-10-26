package layout

import (
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pmezard/go-difflib/difflib"
)

// TestDefaultScaffoldGoFilesAreGofmtClean renders the default (React/pnpm)
// scaffold in-process and runs go/format over every generated Go file. The
// rendered templates are what andurel new writes before its post-scaffold
// generators, so this is the source-level guard for L7.
func TestDefaultScaffoldGoFilesAreGofmtClean(t *testing.T) {
	root := t.TempDir()
	data := &TemplateData{
		AppName:              "testapp",
		ProjectName:          "testapp",
		ModuleName:           "testapp",
		Database:             "postgresql",
		GoVersion:            goVersion,
		SessionKey:           "test-session-key",
		SessionEncryptionKey: "test-session-encryption-key",
		TokenSigningKey:      "test-token-signing-key",
		Pepper:               "test-pepper",
		RunToolVersion:       GetRunToolVersion(),
		FrameworkVersion:     "v2.0.0-alpha.5",
		Inertia:              "react",
		JavaScriptSSRRuntime: "node",
		blueprint:            initializeBlueprint("testapp"),
	}
	if err := processTemplatedFiles(root, data); err != nil {
		t.Fatalf("render default scaffold: %v", err)
	}

	var unformatted []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		formatted, err := format.Source(content)
		if err != nil {
			unformatted = append(unformatted, filepath.ToSlash(relative)+" (parse: "+err.Error()+")")
			return nil
		}
		if string(formatted) == string(content) {
			return nil
		}
		diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(string(content)),
			B:        difflib.SplitLines(string(formatted)),
			FromFile: filepath.ToSlash(relative),
			ToFile:   filepath.ToSlash(relative) + " (gofmt)",
			Context:  2,
		})
		if err != nil {
			return err
		}
		unformatted = append(unformatted, diff)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(unformatted) > 0 {
		slices.Sort(unformatted)
		t.Fatalf("generated Go files are not gofmt-clean:\n%s", strings.Join(unformatted, "\n"))
	}
}
