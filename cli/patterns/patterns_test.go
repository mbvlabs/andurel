package patterns_test

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/patterns"
)

func TestAssetsAreComplete(t *testing.T) {
	if err := patterns.Err(); err != nil {
		t.Fatalf("load embedded patterns: %v", err)
	}

	all := patterns.All()
	if len(all) == 0 {
		t.Fatal("no patterns embedded")
	}

	for _, pattern := range all {
		t.Run(pattern.ID, func(t *testing.T) {
			if strings.TrimSpace(pattern.Title) == "" {
				t.Error("missing title")
			}
			if len(pattern.Files) == 0 {
				t.Error("no files")
			}
			if len(pattern.Rules) == 0 {
				t.Error("no rules")
			}
			if len(pattern.Checklist) == 0 {
				t.Error("no checklist")
			}
			for _, file := range pattern.Files {
				if !isPathHeading(file.Path) {
					t.Errorf("Files heading %q is not a path", file.Path)
				}
			}
		})
	}
}

// TestWrittenPathsAreUnique fails when two patterns ship a concrete file at the
// same path: the full-scaffold compile test writes both, so only the last one
// would be compiled.
func TestWrittenPathsAreUnique(t *testing.T) {
	if err := patterns.Err(); err != nil {
		t.Fatalf("load embedded patterns: %v", err)
	}

	owners := make(map[string]string)
	for _, pattern := range patterns.All() {
		for _, file := range pattern.Files {
			if !file.Concrete() {
				continue
			}
			if owner, ok := owners[file.Path]; ok {
				t.Errorf("%s is shipped by both %q and %q", file.Path, owner, pattern.ID)
				continue
			}
			owners[file.Path] = pattern.ID
		}
	}
}

// TestGoBlocksParse requires every Go block to parse. A block parses as a whole
// file, or as declarations after a package clause, or as statements inside a
// function. Compile-tested blocks (see File.CompileTested) must parse as whole files
// because the full-scaffold test writes them verbatim.
func TestGoBlocksParse(t *testing.T) {
	if err := patterns.Err(); err != nil {
		t.Fatalf("load embedded patterns: %v", err)
	}

	for _, pattern := range patterns.All() {
		for _, file := range pattern.Files {
			if file.Language != "go" {
				continue
			}
			t.Run(pattern.ID+"/"+file.Path, func(t *testing.T) {
				fullErr := parsesAsFile(file.Content)
				if file.CompileTested() {
					if fullErr != nil {
						t.Fatalf(
							"compile-tested block does not parse as a file: %v",
							fullErr,
						)
					}
					return
				}
				if fullErr == nil || parsesAsDeclarations(file.Content) || parsesAsStatements(file.Content) {
					return
				}
				t.Fatalf("go block does not parse as a file or fragment: %v", fullErr)
			})
		}
	}
}

func TestFragmentOnlyAppearsWhenMarked(t *testing.T) {
	plain, err := json.Marshal(patterns.File{
		Path:     "domains/product.go",
		Language: "go",
		Content:  "package domains\n",
	})
	if err != nil {
		t.Fatalf("marshal plain file: %v", err)
	}
	if strings.Contains(string(plain), "fragment") {
		t.Fatalf("unmarked file gained a fragment field: %s", plain)
	}

	marked, err := json.Marshal(patterns.File{
		Path:     "internal/runtime/runtime.go",
		Language: "go",
		Content:  "package runtime\n",
		Fragment: true,
	})
	if err != nil {
		t.Fatalf("marshal fragment file: %v", err)
	}
	if !strings.Contains(string(marked), `"fragment":true`) {
		t.Fatalf("marked file did not emit a fragment field: %s", marked)
	}
}

// isPathHeading accepts a project-relative file path with an extension,
// optionally followed by one parenthesized annotation such as "(edit)".
func isPathHeading(heading string) bool {
	path, annotation, _ := strings.Cut(heading, " ")
	if !strings.Contains(path, "/") || filepath.Ext(path) == "" {
		return false
	}
	return annotation == "" ||
		(strings.HasPrefix(annotation, "(") && strings.HasSuffix(annotation, ")"))
}

func parsesAsFile(source string) error {
	_, err := parser.ParseFile(token.NewFileSet(), "block.go", source, parser.AllErrors)
	return err
}

func parsesAsDeclarations(source string) bool {
	return parsesAsFile("package x\n\n"+source) == nil
}

func parsesAsStatements(source string) bool {
	return parsesAsFile("package x\n\nfunc _() {\n"+source+"\n}\n") == nil
}
